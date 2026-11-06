// Copyright (c) 2026 Jonas Kaninda.
//
// This file is part of Posta.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.
//
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package updatecheck checks GitHub daily for a newer release; it notifies and never upgrades.
package updatecheck

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/jkaninda/okapi/client"
	"golang.org/x/mod/semver"
	"gorm.io/gorm"
)

const (
	// Not /releases/latest: it excludes prereleases, so an all-prerelease project would get a 404.
	githubAPI    = "https://api.github.com"
	releasesPath = "/repos/goposta/posta/releases"

	// httpTimeout bounds the whole exchange. A hung GitHub must never wedge cron.
	httpTimeout = 15 * time.Second
)

type Release struct {
	TagName     string    `json:"tag_name"`
	HTMLURL     string    `json:"html_url"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
}

type Service struct {
	db      *gorm.DB
	client  *client.Client
	version string // the running build, e.g. "v0.12.0" or "dev"
	enabled bool
}

func NewService(db *gorm.DB, version string, enabled bool) *Service {
	s := &Service{db: db, version: version, enabled: enabled}
	s.setBaseURL(githubAPI)
	return s
}

// setBaseURL is for tests only; the User-Agent is the ONLY thing the request says about the install.
func (s *Service) setBaseURL(base string) {
	s.client = client.New(base,
		client.WithTimeout(httpTimeout),
		client.WithUserAgent("posta/"+s.version),
		client.WithHeader("Accept", "application/vnd.github+json"),
	)
}

// Enabled is false for a dev build: its version doesn't compare meaningfully against any release.
func (s *Service) Enabled() bool { return s.enabled && normalize(s.version) != "" }

// normalize accepts "v0.12.0" or "0.12.0" and returns "" for non-versions like "dev" or a commit sha.
func normalize(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	if !semver.IsValid(v) {
		return ""
	}
	return v
}

// wantPrerelease: an rc user hears of later rcs and stable; a stable user is never nudged onto an rc.
func wantPrerelease(current string) bool { return semver.Prerelease(current) != "" }

// Newest orders by semver, never lexically: a string compare would offer an rc.10 user rc.9.
func Newest(current string, releases []Release) (Release, bool) {
	cur := normalize(current)
	if cur == "" {
		return Release{}, false
	}
	allowPre := wantPrerelease(cur)

	var best Release
	var bestTag string
	for _, r := range releases {
		if r.Draft {
			continue
		}
		if r.Prerelease && !allowPre {
			continue
		}
		tag := normalize(r.TagName)
		if tag == "" || semver.Compare(tag, cur) <= 0 {
			continue
		}
		if bestTag == "" || semver.Compare(tag, bestTag) > 0 {
			best, bestTag = r, tag
		}
	}
	return best, bestTag != ""
}

// IsNewer gates the notice at read time, since the daily cached row can describe the build before an upgrade.
func IsNewer(current, latest string) bool {
	cur, lat := normalize(current), normalize(latest)
	if cur == "" || lat == "" {
		return false
	}
	return semver.Compare(lat, cur) > 0
}

// Status returns the cached row, creating the singleton on first read.
func (s *Service) Status() (*models.UpdateStatus, error) {
	var st models.UpdateStatus
	err := s.db.Where("id = ?", 1).First(&st).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		st = models.UpdateStatus{ID: 1}
		if err := s.db.Create(&st).Error; err != nil {
			return nil, err
		}
		return &st, nil
	}
	if err != nil {
		return nil, err
	}
	return &st, nil
}

// Dismiss records the version, not a boolean, so the next release notifies again.
func (s *Service) Dismiss(version string) error {
	st, err := s.Status()
	if err != nil {
		return err
	}
	st.DismissedVersion = version
	return s.db.Save(st).Error
}

// Check stores errors on the row rather than returning them: an air-gapped install fails daily.
func (s *Service) Check(ctx context.Context) error {
	if !s.Enabled() {
		return nil
	}
	st, err := s.Status()
	if err != nil {
		return err
	}
	now := time.Now()
	st.CheckedAt = &now

	// Drop the ETag if the build changed: a 304 would otherwise keep a verdict computed for the old build.
	etagToSend := st.ETag
	if st.CheckedVersion != s.version {
		etagToSend = ""
	}

	releases, notModified, etag, err := s.fetch(ctx, etagToSend)
	if err != nil {
		st.LastError = err.Error()
		return s.db.Save(st).Error
	}
	st.LastError = ""
	if etag != "" {
		st.ETag = etag
	}
	if notModified {
		// Unchanged list and unchanged build (else no ETag was sent), so the cached verdict holds.
		return s.db.Save(st).Error
	}

	st.CheckedVersion = s.version
	if r, ok := Newest(s.version, releases); ok {
		st.LatestVersion = normalize(r.TagName)
		st.ReleaseURL = r.HTMLURL
		published := r.PublishedAt
		st.PublishedAt = &published
	} else {
		// Up to date. Clear any stale pointer from a previous release.
		st.LatestVersion, st.ReleaseURL, st.PublishedAt = "", "", nil
	}
	return s.db.Save(st).Error
}

// fetch replays the stored ETag; a 304 costs no rate-limit quota (unauthenticated budget: 60 req/hour/IP).
func (s *Service) fetch(ctx context.Context, etag string) (rel []Release, notModified bool, newETag string, err error) {
	req := s.client.Get(releasesPath).
		WithContext(ctx).
		QueryParam("per_page", "20")
	if etag != "" {
		req = req.Header("If-None-Match", etag)
	}

	resp, err := req.Do()
	if err != nil {
		return nil, false, "", err
	}

	switch resp.StatusCode {
	case http.StatusNotModified:
		return nil, true, resp.Header.Get("ETag"), nil
	case http.StatusOK:
	case http.StatusForbidden, http.StatusTooManyRequests:
		return nil, false, "", fmt.Errorf("github rate limit reached; will retry on the next scheduled check")
	default:
		return nil, false, "", fmt.Errorf("github returned %s", resp.Status)
	}

	if err := resp.JSON(&rel); err != nil {
		return nil, false, "", fmt.Errorf("decode releases: %w", err)
	}
	return rel, false, resp.Header.Get("ETag"), nil
}
