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
	"github.com/jkaninda/posta/internal/models"
	"github.com/jkaninda/posta/internal/services/eventbus"
)

// Logger records audit trail events via the existing event bus.
type Logger struct {
	bus *eventbus.EventBus
}

// NewLogger creates an audit logger backed by the given event bus.
func NewLogger(bus *eventbus.EventBus) *Logger {
	return &Logger{bus: bus}
}

// Log records an audit event with the given actor, action type, message, and optional metadata.
func (l *Logger) Log(actorID uint, actorEmail, clientIP, action, message string, metadata map[string]any) {
	l.bus.PublishSimple(models.EventCategoryAudit, action, &actorID, actorEmail, clientIP, message, metadata)
}
