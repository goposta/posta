/*
 * Copyright 2026 Jonas Kaninda
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

// Package updatecheck asks GitHub, once a day, whether a newer Posta release
// exists and caches the answer for the dashboard to read.
//
// It notifies; it never upgrades. Replacing a running mail server's binary is a
// decision for a human with a shell, not a cron tick.
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
	// githubAPI is the base URL; releasesPath lists releases newest-first.
	// Deliberately NOT /releases/latest: that endpoint excludes prereleases, and a
	// project whose releases are all prereleases gets a 404 from it — the check
	// would silently never fire.
	githubAPI    = "https://api.github.com"
	releasesPath = "/repos/goposta/posta/releases"

	// httpTimeout bounds the whole exchange. A hung GitHub must never wedge cron.
	httpTimeout = 15 * time.Second
)

// Release is the subset of GitHub's release object we rely on.
type Release struct {
	TagName     string    `json:"tag_name"`
	HTMLURL     string    `json:"html_url"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
}

// Service performs the check and persists its result.
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

// setBaseURL rebuilds the HTTP client against another origin. Only tests call it
// (pointing at an httptest server); production always talks to api.github.com.
//
// The User-Agent identifies the client honestly and is the ONLY thing this
// request says about the install: no id, no host, no domain, no volume.
func (s *Service) setBaseURL(base string) {
	s.client = client.New(base,
		client.WithTimeout(httpTimeout),
		client.WithUserAgent("posta/"+s.version),
		client.WithHeader("Accept", "application/vnd.github+json"),
	)
}

// Enabled reports whether checks run at all. A `dev` build never checks: its
// version does not compare meaningfully against any release.
func (s *Service) Enabled() bool { return s.enabled && normalize(s.version) != "" }

// normalize turns a baked version into a semver string x/mod accepts. Posta
// stamps the tag as-is (e.g. "v0.12.0"), but accept a bare "0.12.0" too so a
// differently-built image still compares. Returns "" when the value is not a
// version at all — "dev", "unknown", or a commit sha.
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

// wantPrerelease reports whether the running build is itself a prerelease. A
// user on v0.12.0-rc.1 wants to hear about rc.2 AND about stable v0.12.0; a user
// on stable must never be nudged onto a release candidate.
func wantPrerelease(current string) bool { return semver.Prerelease(current) != "" }

// Newest picks the newest release the running build should be offered, or false
// when none is newer. Exported for tests.
//
// Ordering is semver, never lexical: "v0.12.0-rc.10" sorts *after* "v0.12.0-rc.9"
// here, and "v0.12.0" after both — a string compare gets both backwards, which
// would tell an rc.10 user to "upgrade" to rc.9.
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

// IsNewer reports whether latest is a strictly newer release than current.
//
// Readers use this to gate the notice rather than trusting the cached row on its
// own. The row is written by a daily cron, so between an upgrade and the next
// tick it still describes the *previous* build — without this guard an install
// that just moved to v0.13.0 would keep advertising the v0.12.1 it is already
// past. Comparing at read time makes offering a downgrade structurally
// impossible, whatever is cached.
//
// A non-version build ("dev", a sha) compares against nothing: false.
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

// Dismiss silences the notice for a specific version. Recording the version —
// rather than a boolean — means the next release notifies again.
func (s *Service) Dismiss(version string) error {
	st, err := s.Status()
	if err != nil {
		return err
	}
	st.DismissedVersion = version
	return s.db.Save(st).Error
}

// Check fetches the release list and updates the cached row. Errors are stored
// on the row rather than returned to a user: an air-gapped install fails this
// every day, and that must be visible to an admin without being noisy.
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

	// The cached verdict is a function of two inputs: GitHub's release list AND the
	// version we compare it against. The ETag only covers the first. If the build
	// moved since the verdict was computed, replaying the ETag would earn a 304 on
	// an unchanged list and preserve a verdict that no longer applies — an upgraded
	// install would keep being offered the release it is already past. Drop the
	// ETag so this check is forced to re-evaluate the list against the new build.
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
		// The list is unchanged AND the build is unchanged (or we would not have sent
		// the ETag), so the cached verdict still holds.
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

// fetch GETs the release list, replaying the stored ETag. A 304 means the list
// is unchanged and, importantly, costs no rate-limit quota — the unauthenticated
// budget is 60 requests/hour per IP.
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
