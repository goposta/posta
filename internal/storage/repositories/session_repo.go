/*
 *  MIT License
 *
 * Copyright (c) 2026 Jonas Kaninda
 *
 *  Permission is hereby granted, free of charge, to any person obtaining a copy
 *  of this software and associated documentation files (the "Software"), to deal
 *  in the Software without restriction, including without limitation the rights
 *  to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 *  copies of the Software, and to permit persons to whom the Software is
 *  furnished to do so, subject to the following conditions:
 *
 *  The above copyright notice and this permission notice shall be included in all
 *  copies or substantial portions of the Software.
 *
 *  THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 *  IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 *  FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 *  AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 *  LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 *  OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 *  SOFTWARE.
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
