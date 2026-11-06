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

package inbox

import (
	"fmt"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/storage/repositories"
	"gorm.io/gorm"
)

// Service is separate from services/notification on purpose: a condition worth a banner is rarely worth a mail.
type Service struct {
	db   *gorm.DB
	repo *repositories.NotificationRepository
}

func NewService(db *gorm.DB, repo *repositories.NotificationRepository) *Service {
	return &Service{db: db, repo: repo}
}

// SyncWorkspaceHealth runs on the dashboard read, not a timer, so a banner never lags the numbers beside it.
func (s *Service) SyncWorkspaceHealth(userID uint, workspaceID uint, role models.WorkspaceRole, snap Snapshot) error {
	ws := workspaceID
	firing := evaluate(snap)

	keep := make([]string, 0, len(firing))
	for _, r := range firing {
		if roleRank(role) < roleRank(r.minRole) {
			// Viewers cannot verify a domain or rotate a key, so the alert is noise they cannot clear.
			continue
		}
		keep = append(keep, r.dedupKey)
		n := &models.Notification{
			UserID:      userID,
			WorkspaceID: &ws,
			Kind:        models.NotificationKindAlert,
			Category:    r.category,
			Severity:    r.severity,
			Title:       r.title,
			Body:        r.body,
			Link:        r.link,
			ActionText:  r.action,
			DedupKey:    r.dedupKey,
			Fingerprint: r.fingerprint,
		}
		if err := s.repo.Upsert(n); err != nil {
			return err
		}
	}

	return s.repo.ResolveExcept(userID, &ws, keep)
}

// Announce writes one row per user so read and dismissed state stay per person.
func (s *Service) Announce(a *models.Announcement) error {
	var users []models.User
	if err := s.db.Model(&models.User{}).Select("id").Where("active = ?", true).Find(&users).Error; err != nil {
		return err
	}

	now := time.Now().UTC()
	a.SentAt = &now
	a.Recipients = len(users)
	if err := s.db.Create(a).Error; err != nil {
		return err
	}

	rows := make([]models.Notification, 0, len(users))
	for i := range users {
		rows = append(rows, models.Notification{
			UserID:      users[i].ID,
			Kind:        models.NotificationKindAnnouncement,
			Category:    models.NotificationCategoryPlatform,
			Severity:    a.Severity,
			Title:       a.Title,
			Body:        a.Message,
			Link:        a.Link,
			DedupKey:    AnnouncementKey(a.ID),
			Fingerprint: fmt.Sprintf("announcement=%d", a.ID),
		})
	}
	if err := s.repo.CreateBatch(rows); err != nil {
		return err
	}
	return nil
}

func (s *Service) Retract(id uint) error {
	if _, err := s.repo.DeleteForAnnouncement(AnnouncementKey(id)); err != nil {
		return err
	}
	return s.db.Delete(&models.Announcement{}, id).Error
}

// Deliver lets cron jobs that email a user about a condition leave the same news in the UI.
func (s *Service) Deliver(userID uint, workspaceID *uint, n models.Notification) error {
	n.UserID = userID
	n.WorkspaceID = workspaceID
	if n.Kind == "" {
		n.Kind = models.NotificationKindAlert
	}
	return s.repo.Upsert(&n)
}

// AnnouncementKey is shared by every delivery of one announcement, which is how a retraction finds them.
func AnnouncementKey(id uint) string { return fmt.Sprintf("announcement:%d", id) }
