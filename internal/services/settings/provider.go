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

package settings

import (
	"strconv"
	"sync"
	"time"

	"github.com/jkaninda/logger"
	"github.com/jkaninda/posta/internal/storage/repositories"
)

// Provider gives live access to platform settings with a short cache.
// It reads from the database and caches values in memory, refreshing
// periodically so that admin changes take effect without a restart.
type Provider struct {
	repo     *repositories.SettingRepository
	mu       sync.RWMutex
	cache    map[string]string
	loadedAt time.Time
	cacheTTL time.Duration
}

// NewProvider creates a settings provider with a default 30-second cache TTL.
func NewProvider(repo *repositories.SettingRepository) *Provider {
	p := &Provider{
		repo:     repo,
		cache:    make(map[string]string),
		cacheTTL: 30 * time.Second,
	}
	p.refresh()
	return p
}

func (p *Provider) refresh() {
	settings, err := p.repo.FindAll()
	if err != nil {
		logger.Error("settings provider: failed to refresh", "error", err)
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cache = make(map[string]string, len(settings))
	for _, s := range settings {
		p.cache[s.Key] = s.Value
	}
	p.loadedAt = time.Now()
}

func (p *Provider) ensureFresh() {
	p.mu.RLock()
	stale := time.Since(p.loadedAt) > p.cacheTTL
	p.mu.RUnlock()
	if stale {
		p.refresh()
	}
}

// GetString returns a string setting or the fallback if not found.
func (p *Provider) GetString(key, fallback string) string {
	p.ensureFresh()
	p.mu.RLock()
	defer p.mu.RUnlock()
	if v, ok := p.cache[key]; ok {
		return v
	}
	return fallback
}

// GetInt returns an integer setting or the fallback if not found or invalid.
func (p *Provider) GetInt(key string, fallback int) int {
	s := p.GetString(key, "")
	if s == "" {
		return fallback
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return v
}

// GetBool returns a boolean setting or the fallback if not found.
func (p *Provider) GetBool(key string, fallback bool) bool {
	s := p.GetString(key, "")
	if s == "" {
		return fallback
	}
	v, err := strconv.ParseBool(s)
	if err != nil {
		return fallback
	}
	return v
}

// Convenience accessors for commonly used settings.

func (p *Provider) MaintenanceMode() bool     { return p.GetBool("maintenance_mode", false) }
func (p *Provider) RegistrationEnabled() bool { return p.GetBool("registration_enabled", false) }
func (p *Provider) RequireEmailVerification() bool {
	return p.GetBool("require_email_verification", true)
}
func (p *Provider) RequireDomainVerification() bool {
	return p.GetBool("require_domain_verification", true)
}
func (p *Provider) TwoFactorRequired() bool     { return p.GetBool("two_factor_required", false) }
func (p *Provider) DefaultRateLimitHourly() int { return p.GetInt("default_rate_limit_hourly", 100) }
func (p *Provider) DefaultRateLimitDaily() int  { return p.GetInt("default_rate_limit_daily", 1000) }
func (p *Provider) MaxBatchSize() int           { return p.GetInt("max_batch_size", 100) }
func (p *Provider) MaxAttachmentSizeMB() int    { return p.GetInt("max_attachment_size_mb", 10) }
func (p *Provider) GlobalBounceThreshold() int  { return p.GetInt("global_bounce_threshold", 5) }
func (p *Provider) SMTPTimeoutSeconds() int     { return p.GetInt("smtp_timeout_seconds", 30) }
func (p *Provider) RetentionDays() int          { return p.GetInt("retention_days", 30) }
func (p *Provider) AuditLogRetentionDays() int  { return p.GetInt("audit_log_retention_days", 90) }
func (p *Provider) WebhookDeliveryRetentionDays() int {
	return p.GetInt("webhook_delivery_retention_days", 30)
}
func (p *Provider) LoginRateLimitCount() int {
	return p.GetInt("login_rate_limit_count", 10)
}
func (p *Provider) LoginRateLimitWindowMinutes() int {
	return p.GetInt("login_rate_limit_window_minutes", 15)
}
func (p *Provider) EmailContentVisibility() bool {
	return p.GetBool("email_content_visibility", false)
}
