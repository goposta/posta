// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package handlers

import (
	"time"

	"github.com/goposta/posta/internal/config"
	"github.com/goposta/posta/internal/dto"
	"github.com/goposta/posta/internal/services/updatecheck"
	"github.com/jkaninda/okapi"
)

// UpdateHandler serves the cached result of the daily release check.
type UpdateHandler struct {
	svc *updatecheck.Service
}

func NewUpdateHandler(svc *updatecheck.Service) *UpdateHandler { return &UpdateHandler{svc: svc} }

// UpdateInfo is what the dashboard renders.
type UpdateInfo struct {
	CurrentVersion  string     `json:"current_version" example:"v0.12.0"`
	LatestVersion   string     `json:"latest_version,omitempty" example:"v0.13.0"`
	ReleaseURL      string     `json:"release_url,omitempty"`
	PublishedAt     *time.Time `json:"published_at,omitempty"`
	UpdateAvailable bool       `json:"update_available"`
	Enabled         bool       `json:"enabled"`
	CheckedAt       *time.Time `json:"checked_at,omitempty"`
	LastError       string     `json:"last_error,omitempty"`
}

// DismissUpdateRequest silences the notice for one version.
type DismissUpdateRequest struct {
	Body struct {
		Version string `json:"version" required:"true"`
	} `json:"body"`
}

// GetUpdate returns the cached release-check result for the running build.
func (h *UpdateHandler) GetUpdate(c *okapi.Context) error {
	info := UpdateInfo{CurrentVersion: config.Version, Enabled: h.svc.Enabled()}
	st, err := h.svc.Status()
	if err != nil {
		return c.AbortInternalServerError("failed to read update status")
	}
	info.CheckedAt, info.LastError = st.CheckedAt, st.LastError

	if st.LatestVersion != "" && updatecheck.IsNewer(config.Version, st.LatestVersion) {
		info.LatestVersion = st.LatestVersion
		info.ReleaseURL = st.ReleaseURL
		info.PublishedAt = st.PublishedAt
		info.UpdateAvailable = h.svc.Enabled() && st.DismissedVersion != st.LatestVersion
	}
	return ok(c, info)
}

// DismissUpdate hides the notice until a newer version appears.
func (h *UpdateHandler) DismissUpdate(c *okapi.Context, req *DismissUpdateRequest) error {
	if err := h.svc.Dismiss(req.Body.Version); err != nil {
		return c.AbortInternalServerError("failed to dismiss update notice")
	}
	return ok(c, dto.MessageData{Message: "update notice dismissed"})
}
