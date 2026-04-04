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

package plan

import (
	"errors"
	"fmt"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/settings"
	"github.com/goposta/posta/internal/storage/repositories"
	"gorm.io/gorm"
)

// Resource type constants for quota enforcement.
const (
	ResourceAPIKey     = "api_keys"
	ResourceDomain     = "domains"
	ResourceSMTPServer = "smtp_servers"
	ResourceWorkspace  = "workspaces"
)

// Limits holds the effective limits for a given scope.
// A value of 0 means unlimited.
type Limits struct {
	HourlyRateLimit       int
	DailyRateLimit        int
	MaxAttachmentSizeMB   int
	MaxBatchSize          int
	MaxAPIKeys            int
	MaxDomains            int
	MaxSMTPServers        int
	MaxWorkspaces         int
	EmailLogRetentionDays int
}

// Service resolves effective plan limits and enforces resource quotas.
type Service struct {
	planRepo *repositories.PlanRepository
	settings *settings.Provider
}

// NewService creates a new plan service.
func NewService(planRepo *repositories.PlanRepository, settings *settings.Provider) *Service {
	return &Service{
		planRepo: planRepo,
		settings: settings,
	}
}

// EffectiveLimits resolves the limits for a given workspace.
// Priority: workspace plan -> default plan -> global settings.
func (s *Service) EffectiveLimits(workspaceID *uint) *Limits {
	if workspaceID != nil {
		plan, err := s.planRepo.FindByWorkspaceID(*workspaceID)
		if err == nil && plan.IsActive {
			return limitsFromPlan(plan)
		}
	}

	// Try the default plan.
	plan, err := s.planRepo.FindDefault()
	if err == nil {
		return limitsFromPlan(plan)
	}

	// Fallback to global settings.
	return s.limitsFromSettings()
}

// EffectivePlan returns the plan assigned to the workspace, the default plan,
// or nil if no plan applies.
func (s *Service) EffectivePlan(workspaceID *uint) *models.Plan {
	if workspaceID != nil {
		plan, err := s.planRepo.FindByWorkspaceID(*workspaceID)
		if err == nil && plan.IsActive {
			return plan
		}
	}
	plan, err := s.planRepo.FindDefault()
	if err == nil {
		return plan
	}
	return nil
}

// CheckQuota verifies that creating one more resource of the given type
// would not exceed the plan limit. Returns nil if within limit.
func (s *Service) CheckQuota(db *gorm.DB, userID uint, workspaceID *uint, resource string) error {
	limits := s.EffectiveLimits(workspaceID)

	var limit int
	var table string

	switch resource {
	case ResourceAPIKey:
		limit = limits.MaxAPIKeys
		table = "api_keys"
	case ResourceDomain:
		limit = limits.MaxDomains
		table = "domains"
	case ResourceSMTPServer:
		limit = limits.MaxSMTPServers
		table = "smtp_servers"
	default:
		return nil
	}

	if limit == 0 {
		return nil // unlimited
	}

	var count int64
	query := db.Table(table)
	if workspaceID != nil {
		query = query.Where("workspace_id = ?", *workspaceID)
	} else {
		query = query.Where("user_id = ? AND workspace_id IS NULL", userID)
	}
	if err := query.Count(&count).Error; err != nil {
		return fmt.Errorf("failed to count %s: %w", resource, err)
	}

	if count >= int64(limit) {
		return fmt.Errorf("plan limit reached: maximum %d %s allowed", limit, resource)
	}
	return nil
}

// CheckWorkspaceQuota verifies that the user has not exceeded their max_workspaces limit.
// This counts all workspaces owned by the user.
func (s *Service) CheckWorkspaceQuota(db *gorm.DB, userID uint) error {
	// For workspace quota, we use the default plan since the user may not
	// have a workspace yet. If they have one, use that workspace's plan.
	var limits *Limits

	// Check if user owns any workspace with a plan.
	var ws models.Workspace
	err := db.Where("owner_id = ? AND plan_id IS NOT NULL", userID).First(&ws).Error
	if err == nil {
		limits = s.EffectiveLimits(ws.PlanID)
	} else {
		limits = s.EffectiveLimits(nil)
	}

	if limits.MaxWorkspaces == 0 {
		return nil // unlimited
	}

	var count int64
	if err := db.Model(&models.Workspace{}).Where("owner_id = ?", userID).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to count workspaces: %w", err)
	}

	if count >= int64(limits.MaxWorkspaces) {
		return fmt.Errorf("plan limit reached: maximum %d workspaces allowed", limits.MaxWorkspaces)
	}
	return nil
}

// IsWithinLimit returns true if current < limit, or limit is 0 (unlimited).
func IsWithinLimit(current int64, limit int) bool {
	return limit == 0 || current < int64(limit)
}

func limitsFromPlan(plan *models.Plan) *Limits {
	return &Limits{
		HourlyRateLimit:       plan.HourlyRateLimit,
		DailyRateLimit:        plan.DailyRateLimit,
		MaxAttachmentSizeMB:   plan.MaxAttachmentSizeMB,
		MaxBatchSize:          plan.MaxBatchSize,
		MaxAPIKeys:            plan.MaxAPIKeys,
		MaxDomains:            plan.MaxDomains,
		MaxSMTPServers:        plan.MaxSMTPServers,
		MaxWorkspaces:         plan.MaxWorkspaces,
		EmailLogRetentionDays: plan.EmailLogRetentionDays,
	}
}

func (s *Service) limitsFromSettings() *Limits {
	return &Limits{
		HourlyRateLimit:       s.settings.DefaultRateLimitHourly(),
		DailyRateLimit:        s.settings.DefaultRateLimitDaily(),
		MaxAttachmentSizeMB:   s.settings.MaxAttachmentSizeMB(),
		MaxBatchSize:          s.settings.MaxBatchSize(),
		MaxAPIKeys:            0, // no global equivalent, unlimited
		MaxDomains:            0,
		MaxSMTPServers:        0,
		MaxWorkspaces:         0,
		EmailLogRetentionDays: s.settings.RetentionDays(),
	}
}

// ErrPlanLimitReached is returned when a resource quota is exceeded.
var ErrPlanLimitReached = errors.New("plan limit reached")
