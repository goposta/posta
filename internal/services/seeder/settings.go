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

package seeder

import (
	"github.com/jkaninda/logger"
	"github.com/jkaninda/posta/internal/models"
	"github.com/jkaninda/posta/internal/storage/repositories"
)

// SeedDefaultSettings creates default platform settings if they don't already exist.
func SeedDefaultSettings(repo *repositories.SettingRepository) {
	defaults := []models.Setting{
		{Key: "registration_enabled", Value: "false", Type: "bool"},
		{Key: "require_email_verification", Value: "true", Type: "bool"},
		{Key: "require_domain_verification", Value: "true", Type: "bool"},
		{Key: "default_rate_limit_hourly", Value: "100", Type: "int"},
		{Key: "default_rate_limit_daily", Value: "1000", Type: "int"},
		{Key: "max_batch_size", Value: "100", Type: "int"},
		{Key: "max_attachment_size_mb", Value: "10", Type: "int"},
		{Key: "retention_days", Value: "30", Type: "int"},
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
