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
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/chat"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
)

// ChatSnippetHandler manages canned replies. Everyone in the workspace reads
// them; editors and above change them.
type ChatSnippetHandler struct {
	repo *repositories.ChatRepository
}

func NewChatSnippetHandler(repo *repositories.ChatRepository) *ChatSnippetHandler {
	return &ChatSnippetHandler{repo: repo}
}

type chatSnippetBody struct {
	Shortcut string `json:"shortcut" required:"true" doc:"Typed after / in the composer: lowercase letters, digits and hyphens"`
	Title    string `json:"title" required:"true"`
	Body     string `json:"body" required:"true"`
}

type CreateChatSnippetRequest struct {
	Body chatSnippetBody `json:"body"`
}

type UpdateChatSnippetRequest struct {
	ID   int             `param:"id"`
	Body chatSnippetBody `json:"body"`
}

type ChatSnippetIDRequest struct {
	ID int `param:"id"`
}

func (h *ChatSnippetHandler) List(c *okapi.Context) error {
	ws, err := workspaceOf(c)
	if err != nil {
		return c.AbortBadRequest(err.Error())
	}
	items, err := h.repo.ListSnippets(ws)
	if err != nil {
		return c.AbortInternalServerError("failed to list canned replies")
	}
	if items == nil {
		items = []models.ChatSnippet{}
	}
	return ok(c, items)
}

func (h *ChatSnippetHandler) Create(c *okapi.Context, req *CreateChatSnippetRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	ws, err := workspaceOf(c)
	if err != nil {
		return c.AbortBadRequest(err.Error())
	}
	userID := uint(c.GetInt("user_id"))
	s := &models.ChatSnippet{WorkspaceID: &ws, CreatedByID: &userID}
	if err := h.apply(s, ws, 0, req.Body); err != nil {
		return err.(abortErr).send(c)
	}
	if err := h.repo.SaveSnippet(s); err != nil {
		return c.AbortConflict("failed to save canned reply")
	}
	return created(c, s)
}

func (h *ChatSnippetHandler) Update(c *okapi.Context, req *UpdateChatSnippetRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	ws, err := workspaceOf(c)
	if err != nil {
		return c.AbortBadRequest(err.Error())
	}
	s, err := h.repo.FindSnippet(ws, uint(req.ID))
	if err != nil {
		return c.AbortNotFound("canned reply not found")
	}
	if err := h.apply(s, ws, s.ID, req.Body); err != nil {
		return err.(abortErr).send(c)
	}
	now := time.Now()
	s.UpdatedAt = &now
	if err := h.repo.SaveSnippet(s); err != nil {
		return c.AbortConflict("failed to save canned reply")
	}
	return ok(c, s)
}

func (h *ChatSnippetHandler) Delete(c *okapi.Context, req *ChatSnippetIDRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	ws, err := workspaceOf(c)
	if err != nil {
		return c.AbortBadRequest(err.Error())
	}
	s, err := h.repo.FindSnippet(ws, uint(req.ID))
	if err != nil {
		return c.AbortNotFound("canned reply not found")
	}
	if err := h.repo.DeleteSnippet(s.ID); err != nil {
		return c.AbortInternalServerError("failed to delete canned reply")
	}
	return noContent(c)
}

// abortErr carries the status a validation failure answers with.
type abortErr struct {
	conflict bool
	msg      string
}

func (e abortErr) Error() string { return e.msg }

func (e abortErr) send(c *okapi.Context) error {
	if e.conflict {
		return c.AbortConflict(e.msg)
	}
	return c.AbortBadRequest(e.msg)
}

func (h *ChatSnippetHandler) apply(s *models.ChatSnippet, ws, id uint, b chatSnippetBody) error {
	shortcut := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(b.Shortcut, "/")))
	if err := models.CheckIdentifier(shortcut); err != nil {
		return abortErr{msg: "shortcut: " + err.Error()}
	}
	if h.repo.SnippetShortcutExists(ws, shortcut, id) {
		return abortErr{conflict: true, msg: "a canned reply with shortcut /" + shortcut + " already exists"}
	}
	title := chat.CleanLine(b.Title, 80)
	body := chat.CleanBody(b.Body)
	if title == "" || body == "" || len(body) > 4096 {
		return abortErr{msg: "title and body are required; body at most 4096 bytes"}
	}
	s.Shortcut, s.Title, s.Body = shortcut, title, body
	return nil
}
