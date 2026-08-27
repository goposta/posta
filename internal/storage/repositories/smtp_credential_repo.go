// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package repositories

import (
	"time"

	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
)

type SMTPCredentialRepository struct {
	db *gorm.DB
}

func NewSMTPCredentialRepository(db *gorm.DB) *SMTPCredentialRepository {
	return &SMTPCredentialRepository{db: db}
}

func (r *SMTPCredentialRepository) Create(cred *models.SMTPCredential) error {
	return r.db.Create(cred).Error
}

func (r *SMTPCredentialRepository) FindByUsername(username string) (*models.SMTPCredential, error) {
	var cred models.SMTPCredential
	if err := r.db.Where("username = ?", username).First(&cred).Error; err != nil {
		return nil, err
	}
	return &cred, nil
}

func (r *SMTPCredentialRepository) FindByWorkspaceID(workspaceID uint, limit, offset int) ([]models.SMTPCredential, int64, error) {
	var creds []models.SMTPCredential
	var total int64

	r.db.Model(&models.SMTPCredential{}).Where("workspace_id = ?", workspaceID).Count(&total)

	if err := r.db.Where("workspace_id = ?", workspaceID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&creds).Error; err != nil {
		return nil, 0, err
	}
	return creds, total, nil
}

func (r *SMTPCredentialRepository) FindByID(id uint) (*models.SMTPCredential, error) {
	var cred models.SMTPCredential
	if err := r.db.First(&cred, id).Error; err != nil {
		return nil, err
	}
	return &cred, nil
}

func (r *SMTPCredentialRepository) UpdateLastUsed(id uint) error {
	now := time.Now()
	return r.db.Model(&models.SMTPCredential{}).Where("id = ?", id).Update("last_used_at", now).Error
}

func (r *SMTPCredentialRepository) Revoke(id uint) error {
	return r.db.Model(&models.SMTPCredential{}).Where("id = ?", id).Update("revoked", true).Error
}

func (r *SMTPCredentialRepository) Delete(id uint) error {
	return r.db.Delete(&models.SMTPCredential{}, id).Error
}
