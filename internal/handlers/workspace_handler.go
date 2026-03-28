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
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jkaninda/okapi"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/storage/repositories"
	"gorm.io/gorm"
)

type WorkspaceHandler struct {
	workspaceRepo *repositories.WorkspaceRepository
	userRepo      *repositories.UserRepository
	db            *gorm.DB
	planService   planService
}

// planService is an optional interface for resolving workspace plans and quotas.
type planService interface {
	EffectivePlan(workspaceID *uint) *models.Plan
	CheckWorkspaceQuota(db *gorm.DB, userID uint) error
}

func NewWorkspaceHandler(workspaceRepo *repositories.WorkspaceRepository, userRepo *repositories.UserRepository, db *gorm.DB) *WorkspaceHandler {
	return &WorkspaceHandler{
		workspaceRepo: workspaceRepo,
		userRepo:      userRepo,
		db:            db,
	}
}


type CreateWorkspaceRequest struct {
	Body struct {
		Name            string `json:"name" required:"true" minLength:"1"`
		Slug            string `json:"slug"`
		Description     string `json:"description"`
		DefaultLanguage string `json:"default_language"`
	} `json:"body"`
}

type UpdateWorkspaceRequest struct {
	Body struct {
		Name            string `json:"name"`
		Description     string `json:"description"`
		DefaultLanguage string `json:"default_language"`
	} `json:"body"`
}

type InviteMemberRequest struct {
	Body struct {
		Email string              `json:"email" required:"true" format:"email"`
		Role  models.WorkspaceRole `json:"role" required:"true"`
	} `json:"body"`
}

type UpdateMemberRoleRequest struct {
	MemberID int `param:"member_id"`
	Body     struct {
		Role models.WorkspaceRole `json:"role" required:"true"`
	} `json:"body"`
}

type RemoveWorkspaceMemberRequest struct {
	MemberID int `param:"member_id"`
}

type AcceptInvitationRequest struct {
	Body struct {
		Token string `json:"token" required:"true"`
	} `json:"body"`
}

type DeclineInvitationRequest struct {
	Body struct {
		Token string `json:"token" required:"true"`
	} `json:"body"`
}

type WorkspaceResponse struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	OwnerID     uint      `json:"owner_id"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
}

type WorkspaceMemberResponse struct {
	ID        uint      `json:"id"`
	UserID    uint      `json:"user_id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

