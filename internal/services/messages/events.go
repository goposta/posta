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

package messages

import (
	"context"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/realtime"
	"github.com/jkaninda/logger"
)

// Live event types on a workspace's message stream.
const (
	EventReceived      = "message.received"
	EventUpdated       = "message.updated"
	EventReplied       = "message.replied"
	EventReplyReceived = "message.reply_received"
)

// PublishEvent sends a message.* event to the workspace's live stream. The
// payload carries identifiers and status only; the dashboard refetches what it
// shows, so a dropped event costs freshness, never correctness.
func PublishEvent(ctx context.Context, bus realtime.Bus, eventType string, msg *models.Message) {
	if bus == nil || msg == nil || msg.WorkspaceID == nil {
		return
	}
	ev := realtime.Event{
		Type:        eventType,
		WorkspaceID: *msg.WorkspaceID,
		Topic:       realtime.WorkspaceTopic(*msg.WorkspaceID),
		Data: map[string]any{
			"message_id": msg.UUID,
			"form_id":    msg.FormID,
			"status":     msg.Status,
			"state":      msg.State,
		},
	}
	if err := bus.Publish(ctx, ev); err != nil {
		logger.Warn("failed to publish message event", "type", eventType, "message_id", msg.ID, "error", err)
	}
}
