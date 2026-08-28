// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package repositories

import (
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
)

type MessageFilterQuery struct {
	FormID   uint
	Status   string
	State    string
	Assigned *uint
	Unread   bool
	Query    string
	From     *time.Time
	To       *time.Time
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

func (r *MessageRepository) Update(m *models.Message) error {
	return r.db.Save(m).Error
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

	apply := func(q *gorm.DB) *gorm.DB {
		q = ApplyWorkspaceScope(q, scope)
		if f.FormID > 0 {
			q = q.Where("form_id = ?", f.FormID)
		}
		if f.Status != "" {
			q = q.Where("status = ?", f.Status)
		} else {
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
		if f.Query != "" {
			like := "%" + strings.ToLower(f.Query) + "%"
			q = q.Where(
				"LOWER(subject) LIKE ? OR LOWER(body) LIKE ? OR LOWER(sender_email) LIKE ? OR LOWER(sender_name) LIKE ? OR sender_phone LIKE ?",
				like, like, like, like, like)
		}
		if f.From != nil {
			q = q.Where("created_at >= ?", *f.From)
		}
		if f.To != nil {
			q = q.Where("created_at <= ?", *f.To)
		}
		return q
	}

	apply(r.db.Model(&models.Message{})).Count(&total)

	if err := apply(r.db).
		Preload("Form").
		Preload("AssignedTo").
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
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

func (r *MessageRepository) CountByScope(scope ResourceScope) (total, unread, spam int64) {
	ApplyWorkspaceScope(r.db.Model(&models.Message{}), scope).
		Where("status <> ?", models.MessageStatusRejected).Count(&total)
	ApplyWorkspaceScope(r.db.Model(&models.Message{}), scope).
		Where("read_at IS NULL AND status IN ?", []models.MessageStatus{models.MessageStatusReceived, models.MessageStatusFlagged}).Count(&unread)
	ApplyWorkspaceScope(r.db.Model(&models.Message{}), scope).
		Where("status IN ?", []models.MessageStatus{models.MessageStatusQuarantined, models.MessageStatusRejected}).Count(&spam)
	return
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
