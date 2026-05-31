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

package upgrade

import (
	"errors"
	"time"

	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
)

// readVersion returns the stored app version. The fresh flag is true when no
// app.version row exists yet
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

// writeVersion updates the app.version and app.last_started_at settings rows
// in a single transaction. It also creates app.first_started_at on first
// successful boot.
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

// upsertSetting writes a string value, replacing any existing row for the key.
func upsertSetting(tx *gorm.DB, key, value string) error {
	var existing models.Setting
	err := tx.Where("key = ?", key).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return tx.Create(&models.Setting{Key: key, Value: value, Type: "string"}).Error
	}
	if err != nil {
		return err
	}
	existing.Value = value
	existing.Type = "string"
	return tx.Save(&existing).Error
}

// ensureSetting inserts the row only if it does not already exist.
func ensureSetting(tx *gorm.DB, key, value string) error {
	var existing models.Setting
	err := tx.Where("key = ?", key).First(&existing).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return tx.Create(&models.Setting{Key: key, Value: value, Type: "string"}).Error
}
