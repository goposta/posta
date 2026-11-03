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
	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
)

type LDAPRepository struct {
	db *gorm.DB
}

func NewLDAPRepository(db *gorm.DB) *LDAPRepository {
	return &LDAPRepository{db: db}
}

func withPasswordFlag(cfg *models.LDAPConfig) *models.LDAPConfig {
	cfg.BindPasswordSet = cfg.BindPasswordEnc != ""
	return cfg
}

// FindAll lists every directory with its group mappings, oldest first: the
// order sign-in tries them in.
func (r *LDAPRepository) FindAll() ([]models.LDAPConfig, error) {
	var cfgs []models.LDAPConfig
	if err := r.db.Preload("Mappings", func(db *gorm.DB) *gorm.DB { return db.Order("id ASC") }).
		Order("id ASC").Find(&cfgs).Error; err != nil {
		return nil, err
	}
	for i := range cfgs {
		withPasswordFlag(&cfgs[i])
	}
	return cfgs, nil
}

// Enabled lists the directories sign-in tries, in order.
func (r *LDAPRepository) Enabled() ([]models.LDAPConfig, error) {
	var cfgs []models.LDAPConfig
	err := r.db.Where("enabled = ?", true).Order("id ASC").Find(&cfgs).Error
	return cfgs, err
}

func (r *LDAPRepository) FindByID(id uint) (*models.LDAPConfig, error) {
	var cfg models.LDAPConfig
	if err := r.db.Preload("Mappings", func(db *gorm.DB) *gorm.DB { return db.Order("id ASC") }).
		First(&cfg, id).Error; err != nil {
		return nil, err
	}
	return withPasswordFlag(&cfg), nil
}

func (r *LDAPRepository) FindByName(name string) (*models.LDAPConfig, error) {
	var cfg models.LDAPConfig
	if err := r.db.Preload("Mappings").Where("name = ?", name).First(&cfg).Error; err != nil {
		return nil, err
	}
	return withPasswordFlag(&cfg), nil
}

func (r *LDAPRepository) NameTaken(name string, exceptID uint) bool {
	var n int64
	r.db.Model(&models.LDAPConfig{}).Where("name = ? AND id <> ?", name, exceptID).Count(&n)
	return n > 0
}

func (r *LDAPRepository) Create(cfg *models.LDAPConfig) error {
	return r.db.Omit("Mappings").Create(cfg).Error
}

func (r *LDAPRepository) Update(cfg *models.LDAPConfig) error {
	return r.db.Omit("Mappings").Save(cfg).Error
}

// Delete removes a directory and its group mappings.
func (r *LDAPRepository) Delete(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("ldap_config_id = ?", id).Delete(&models.LDAPGroupMapping{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.LDAPConfig{}, id).Error
	})
}

func (r *LDAPRepository) CreateMapping(m *models.LDAPGroupMapping) error {
	return r.db.Create(m).Error
}

// DeleteMapping removes one mapping of a directory; false when there was none.
func (r *LDAPRepository) DeleteMapping(configID, mappingID uint) (bool, error) {
	res := r.db.Where("id = ? AND ldap_config_id = ?", mappingID, configID).Delete(&models.LDAPGroupMapping{})
	return res.RowsAffected > 0, res.Error
}
