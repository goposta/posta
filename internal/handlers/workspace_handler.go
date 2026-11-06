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

package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/audit"
	"github.com/goposta/posta/internal/services/notification"
	"github.com/goposta/posta/internal/services/workspace"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
	"gorm.io/gorm"
)

type WorkspaceHandler struct {
	workspaceRepo *repositories.WorkspaceRepository
	userRepo      *repositories.UserRepository
	db            *gorm.DB
	planService   planService
	notifier      *notification.Service
	appURL        string
	audit         *audit.Logger
	seeder        workspaceSeeder
}

// workspaceSeeder is optional: without one, workspaces are simply created empty.
type workspaceSeeder interface {
	SeedWorkspaceDefaults(workspaceID, userID uint, userName string)
}

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

// Without a seeder the seed_defaults flag has no effect.
func (h *WorkspaceHandler) SetSeeder(s workspaceSeeder) { h.seeder = s }

type CreateWorkspaceRequest struct {
	Body struct {
		Name            string `json:"name" doc:"The workspace's name, as typed: \"Acme Corp\" is stored as the identifier acme-corp, unique on the platform, with \"Acme Corp\" as display_name. It cannot change later."`
		DisplayName     string `json:"display_name" doc:"The workspace's label. Defaults to name."`
		Slug            string `json:"slug,omitempty" doc:"Deprecated: the identifier before 0.18, now name."`
		Description     string `json:"description"`
		DefaultLanguage string `json:"default_language"`
		// Omitted means yes: an empty workspace is rarely wanted; send false to populate from an export or the API.
		SeedDefaults *bool `json:"seed_defaults" doc:"Seed starter templates and a stylesheet. Defaults to true."`
	} `json:"body"`
}

type UpdateWorkspaceRequest struct {
	Body struct {
		Name            string `json:"name" doc:"A workspace's name cannot change; only accepted when it is the current one."`
		DisplayName     string `json:"display_name"`
		Description     string `json:"description"`
		DefaultLanguage string `json:"default_language"`
	} `json:"body"`
}

type InviteMemberRequest struct {
	Body struct {
		Email string               `json:"email" required:"true" format:"email"`
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
	ID          uint   `json:"id"`
	DisplayName string `json:"display_name"`
	Name        string `json:"name"`
	Description string `json:"description"`
	OwnerID     uint   `json:"owner_id"`
	Role        string `json:"role"`
	System      bool   `json:"system"`
	// Deprecated: the personal workspace type was removed; always false, kept one release for older dashboards.
	IsPersonal bool      `json:"is_personal"`
	CreatedAt  time.Time `json:"created_at"`
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
			return c.AbortForbidden("Workspace quota exceeded for your plan", err)
		}
	}

	name, displayName, err := resolveNames(req.Body.Name, req.Body.DisplayName, req.Body.Slug)
	if err != nil {
		return c.AbortBadRequest(err.Error())
	}
	if workspace.IsReservedName(name) {
		return c.AbortConflict("the name \"" + name + "\" is reserved by the platform")
	}
	if _, err := h.workspaceRepo.FindByName(name); err == nil {
		return c.AbortConflict("a workspace named \"" + name + "\" already exists")
	}

	defaultLang := strings.TrimSpace(req.Body.DefaultLanguage)
	if defaultLang == "" {
		defaultLang = "en"
	}

	ws := &models.Workspace{
		Name:            name,
		DisplayName:     displayName,
		Description:     req.Body.Description,
		DefaultLanguage: defaultLang,
		OwnerID:         uint(userID),
	}

	if err := h.workspaceRepo.Create(ws); err != nil {
		return c.AbortInternalServerError("failed to create workspace")
	}

	member := &models.WorkspaceMember{
		WorkspaceID: ws.ID,
		UserID:      uint(userID),
		Role:        models.WorkspaceRoleOwner,
	}
	if err := h.workspaceRepo.AddMember(member); err != nil {
		return c.AbortInternalServerError("failed to add workspace member")
	}

	seeded := shouldSeedWorkspace(req.Body.SeedDefaults)
	if seeded && h.seeder != nil {
		// Best effort: a workspace without starter content beats failing after the row is already committed.
		h.seeder.SeedWorkspaceDefaults(ws.ID, uint(userID), h.ownerName(uint(userID)))
	}

	h.logAudit(c, ws.ID, "workspace.created", "Workspace created: "+ws.DisplayName, map[string]any{
		metaName: ws.Name, "seeded": seeded,
	})

	return created(c, WorkspaceResponse{
		ID:          ws.ID,
		DisplayName: ws.DisplayName,
		Name:        ws.Name,
		Description: ws.Description,
		OwnerID:     ws.OwnerID,
		Role:        string(models.WorkspaceRoleOwner),
		System:      ws.System,
		CreatedAt:   ws.CreatedAt,
	})
}

// Absent means yes, so clients that predate the flag get starter content rather than an empty workspace.
func shouldSeedWorkspace(flag *bool) bool {
	return flag == nil || *flag
}

