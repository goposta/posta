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
	"strings"

	"github.com/goposta/posta/internal/dto"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/audit"
	"github.com/goposta/posta/internal/services/inbox"
	"github.com/jkaninda/okapi"
	"gorm.io/gorm"
)

// AdminAnnouncementHandler broadcasts platform notices into every user's in-app inbox.
type AdminAnnouncementHandler struct {
	db    *gorm.DB
	inbox *inbox.Service
	audit *audit.Logger
}

func NewAdminAnnouncementHandler(db *gorm.DB, svc *inbox.Service, auditLogger *audit.Logger) *AdminAnnouncementHandler {
	return &AdminAnnouncementHandler{db: db, inbox: svc, audit: auditLogger}
}

type CreateAnnouncementRequest struct {
	Body struct {
		Title    string `json:"title" required:"true" max:"200"`
		Message  string `json:"message" max:"2000"`
		Link     string `json:"link" max:"500"`
		Severity string `json:"severity" enum:"info,warning,critical"`
	} `json:"body"`
}

type AnnouncementIDRequest struct {
	ID int `param:"id"`
}

type ListAnnouncementsRequest struct {
	Page int `query:"page" default:"0"`
	Size int `query:"size" default:"20"`
}

func (h *AdminAnnouncementHandler) List(c *okapi.Context, req *ListAnnouncementsRequest) error {
	page, size, offset := normalizePageParams(req.Page, req.Size)

	var total int64
	if err := h.db.Model(&models.Announcement{}).Count(&total).Error; err != nil {
		return c.AbortInternalServerError("failed to count announcements")
	}
	var rows []models.Announcement
	if err := h.db.Order("id DESC").Limit(size).Offset(offset).Find(&rows).Error; err != nil {
		return c.AbortInternalServerError("failed to list announcements")
	}
	return paginated(c, rows, total, page, size)
}

// Create broadcasts at once: there is no draft state, and Retract covers a send that went out wrong.
func (h *AdminAnnouncementHandler) Create(c *okapi.Context, req *CreateAnnouncementRequest) error {
	title := strings.TrimSpace(req.Body.Title)
	if title == "" {
		return c.AbortBadRequest("a title is required")
	}

	severity := models.NotificationSeverity(req.Body.Severity)
	if severity == "" {
		severity = models.NotificationInfo
	}

	a := &models.Announcement{
		Title:      title,
		Message:    strings.TrimSpace(req.Body.Message),
		Link:       strings.TrimSpace(req.Body.Link),
		Severity:   severity,
		CreatedBy:  uint(c.GetInt("user_id")),
		AuthorName: c.GetString("user_name"),
	}
	if err := h.inbox.Announce(a); err != nil {
		return c.AbortInternalServerError("failed to broadcast the announcement")
	}

	h.audit.LogCtx(c, "admin.announcement.sent", "Broadcast \""+a.Title+"\"", map[string]any{
		"announcement_id": a.ID,
		"recipients":      a.Recipients,
		"severity":        string(a.Severity),
	})
	return created(c, a)
}

// Retract deletes the announcement and every delivery it made, instead of appending a correction.
func (h *AdminAnnouncementHandler) Retract(c *okapi.Context, req *AnnouncementIDRequest) error {
	var a models.Announcement
	if err := h.db.First(&a, req.ID).Error; err != nil {
		return c.AbortNotFound("announcement not found")
	}
	if err := h.inbox.Retract(a.ID); err != nil {
		return c.AbortInternalServerError("failed to retract the announcement")
	}

	h.audit.LogCtx(c, "admin.announcement.retracted", "Retracted \""+a.Title+"\"", map[string]any{
		"announcement_id": a.ID,
	})
	return ok(c, dto.MessageData{Message: "announcement retracted"})
}
