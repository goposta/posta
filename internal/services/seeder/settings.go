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
		{Key: "registration_enabled", Value: "false", Type: "bool"},
		{Key: "require_email_verification", Value: "true", Type: "bool"},
		{Key: "require_domain_verification", Value: "true", Type: "bool"},
		{Key: "default_rate_limit_hourly", Value: "100", Type: "int"},
		{Key: "default_rate_limit_daily", Value: "1000", Type: "int"},
		{Key: "max_batch_size", Value: "100", Type: "int"},
		{Key: "max_attachment_size_mb", Value: "10", Type: "int"},
		{Key: "retention_days", Value: "30", Type: "int"},
		{Key: "email_body_retention_days", Value: retentionDays, Type: "int"},
		{Key: "email_attachment_retention_days", Value: retentionDays, Type: "int"},
		{Key: "global_bounce_threshold", Value: "5", Type: "int"},
		{Key: "smtp_timeout_seconds", Value: "30", Type: "int"},
		{Key: "maintenance_mode", Value: "false", Type: "bool"},
		{Key: "allowed_signup_domains", Value: "", Type: "string"},
		{Key: "two_factor_required", Value: "false", Type: "bool"},
		{Key: "login_rate_limit_count", Value: "10", Type: "int"},
		{Key: "login_rate_limit_window_minutes", Value: "15", Type: "int"},
		{Key: "audit_log_retention_days", Value: "90", Type: "int"},
		{Key: "webhook_delivery_retention_days", Value: "30", Type: "int"},
		{Key: "email_content_visibility", Value: "false", Type: "bool"},
		{Key: "custom_headers_enabled", Value: "false", Type: "bool"},
		{Key: "password_reset_enabled", Value: "false", Type: "bool"},
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
