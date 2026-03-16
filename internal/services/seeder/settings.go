/*
 *  MIT License
 *
 * Copyright (c) 2026 Jonas Kaninda
 *
 *  Permission is hereby granted, free of charge, to any person obtaining a copy
 *  of this software and associated documentation files (the "Software"), to deal
 *  in the Software without restriction, including without limitation the rights
 *  to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 *  copies of the Software, and to permit persons to whom the Software is
 *  furnished to do so, subject to the following conditions:
 *
 *  The above copyright notice and this permission notice shall be included in all
 *  copies or substantial portions of the Software.
 *
 *  THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 *  IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 *  FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 *  AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 *  LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 *  OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 *  SOFTWARE.
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
