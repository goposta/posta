/*
 *  MIT License
 *
 * Copyright (c) 2026 Jonas Kaninda
 *
 *  Permission is hereby granted, free of charge, to any person obtaining a copy
 *  of this software and associated documentation files (the "Software"), to deal
 *  in the Software without restriction, including without limitation the rights
 *  to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 *  copies of the Software, and to permit persons to whom the Software is
 *  furnished to do so, subject to the following conditions:
 *
 *  The above copyright notice and this permission notice shall be included in all
 *  copies or substantial portions of the Software.
 *
 *  THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 *  IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 *  FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 *  AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 *  LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 *  OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 *  SOFTWARE.
 */

package handlers

import (
	"fmt"

	"github.com/jkaninda/okapi"
	"github.com/jkaninda/posta/internal/services/session"
	"github.com/jkaninda/posta/internal/storage/repositories"
)

type SessionHandler struct {
	repo  *repositories.SessionRepository
	store *session.Store
}

func NewSessionHandler(repo *repositories.SessionRepository, store *session.Store) *SessionHandler {
	return &SessionHandler{repo: repo, store: store}
}

// SessionResponse is the API representation of a session.
type SessionResponse struct {
	ID        uint   `json:"id"`
	IPAddress string `json:"ip_address"`
	UserAgent string `json:"user_agent"`
	Current   bool   `json:"current"`
	CreatedAt string `json:"created_at"`
	ExpiresAt string `json:"expires_at"`
}

type RevokeSessionRequest struct {
	ID int `param:"id"`
}

// List returns all active sessions for the current user.
func (h *SessionHandler) List(c *okapi.Context) error {
	userID := uint(c.GetInt("user_id"))
	currentJTI := c.GetString("jti")

	sessions, err := h.repo.FindActiveByUserID(userID)
	if err != nil {
		return c.AbortInternalServerError("failed to load sessions")
	}

	result := make([]SessionResponse, 0, len(sessions))
	for _, s := range sessions {
		result = append(result, SessionResponse{
			ID:        s.ID,
			IPAddress: s.IPAddress,
			UserAgent: s.UserAgent,
			Current:   s.JTI == currentJTI,
			CreatedAt: s.CreatedAt.Format("2006-01-02T15:04:05Z"),
			ExpiresAt: s.ExpiresAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	return ok(c, result)
}

// Revoke terminates a specific session by ID.
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

// RevokeOthers revokes all sessions except the current one.
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

	// Blacklist each revoked session in Redis
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

// Logout revokes the current session.
func (h *SessionHandler) Logout(c *okapi.Context) error {
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
