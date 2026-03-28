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

package handlers

import (
	"fmt"
	"net/http"

	"github.com/jkaninda/okapi"
	"github.com/goposta/posta/internal/dto"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/audit"
	planpkg "github.com/goposta/posta/internal/services/plan"
	"github.com/goposta/posta/internal/storage/repositories"
	"gorm.io/gorm"
)

// PlanHandler handles admin management of plans/packages.
type PlanHandler struct {
	planRepo      *repositories.PlanRepository
	workspaceRepo *repositories.WorkspaceRepository
	planService   *planpkg.Service
	audit         *audit.Logger
}

func NewPlanHandler(
	planRepo *repositories.PlanRepository,
	workspaceRepo *repositories.WorkspaceRepository,
	planService *planpkg.Service,
	audit *audit.Logger,
) *PlanHandler {
	return &PlanHandler{
		planRepo:      planRepo,
		workspaceRepo: workspaceRepo,
		planService:   planService,
		audit:         audit,
	}
}

// Request types

type CreatePlanRequest struct {
	Body struct {
		Name                  string `json:"name" required:"true"`
		Description           string `json:"description"`
		IsDefault             bool   `json:"is_default"`
		DailyRateLimit        int    `json:"daily_rate_limit"`
		HourlyRateLimit       int    `json:"hourly_rate_limit"`
		MaxAttachmentSizeMB   int    `json:"max_attachment_size_mb"`
		MaxBatchSize          int    `json:"max_batch_size"`
		MaxAPIKeys            int    `json:"max_api_keys"`
		MaxDomains            int    `json:"max_domains"`
		MaxSMTPServers        int    `json:"max_smtp_servers"`
		MaxWorkspaces         int    `json:"max_workspaces"`
		EmailLogRetentionDays int    `json:"email_log_retention_days"`
	} `json:"body"`
}

type UpdatePlanRequest struct {
	ID   int `param:"id"`
	Body struct {
		Name                  *string `json:"name"`
		Description           *string `json:"description"`
		IsDefault             *bool   `json:"is_default"`
		IsActive              *bool   `json:"is_active"`
		DailyRateLimit        *int    `json:"daily_rate_limit"`
		HourlyRateLimit       *int    `json:"hourly_rate_limit"`
		MaxAttachmentSizeMB   *int    `json:"max_attachment_size_mb"`
		MaxBatchSize          *int    `json:"max_batch_size"`
		MaxAPIKeys            *int    `json:"max_api_keys"`
		MaxDomains            *int    `json:"max_domains"`
		MaxSMTPServers        *int    `json:"max_smtp_servers"`
		MaxWorkspaces         *int    `json:"max_workspaces"`
		EmailLogRetentionDays *int    `json:"email_log_retention_days"`
	} `json:"body"`
}

type DeletePlanRequest struct {
	ID    int  `param:"id"`
	Force bool `query:"force"`
}

type PlanIDRequest struct {
	ID int `param:"id"`
}

type AssignWorkspacePlanRequest struct {
	ID   int `param:"id"`
	Body struct {
		PlanID uint `json:"plan_id" required:"true"`
	} `json:"body"`
}

type WorkspacePlanRequest struct {
	ID int `param:"id"`
}

