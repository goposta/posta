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
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
)

// UnifiedInboxHandler lists form messages, loose inbound emails and chats in
// one list. Each row links to its own detail view.
type UnifiedInboxHandler struct {
	repo    *repositories.UnifiedInboxRepository
	sources []string
}

// NewUnifiedInboxHandler covers the sources this deployment has enabled.
func NewUnifiedInboxHandler(repo *repositories.UnifiedInboxRepository, sources []string) *UnifiedInboxHandler {
	return &UnifiedInboxHandler{repo: repo, sources: sources}
}

type UnifiedInboxRequest struct {
	Page     int    `query:"page" default:"0"`
	Size     int    `query:"size" default:"30"`
	Source   string `query:"source" enum:"form,inbound,chat" doc:"One source only; all enabled sources by default"`
	State    string `query:"state" enum:"open,closed,spam" doc:"Open, closed or spam; all by default"`
	Assigned string `query:"assigned" enum:"me,unassigned,any"`
	Q        string `query:"q" doc:"Search titles, senders and previews"`
}

func (h *UnifiedInboxHandler) List(c *okapi.Context, req *UnifiedInboxRequest) error {
	ws, err := workspaceOf(c)
	if err != nil {
		return c.AbortBadRequest(err.Error())
	}
	sources := h.sources
	if req.Source != "" {
		sources = nil
		for _, s := range h.sources {
			if s == req.Source {
				sources = []string{s}
			}
		}
	}
	page, size := req.Page, req.Size
	if size <= 0 || size > 100 {
		size = 30
	}
	assigned := req.Assigned
	if assigned == "any" {
		assigned = ""
	}
	items, total, err := h.repo.List(ws, repositories.InboxFilter{
		Sources: sources, State: req.State, Assigned: assigned, UserID: uint(c.GetInt("user_id")), Q: req.Q,
	}, size, page*size)
	if err != nil {
		return c.AbortInternalServerError("failed to list the inbox")
	}
	return paginated(c, items, total, page, size)
}
