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
	"errors"
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrChatConversationGone means a message was sent to a conversation that no longer exists.
var ErrChatConversationGone = errors.New("conversation not found")

type ChatRepository struct {
	db *gorm.DB
}

func NewChatRepository(db *gorm.DB) *ChatRepository {
	return &ChatRepository{db: db}
}

func (r *ChatRepository) CreateWidget(w *models.ChatWidget) error {
	return r.db.Create(w).Error
}

func (r *ChatRepository) UpdateWidget(w *models.ChatWidget) error {
	return r.db.Omit(clause.Associations).Save(w).Error
}

func (r *ChatRepository) DeleteWidget(id uint) error {
	return r.db.Delete(&models.ChatWidget{}, id).Error
}

func (r *ChatRepository) FindWidgetForScope(scope ResourceScope, id uint) (*models.ChatWidget, error) {
	var w models.ChatWidget
	if err := ApplyScope(r.db, scope).Preload("LastEditedBy").First(&w, id).Error; err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *ChatRepository) FindWidgetByID(id uint) (*models.ChatWidget, error) {
	var w models.ChatWidget
	if err := r.db.First(&w, id).Error; err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *ChatRepository) FindWidgetByKey(key string) (*models.ChatWidget, error) {
	var w models.ChatWidget
	if err := r.db.Where("public_key = ?", key).First(&w).Error; err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *ChatRepository) ListWidgets(scope ResourceScope, limit, offset int) ([]models.ChatWidget, int64, error) {
	var items []models.ChatWidget
	var total int64
	ApplyScope(r.db.Model(&models.ChatWidget{}), scope).Count(&total)
	if err := ApplyScope(r.db, scope).
		Preload("LastEditedBy").
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *ChatRepository) WidgetNameExists(workspaceID uint, name string, excludeID uint) bool {
	var n int64
	q := r.db.Model(&models.ChatWidget{}).Where("workspace_id = ? AND name = ?", workspaceID, name)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	q.Count(&n)
	return n > 0
}

func (r *ChatRepository) CreateVisitor(v *models.ChatVisitor) error {
	return r.db.Create(v).Error
}

func (r *ChatRepository) FindVisitorByTokenHash(hash string) (*models.ChatVisitor, error) {
	var v models.ChatVisitor
	if err := r.db.Where("token_hash = ?", hash).First(&v).Error; err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *ChatRepository) TouchVisitor(id uint, ip, page string) error {
	fields := map[string]any{"last_seen_at": time.Now(), "last_seen_ip": ip}
	if page != "" {
		fields["last_page_url"] = page
	}
	return r.db.Model(&models.ChatVisitor{}).Where("id = ?", id).Updates(fields).Error
}

// UpdateVisitorContact never clears a name or email the visitor gave before.
func (r *ChatRepository) UpdateVisitorContact(id uint, name, email string) error {
	fields := map[string]any{}
	if name != "" {
		fields["name"] = name
	}
	if email != "" {
		fields["email"] = email
	}
	if len(fields) == 0 {
		return nil
	}
	return r.db.Model(&models.ChatVisitor{}).Where("id = ?", id).Updates(fields).Error
}

// CountVisitorStartsFromIP feeds the scanner's repeat-sender signal.
func (r *ChatRepository) CountVisitorStartsFromIP(ip string, since time.Time) int64 {
	var n int64
	r.db.Model(&models.ChatConversation{}).
		Joins("JOIN chat_visitors ON chat_visitors.id = chat_conversations.visitor_id").
		Where("chat_visitors.first_seen_ip = ? AND chat_conversations.created_at >= ?", ip, since).
		Count(&n)
	return n
}

// CreateConversation stores the conversation and its first message (seq 1) in one transaction.
func (r *ChatRepository) CreateConversation(conv *models.ChatConversation, first *models.ChatMessage) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		conv.LastSeq = 1
		conv.LastAuthorKind = first.AuthorKind
		conv.LastMessagePreview = MessagePreview(first.Body)
		conv.LastMessageAt = now
		if first.AuthorKind == models.ChatAuthorVisitor {
			conv.LastVisitorMsgAt = &now
		}
		if err := tx.Omit(clause.Associations).Create(conv).Error; err != nil {
			return err
		}
		first.ConversationID = conv.ID
		first.WorkspaceID = conv.WorkspaceID
		first.Seq = 1
		return tx.Omit(clause.Associations).Create(first).Error
	})
}