type InvitationResponse struct {
	ID          uint      `json:"id"`
	WorkspaceID uint      `json:"workspace_id"`
	Workspace   string    `json:"workspace"`
	Email       string    `json:"email"`
	Role        string    `json:"role"`
	Status      string    `json:"status"`
	ExpiresAt   time.Time `json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
}

func (h *WorkspaceHandler) Create(c *okapi.Context, req *CreateWorkspaceRequest) error {
	userID := c.GetInt("user_id")

	if h.planService != nil {
		if err := h.planService.CheckWorkspaceQuota(h.db, uint(userID)); err != nil {
			return c.AbortForbidden(err.Error())
		}
	}

	slug := req.Body.Slug
	if slug == "" {
		slug = slugify(req.Body.Name)
	}

	// Validate slug format
	if !isValidSlug(slug) {
		return c.AbortBadRequest("slug must contain only lowercase letters, numbers, and hyphens")
	}

	// Check slug uniqueness
	if _, err := h.workspaceRepo.FindBySlug(slug); err == nil {
		return c.AbortConflict("workspace slug already exists")
	}

	defaultLang := strings.TrimSpace(req.Body.DefaultLanguage)
	if defaultLang == "" {
		defaultLang = "en"
	}

	ws := &models.Workspace{
		Name:            strings.TrimSpace(req.Body.Name),
		Slug:            slug,
		Description:     req.Body.Description,
		DefaultLanguage: defaultLang,
		OwnerID:         uint(userID),
	}

	if err := h.workspaceRepo.Create(ws); err != nil {
		return c.AbortInternalServerError("failed to create workspace")
	}

	// Add creator as owner
	member := &models.WorkspaceMember{
		WorkspaceID: ws.ID,
		UserID:      uint(userID),
		Role:        models.WorkspaceRoleOwner,
	}
	if err := h.workspaceRepo.AddMember(member); err != nil {
		return c.AbortInternalServerError("failed to add workspace member")
	}

	return created(c, WorkspaceResponse{
		ID:          ws.ID,
		Name:        ws.Name,
		Slug:        ws.Slug,
		Description: ws.Description,
		OwnerID:     ws.OwnerID,
		Role:        string(models.WorkspaceRoleOwner),
		CreatedAt:   ws.CreatedAt,
	})
}

func (h *WorkspaceHandler) List(c *okapi.Context) error {
	userID := c.GetInt("user_id")

	workspaces, err := h.workspaceRepo.FindByUserID(uint(userID))
	if err != nil {
		return c.AbortInternalServerError("failed to list workspaces")
	}

	var result []WorkspaceResponse
	for _, ws := range workspaces {
		member, _ := h.workspaceRepo.FindMember(ws.ID, uint(userID))
		role := ""
		if member != nil {
			role = string(member.Role)
		}
		result = append(result, WorkspaceResponse{
			ID:          ws.ID,
			Name:        ws.Name,
			Slug:        ws.Slug,
			Description: ws.Description,
			OwnerID:     ws.OwnerID,
			Role:        role,
			CreatedAt:   ws.CreatedAt,
		})
	}

	return ok(c, result)
}

func (h *WorkspaceHandler) Get(c *okapi.Context) error {
	wsID := c.GetInt("workspace_id")

	ws, err := h.workspaceRepo.FindByID(uint(wsID))
	if err != nil {
		return c.AbortNotFound("workspace not found")
	}

	role := c.GetString("workspace_role")

	return ok(c, WorkspaceResponse{
		ID:          ws.ID,
		Name:        ws.Name,
		Slug:        ws.Slug,
		Description: ws.Description,
		OwnerID:     ws.OwnerID,
		Role:        role,
		CreatedAt:   ws.CreatedAt,
	})
}

func (h *WorkspaceHandler) Update(c *okapi.Context, req *UpdateWorkspaceRequest) error {
	wsID := c.GetInt("workspace_id")

	ws, err := h.workspaceRepo.FindByID(uint(wsID))
	if err != nil {
		return c.AbortNotFound("workspace not found")
	}

	if req.Body.Name != "" {
		ws.Name = strings.TrimSpace(req.Body.Name)
	}
	if req.Body.Description != "" {
		ws.Description = req.Body.Description
	}
	if req.Body.DefaultLanguage != "" {
		ws.DefaultLanguage = req.Body.DefaultLanguage
	}
	ws.UpdatedAt = time.Now()

	if err := h.workspaceRepo.Update(ws); err != nil {
		return c.AbortInternalServerError("failed to update workspace")
	}

	role := c.GetString("workspace_role")

	return ok(c, WorkspaceResponse{
		ID:          ws.ID,
		Name:        ws.Name,
		Slug:        ws.Slug,
		Description: ws.Description,
		OwnerID:     ws.OwnerID,
		Role:        role,
		CreatedAt:   ws.CreatedAt,
	})
}

func (h *WorkspaceHandler) Delete(c *okapi.Context) error {
	wsID := c.GetInt("workspace_id")

	if err := h.workspaceRepo.Delete(uint(wsID)); err != nil {
		return c.AbortInternalServerError("failed to delete workspace")
	}

	return noContent(c)
}


func (h *WorkspaceHandler) ListMembers(c *okapi.Context) error {
	wsID := c.GetInt("workspace_id")

	members, err := h.workspaceRepo.ListMembers(uint(wsID))
	if err != nil {
		return c.AbortInternalServerError("failed to list members")
	}

	var result []WorkspaceMemberResponse
	for _, m := range members {
		result = append(result, WorkspaceMemberResponse{
			ID:        m.ID,
			UserID:    m.UserID,
			Name:      m.User.Name,
			Email:     m.User.Email,
			Role:      string(m.Role),
			CreatedAt: m.CreatedAt,
		})
	}

	return ok(c, result)
}

func (h *WorkspaceHandler) UpdateMemberRole(c *okapi.Context, req *UpdateMemberRoleRequest) error {
	wsID := c.GetInt("workspace_id")

	// Cannot change owner role
	member, err := h.workspaceRepo.FindMember(uint(wsID), uint(req.MemberID))
	if err != nil {
		return c.AbortNotFound("member not found")
	}

	if member.Role == models.WorkspaceRoleOwner {
		return c.AbortBadRequest("cannot change the owner's role")
	}

	if req.Body.Role == models.WorkspaceRoleOwner {
		return c.AbortBadRequest("cannot assign owner role")
	}

	if err := h.workspaceRepo.UpdateMemberRole(uint(wsID), uint(req.MemberID), req.Body.Role); err != nil {
		return c.AbortInternalServerError("failed to update member role")
	}

	return ok(c, okapi.M{"message": "member role updated"})
}

func (h *WorkspaceHandler) RemoveMember(c *okapi.Context, req *RemoveWorkspaceMemberRequest) error {
	wsID := c.GetInt("workspace_id")

	// Cannot remove the owner
	member, err := h.workspaceRepo.FindMember(uint(wsID), uint(req.MemberID))
	if err != nil {
		return c.AbortNotFound("member not found")
	}

	if member.Role == models.WorkspaceRoleOwner {
		return c.AbortBadRequest("cannot remove the workspace owner")
	}

	if err := h.workspaceRepo.RemoveMember(uint(wsID), uint(req.MemberID)); err != nil {
		return c.AbortInternalServerError("failed to remove member")
	}

	return noContent(c)
}


func (h *WorkspaceHandler) InviteMember(c *okapi.Context, req *InviteMemberRequest) error {
	wsID := c.GetInt("workspace_id")
	inviterID := c.GetInt("user_id")

	email := strings.TrimSpace(strings.ToLower(req.Body.Email))

	// Validate role
	if req.Body.Role == models.WorkspaceRoleOwner {
		return c.AbortBadRequest("cannot invite as owner")
	}

	// Check if user is already a member
	user, _ := h.userRepo.FindByEmail(email)
	if user != nil {
		if _, err := h.workspaceRepo.FindMember(uint(wsID), user.ID); err == nil {
			return c.AbortConflict("user is already a member of this workspace")
		}
	}

	// Generate invitation token
	token, err := generateToken()
	if err != nil {
		return c.AbortInternalServerError("failed to generate invitation token")
	}

	inv := &models.WorkspaceInvitation{
		WorkspaceID: uint(wsID),
		Email:       email,
		Role:        req.Body.Role,
		Token:       token,
		Status:      models.InvitationStatusPending,
		InvitedBy:   uint(inviterID),
		ExpiresAt:   time.Now().Add(7 * 24 * time.Hour), // 7 days
	}

	if err := h.workspaceRepo.CreateInvitation(inv); err != nil {
		return c.AbortInternalServerError("failed to create invitation")
	}

	return created(c, InvitationResponse{
		ID:          inv.ID,
		WorkspaceID: inv.WorkspaceID,
		Email:       inv.Email,
		Role:        string(inv.Role),
		Status:      string(inv.Status),
		ExpiresAt:   inv.ExpiresAt,
		CreatedAt:   inv.CreatedAt,
	})
}

func (h *WorkspaceHandler) ListInvitations(c *okapi.Context) error {
	wsID := c.GetInt("workspace_id")

	invitations, err := h.workspaceRepo.ListPendingInvitations(uint(wsID))
	if err != nil {
		return c.AbortInternalServerError("failed to list invitations")
	}

	var result []InvitationResponse
	for _, inv := range invitations {
		result = append(result, InvitationResponse{
			ID:          inv.ID,
			WorkspaceID: inv.WorkspaceID,
			Email:       inv.Email,
			Role:        string(inv.Role),
			Status:      string(inv.Status),
			ExpiresAt:   inv.ExpiresAt,
			CreatedAt:   inv.CreatedAt,
		})
	}

	return ok(c, result)
}

type DeleteInvitationRequest struct {
	InvitationID int `param:"invitation_id"`
}

func (h *WorkspaceHandler) DeleteInvitation(c *okapi.Context, req *DeleteInvitationRequest) error {
	if err := h.workspaceRepo.DeleteInvitation(uint(req.InvitationID)); err != nil {
		return c.AbortNotFound("invitation not found")
	}

	return noContent(c)
}


func (h *WorkspaceHandler) MyInvitations(c *okapi.Context) error {
	userEmail := c.GetString("email")

	invitations, err := h.workspaceRepo.FindPendingInvitationsByEmail(userEmail)
	if err != nil {
		return c.AbortInternalServerError("failed to list invitations")
	}

	var result []InvitationResponse
	for _, inv := range invitations {
		result = append(result, InvitationResponse{
			ID:          inv.ID,
			WorkspaceID: inv.WorkspaceID,
			Workspace:   inv.Workspace.Name,
			Email:       inv.Email,
			Role:        string(inv.Role),
			Status:      string(inv.Status),
			ExpiresAt:   inv.ExpiresAt,
			CreatedAt:   inv.CreatedAt,
		})
	}

	return ok(c, result)
}

func (h *WorkspaceHandler) AcceptInvitation(c *okapi.Context, req *AcceptInvitationRequest) error {
	userID := c.GetInt("user_id")

	inv, err := h.workspaceRepo.FindInvitationByToken(req.Body.Token)
	if err != nil {
		return c.AbortNotFound("invitation not found or expired")
	}

	if inv.Status != models.InvitationStatusPending {
		return c.AbortBadRequest("invitation is no longer pending")
	}

	if time.Now().After(inv.ExpiresAt) {
		return c.AbortBadRequest("invitation has expired")
	}

	// Verify the invitation email matches the current user
	user, err := h.userRepo.FindByID(uint(userID))
	if err != nil {
		return c.AbortInternalServerError("user not found")
	}

	if strings.ToLower(user.Email) != strings.ToLower(inv.Email) {
		return c.AbortForbidden("invitation is for a different email address")
	}

	// Check not already a member
	if _, err := h.workspaceRepo.FindMember(inv.WorkspaceID, uint(userID)); err == nil {
		inv.Status = models.InvitationStatusAccepted
		_ = h.workspaceRepo.UpdateInvitation(inv)
		return c.AbortConflict("you are already a member of this workspace")
	}

	// Add as member
	member := &models.WorkspaceMember{
		WorkspaceID: inv.WorkspaceID,
		UserID:      uint(userID),
		Role:        inv.Role,
	}
	if err := h.workspaceRepo.AddMember(member); err != nil {
		return c.AbortInternalServerError("failed to join workspace")
	}

	inv.Status = models.InvitationStatusAccepted
	_ = h.workspaceRepo.UpdateInvitation(inv)

	return ok(c, okapi.M{
		"message":      fmt.Sprintf("joined workspace %q", inv.Workspace.Name),
		"workspace_id": inv.WorkspaceID,
	})
}

type AcceptInvitationByIDRequest struct {
	ID int `param:"id"`
}

type DeclineInvitationByIDRequest struct {
	ID int `param:"id"`
}

// AcceptInvitationByID accepts an invitation by its ID (for logged-in users viewing their invitations).
func (h *WorkspaceHandler) AcceptInvitationByID(c *okapi.Context, req *AcceptInvitationByIDRequest) error {
	userID := c.GetInt("user_id")

	inv, err := h.workspaceRepo.FindInvitationByID(uint(req.ID))
	if err != nil {
		return c.AbortNotFound("invitation not found")
	}

	if inv.Status != models.InvitationStatusPending {
		return c.AbortBadRequest("invitation is no longer pending")
	}

	if time.Now().After(inv.ExpiresAt) {
		return c.AbortBadRequest("invitation has expired")
	}

	// Verify the invitation email matches the current user
	user, err := h.userRepo.FindByID(uint(userID))
	if err != nil {
		return c.AbortInternalServerError("user not found")
	}

	if strings.ToLower(user.Email) != strings.ToLower(inv.Email) {
		return c.AbortForbidden("invitation is for a different email address")
	}

	// Check not already a member
	if _, err := h.workspaceRepo.FindMember(inv.WorkspaceID, uint(userID)); err == nil {
		inv.Status = models.InvitationStatusAccepted
		_ = h.workspaceRepo.UpdateInvitation(inv)
		return c.AbortConflict("you are already a member of this workspace")
	}

	member := &models.WorkspaceMember{
		WorkspaceID: inv.WorkspaceID,
		UserID:      uint(userID),
		Role:        inv.Role,
	}
	if err := h.workspaceRepo.AddMember(member); err != nil {
		return c.AbortInternalServerError("failed to join workspace")
	}

	inv.Status = models.InvitationStatusAccepted
	_ = h.workspaceRepo.UpdateInvitation(inv)

	return ok(c, okapi.M{
		"message":      fmt.Sprintf("joined workspace %q", inv.Workspace.Name),
		"workspace_id": inv.WorkspaceID,
	})
}

// DeclineInvitationByID declines an invitation by its ID.
func (h *WorkspaceHandler) DeclineInvitationByID(c *okapi.Context, req *DeclineInvitationByIDRequest) error {
	userID := c.GetInt("user_id")

	inv, err := h.workspaceRepo.FindInvitationByID(uint(req.ID))
	if err != nil {
		return c.AbortNotFound("invitation not found")
	}

	if inv.Status != models.InvitationStatusPending {
		return c.AbortBadRequest("invitation is no longer pending")
	}

	// Verify belongs to current user
	user, err := h.userRepo.FindByID(uint(userID))
	if err != nil {
		return c.AbortInternalServerError("user not found")
	}
	if strings.ToLower(user.Email) != strings.ToLower(inv.Email) {
		return c.AbortForbidden("invitation is for a different email address")
	}

	inv.Status = models.InvitationStatusDeclined
	_ = h.workspaceRepo.UpdateInvitation(inv)

	return ok(c, okapi.M{"message": "invitation declined"})
}

func (h *WorkspaceHandler) DeclineInvitation(c *okapi.Context, req *DeclineInvitationRequest) error {
	inv, err := h.workspaceRepo.FindInvitationByToken(req.Body.Token)
	if err != nil {
		return c.AbortNotFound("invitation not found or expired")
	}

	if inv.Status != models.InvitationStatusPending {
		return c.AbortBadRequest("invitation is no longer pending")
	}

	inv.Status = models.InvitationStatusDeclined
	_ = h.workspaceRepo.UpdateInvitation(inv)

	return ok(c, okapi.M{"message": "invitation declined"})
}


type TransferDataRequest struct {
	Body struct {
		Resources []string `json:"resources" required:"true"`
	} `json:"body"`
}

type TransferResult struct {
	Resource string `json:"resource"`
	Count    int64  `json:"count"`
}

type TransferResponse struct {
	Message  string           `json:"message"`
	Results  []TransferResult `json:"results"`
	Total    int64            `json:"total"`
}

// TransferData moves the current user's personal data into the workspace.
func (h *WorkspaceHandler) TransferData(c *okapi.Context, req *TransferDataRequest) error {
	userID := c.GetInt("user_id")
	wsID := c.GetInt("workspace_id")
	if wsID == 0 {
		return c.AbortBadRequest("workspace context required")
	}

	allowedResources := map[string]string{
		"templates":     "templates",
		"stylesheets":   "style_sheets",
		"languages":     "languages",
		"smtp_servers":  "smtp_servers",
		"domains":       "domains",
		"webhooks":      "webhooks",
		"contacts":          "contacts",
		"subscribers":       "subscribers",
		"subscriber_lists":  "subscriber_lists",
		"suppressions":      "suppressions",
		"api_keys":      "api_keys",
		"bounces":       "bounces",
		"emails":        "emails",
	}

	var results []TransferResult
	var totalMoved int64

	wsIDVal := uint(wsID)

	err := h.db.Transaction(func(tx *gorm.DB) error {
		for _, res := range req.Body.Resources {
			table, ok := allowedResources[res]
			if !ok {
				continue
			}
			result := tx.Exec(
				fmt.Sprintf(`UPDATE %s SET workspace_id = ? WHERE user_id = ? AND workspace_id IS NULL`, table),
				wsIDVal, userID,
			)
			if result.Error != nil {
				return result.Error
			}
			results = append(results, TransferResult{
				Resource: res,
				Count:    result.RowsAffected,
			})
			totalMoved += result.RowsAffected
		}
		return nil
	})

	if err != nil {
		return c.AbortInternalServerError("failed to transfer data: " + err.Error())
	}

	return ok(c, TransferResponse{
		Message: fmt.Sprintf("transferred %d records to workspace", totalMoved),
		Results: results,
		Total:   totalMoved,
	})
}


var slugRegex = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func isValidSlug(s string) bool {
	return slugRegex.MatchString(s)
}

func slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = regexp.MustCompile(`[^a-z0-9\s-]`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`[\s]+`).ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "workspace"
	}
	return s
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// SetPlanService sets the plan service for workspace plan resolution.
func (h *WorkspaceHandler) SetPlanService(ps planService) {
	h.planService = ps
}

// GetPlan returns the effective plan for the current workspace.
func (h *WorkspaceHandler) GetPlan(c *okapi.Context) error {
	wsID := uint(c.GetInt("workspace_id"))
	if h.planService == nil {
		return ok(c, okapi.M{"plan": nil, "source": "global_settings"})
	}
	plan := h.planService.EffectivePlan(&wsID)
	if plan == nil {
		return ok(c, okapi.M{"plan": nil, "source": "global_settings"})
	}
	return ok(c, plan)
}