// Create creates a new plan.
func (h *PlanHandler) Create(c *okapi.Context, req *CreatePlanRequest) error {
	userID := uint(c.GetInt("user_id"))

	plan := &models.Plan{
		Name:                  req.Body.Name,
		Description:           req.Body.Description,
		IsDefault:             req.Body.IsDefault,
		IsActive:              true,
		DailyRateLimit:        req.Body.DailyRateLimit,
		HourlyRateLimit:       req.Body.HourlyRateLimit,
		MaxAttachmentSizeMB:   req.Body.MaxAttachmentSizeMB,
		MaxBatchSize:          req.Body.MaxBatchSize,
		MaxAPIKeys:            req.Body.MaxAPIKeys,
		MaxDomains:            req.Body.MaxDomains,
		MaxSMTPServers:        req.Body.MaxSMTPServers,
		MaxWorkspaces:         req.Body.MaxWorkspaces,
		EmailLogRetentionDays: req.Body.EmailLogRetentionDays,
	}

	if plan.IsDefault {
		if err := h.planRepo.ClearDefault(); err != nil {
			return c.AbortInternalServerError("failed to clear existing default plan", err)
		}
	}

	if err := h.planRepo.Create(plan); err != nil {
		return c.AbortWithError(http.StatusConflict, fmt.Errorf("failed to create plan: %w", err))
	}

	h.audit.Log(userID, c.GetString("email"), c.RealIP(), "plan.created", "Plan created: "+plan.Name, nil)

	return created(c, plan)
}

// List returns all plans with pagination.
func (h *PlanHandler) List(c *okapi.Context, req *ListRequest) error {
	page, size, offset := normalizePageParams(req.Page, req.Size)

	plans, total, err := h.planRepo.FindAll(size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list plans")
	}

	return paginated(c, plans, total, page, size)
}

// Get returns a single plan by ID.
func (h *PlanHandler) Get(c *okapi.Context, req *PlanIDRequest) error {
	plan, err := h.planRepo.FindByID(uint(req.ID))
	if err != nil {
		return c.AbortNotFound("plan not found")
	}
	return ok(c, plan)
}

// Update modifies an existing plan.
func (h *PlanHandler) Update(c *okapi.Context, req *UpdatePlanRequest) error {
	userID := uint(c.GetInt("user_id"))

	plan, err := h.planRepo.FindByID(uint(req.ID))
	if err != nil {
		return c.AbortNotFound("plan not found")
	}

	if req.Body.Name != nil {
		plan.Name = *req.Body.Name
	}
	if req.Body.Description != nil {
		plan.Description = *req.Body.Description
	}
	if req.Body.IsActive != nil {
		plan.IsActive = *req.Body.IsActive
	}
	if req.Body.DailyRateLimit != nil {
		plan.DailyRateLimit = *req.Body.DailyRateLimit
	}
	if req.Body.HourlyRateLimit != nil {
		plan.HourlyRateLimit = *req.Body.HourlyRateLimit
	}
	if req.Body.MaxAttachmentSizeMB != nil {
		plan.MaxAttachmentSizeMB = *req.Body.MaxAttachmentSizeMB
	}
	if req.Body.MaxBatchSize != nil {
		plan.MaxBatchSize = *req.Body.MaxBatchSize
	}
	if req.Body.MaxAPIKeys != nil {
		plan.MaxAPIKeys = *req.Body.MaxAPIKeys
	}
	if req.Body.MaxDomains != nil {
		plan.MaxDomains = *req.Body.MaxDomains
	}
	if req.Body.MaxSMTPServers != nil {
		plan.MaxSMTPServers = *req.Body.MaxSMTPServers
	}
	if req.Body.MaxWorkspaces != nil {
		plan.MaxWorkspaces = *req.Body.MaxWorkspaces
	}
	if req.Body.EmailLogRetentionDays != nil {
		plan.EmailLogRetentionDays = *req.Body.EmailLogRetentionDays
	}

	// Handle default flag change
	if req.Body.IsDefault != nil && *req.Body.IsDefault && !plan.IsDefault {
		if err := h.planRepo.ClearDefault(); err != nil {
			return c.AbortInternalServerError("failed to clear existing default plan", err)
		}
		plan.IsDefault = true
	} else if req.Body.IsDefault != nil && !*req.Body.IsDefault {
		plan.IsDefault = false
	}

	if err := h.planRepo.Update(plan); err != nil {
		return c.AbortInternalServerError("failed to update plan", err)
	}

	h.audit.Log(userID, c.GetString("email"), c.RealIP(), "plan.updated", "Plan updated: "+plan.Name, nil)

	return ok(c, plan)
}