func (r *ChatRepository) FindConversation(workspaceID uint, uuid string) (*models.ChatConversation, error) {
	var c models.ChatConversation
	if err := r.db.Preload("Visitor").Preload("Widget").Preload("AssignedTo").
		Where("workspace_id = ? AND uuid = ?", workspaceID, uuid).
		First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *ChatRepository) FindConversationByID(id uint) (*models.ChatConversation, error) {
	var c models.ChatConversation
	if err := r.db.Preload("Visitor").Preload("Widget").First(&c, id).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

// FindVisitorConversation treats another visitor's conversation exactly like a missing one.
func (r *ChatRepository) FindVisitorConversation(visitorID uint, uuid string) (*models.ChatConversation, error) {
	var c models.ChatConversation
	if err := r.db.Where("visitor_id = ? AND uuid = ?", visitorID, uuid).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *ChatRepository) ListVisitorConversations(visitorID uint, limit int) ([]models.ChatConversation, error) {
	var items []models.ChatConversation
	err := r.db.Where("visitor_id = ? AND state <> ?", visitorID, models.ChatStateSpam).
		Order("last_message_at DESC").Limit(limit).Find(&items).Error
	return items, err
}

type ChatConversationFilter struct {
	State string
	// Assigned is "me", "unassigned", or empty for any.
	Assigned string
	UserID   uint
	WidgetID uint
	Q        string
	// Quarantined conversations are left out unless asked for, or unless State is spam.
	Quarantined bool
}

func (r *ChatRepository) ListConversations(workspaceID uint, f ChatConversationFilter, limit, offset int) ([]models.ChatConversation, int64, error) {
	q := r.db.Model(&models.ChatConversation{}).Where("chat_conversations.workspace_id = ?", workspaceID)
	if f.State != "" {
		q = q.Where("chat_conversations.state = ?", f.State)
	}
	if !f.Quarantined && f.State != string(models.ChatStateSpam) {
		q = q.Where("chat_conversations.status <> ?", models.MessageStatusQuarantined)
	}
	switch f.Assigned {
	case "me":
		q = q.Where("chat_conversations.assigned_to_id = ?", f.UserID)
	case "unassigned":
		q = q.Where("chat_conversations.assigned_to_id IS NULL")
	}
	if f.WidgetID > 0 {
		q = q.Where("chat_conversations.widget_id = ?", f.WidgetID)
	}
	if term := strings.TrimSpace(f.Q); term != "" {
		like := "%" + strings.ToLower(term) + "%"
		q = q.Joins("JOIN chat_visitors ON chat_visitors.id = chat_conversations.visitor_id").
			Where("LOWER(chat_visitors.name) LIKE ? OR LOWER(chat_visitors.email) LIKE ?", like, like)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.ChatConversation
	err := q.Preload("Visitor").Preload("Widget").Preload("AssignedTo").
		Order("chat_conversations.last_message_at DESC").
		Limit(limit).Offset(offset).
		Find(&items).Error
	return items, total, err
}

// CountAwaitingReply counts open conversations whose last message is the visitor's.
func (r *ChatRepository) CountAwaitingReply(workspaceID uint) (int64, error) {
	var n int64
	err := r.db.Model(&models.ChatConversation{}).
		Where("workspace_id = ? AND state = ? AND last_author_kind = ? AND status <> ?",
			workspaceID, models.ChatStateOpen, models.ChatAuthorVisitor, models.MessageStatusQuarantined).
		Count(&n).Error
	return n, err
}

func (r *ChatRepository) UpdateConversationFields(id uint, fields map[string]any) error {
	return r.db.Model(&models.ChatConversation{}).Where("id = ?", id).Updates(fields).Error
}

// MessagePreview is the first line of a message, cut to fit the list.
func MessagePreview(body string) string {
	line := strings.Join(strings.Fields(body), " ")
	if r := []rune(line); len(r) > 140 {
		line = string(r[:139]) + "…"
	}
	return line
}

// AppendMessage locks the conversation row for gapless seqs; duplicate=true (known ClientMsgID) must not be re-announced.
func (r *ChatRepository) AppendMessage(msg *models.ChatMessage) (duplicate bool, err error) {
	if msg.ClientMsgID != "" {
		if existing, findErr := r.findByClientID(msg.ConversationID, msg.ClientMsgID); findErr == nil {
			*msg = *existing
			return true, nil
		}
	}

	visitor := msg.AuthorKind == models.ChatAuthorVisitor
	agent := msg.AuthorKind == models.ChatAuthorAgent
	err = r.db.Transaction(func(tx *gorm.DB) error {
		var seq []int64
		if err := tx.Raw(`UPDATE chat_conversations SET
				last_seq = last_seq + 1,
				last_message_at = now(),
				updated_at = now(),
				last_author_kind = ?,
				last_message_preview = ?,
				last_visitor_msg_at = CASE WHEN ? THEN now() ELSE last_visitor_msg_at END,
				first_response_at = CASE WHEN ? AND first_response_at IS NULL THEN now() ELSE first_response_at END,
				state = CASE WHEN ? AND state IN ('pending', 'closed') THEN 'open' ELSE state END,
				closed_at = CASE WHEN ? AND state IN ('pending', 'closed') THEN NULL ELSE closed_at END
			WHERE id = ? AND deleted_at IS NULL
			RETURNING last_seq`,
			msg.AuthorKind, MessagePreview(msg.Body), visitor, agent, visitor, visitor, msg.ConversationID).
			Scan(&seq).Error; err != nil {
			return err
		}
		if len(seq) == 0 {
			return ErrChatConversationGone
		}
		msg.Seq = seq[0]
		return tx.Omit(clause.Associations).Create(msg).Error
	})
	if err != nil && msg.ClientMsgID != "" && IsDuplicateName(err) {
		// Two copies of one send raced; the other stored it.
		if existing, findErr := r.findByClientID(msg.ConversationID, msg.ClientMsgID); findErr == nil {
			*msg = *existing
			return true, nil
		}
	}
	return false, err
}

func (r *ChatRepository) findByClientID(conversationID uint, clientMsgID string) (*models.ChatMessage, error) {
	var m models.ChatMessage
	if err := r.db.Preload("Author").
		Where("conversation_id = ? AND client_msg_id = ?", conversationID, clientMsgID).
		First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// MessagesAfter returns messages above after, oldest first.
func (r *ChatRepository) MessagesAfter(conversationID uint, after int64, limit int) ([]models.ChatMessage, error) {
	var items []models.ChatMessage
	err := r.db.Preload("Author").
		Where("conversation_id = ? AND seq > ?", conversationID, after).
		Order("seq ASC").Limit(limit).Find(&items).Error
	return items, err
}

// MessagesBefore returns the page just above what a client shows, oldest first.
func (r *ChatRepository) MessagesBefore(conversationID uint, before int64, limit int) ([]models.ChatMessage, error) {
	var items []models.ChatMessage
	if err := r.db.Preload("Author").
		Where("conversation_id = ? AND seq < ?", conversationID, before).
		Order("seq DESC").Limit(limit).Find(&items).Error; err != nil {
		return nil, err
	}
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
	return items, nil
}

func (r *ChatRepository) FirstMessage(conversationID uint) (*models.ChatMessage, error) {
	var m models.ChatMessage
	if err := r.db.Where("conversation_id = ? AND seq = 1", conversationID).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *ChatRepository) FindConversationByThreadToken(token string) (*models.ChatConversation, error) {
	var c models.ChatConversation
	if err := r.db.Preload("Visitor").Preload("Widget").Where("thread_token = ?", token).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

// MarkRead never moves backwards or past the newest message, and returns the stored value.
func (r *ChatRepository) MarkRead(conversationID uint, visitor bool, seq int64) (int64, error) {
	column := "agent_read_seq"
	if visitor {
		column = "visitor_read_seq"
	}
	var out []int64
	err := r.db.Raw(`UPDATE chat_conversations
		SET `+column+` = GREATEST(`+column+`, LEAST(?, last_seq))
		WHERE id = ? AND deleted_at IS NULL
		RETURNING `+column, seq, conversationID).Scan(&out).Error
	if err != nil {
		return 0, err
	}
	if len(out) == 0 {
		return 0, ErrChatConversationGone
	}
	return out[0], nil
}

// LatestSeqBy returns 0 when the author kind has no messages.
func (r *ChatRepository) LatestSeqBy(conversationID uint, authorKind string) (int64, error) {
	var seq int64
	err := r.db.Model(&models.ChatMessage{}).
		Where("conversation_id = ? AND author_kind = ?", conversationID, authorKind).
		Select("COALESCE(MAX(seq), 0)").Scan(&seq).Error
	return seq, err
}

// AgentMessagesToEmail returns agent messages after afterSeq: those the visitor has neither read nor been emailed.
func (r *ChatRepository) AgentMessagesToEmail(conversationID uint, afterSeq int64) ([]models.ChatMessage, error) {
	var items []models.ChatMessage
	err := r.db.Preload("Author").
		Where("conversation_id = ? AND author_kind = ? AND seq > ?",
			conversationID, models.ChatAuthorAgent, afterSeq).
		Order("seq ASC").Limit(100).Find(&items).Error
	return items, err
}

func (r *ChatRepository) MarkEmailed(conversationID uint, messageIDs []uint, throughSeq int64, emailUUID string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.ChatConversation{}).Where("id = ?", conversationID).
			Update("emailed_through_seq", gorm.Expr("GREATEST(emailed_through_seq, ?)", throughSeq)).Error; err != nil {
			return err
		}
		if len(messageIDs) == 0 {
			return nil
		}
		return tx.Model(&models.ChatMessage{}).Where("id IN ?", messageIDs).Update("email_uuid", emailUUID).Error
	})
}

func (r *ChatRepository) UpdateVisitor(id uint, fields map[string]any) error {
	return r.db.Model(&models.ChatVisitor{}).Where("id = ?", id).Updates(fields).Error
}

func (r *ChatRepository) FindVisitorByID(id uint) (*models.ChatVisitor, error) {
	var v models.ChatVisitor
	if err := r.db.First(&v, id).Error; err != nil {
		return nil, err
	}
	return &v, nil
}

// LinkedVisitorIDs adds verified visitors of the same widget and customer user ID; typed details never link anyone.
func (r *ChatRepository) LinkedVisitorIDs(v *models.ChatVisitor) []uint {
	if !v.EmailVerified || v.ExternalID == "" {
		return []uint{v.ID}
	}
	var ids []uint
	if err := r.db.Model(&models.ChatVisitor{}).
		Where("widget_id = ? AND external_id = ? AND email_verified", v.WidgetID, v.ExternalID).
		Pluck("id", &ids).Error; err != nil || len(ids) == 0 {
		return []uint{v.ID}
	}
	return ids
}

// FindLinkedConversation treats a conversation of any other visitor as not found.
func (r *ChatRepository) FindLinkedConversation(visitorIDs []uint, uuid string) (*models.ChatConversation, error) {
	var c models.ChatConversation
	if err := r.db.Where("visitor_id IN ? AND uuid = ?", visitorIDs, uuid).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *ChatRepository) ListLinkedConversations(visitorIDs []uint, limit int) ([]models.ChatConversation, error) {
	var items []models.ChatConversation
	err := r.db.Where("visitor_id IN ? AND state <> ?", visitorIDs, models.ChatStateSpam).
		Order("last_message_at DESC").Limit(limit).Find(&items).Error
	return items, err
}

// VisitorUnread counts, per conversation, the agent messages past the visitor's read receipt.
func (r *ChatRepository) VisitorUnread(convIDs []uint) (map[uint]int, error) {
	out := make(map[uint]int, len(convIDs))
	if len(convIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		ConversationID uint
		N              int
	}
	err := r.db.Raw(`SELECT m.conversation_id, count(*) AS n
		FROM chat_messages m JOIN chat_conversations c ON c.id = m.conversation_id
		WHERE m.conversation_id IN ? AND m.author_kind = ? AND m.seq > c.visitor_read_seq
		GROUP BY m.conversation_id`, convIDs, models.ChatAuthorAgent).Scan(&rows).Error
	for _, row := range rows {
		out[row.ConversationID] = row.N
	}
	return out, err
}

// VisitorHistory includes, for a verified visitor, the same person's conversations on other devices.
func (r *ChatRepository) VisitorHistory(conv *models.ChatConversation, limit int) ([]models.ChatConversation, error) {
	v, err := r.FindVisitorByID(conv.VisitorID)
	if err != nil {
		return nil, err
	}
	var items []models.ChatConversation
	err = r.db.Preload("Widget").
		Where("visitor_id IN ? AND id <> ? AND workspace_id = ?", r.LinkedVisitorIDs(v), conv.ID, *conv.WorkspaceID).
		Order("last_message_at DESC").Limit(limit).Find(&items).Error
	return items, err
}

func (r *ChatRepository) FindContactByEmail(workspaceID uint, email string) (*models.Contact, error) {
	var c models.Contact
	if err := r.db.Where("workspace_id = ? AND LOWER(email) = LOWER(?)", workspaceID, email).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

// AddSpamHit counts a rejected message and returns the new total.
func (r *ChatRepository) AddSpamHit(conversationID uint) (int, error) {
	var hits []int
	err := r.db.Raw(`UPDATE chat_conversations SET spam_hits = spam_hits + 1
		WHERE id = ? RETURNING spam_hits`, conversationID).Scan(&hits).Error
	if err != nil || len(hits) == 0 {
		return 0, err
	}
	return hits[0], nil
}

func (r *ChatRepository) OpenAssignedCounts(workspaceID uint) (map[uint]int, error) {
	var rows []struct {
		AssignedToID uint
		N            int
	}
	err := r.db.Model(&models.ChatConversation{}).
		Select("assigned_to_id, COUNT(*) AS n").
		Where("workspace_id = ? AND state = ? AND assigned_to_id IS NOT NULL", workspaceID, models.ChatStateOpen).
		Group("assigned_to_id").Scan(&rows).Error
	out := make(map[uint]int, len(rows))
	for _, row := range rows {
		out[row.AssignedToID] = row.N
	}
	return out, err
}

func (r *ChatRepository) FindMessage(conversationID uint, uuid string) (*models.ChatMessage, error) {
	var m models.ChatMessage
	if err := r.db.Where("conversation_id = ? AND uuid = ?", conversationID, uuid).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *ChatRepository) ListSnippets(workspaceID uint) ([]models.ChatSnippet, error) {
	var items []models.ChatSnippet
	err := r.db.Where("workspace_id = ?", workspaceID).Order("shortcut ASC").Find(&items).Error
	return items, err
}

func (r *ChatRepository) FindSnippet(workspaceID, id uint) (*models.ChatSnippet, error) {
	var s models.ChatSnippet
	if err := r.db.Where("workspace_id = ?", workspaceID).First(&s, id).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *ChatRepository) SnippetShortcutExists(workspaceID uint, shortcut string, excludeID uint) bool {
	var n int64
	q := r.db.Model(&models.ChatSnippet{}).Where("workspace_id = ? AND shortcut = ?", workspaceID, shortcut)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	q.Count(&n)
	return n > 0
}

func (r *ChatRepository) SaveSnippet(s *models.ChatSnippet) error {
	return r.db.Save(s).Error
}

func (r *ChatRepository) DeleteSnippet(id uint) error {
	return r.db.Delete(&models.ChatSnippet{}, id).Error
}

// ChatAnalytics times are in seconds, nil when there is nothing to measure.
type ChatAnalytics struct {
	Conversations       int64             `json:"conversations"`
	Open                int64             `json:"open"`
	Closed              int64             `json:"closed"`
	Spam                int64             `json:"spam"`
	Messages            int64             `json:"messages"`
	FirstResponseAvg    *float64          `json:"first_response_avg"`
	FirstResponseMedian *float64          `json:"first_response_median"`
	ResolutionAvg       *float64          `json:"resolution_avg"`
	ResolutionMedian    *float64          `json:"resolution_median"`
	Unanswered          int64             `json:"unanswered" doc:"Conversations with no agent reply yet"`
	ByDay               []ChatAnalyticDay `json:"by_day"`
}

type ChatAnalyticDay struct {
	Day           string `json:"day" doc:"YYYY-MM-DD, UTC"`
	Conversations int64  `json:"conversations"`
}

// Analytics counts conversations started in [from, to), quarantined ones excluded.
func (r *ChatRepository) Analytics(workspaceID uint, widgetID uint, from, to time.Time) (*ChatAnalytics, error) {
	base := func() *gorm.DB {
		q := r.db.Model(&models.ChatConversation{}).
			Where("workspace_id = ? AND created_at >= ? AND created_at < ? AND status <> ?",
				workspaceID, from, to, models.MessageStatusQuarantined)
		if widgetID > 0 {
			q = q.Where("widget_id = ?", widgetID)
		}
		return q
	}
	out := &ChatAnalytics{ByDay: []ChatAnalyticDay{}}
	var agg struct {
		Conversations, Open, Closed, Spam, Unanswered       int64
		FirstAvg, FirstMedian, ResolutionAvg, ResolutionMed *float64
	}
	err := base().Select(`
		COUNT(*) AS conversations,
		COUNT(*) FILTER (WHERE state = 'open') AS open,
		COUNT(*) FILTER (WHERE state = 'closed') AS closed,
		COUNT(*) FILTER (WHERE state = 'spam') AS spam,
		COUNT(*) FILTER (WHERE first_response_at IS NULL AND state <> 'spam') AS unanswered,
		AVG(EXTRACT(EPOCH FROM first_response_at - created_at)) AS first_avg,
		PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM first_response_at - created_at)) AS first_median,
		AVG(EXTRACT(EPOCH FROM closed_at - created_at)) FILTER (WHERE state = 'closed') AS resolution_avg,
		PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM closed_at - created_at)) FILTER (WHERE state = 'closed') AS resolution_med`).
		Scan(&agg).Error
	if err != nil {
		return nil, err
	}
	out.Conversations, out.Open, out.Closed, out.Spam, out.Unanswered = agg.Conversations, agg.Open, agg.Closed, agg.Spam, agg.Unanswered
	out.FirstResponseAvg, out.FirstResponseMedian = agg.FirstAvg, agg.FirstMedian
	out.ResolutionAvg, out.ResolutionMedian = agg.ResolutionAvg, agg.ResolutionMed

	if err := base().Select("TO_CHAR(DATE_TRUNC('day', created_at AT TIME ZONE 'UTC'), 'YYYY-MM-DD') AS day, COUNT(*) AS conversations").
		Group("day").Order("day").Scan(&out.ByDay).Error; err != nil {
		return nil, err
	}
	msgs := r.db.Model(&models.ChatMessage{}).
		Joins("JOIN chat_conversations c ON c.id = chat_messages.conversation_id").
		Where("c.workspace_id = ? AND chat_messages.created_at >= ? AND chat_messages.created_at < ?", workspaceID, from, to)
	if widgetID > 0 {
		msgs = msgs.Where("c.widget_id = ?", widgetID)
	}
	if err := msgs.Count(&out.Messages).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func (r *ChatRepository) FindMessageByID(id uint) (*models.ChatMessage, error) {
	var m models.ChatMessage
	if err := r.db.Preload("Author").First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// EraseConversation deletes the visitor too if this was their last conversation; delete the returned blobs after commit.
func (r *ChatRepository) EraseConversation(conversationID uint) ([]string, error) {
	var keys []string
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var conv models.ChatConversation
		if err := tx.Unscoped().First(&conv, conversationID).Error; err != nil {
			return err
		}
		k, err := attachmentKeys(tx, "conversation_id = ?", conv.ID)
		if err != nil {
			return err
		}
		keys = k
		if err := tx.Where("conversation_id = ?", conv.ID).Delete(&models.ChatMessage{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Delete(&models.ChatConversation{}, conv.ID).Error; err != nil {
			return err
		}
		return deleteOrphanVisitor(tx, conv.VisitorID)
	})
	return keys, err
}

// EraseChatsByEmail matches typed or verified email; delete the returned blob keys once the transaction has committed.
func (r *ChatRepository) EraseChatsByEmail(workspaceID uint, email string) (int64, []string, error) {
	var count int64
	var keys []string
	err := r.db.Transaction(func(tx *gorm.DB) error {
		visitors := tx.Model(&models.ChatVisitor{}).Select("id").
			Where("workspace_id = ? AND LOWER(email) = LOWER(?)", workspaceID, email)
		convs := tx.Unscoped().Model(&models.ChatConversation{}).Select("id").Where("visitor_id IN (?)", visitors)
		k, err := attachmentKeys(tx, "conversation_id IN (?)", convs)
		if err != nil {
			return err
		}
		keys = k
		if err := tx.Where("conversation_id IN (?)", convs).Delete(&models.ChatMessage{}).Error; err != nil {
			return err
		}
		res := tx.Unscoped().Where("visitor_id IN (?)", visitors).Delete(&models.ChatConversation{})
		if res.Error != nil {
			return res.Error
		}
		count = res.RowsAffected
		return tx.Where("workspace_id = ? AND LOWER(email) = LOWER(?)", workspaceID, email).Delete(&models.ChatVisitor{}).Error
	})
	return count, keys, err
}

func (r *ChatRepository) ExportChatsByEmail(workspaceID uint, email string) ([]ChatExport, error) {
	var convs []models.ChatConversation
	err := r.db.Unscoped().Preload("Visitor").Preload("Widget").
		Joins("JOIN chat_visitors ON chat_visitors.id = chat_conversations.visitor_id").
		Where("chat_visitors.workspace_id = ? AND LOWER(chat_visitors.email) = LOWER(?)", workspaceID, email).
		Order("chat_conversations.created_at").Find(&convs).Error
	if err != nil {
		return nil, err
	}
	out := make([]ChatExport, 0, len(convs))
	for i := range convs {
		var msgs []models.ChatMessage
		if err := r.db.Preload("Author").Where("conversation_id = ?", convs[i].ID).Order("seq").Find(&msgs).Error; err != nil {
			return nil, err
		}
		out = append(out, ChatExport{Conversation: convs[i], Messages: msgs})
	}
	return out, nil
}

type ChatExport struct {
	Conversation models.ChatConversation `json:"conversation"`
	Messages     []models.ChatMessage    `json:"messages"`
}

func attachmentKeys(tx *gorm.DB, where string, args ...any) ([]string, error) {
	var rows []models.ChatMessage
	if err := tx.Select("attachments").Where(where, args...).Where("attachments IS NOT NULL").Find(&rows).Error; err != nil {
		return nil, err
	}
	var keys []string
	for _, m := range rows {
		for _, a := range m.Attachments {
			if a.Key != "" {
				keys = append(keys, a.Key)
			}
		}
	}
	return keys, nil
}

func deleteOrphanVisitor(tx *gorm.DB, visitorID uint) error {
	var n int64
	if err := tx.Unscoped().Model(&models.ChatConversation{}).Where("visitor_id = ?", visitorID).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	return tx.Delete(&models.ChatVisitor{}, visitorID).Error
}

type ChatRetentionWidget struct {
	ID            uint
	RetentionDays int
}

// RetentionWidgets includes deleted widgets so their conversations age out like the rest.
func (r *ChatRepository) RetentionWidgets() ([]ChatRetentionWidget, error) {
	var out []ChatRetentionWidget
	err := r.db.Unscoped().Model(&models.ChatWidget{}).Select("id, retention_days").Scan(&out).Error
	return out, err
}

// ExpiredConversationIDs selects only closed or spam conversations.
func (r *ChatRepository) ExpiredConversationIDs(widgetID uint, cutoff time.Time, limit int) ([]uint, error) {
	var ids []uint
	err := r.db.Unscoped().Model(&models.ChatConversation{}).
		Where("widget_id = ? AND state IN ? AND last_message_at < ?", widgetID,
			[]models.ChatState{models.ChatStateClosed, models.ChatStateSpam}, cutoff).
		Limit(limit).Pluck("id", &ids).Error
	return ids, err
}

// DeleteIdleVisitors removes visitors with no conversations left who have not been seen since cutoff.
func (r *ChatRepository) DeleteIdleVisitors(cutoff time.Time) (int64, error) {
	res := r.db.Where("last_seen_at < ? AND NOT EXISTS (SELECT 1 FROM chat_conversations c WHERE c.visitor_id = chat_visitors.id)", cutoff).
		Delete(&models.ChatVisitor{})
	return res.RowsAffected, res.Error
}

type ChatVisitorIPs struct {
	ID          uint
	FirstSeenIP string
	LastSeenIP  string
}

// VisitorsWithIPsBefore pages by afterID over visitors whose addresses may still be whole.
func (r *ChatRepository) VisitorsWithIPsBefore(cutoff time.Time, afterID uint, limit int) ([]ChatVisitorIPs, error) {
	var out []ChatVisitorIPs
	err := r.db.Model(&models.ChatVisitor{}).Select("id, first_seen_ip, last_seen_ip").
		Where("last_seen_at < ? AND id > ? AND (first_seen_ip <> '' OR last_seen_ip <> '')", cutoff, afterID).
		Order("id").Limit(limit).Scan(&out).Error
	return out, err
}

// CountConversationsSince excludes quarantined conversations.
func (r *ChatRepository) CountConversationsSince(workspaceID uint, t time.Time) (int64, error) {
	var n int64
	err := r.db.Unscoped().Model(&models.ChatConversation{}).
		Where("workspace_id = ? AND created_at >= ? AND status <> ?", workspaceID, t, models.MessageStatusQuarantined).
		Count(&n).Error
	return n, err
}
