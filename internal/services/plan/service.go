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

package plan

import (
	"errors"
	"fmt"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/settings"
	"github.com/goposta/posta/internal/storage/repositories"
	"gorm.io/gorm"
)

const (
	ResourceAPIKey     = "api_keys"
	ResourceDomain     = "domains"
	ResourceSMTPServer = "smtp_servers"
	ResourceWorkspace  = "workspaces"
)

// Limits holds the effective limits for a scope; a value of 0 means unlimited.
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

type Service struct {
	planRepo *repositories.PlanRepository
	settings *settings.Provider
}

func NewService(planRepo *repositories.PlanRepository, settings *settings.Provider) *Service {
	return &Service{
		planRepo: planRepo,
		settings: settings,
	}
}

// EffectiveLimits resolves the workspace plan, then the default plan, then global settings.
func (s *Service) EffectiveLimits(workspaceID *uint) *Limits {
	if workspaceID != nil {
		plan, err := s.planRepo.FindByWorkspaceID(*workspaceID)
		if err == nil && plan.IsActive {
			return limitsFromPlan(plan)
		}
	}

	plan, err := s.planRepo.FindDefault()
	if err == nil {
		return limitsFromPlan(plan)
	}

	return s.limitsFromSettings()
}

// EffectivePlan returns the workspace's plan, else the default plan, else nil.
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

// EffectiveUserPlan returns the user's plan, else the default plan, else nil.
func (s *Service) EffectiveUserPlan(userID uint) *models.Plan {
	plan, err := s.planRepo.FindByUserID(userID)
	if err == nil && plan.IsActive {
		return plan
	}
	plan, err = s.planRepo.FindDefault()
	if err == nil {
		return plan
	}
	return nil
}

// EffectiveLimitsForUser resolves the user plan, then the default plan, then global settings.
func (s *Service) EffectiveLimitsForUser(userID uint) *Limits {
	plan, err := s.planRepo.FindByUserID(userID)
	if err == nil && plan.IsActive {
		return limitsFromPlan(plan)
	}

	plan, err = s.planRepo.FindDefault()
	if err == nil {
		return limitsFromPlan(plan)
	}

	return s.limitsFromSettings()
}

// CheckQuota fails when one more resource of the given type would exceed the plan limit.
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

// CheckWorkspaceQuota counts every workspace the user owns against the user's plan.
func (s *Service) CheckWorkspaceQuota(db *gorm.DB, userID uint) error {
	limits := s.EffectiveLimitsForUser(userID)

	if limits.MaxWorkspaces == 0 {
		return nil // unlimited
	}

	// The system workspace is platform infrastructure, so it never counts against the admin who owns it.
	var count int64
	if err := db.Model(&models.Workspace{}).
		Where("owner_id = ? AND system = ?", userID, false).
		Count(&count).Error; err != nil {
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

var ErrPlanLimitReached = errors.New("plan limit reached")