// Delete removes a plan. If the plan is assigned to workspaces, it requires ?force=true.
func (h *PlanHandler) Delete(c *okapi.Context, req *DeletePlanRequest) error {
	userID := uint(c.GetInt("user_id"))

	plan, err := h.planRepo.FindByID(uint(req.ID))
	if err != nil {
		return c.AbortNotFound("plan not found")
	}

	count, err := h.planRepo.CountWorkspaces(plan.ID)
	if err != nil {
		return c.AbortInternalServerError("failed to check plan usage", err)
	}

	if count > 0 && !req.Force {
		return c.AbortWithError(http.StatusConflict,
			fmt.Errorf("plan is assigned to %d workspace(s); use ?force=true to delete", count))
	}

	if count > 0 {
		if err := h.planRepo.UnassignAllFromPlan(plan.ID); err != nil {
			return c.AbortInternalServerError("failed to unassign workspaces from plan", err)
		}
	}

	if err := h.planRepo.Delete(plan.ID); err != nil {
		return c.AbortInternalServerError("failed to delete plan", err)
	}

	h.audit.Log(userID, c.GetString("email"), c.RealIP(), "plan.deleted", "Plan deleted: "+plan.Name, nil)

	return noContent(c)
}

// SetDefault sets a plan as the default, unsetting any previous default.
func (h *PlanHandler) SetDefault(c *okapi.Context, req *PlanIDRequest) error {
	userID := uint(c.GetInt("user_id"))

	plan, err := h.planRepo.FindByID(uint(req.ID))
	if err != nil {
		return c.AbortNotFound("plan not found")
	}

	db := h.planRepo.DB()
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := h.planRepo.ClearDefaultTx(tx); err != nil {
			return err
		}
		return tx.Model(plan).Update("is_default", true).Error
	}); err != nil {
		return c.AbortInternalServerError("failed to set default plan", err)
	}

	plan.IsDefault = true

	h.audit.Log(userID, c.GetString("email"), c.RealIP(), "plan.default_set", "Default plan set: "+plan.Name, nil)

	return ok(c, plan)
}

// AssignToWorkspace assigns a plan to a workspace.
func (h *PlanHandler) AssignToWorkspace(c *okapi.Context, req *AssignWorkspacePlanRequest) error {
	userID := uint(c.GetInt("user_id"))

	ws, err := h.workspaceRepo.FindByID(uint(req.ID))
	if err != nil {
		return c.AbortNotFound("workspace not found")
	}

	plan, err := h.planRepo.FindByID(req.Body.PlanID)
	if err != nil {
		return c.AbortNotFound("plan not found")
	}

	if !plan.IsActive {
		return c.AbortWithError(http.StatusBadRequest, fmt.Errorf("cannot assign inactive plan"))
	}

	if err := h.planRepo.AssignToWorkspace(ws.ID, plan.ID); err != nil {
		return c.AbortInternalServerError("failed to assign plan to workspace", err)
	}

	h.audit.Log(userID, c.GetString("email"), c.RealIP(), "plan.assigned",
		fmt.Sprintf("Plan '%s' assigned to workspace '%s'", plan.Name, ws.Name), nil)

	return ok(c, dto.MessageData{Message: fmt.Sprintf("plan '%s' assigned to workspace '%s'", plan.Name, ws.Name)})
}

// GetWorkspacePlan returns the effective plan for a workspace.
func (h *PlanHandler) GetWorkspacePlan(c *okapi.Context, req *WorkspacePlanRequest) error {
	wsID := uint(req.ID)

	_, err := h.workspaceRepo.FindByID(wsID)
	if err != nil {
		return c.AbortNotFound("workspace not found")
	}

	plan := h.planService.EffectivePlan(&wsID)
	if plan == nil {
		return ok(c, okapi.M{"plan": nil, "source": "global_settings"})
	}

	return ok(c, plan)
}
