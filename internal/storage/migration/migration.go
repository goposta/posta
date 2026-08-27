// SPDX-FileCopyrightText: 2026 Jonas Kaninda
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
		&models.UpgradeStep{},
		&models.UpdateStatus{},
	); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	// run constraints
	runConstraints(db)

	logger.Info("database migrated")
	return nil
}
