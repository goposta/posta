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

package repositories

import (
	"time"

	"github.com/jkaninda/posta/internal/models"
	"gorm.io/gorm"
)

type SessionRepository struct {
	db *gorm.DB
}

func NewSessionRepository(db *gorm.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

func (r *SessionRepository) Create(session *models.Session) error {
	return r.db.Create(session).Error
}

// FindActiveByUserID returns non-revoked, non-expired sessions for a user.
func (r *SessionRepository) FindActiveByUserID(userID uint) ([]models.Session, error) {
	var sessions []models.Session
	if err := r.db.Where("user_id = ? AND revoked = false AND expires_at > ?", userID, time.Now()).
		Order("created_at DESC").
		Find(&sessions).Error; err != nil {
		return nil, err
	}
	return sessions, nil
}

func (r *SessionRepository) FindByID(id uint) (*models.Session, error) {
	var session models.Session
	if err := r.db.First(&session, id).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *SessionRepository) FindByJTI(jti string) (*models.Session, error) {
	var session models.Session
	if err := r.db.Where("jti = ?", jti).First(&session).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

// Revoke marks a session as revoked.
func (r *SessionRepository) Revoke(id uint) error {
	return r.db.Model(&models.Session{}).Where("id = ?", id).Update("revoked", true).Error
}

// RevokeByJTI marks a session as revoked by JTI.
func (r *SessionRepository) RevokeByJTI(jti string) error {
	return r.db.Model(&models.Session{}).Where("jti = ?", jti).Update("revoked", true).Error
}

// RevokeAllByUserID revokes all active sessions for a user.
func (r *SessionRepository) RevokeAllByUserID(userID uint) (int64, error) {
	result := r.db.Model(&models.Session{}).
		Where("user_id = ? AND revoked = false AND expires_at > ?", userID, time.Now()).
		Update("revoked", true)
	return result.RowsAffected, result.Error
}

// RevokeOthersByUserID revokes all active sessions except the given JTI.
func (r *SessionRepository) RevokeOthersByUserID(userID uint, exceptJTI string) (int64, error) {
	result := r.db.Model(&models.Session{}).
		Where("user_id = ? AND jti != ? AND revoked = false AND expires_at > ?", userID, exceptJTI, time.Now()).
		Update("revoked", true)
	return result.RowsAffected, result.Error
}

// CleanExpired removes expired sessions from the database.
func (r *SessionRepository) CleanExpired() (int64, error) {
	result := r.db.Where("expires_at < ?", time.Now()).Delete(&models.Session{})
	return result.RowsAffected, result.Error
}

// IsRevoked checks if a session with the given JTI is revoked.
func (r *SessionRepository) IsRevoked(jti string) (bool, error) {
	var count int64
	err := r.db.Model(&models.Session{}).
		Where("jti = ? AND revoked = true", jti).
		Count(&count).Error
	return count > 0, err
}
