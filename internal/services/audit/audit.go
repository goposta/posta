// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package audit

import (
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/eventbus"
	"github.com/jkaninda/okapi"
)

// Logger records audit trail events via the existing event bus.
type Logger struct {
	bus *eventbus.EventBus
}

// NewLogger creates an audit logger backed by the given event bus.
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
