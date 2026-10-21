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
	"context"
	"fmt"
	"strings"

	"github.com/goposta/posta/internal/services/messages"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/logger"
	"github.com/jkaninda/okapi"
)

type GDPRDeleteMessagesRequest struct {
	Body struct {
		Email string `json:"email" required:"true" doc:"Sender address whose web form messages, thread replies, and attachments are erased."`
	} `json:"body"`
}

// DeleteMessages (GDPR) erases every web form message a person submitted to
// the workspace, the thread replies on them, the inbound emails those replies
// came from, and all of their stored attachments.
func (h *WorkspaceDataHandler) DeleteMessages(c *okapi.Context, req *GDPRDeleteMessagesRequest) error {
	wsID := uint(c.GetInt("workspace_id"))
	if wsID == 0 {
		return c.AbortBadRequest("workspace context required")
	}
	email := messages.NormalizeEmail(req.Body.Email)
	if email == "" || !strings.Contains(email, "@") {
		return c.AbortBadRequest("a valid email address is required")
	}

	deleted, blobKeys, err := repositories.EraseMessagesBySender(h.db, wsID, email)
	if err != nil {
		return c.AbortInternalServerError("failed to delete messages")
	}

	// Blobs go after the commit: a failed transaction must leave them in place.
	if h.blobStore != nil {
		ctx := context.Background()
		for _, key := range blobKeys {
			if err := h.blobStore.Delete(ctx, key); err != nil {
				logger.Warn("gdpr: failed to delete message blob", "key", key, "error", err)
			}
		}
	}

	if h.audit != nil {
		// The erased address is deliberately not logged.
		h.audit.LogCtxScoped(c, wsID, "gdpr.messages_deleted",
			fmt.Sprintf("Erased %d web form messages for a data subject", deleted),
			map[string]any{"deleted": deleted})
	}

	return ok(c, GDPRDeleteResult{
		Deleted: deleted,
		Message: fmt.Sprintf("%d messages and their replies deleted", deleted),
	})
}
