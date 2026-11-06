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

package handlers

import (
	"fmt"

	"github.com/goposta/posta/internal/services/clientinfo"
	"github.com/goposta/posta/internal/services/session"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
)

type SessionHandler struct {
	repo  *repositories.SessionRepository
	store *session.Store
}

func NewSessionHandler(repo *repositories.SessionRepository, store *session.Store) *SessionHandler {
	return &SessionHandler{repo: repo, store: store}
}

type SessionResponse struct {
	ID        uint   `json:"id"`
	IPAddress string `json:"ip_address"`
	UserAgent string `json:"user_agent"`
	Browser   string `json:"browser"`
	OS        string `json:"os"`
	Device    string `json:"device"`
	Label     string `json:"label"`
	Current   bool   `json:"current"`
	CreatedAt string `json:"created_at"`
	ExpiresAt string `json:"expires_at"`
}

type RevokeSessionRequest struct {
	ID int `param:"id"`
}

func (h *SessionHandler) List(c *okapi.Context) error {
	userID := uint(c.GetInt("user_id"))
	currentJTI := c.GetString("jti")

	sessions, err := h.repo.FindActiveByUserID(userID)
	if err != nil {
		return c.AbortInternalServerError("failed to load sessions")
	}

	result := make([]SessionResponse, 0, len(sessions))
	for _, s := range sessions {
		client := clientinfo.Parse(s.UserAgent)
		result = append(result, SessionResponse{
			ID:        s.ID,
			IPAddress: s.IPAddress,
			UserAgent: s.UserAgent,
			Browser:   client.Browser,
			OS:        client.OS,
			Device:    client.Device,
			Label:     client.Label(),
			Current:   s.JTI == currentJTI,
			CreatedAt: s.CreatedAt.Format("2006-01-02T15:04:05Z"),
			ExpiresAt: s.ExpiresAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	return ok(c, result)
}

func (h *SessionHandler) Revoke(c *okapi.Context, req *RevokeSessionRequest) error {
	userID := uint(c.GetInt("user_id"))

	sess, err := h.repo.FindByID(uint(req.ID))
	if err != nil || sess.UserID != userID {
		return c.AbortNotFound("session not found")
	}

	if sess.Revoked {
		return c.AbortBadRequest("session already revoked")
	}

	if err := h.repo.Revoke(sess.ID); err != nil {
		return c.AbortInternalServerError("failed to revoke session")
	}

	// Add to Redis blacklist for immediate effect
	h.store.MarkRevoked(c.Request().Context(), sess.JTI, sess.ExpiresAt)

	return ok(c, okapi.M{"message": fmt.Sprintf("Session %d revoked", sess.ID)})
}

func (h *SessionHandler) RevokeOthers(c *okapi.Context) error {
	userID := uint(c.GetInt("user_id"))
	currentJTI := c.GetString("jti")

	// Get all active sessions first (need JTIs for Redis blacklist)
	sessions, err := h.repo.FindActiveByUserID(userID)
	if err != nil {
		return c.AbortInternalServerError("failed to load sessions")
	}

	count, err := h.repo.RevokeOthersByUserID(userID, currentJTI)
	if err != nil {
		return c.AbortInternalServerError("failed to revoke sessions")
	}

	for _, s := range sessions {
		if s.JTI != currentJTI {
			h.store.MarkRevoked(c.Request().Context(), s.JTI, s.ExpiresAt)
		}
	}

	return ok(c, okapi.M{
		"message": fmt.Sprintf("%d other session(s) revoked", count),
		"revoked": count,
	})
}

// Logout clears the cookie unconditionally so a browser holding an unresolvable token still logs out.
func (h *SessionHandler) Logout(c *okapi.Context) error {
	session.ClearCookie(c)

	currentJTI := c.GetString("jti")
	if currentJTI == "" {
		return ok(c, okapi.M{"message": "logged out"})
	}

	sess, err := h.repo.FindByJTI(currentJTI)
	if err == nil && sess != nil && !sess.Revoked {
		_ = h.repo.Revoke(sess.ID)
		h.store.MarkRevoked(c.Request().Context(), sess.JTI, sess.ExpiresAt)
	}

	return ok(c, okapi.M{"message": "logged out"})
}
