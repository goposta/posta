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

package audit

import (
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/eventbus"
	"github.com/jkaninda/okapi"
)

type Logger struct {
	bus *eventbus.EventBus
}

func NewLogger(bus *eventbus.EventBus) *Logger {
	return &Logger{bus: bus}
}

func (l *Logger) Log(actorID uint, actorEmail, clientIP, action, message string, metadata map[string]any) {
	l.LogScoped(nil, actorID, actorEmail, clientIP, action, message, metadata)
}

func (l *Logger) LogScoped(workspaceID *uint, actorID uint, actorEmail, clientIP, action, message string, metadata map[string]any) {
	l.bus.PublishScoped(workspaceID, models.EventCategoryAudit, action, &actorID, actorEmail, clientIP, message, metadata)
}

func (l *Logger) LogCtx(c *okapi.Context, action, message string, metadata map[string]any) {
	l.LogScoped(ctxWorkspaceID(c), actorID(c), c.GetString("email"), c.RealIP(), action, message, metadata)
}

func (l *Logger) LogCtxScoped(c *okapi.Context, workspaceID uint, action, message string, metadata map[string]any) {
	l.LogScoped(&workspaceID, actorID(c), c.GetString("email"), c.RealIP(), action, message, metadata)
}

func actorID(c *okapi.Context) uint {
	return uint(c.GetInt("user_id"))
}

func ctxWorkspaceID(c *okapi.Context) *uint {
	if w := c.GetInt("workspace_id"); w > 0 {
		id := uint(w)
		return &id
	}
	return nil
}
