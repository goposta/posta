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

package upgrade

import (
	"errors"
	"time"

	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
)

// readVersion reports fresh when no app.version row exists yet.
func readVersion(db *gorm.DB) (version string, fresh bool, err error) {
	var s models.Setting
	err = db.Where("key = ?", KeyAppVersion).First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", true, nil
	}
	if err != nil {
		return "", false, err
	}
	return s.Value, false, nil
}

// writeVersion also creates app.first_started_at on the first successful boot.
func writeVersion(db *gorm.DB, version string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return db.Transaction(func(tx *gorm.DB) error {
		if err := upsertSetting(tx, KeyAppVersion, version); err != nil {
			return err
		}
		if err := upsertSetting(tx, KeyAppLastStartedAt, now); err != nil {
			return err
		}
		return ensureSetting(tx, KeyAppFirstStartedAt, now)
	})
}

func upsertSetting(tx *gorm.DB, key, value string) error {
	var existing models.Setting
	err := tx.Where("key = ?", key).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return tx.Create(&models.Setting{Key: key, Value: value, Type: models.SettingTypeString}).Error
	}
	if err != nil {
		return err
	}
	existing.Value = value
	existing.Type = "string"
	return tx.Save(&existing).Error
}

func ensureSetting(tx *gorm.DB, key, value string) error {
	var existing models.Setting
	err := tx.Where("key = ?", key).First(&existing).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return tx.Create(&models.Setting{Key: key, Value: value, Type: models.SettingTypeString}).Error
}
