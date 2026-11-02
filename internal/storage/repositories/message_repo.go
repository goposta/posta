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
	"encoding/json"
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MineFilter is a member's own work: assigned to them, or to their teams.
type MineFilter struct {
	UserID  uint
	TeamIDs []uint
}

type MessageFilterQuery struct {
	FormID   uint
	Status   string
	State    string
	Assigned *uint
	Unread   bool
	Query    string
	From     *time.Time
	To       *time.Time

	// Tags keeps messages carrying every listed tag; Priority one priority.
	Tags     []string
	Priority string
	// TeamID keeps one team's submissions. Mine keeps what is assigned to
	// Mine.UserID or routed to one of Mine.TeamIDs: the "My teams" view.
	TeamID uint
	Mine   *MineFilter
	// InboxOnly leaves out collect-mode forms, whose responses are read as a
	// table on the form rather than worked as conversations.
	InboxOnly bool

	// SubmissionsOnly leaves out quarantined and rejected messages unless
	// Status asks for one: spam is not a response.
	SubmissionsOnly bool
	// DataEquals matches typed answers by containment (data @> ...), so it is
	// served by the GIN index. A multiselect value is a one-element array.
	DataEquals map[string]any
	// DataRanges bound answers from below or above. Keys are checked against
	// the form schema by the caller and always bound as parameters.
	DataRanges []DataRange
}

// DataRange is one bound on a typed answer. Numeric compares numbers and
// skips answers that are not numbers; otherwise the text is compared, which
// orders YYYY-MM-DD dates and HH:MM times correctly.
type DataRange struct {
	Key     string
	Op      string // gte | lte
	Value   any
	Numeric bool
}

// applyMessageFilter narrows q to the messages f selects.
func applyMessageFilter(q *gorm.DB, scope ResourceScope, f MessageFilterQuery) *gorm.DB {
	q = ApplyWorkspaceScope(q, scope)
	if f.FormID > 0 {
		q = q.Where("form_id = ?", f.FormID)
	}
	switch {
	case f.Status != "":
		q = q.Where("status = ?", f.Status)
	case f.SubmissionsOnly:
		q = q.Where("status IN ?", []models.MessageStatus{models.MessageStatusReceived, models.MessageStatusFlagged})
	default:
		q = q.Where("status <> ?", models.MessageStatusRejected)
	}
	if f.State != "" {
		q = q.Where("state = ?", f.State)
	}
	if f.Assigned != nil {
		q = q.Where("assigned_to_id = ?", *f.Assigned)
	}
	if f.Unread {
		q = q.Where("read_at IS NULL")
	}
	if len(f.Tags) > 0 {
		q = q.Where("tags @> ?", pq.StringArray(f.Tags))
	}
	if f.Priority != "" {
		q = q.Where("priority = ?", f.Priority)
	}
	if f.TeamID > 0 {
		q = q.Where("team_id = ?", f.TeamID)
	}
	if f.Mine != nil {
		if len(f.Mine.TeamIDs) > 0 {
			q = q.Where("(assigned_to_id = ? OR team_id IN ?)", f.Mine.UserID, f.Mine.TeamIDs)
		} else {
			q = q.Where("assigned_to_id = ?", f.Mine.UserID)
		}
	}
	if f.InboxOnly {
		q = q.Where(notCollectForm)
	}
	if f.Query != "" {
		like := "%" + strings.ToLower(f.Query) + "%"
		q = q.Where(
			"LOWER(subject) LIKE ? OR LOWER(body) LIKE ? OR LOWER(sender_email) LIKE ? OR LOWER(sender_name) LIKE ? OR sender_phone LIKE ? OR LOWER(COALESCE(data::text, '')) LIKE ?",
			like, like, like, like, like, like)
	}
	if f.From != nil {
		q = q.Where("created_at >= ?", *f.From)
	}
	if f.To != nil {
		q = q.Where("created_at <= ?", *f.To)
	}
	if len(f.DataEquals) > 0 {
		if raw, err := json.Marshal(f.DataEquals); err == nil {
			q = q.Where("data @> ?::jsonb", string(raw))
		}
	}
	for _, r := range f.DataRanges {
		op := ">="
		if r.Op == "lte" {
			op = "<="
		}
		if r.Numeric {
			q = q.Where("(CASE WHEN jsonb_typeof(data -> ?) = 'number' THEN (data ->> ?)::numeric END) "+op+" ?", r.Key, r.Key, r.Value)
		} else {
			q = q.Where("(data ->> ?) "+op+" ?", r.Key, r.Value)
		}
	}
	return q
}

