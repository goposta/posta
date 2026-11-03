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

package migration

import (
	"fmt"

	"github.com/goposta/posta/internal/models"
	"github.com/jkaninda/logger"
	"gorm.io/gorm"
)

// Run executes all schema migrations and adds FK constraints.
func Run(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&models.User{},
		&models.Plan{},
		&models.Workspace{},
		&models.WorkspaceMember{},
		&models.WorkspaceInvitation{},
		&models.OAuthProvider{},
		&models.OAuthAccount{},
		&models.WorkspaceSSOConfig{},
		&models.APIKey{},
		&models.SMTPCredential{},
		&models.Email{},
		&models.InboundEmail{},
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
		&models.UnsubscribeList{},
		&models.Contact{},
		&models.ContactList{},
		&models.ContactListMember{},
		&models.Event{},
		&models.Setting{},
		&models.UserSetting{},
		&models.WorkspaceSetting{},
		&models.WebhookDelivery{},
		&models.Session{},
		&models.UserEmailVerification{},
		&models.PasswordResetToken{},
		&models.Subscriber{},
		&models.SubscriberList{},
		&models.SubscriberListMember{},
		&models.SubscriberListUnsubscribe{},
		&models.Campaign{},
		&models.CampaignMessage{},
		&models.TrackedLink{},
		&models.TrackingEvent{},
		&models.Form{},
		&models.Message{},
		&models.MessageReply{},
		&models.MessageFilter{},
		&models.Team{},
		&models.TeamMember{},
		&models.FormRule{},
		&models.Notification{},
		&models.Announcement{},
		&models.UpgradeStep{},
		&models.UpdateStatus{},
		&models.License{},
		&models.LDAPConfig{},
		&models.LDAPGroupMapping{},
	); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	// run constraints
	runConstraints(db)

	logger.Info("database migrated")
	return nil
}
