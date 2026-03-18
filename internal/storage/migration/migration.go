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

package migration

import (
	"fmt"

	"github.com/jkaninda/logger"
	"github.com/jkaninda/posta/internal/models"
	"gorm.io/gorm"
)

// Run executes all schema migrations and adds FK constraints.
func Run(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&models.User{},
		&models.APIKey{},
		&models.Email{},
		&models.StyleSheet{},
		&models.Template{},
		&models.TemplateVersion{},
		&models.TemplateLocalization{},
		&models.Language{},
		&models.SMTPServer{},
		&models.Server{},
		&models.Webhook{},
		&models.Domain{},
		&models.Bounce{},
		&models.Suppression{},
		&models.Contact{},
		&models.ContactList{},
		&models.ContactListMember{},
		&models.Event{},
		&models.Setting{},
		&models.UserSetting{},
		&models.WebhookDelivery{},
		&models.Session{},
	); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}
	// run constraints
	runConstraints(db)

	logger.Info("database migrated")
	return nil
}