type MessageRepository struct {
	db *gorm.DB
}

func NewMessageRepository(db *gorm.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

func (r *MessageRepository) Create(m *models.Message) error {
	return r.db.Create(m).Error
}

// Update writes every column of m. It must not be used after a slow operation
// (sending mail, calling out): the row may have changed in the meantime, and a
// full-row save would silently undo those changes. Use UpdateFields instead.
func (r *MessageRepository) Update(m *models.Message) error {
	return r.db.Omit(clause.Associations).Save(m).Error
}

// UpdateFields writes only the given columns, so concurrent changes to other
// columns of the same message survive.
func (r *MessageRepository) UpdateFields(id uint, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	return r.db.Model(&models.Message{}).Where("id = ?", id).Updates(fields).Error
}

// RecordReply applies the effects of an operator reply atomically: it bumps the
// reply count, stamps replied_at, marks the message read if it was not, and
// moves it to replied unless an operator has already closed it.
func (r *MessageRepository) RecordReply(id uint, at time.Time) error {
	return r.db.Model(&models.Message{}).Where("id = ?", id).Updates(map[string]any{
		"replied_at":  at,
		"updated_at":  at,
		"reply_count": gorm.Expr("reply_count + 1"),
		"read_at":     gorm.Expr("COALESCE(read_at, ?)", at),
		"state": gorm.Expr("CASE WHEN state = ? THEN state ELSE ? END",
			models.MessageStateClosed, models.MessageStateReplied),
	}).Error
}

// MarkRead stamps read_at if unset and moves a new message to open, without
// touching any other column.
func (r *MessageRepository) MarkRead(id uint, at time.Time) error {
	return r.db.Model(&models.Message{}).Where("id = ? AND read_at IS NULL", id).Updates(map[string]any{
		"read_at": at,
		"state": gorm.Expr("CASE WHEN state = ? THEN ? ELSE state END",
			models.MessageStateNew, models.MessageStateOpen),
	}).Error
}

// SetRootMessageID records the Message-ID of a thread's first reply. It only
// writes when none is set, so two concurrent first replies cannot both claim it.
func (r *MessageRepository) SetRootMessageID(id uint, messageID string) error {
	return r.db.Model(&models.Message{}).
		Where("id = ? AND (root_message_id IS NULL OR root_message_id = '')", id).
		Update("root_message_id", messageID).Error
}

// ReplyMessageIDs returns the Message-ID headers of a thread's replies, oldest
// first, skipping replies recorded before Message-IDs were stored.
func (r *MessageRepository) ReplyMessageIDs(messageID uint) ([]string, error) {
	var ids []string
	err := r.db.Model(&models.MessageReply{}).
		Where("message_id = ? AND message_id_header <> ''", messageID).
		Order("created_at ASC, id ASC").
		Pluck("message_id_header", &ids).Error
	return ids, err
}

func (r *MessageRepository) FindByID(id uint) (*models.Message, error) {
	var m models.Message
	if err := r.db.First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *MessageRepository) FindByUUIDForScope(scope ResourceScope, uuid string) (*models.Message, error) {
	var m models.Message
	err := ApplyWorkspaceScope(r.db, scope).
		Preload("Form").
		Preload("AssignedTo").
		Preload("Team").
		Preload("Replies", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC").Preload("Author")
		}).
		Where("uuid = ?", uuid).First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *MessageRepository) FindByThreadToken(token string) (*models.Message, error) {
	var m models.Message
	if err := r.db.Where("thread_token = ?", token).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *MessageRepository) FindByScopeFiltered(scope ResourceScope, f MessageFilterQuery, limit, offset int) ([]models.Message, int64, error) {
	var items []models.Message
	var total int64

	apply := func(q *gorm.DB) *gorm.DB { return applyMessageFilter(q, scope, f) }

	apply(r.db.Model(&models.Message{})).Count(&total)

	if err := apply(r.db).
		Preload("Form").
		Preload("AssignedTo").
		Preload("Team").
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// EachMessage walks the messages f selects, newest first, handing them to fn
// in batches. It pages by id rather than offset, so submissions arriving
// during an export neither repeat nor shift rows. It stops after max messages
// (0 means no limit) and reports whether it stopped early.
func (r *MessageRepository) EachMessage(scope ResourceScope, f MessageFilterQuery, batch, max int, fn func([]models.Message) error) (bool, error) {
	if batch <= 0 {
		batch = 500
	}
	var lastID uint
	seen := 0
	for {
		size := batch
		if max > 0 && max-seen < size {
			size = max - seen
		}
		q := applyMessageFilter(r.db.Model(&models.Message{}), scope, f)
		if lastID > 0 {
			q = q.Where("id < ?", lastID)
		}
		var items []models.Message
		if err := q.Order("id DESC").Limit(size + 1).Find(&items).Error; err != nil {
			return false, err
		}
		more := len(items) > size
		if more {
			items = items[:size]
		}
		if len(items) > 0 {
			if err := fn(items); err != nil {
				return false, err
			}
			seen += len(items)
			lastID = items[len(items)-1].ID
		}
		if !more {
			return false, nil
		}
		if max > 0 && seen >= max {
			return true, nil
		}
	}
}

func (r *MessageRepository) FindRecentDuplicate(formID uint, hash string, since time.Time) (*models.Message, error) {
	var m models.Message
	err := r.db.Where("form_id = ? AND dedup_hash = ? AND created_at >= ?", formID, hash, since).
		Order("created_at DESC").First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *MessageRepository) Delete(id uint) error {
	return r.db.Delete(&models.Message{}, id).Error
}

// notCollectForm keeps messages whose form is in inbox mode.
const notCollectForm = "NOT EXISTS (SELECT 1 FROM forms WHERE forms.id = messages.form_id AND forms.mode = 'collect')"

// CountByScope counts the inbox: total and unread leave out collect-mode
// forms, as the inbox list does; spam counts every form.
func (r *MessageRepository) CountByScope(scope ResourceScope) (total, unread, spam int64) {
	ApplyWorkspaceScope(r.db.Model(&models.Message{}), scope).
		Where("status <> ?", models.MessageStatusRejected).Where(notCollectForm).Count(&total)
	ApplyWorkspaceScope(r.db.Model(&models.Message{}), scope).
		Where("read_at IS NULL AND status IN ?", []models.MessageStatus{models.MessageStatusReceived, models.MessageStatusFlagged}).
		Where(notCollectForm).Count(&unread)
	ApplyWorkspaceScope(r.db.Model(&models.Message{}), scope).
		Where("status IN ?", []models.MessageStatus{models.MessageStatusQuarantined, models.MessageStatusRejected}).Count(&spam)
	return
}

// TagCount is one tag in use and how many messages carry it.
type TagCount struct {
	Tag   string `json:"tag"`
	Count int64  `json:"count"`
}

// RecentTags lists the tags on the newest tagged messages, most used first,
// for suggestions. It reads a bounded window so a large workspace stays cheap.
func (r *MessageRepository) RecentTags(scope ResourceScope, window, limit int) ([]TagCount, error) {
	recent := ApplyWorkspaceScope(r.db.Model(&models.Message{}), scope).
		Select("tags").Where("tags IS NOT NULL AND cardinality(tags) > 0").
		Order("id DESC").Limit(window)
	var out []TagCount
	err := r.db.Raw("SELECT t.tag, COUNT(*) AS count FROM (?) AS recent, unnest(recent.tags) AS t(tag) GROUP BY t.tag ORDER BY count DESC, t.tag ASC LIMIT ?",
		recent, limit).Scan(&out).Error
	return out, err
}

func (r *MessageRepository) CountBySenderSince(formID uint, email string, since time.Time) int64 {
	var count int64
	r.db.Model(&models.Message{}).
		Where("form_id = ? AND LOWER(sender_email) = ? AND created_at >= ?", formID, strings.ToLower(email), since).
		Count(&count)
	return count
}

func (r *MessageRepository) CountByIPSince(ip string, since time.Time) int64 {
	var count int64
	if ip == "" {
		return 0
	}
	r.db.Model(&models.Message{}).
		Where("client_ip = ? AND created_at >= ?", ip, since).
		Count(&count)
	return count
}

func (r *MessageRepository) FindUnnotified(formID uint, since time.Time) ([]models.Message, error) {
	var items []models.Message
	err := r.db.Where("form_id = ? AND notified_at IS NULL AND created_at >= ? AND status IN ?",
		formID, since, []models.MessageStatus{models.MessageStatusReceived, models.MessageStatusFlagged}).
		Order("created_at ASC").Find(&items).Error
	return items, err
}

func (r *MessageRepository) MarkNotified(ids []uint, at time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.Model(&models.Message{}).Where("id IN ?", ids).Update("notified_at", at).Error
}

func (r *MessageRepository) AddReply(reply *models.MessageReply) error {
	return r.db.Create(reply).Error
}

// AddInboundReply stores an inbound reply unless the same inbound email is
// already attached (the unique index on inbound_email_id), so a retried parse
// task cannot add it twice. It reports whether a row was created.
func (r *MessageRepository) AddInboundReply(reply *models.MessageReply) (bool, error) {
	if err := r.db.Create(reply).Error; err != nil {
		if IsDuplicateName(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *MessageRepository) FindReplyByID(id uint) (*models.MessageReply, error) {
	var reply models.MessageReply
	if err := r.db.First(&reply, id).Error; err != nil {
		return nil, err
	}
	return &reply, nil
}

func (r *MessageRepository) ListReplies(messageID uint) ([]models.MessageReply, error) {
	var items []models.MessageReply
	err := r.db.Where("message_id = ?", messageID).Preload("Author").Order("created_at ASC").Find(&items).Error
	return items, err
}

func (r *MessageRepository) DeleteOlderThan(before time.Time) (int64, error) {
	result := r.db.Unscoped().Where("created_at < ?", before).Delete(&models.Message{})
	return result.RowsAffected, result.Error
}

func (r *MessageRepository) DeleteSpamOlderThan(before time.Time) (int64, error) {
	result := r.db.Unscoped().
		Where("created_at < ? AND status IN ?", before,
			[]models.MessageStatus{models.MessageStatusQuarantined, models.MessageStatusRejected}).
		Delete(&models.Message{})
	return result.RowsAffected, result.Error
}

// DeleteByFormOlderThan hard-deletes one form's messages received before the
// cutoff; replies go with them through the ON DELETE CASCADE foreign key.
func (r *MessageRepository) DeleteByFormOlderThan(formID uint, before time.Time) (int64, error) {
	result := r.db.Unscoped().Where("form_id = ? AND created_at < ?", formID, before).Delete(&models.Message{})
	return result.RowsAffected, result.Error
}

func (r *MessageRepository) AttachmentKeysByFormOlderThan(formID uint, before time.Time) ([]string, error) {
	var payloads []string
	err := r.db.Unscoped().Model(&models.Message{}).
		Where("form_id = ? AND created_at < ? AND attachments_json <> ''", formID, before).
		Pluck("attachments_json", &payloads).Error
	return payloads, err
}

func (r *MessageRepository) SpamAttachmentKeysOlderThan(before time.Time) ([]string, error) {
	var payloads []string
	err := r.db.Unscoped().Model(&models.Message{}).
		Where("created_at < ? AND attachments_json <> '' AND status IN ?", before,
			[]models.MessageStatus{models.MessageStatusQuarantined, models.MessageStatusRejected}).
		Pluck("attachments_json", &payloads).Error
	return payloads, err
}

func (r *MessageRepository) AttachmentKeysOlderThan(before time.Time) ([]string, error) {
	var payloads []string
	err := r.db.Unscoped().Model(&models.Message{}).
		Where("created_at < ? AND attachments_json <> ''", before).
		Pluck("attachments_json", &payloads).Error
	return payloads, err
}

func (r *MessageRepository) DailyCounts(scope ResourceScope, since time.Time) ([]MessageDailyCount, error) {
	var rows []MessageDailyCount
	err := ApplyWorkspaceScope(r.db.Model(&models.Message{}), scope).
		Select("DATE(created_at) AS day, COUNT(*) AS total, COUNT(*) FILTER (WHERE status IN ('quarantined','rejected')) AS spam").
		Where("created_at >= ?", since).
		Group("DATE(created_at)").
		Order("day ASC").
		Scan(&rows).Error
	return rows, err
}

type MessageDailyCount struct {
	Day   time.Time `json:"day"`
	Total int64     `json:"total"`
	Spam  int64     `json:"spam"`
}
