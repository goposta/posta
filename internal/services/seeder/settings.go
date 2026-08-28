// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package seeder

import (
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/logger"
)

// SeedDefaultSettings creates default platform settings if they don't already exist.
func SeedDefaultSettings(repo *repositories.SettingRepository) {
	// Content-retention windows default to the record retention so that an upgrade
	// never starts scrubbing content earlier than emails were already kept.
	retentionDays := "30"
	if s, err := repo.FindByKey("retention_days"); err == nil && s.Value != "" {
		retentionDays = s.Value
	}

	defaults := []models.Setting{
		{Key: "registration_enabled", Value: models.SettingFalse, Type: models.SettingTypeBool},
		{Key: "require_email_verification", Value: models.SettingTrue, Type: models.SettingTypeBool},
		{Key: "require_domain_verification", Value: models.SettingTrue, Type: models.SettingTypeBool},
		{Key: "default_rate_limit_hourly", Value: "100", Type: models.SettingTypeInt},
		{Key: "default_rate_limit_daily", Value: "1000", Type: models.SettingTypeInt},
		{Key: "max_batch_size", Value: "100", Type: models.SettingTypeInt},
		{Key: "max_attachment_size_mb", Value: "10", Type: models.SettingTypeInt},
		{Key: "retention_days", Value: "30", Type: models.SettingTypeInt},
		{Key: "email_body_retention_days", Value: retentionDays, Type: models.SettingTypeInt},
		{Key: "email_attachment_retention_days", Value: retentionDays, Type: models.SettingTypeInt},
		{Key: "global_bounce_threshold", Value: "5", Type: models.SettingTypeInt},
		{Key: "smtp_timeout_seconds", Value: "30", Type: models.SettingTypeInt},
		{Key: "maintenance_mode", Value: models.SettingFalse, Type: models.SettingTypeBool},
		{Key: "allowed_signup_domains", Value: "", Type: models.SettingTypeString},
		{Key: "two_factor_required", Value: models.SettingFalse, Type: models.SettingTypeBool},
		{Key: "login_rate_limit_count", Value: "10", Type: models.SettingTypeInt},
		{Key: "login_rate_limit_window_minutes", Value: "15", Type: models.SettingTypeInt},
		{Key: "audit_log_retention_days", Value: "90", Type: models.SettingTypeInt},
		{Key: "webhook_delivery_retention_days", Value: "30", Type: models.SettingTypeInt},
		{Key: "email_content_visibility", Value: models.SettingFalse, Type: models.SettingTypeBool},
		{Key: "custom_headers_enabled", Value: models.SettingFalse, Type: models.SettingTypeBool},
		{Key: "password_reset_enabled", Value: models.SettingFalse, Type: models.SettingTypeBool},
	}

	for i := range defaults {
		if _, err := repo.FindByKey(defaults[i].Key); err != nil {
			if err := repo.Upsert(&defaults[i]); err != nil {
				logger.Error("failed to seed setting", "key", defaults[i].Key, "error", err)
			}
		}
	}
	logger.Info("default platform settings seeded")
}