// An empty result is fine: the seeder greets generically.
func (h *WorkspaceHandler) ownerName(userID uint) string {
	if h.userRepo == nil {
		return ""
	}
	user, err := h.userRepo.FindByID(userID)
	if err != nil || user == nil {
		return ""
	}
	return user.Name
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
			DisplayName: ws.DisplayName,
			Name:        ws.Name,
			Description: ws.Description,
			OwnerID:     ws.OwnerID,
			Role:        role,
			System:      ws.System,
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
		DisplayName: ws.DisplayName,
		Name:        ws.Name,
		Description: ws.Description,
		OwnerID:     ws.OwnerID,
		Role:        role,
		System:      ws.System,
		CreatedAt:   ws.CreatedAt,
	})
}

func (h *WorkspaceHandler) Update(c *okapi.Context, req *UpdateWorkspaceRequest) error {
	wsID := c.GetInt("workspace_id")

	ws, err := h.workspaceRepo.FindByID(uint(wsID))
	if err != nil {
		return c.AbortNotFound("workspace not found")
	}

	displayName, err := resolveRename(req.Body.Name, req.Body.DisplayName, "", ws.Name, "workspace")
	if err != nil {
		return c.AbortBadRequest(err.Error())
	}
	if ws.System && displayName != "" && displayName != ws.DisplayName {
		return c.AbortConflict("the system workspace cannot be renamed")
	}
	if displayName != "" {
		ws.DisplayName = displayName
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

	h.logAudit(c, ws.ID, "workspace.updated", "Workspace updated: "+ws.DisplayName, nil)

	role := c.GetString("workspace_role")

	return ok(c, WorkspaceResponse{
		ID:          ws.ID,
		DisplayName: ws.DisplayName,
		Name:        ws.Name,
		Description: ws.Description,
		OwnerID:     ws.OwnerID,
		Role:        role,
		System:      ws.System,
		CreatedAt:   ws.CreatedAt,
	})
}

func (h *WorkspaceHandler) Delete(c *okapi.Context) error {
	wsID := c.GetInt("workspace_id")

	ws, err := h.workspaceRepo.FindByID(uint(wsID))
	if err != nil {
		return c.AbortNotFound("workspace not found")
	}
	if ws.System {
		return c.AbortConflict("the system workspace cannot be deleted")
	}
	userID := uint(c.GetInt("user_id"))
	if h.workspaceRepo.CountNonSystemMemberships(userID) <= 1 {
		return c.AbortBadRequest("this is your only workspace; create another before deleting this one")
	}

	if err := h.workspaceRepo.Delete(uint(wsID)); err != nil {
		return c.AbortInternalServerError("failed to delete workspace")
	}

	h.logAudit(c, ws.ID, "workspace.deleted", "Workspace deleted: "+ws.DisplayName, map[string]any{metaName: ws.Name})

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

	oldRole := member.Role

	if err := h.workspaceRepo.UpdateMemberRole(uint(wsID), uint(req.MemberID), req.Body.Role); err != nil {
		return c.AbortInternalServerError("failed to update member role")
	}

	h.logAudit(c, uint(wsID), "member.role_updated",
		fmt.Sprintf("Member role changed from %s to %s", oldRole, req.Body.Role),
		map[string]any{"member_user_id": req.MemberID, "old_role": string(oldRole), "new_role": string(req.Body.Role)})

	if h.notifier != nil && req.Body.Role != oldRole {
		ws, _ := h.workspaceRepo.FindByID(uint(wsID))
		changer, _ := h.userRepo.FindByID(uint(c.GetInt("user_id")))
		wsName := ""
		changerName := "An administrator"
		if ws != nil {
			wsName = ws.DisplayName
		}
		if changer != nil {
			changerName = changer.Name
			if changerName == "" {
				changerName = changer.Email
			}
		}
		go func() {
			_ = h.notifier.SendToUser(uint(req.MemberID), fmt.Sprintf("Your role in %s has been updated", wsName), notification.TemplateRoleChanged, map[string]any{
				"WorkspaceName": wsName,
				"OldRole":       string(oldRole),
				"NewRole":       string(req.Body.Role),
				"ChangedBy":     changerName,
			})
		}()
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

	h.logAudit(c, uint(wsID), "member.removed", "Member removed from workspace",
		map[string]any{"member_user_id": req.MemberID, "role": string(member.Role)})

	return noContent(c)
}

func (h *WorkspaceHandler) InviteMember(c *okapi.Context, req *InviteMemberRequest) error {
	wsID := c.GetInt("workspace_id")
	inviterID := c.GetInt("user_id")

	email := strings.TrimSpace(strings.ToLower(req.Body.Email))

	if req.Body.Role == models.WorkspaceRoleOwner {
		return c.AbortBadRequest("cannot invite as owner")
	}

	user, _ := h.userRepo.FindByEmail(email)
	if user != nil {
		if _, err := h.workspaceRepo.FindMember(uint(wsID), user.ID); err == nil {
			return c.AbortConflict("user is already a member of this workspace")
		}
	}

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
		ExpiresAt:   time.Now().Add(7 * 24 * time.Hour),
	}

	if err := h.workspaceRepo.CreateInvitation(inv); err != nil {
		return c.AbortInternalServerError("failed to create invitation")
	}

	h.logAudit(c, uint(wsID), "member.invited", "Invitation sent to "+email,
		map[string]any{"email": email, "role": string(req.Body.Role)})

	// Send invitation email (best-effort, don't fail the request)
	if h.notifier != nil {
		ws, _ := h.workspaceRepo.FindByID(uint(wsID))
		inviter, _ := h.userRepo.FindByID(uint(inviterID))
		wsName := ""
		inviterName := "A team member"
		if ws != nil {
			wsName = ws.DisplayName
		}
		if inviter != nil {
			inviterName = inviter.Name
			if inviterName == "" {
				inviterName = inviter.Email
			}
		}
		roleArticle := "a"
		if inv.Role == "admin" || inv.Role == "editor" || inv.Role == "owner" {
			roleArticle = "an"
		}
		go func() {
			_ = h.notifier.Send(email, fmt.Sprintf("You've been invited to %s", wsName), notification.TemplateInvitation, map[string]any{
				"WorkspaceName": wsName,
				"InviterName":   inviterName,
				"Role":          string(inv.Role),
				"RoleArticle":   roleArticle,
				"AcceptURL":     fmt.Sprintf("%s/invitations?token=%s", h.appURL, inv.Token),
				"ExpiresAt":     inv.ExpiresAt.Format("January 2, 2006"),
			})
		}()
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
	wsID := c.GetInt("workspace_id")

	inv, err := h.workspaceRepo.FindInvitationByID(uint(req.InvitationID))
	if err != nil {
		return c.AbortNotFound("invitation not found")
	}

	// Invitation IDs are guessable: require this workspace, and answer not-found so other workspaces can't be probed.
	if inv.WorkspaceID != uint(wsID) {
		return c.AbortNotFound("invitation not found")
	}

	if err := h.workspaceRepo.DeleteInvitation(uint(req.InvitationID)); err != nil {
		return c.AbortNotFound("invitation not found")
	}

	h.logAudit(c, inv.WorkspaceID, "invitation.revoked", "Invitation revoked for "+inv.Email,
		map[string]any{"email": inv.Email})

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
			Workspace:   inv.Workspace.DisplayName,
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

	if !strings.EqualFold(user.Email, inv.Email) {
		return c.AbortForbidden("invitation is for a different email address")
	}

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

	h.logAudit(c, inv.WorkspaceID, "member.joined", "Joined workspace "+inv.Workspace.DisplayName,
		map[string]any{"role": string(inv.Role)})

	return ok(c, okapi.M{
		"message":      fmt.Sprintf("joined workspace %q", inv.Workspace.DisplayName),
		"workspace_id": inv.WorkspaceID,
	})
}

type AcceptInvitationByIDRequest struct {
	ID int `param:"id"`
}

type DeclineInvitationByIDRequest struct {
	ID int `param:"id"`
}

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

	if !strings.EqualFold(user.Email, inv.Email) {
		return c.AbortForbidden("invitation is for a different email address")
	}

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

	h.logAudit(c, inv.WorkspaceID, "member.joined", "Joined workspace "+inv.Workspace.DisplayName,
		map[string]any{"role": string(inv.Role)})

	return ok(c, okapi.M{
		"message":      fmt.Sprintf("joined workspace %q", inv.Workspace.DisplayName),
		"workspace_id": inv.WorkspaceID,
	})
}

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
	if !strings.EqualFold(user.Email, inv.Email) {
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

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (h *WorkspaceHandler) SetPlanService(ps planService) {
	h.planService = ps
}

func (h *WorkspaceHandler) SetNotifier(n *notification.Service, appURL string) {
	h.notifier = n
	h.appURL = appURL
}

func (h *WorkspaceHandler) SetAuditLogger(l *audit.Logger) {
	h.audit = l
}

func (h *WorkspaceHandler) logAudit(c *okapi.Context, workspaceID uint, action, message string, metadata map[string]any) {
	if h.audit == nil {
		return
	}
	h.audit.LogCtxScoped(c, workspaceID, action, message, metadata)
}

func (h *WorkspaceHandler) GetPlan(c *okapi.Context) error {
	wsID := uint(c.GetInt("workspace_id"))
	if h.planService == nil {
		return ok(c, okapi.M{"plan": nil, "source": planSourceGlobalSettings})
	}
	plan := h.planService.EffectivePlan(&wsID)
	if plan == nil {
		return ok(c, okapi.M{"plan": nil, "source": planSourceGlobalSettings})
	}
	return ok(c, plan)
}
