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

package repositories

import (
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
)

// Unified inbox sources.
const (
	InboxSourceForm    = "form"
	InboxSourceInbound = "inbound"
	InboxSourceChat    = "chat"
)

// InboxItem is one row of the unified inbox: a form message, a loose
// inbound email, or a chat, in one shape.
type InboxItem struct {
	Source         string    `json:"source" enum:"form,inbound,chat"`
	ID             string    `json:"id" doc:"UUID of the message, inbound email or conversation"`
	Title          string    `json:"title"`
	FromName       string    `json:"from_name"`
	FromEmail      string    `json:"from_email"`
	Preview        string    `json:"preview"`
	State          string    `json:"state" enum:"open,closed,spam"`
	AssignedToID   *uint     `json:"assigned_to_id,omitempty"`
	AssignedToName string    `json:"assigned_to_name,omitempty"`
	Unread         bool      `json:"unread" doc:"Form messages not yet read, chats whose visitor wrote last"`
	LastActivityAt time.Time `json:"last_activity_at"`
	CreatedAt      time.Time `json:"created_at"`
}

// InboxFilter narrows the unified inbox.
type InboxFilter struct {
	Sources []string
	// State is open, closed, spam, or empty for all.
	State string
	// Assigned is me, unassigned, or empty for any.
	Assigned string
	UserID   uint
	Q        string
}

// UnifiedInboxRepository lists form messages, loose inbound emails and chats
// together. It is one UNION ALL over the three tables with a shared
// projection, not a merged model: each source keeps its own table and its own
// detail view.
type UnifiedInboxRepository struct {
	db *gorm.DB
}

func NewUnifiedInboxRepository(db *gorm.DB) *UnifiedInboxRepository {
	return &UnifiedInboxRepository{db: db}
}

// Inbound emails that became a reply on a form thread or a chat are already
// shown there.
const looseInbound = `NOT EXISTS (SELECT 1 FROM message_replies r WHERE r.inbound_email_id = i.id)
	AND NOT EXISTS (SELECT 1 FROM chat_messages cm WHERE cm.client_msg_id = 'inbound-' || i.id::text)`

var inboxSelects = map[string]string{
	InboxSourceForm: `SELECT 'form' AS source, m.uuid::text AS id,
			COALESCE(NULLIF(m.subject, ''), f.display_name) AS title,
			m.sender_name AS from_name, m.sender_email AS from_email, LEFT(m.body, 200) AS preview,
			CASE WHEN m.state IN ('closed', 'spam') THEN m.state ELSE 'open' END AS state,
			m.assigned_to_id, u.name AS assigned_to_name, (m.read_at IS NULL) AS unread,
			COALESCE(m.updated_at, m.created_at) AS last_activity_at, m.created_at
		FROM messages m JOIN forms f ON f.id = m.form_id
		LEFT JOIN users u ON u.id = m.assigned_to_id
		WHERE m.workspace_id = @ws AND m.deleted_at IS NULL AND f.mode <> 'collect'
			AND m.status NOT IN ('quarantined', 'rejected')`,
	InboxSourceInbound: `SELECT 'inbound' AS source, i.uuid::text AS id, i.subject AS title,
			'' AS from_name, i.sender AS from_email, LEFT(i.text_body, 200) AS preview,
			'open' AS state, NULL::bigint AS assigned_to_id, NULL::text AS assigned_to_name, false AS unread,
			COALESCE(i.received_at, i.created_at) AS last_activity_at, i.created_at
		FROM inbound_emails i
		WHERE i.workspace_id = @ws AND i.status NOT IN ('rejected', 'quarantined') AND ` + looseInbound,
	InboxSourceChat: `SELECT 'chat' AS source, c.uuid::text AS id, COALESCE(w.display_name, 'Chat') AS title,
			v.name AS from_name, v.email AS from_email, c.last_message_preview AS preview,
			CASE WHEN c.state IN ('closed', 'spam') THEN c.state ELSE 'open' END AS state,
			c.assigned_to_id, u.name AS assigned_to_name, (c.state = 'open' AND c.last_author_kind = 'visitor') AS unread,
			c.last_message_at AS last_activity_at, c.created_at
		FROM chat_conversations c
		LEFT JOIN users u ON u.id = c.assigned_to_id
		JOIN chat_visitors v ON v.id = c.visitor_id
		LEFT JOIN chat_widgets w ON w.id = c.widget_id
		WHERE c.workspace_id = @ws AND c.deleted_at IS NULL AND c.status <> 'quarantined'`,
}

// List returns one page of the inbox, newest activity first, and the total.
func (r *UnifiedInboxRepository) List(workspaceID uint, f InboxFilter, limit, offset int) ([]InboxItem, int64, error) {
	var parts []string
	for _, src := range f.Sources {
		if sel, ok := inboxSelects[src]; ok {
			parts = append(parts, sel)
		}
	}
	if len(parts) == 0 {
		return []InboxItem{}, 0, nil
	}
	where := []string{"TRUE"}
	args := map[string]any{"ws": workspaceID}
	switch f.State {
	case "open", "closed", "spam":
		where = append(where, "x.state = @state")
		args["state"] = f.State
	}
	switch f.Assigned {
	case "me":
		where = append(where, "x.assigned_to_id = @user")
		args["user"] = f.UserID
	case "unassigned":
		where = append(where, "x.assigned_to_id IS NULL")
	}
	if q := strings.TrimSpace(f.Q); q != "" {
		where = append(where, "(x.title ILIKE @q OR x.from_name ILIKE @q OR x.from_email ILIKE @q OR x.preview ILIKE @q)")
		args["q"] = "%" + strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(q) + "%"
	}
	from := "(" + strings.Join(parts, " UNION ALL ") + ") x WHERE " + strings.Join(where, " AND ")

	var total int64
	if err := r.db.Raw("SELECT COUNT(*) FROM "+from, args).Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	args["limit"], args["offset"] = limit, offset
	var items []InboxItem
	if err := r.db.Raw("SELECT x.* FROM "+from+" ORDER BY x.last_activity_at DESC LIMIT @limit OFFSET @offset", args).
		Scan(&items).Error; err != nil {
		return nil, 0, err
	}
	if items == nil {
		items = []InboxItem{}
	}
	r.decryptForms(items)
	return items, total, nil
}

// decryptForms re-reads the page's form messages through GORM, whose hooks
// decrypt answers kept encrypted at rest; raw SQL sees only ciphertext.
func (r *UnifiedInboxRepository) decryptForms(items []InboxItem) {
	var ids []string
	for _, it := range items {
		if it.Source == InboxSourceForm {
			ids = append(ids, it.ID)
		}
	}
	if len(ids) == 0 {
		return
	}
	var msgs []models.Message
	if err := r.db.Select("uuid, subject, sender_name, body, fields_json, data").Where("uuid IN ?", ids).Find(&msgs).Error; err != nil {
		return
	}
	byID := make(map[string]*models.Message, len(msgs))
	for i := range msgs {
		byID[msgs[i].UUID] = &msgs[i]
	}
	for i := range items {
		m := byID[items[i].ID]
		if items[i].Source != InboxSourceForm || m == nil {
			continue
		}
		if m.Subject != "" {
			items[i].Title = m.Subject
		}
		items[i].FromName = m.SenderName
		items[i].Preview = MessagePreview(m.Body)
	}
}
