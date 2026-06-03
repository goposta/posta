/*
 * Copyright 2026 Jonas Kaninda
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

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
