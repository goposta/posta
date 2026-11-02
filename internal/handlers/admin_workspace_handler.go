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
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/audit"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
	"gorm.io/gorm"
)

type AdminWorkspaceHandler struct {
	db    *gorm.DB
	repo  *repositories.WorkspaceRepository
	audit *audit.Logger
}

func NewAdminWorkspaceHandler(db *gorm.DB, repo *repositories.WorkspaceRepository, auditLogger *audit.Logger) *AdminWorkspaceHandler {
	return &AdminWorkspaceHandler{db: db, repo: repo, audit: auditLogger}
}

// AdminWorkspaceRow is one line of the platform-wide workspace table.
type AdminWorkspaceRow struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	System      bool   `json:"system"`
	OwnerID     uint   `json:"owner_id"`
	OwnerName   string `json:"owner_name"`
	OwnerEmail  string `json:"owner_email"`
	PlanID      *uint  `json:"plan_id"`
	PlanName    string `json:"plan_name"`
	MemberCount int64  `json:"member_count"`
	// HostedFormsDisabled is set when a platform admin took the workspace's
	// hosted form pages offline.
	HostedFormsDisabled bool      `json:"hosted_forms_disabled"`
	CreatedAt           time.Time `json:"created_at"`
}

// AdminWorkspaceMember is a workspace member with the account details an
// administrator needs to follow up on them.
type AdminWorkspaceMember struct {
	UserID   uint                 `json:"user_id"`
	Name     string               `json:"name"`
	Email    string               `json:"email"`
	Active   bool                 `json:"active"`
	Role     models.WorkspaceRole `json:"role"`
	JoinedAt time.Time            `json:"joined_at"`
}

// AdminWorkspaceUsage counts the resources a workspace holds.
type AdminWorkspaceUsage struct {
	Emails      int64 `json:"emails"`
	Templates   int64 `json:"templates"`
	Contacts    int64 `json:"contacts"`
	Subscribers int64 `json:"subscribers"`
	Campaigns   int64 `json:"campaigns"`
	Domains     int64 `json:"domains"`
	SMTPServers int64 `json:"smtp_servers"`
	APIKeys     int64 `json:"api_keys"`
	Webhooks    int64 `json:"webhooks"`
}

// AdminWorkspaceDetail adds membership, pending invitations and resource usage.
type AdminWorkspaceDetail struct {
	AdminWorkspaceRow
	Description        string                 `json:"description"`
	DefaultLanguage    string                 `json:"default_language"`
	UpdatedAt          time.Time              `json:"updated_at"`
	Members            []AdminWorkspaceMember `json:"members"`
	PendingInvitations int64                  `json:"pending_invitations"`
	Usage              AdminWorkspaceUsage    `json:"usage"`
}

type AdminListWorkspacesRequest struct {
	Page   int    `query:"page" default:"0"`
	Size   int    `query:"size" default:"20"`
	Search string `query:"search" doc:"Match part of the workspace name or slug"`
	Owner  int    `query:"owner" doc:"Restrict to workspaces owned by this user id"`
	Plan   int    `query:"plan" doc:"Restrict to workspaces assigned this plan id"`
}

type AdminWorkspaceIDRequest struct {
	ID int `param:"id"`
}

type AdminSetHostedFormsRequest struct {
	ID   int `param:"id"`
	Body struct {
		Enabled bool `json:"enabled" doc:"false takes the workspace's hosted form pages offline"`
	} `json:"body"`
}

// SetHostedForms turns a workspace's hosted form pages (/f/{key}) on or off.
// Off, the pages answer 404 and the widget refuses to load or submit from
// Posta's own domain, so a phishing form stops at once. Forms embedded on the
// workspace's own sites, its submissions and its data are untouched.
func (h *AdminWorkspaceHandler) SetHostedForms(c *okapi.Context, req *AdminSetHostedFormsRequest) error {
	ws, err := h.repo.FindByID(uint(req.ID))
	if err != nil {
		return c.AbortNotFound("workspace not found")
	}
	disabled := !req.Body.Enabled
	if ws.HostedFormsDisabled != disabled {
		if err := h.db.Model(&models.Workspace{}).Where("id = ?", ws.ID).
			Update("hosted_forms_disabled", disabled).Error; err != nil {
			return c.AbortInternalServerError("failed to update workspace")
		}
		ws.HostedFormsDisabled = disabled
		if h.audit != nil {
			action, message := "workspace.hosted_forms_enabled", "Hosted form pages turned back on: "+ws.Name
			if disabled {
				action, message = "workspace.hosted_forms_disabled", "Hosted form pages taken offline: "+ws.Name
			}
			h.audit.LogCtxScoped(c, ws.ID, action, message, map[string]any{"workspace_id": ws.ID})
		}
	}
	one := []models.Workspace{*ws}
	return ok(c, h.row(ws, h.owners(one), h.planNames(one), h.memberCounts(one)))
}

func (h *AdminWorkspaceHandler) List(c *okapi.Context, req *AdminListWorkspacesRequest) error {
	page, size, offset := normalizePageParams(req.Page, req.Size)

	filter := repositories.AdminWorkspaceFilter{Search: req.Search}
	if req.Owner > 0 {
		owner := uint(req.Owner)
		filter.OwnerID = &owner
	}
	if req.Plan > 0 {
		plan := uint(req.Plan)
		filter.PlanID = &plan
	}

	workspaces, total, err := h.repo.FindAllFiltered(filter, size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list workspaces")
	}

	owners := h.owners(workspaces)
	plans := h.planNames(workspaces)
	members := h.memberCounts(workspaces)

	rows := make([]AdminWorkspaceRow, 0, len(workspaces))
	for i := range workspaces {
		rows = append(rows, h.row(&workspaces[i], owners, plans, members))
	}
	return paginated(c, rows, total, page, size)
}

func (h *AdminWorkspaceHandler) Get(c *okapi.Context, req *AdminWorkspaceIDRequest) error {
	ws, err := h.repo.FindByID(uint(req.ID))
	if err != nil {
		return c.AbortNotFound("workspace not found")
	}
	one := []models.Workspace{*ws}

	members, err := h.repo.ListMembers(ws.ID)
	if err != nil {
		return c.AbortInternalServerError("failed to list workspace members")
	}
	memberRows := make([]AdminWorkspaceMember, 0, len(members))
	for i := range members {
		m := &members[i]
		memberRows = append(memberRows, AdminWorkspaceMember{
			UserID:   m.UserID,
			Name:     m.User.Name,
			Email:    m.User.Email,
			Active:   m.User.Active,
			Role:     m.Role,
			JoinedAt: m.CreatedAt,
		})
	}

	detail := AdminWorkspaceDetail{
		AdminWorkspaceRow: h.row(ws, h.owners(one), h.planNames(one), map[uint]int64{ws.ID: int64(len(members))}),
		Description:       ws.Description,
		DefaultLanguage:   ws.DefaultLanguage,
		UpdatedAt:         ws.UpdatedAt,
		Members:           memberRows,
		Usage:             h.usage(ws.ID),
	}
	h.db.Model(&models.WorkspaceInvitation{}).
		Where("workspace_id = ? AND status = ? AND expires_at > ?", ws.ID, models.InvitationStatusPending, time.Now()).
		Count(&detail.PendingInvitations)

	return ok(c, detail)
}

func (h *AdminWorkspaceHandler) row(ws *models.Workspace, owners map[uint]models.User, plans map[uint]string, members map[uint]int64) AdminWorkspaceRow {
	row := AdminWorkspaceRow{
		ID:          ws.ID,
		Name:        ws.Name,
		Slug:        ws.Slug,
		System:      ws.System,
		OwnerID:     ws.OwnerID,
		OwnerName:   owners[ws.OwnerID].Name,
		OwnerEmail:  owners[ws.OwnerID].Email,
		PlanID:      ws.PlanID,
		MemberCount: members[ws.ID],
		CreatedAt:   ws.CreatedAt,

		HostedFormsDisabled: ws.HostedFormsDisabled,
	}
	if ws.PlanID != nil {
		row.PlanName = plans[*ws.PlanID]
	}
	return row
}

// usage counts each resource type through its model so soft-deleted rows are
// left out the same way the workspace's own views leave them out.
func (h *AdminWorkspaceHandler) usage(workspaceID uint) AdminWorkspaceUsage {
	var u AdminWorkspaceUsage
	count := func(model any, dst *int64) {
		h.db.Model(model).Where("workspace_id = ?", workspaceID).Count(dst)
	}
	count(&models.Email{}, &u.Emails)
	count(&models.Template{}, &u.Templates)
	count(&models.Contact{}, &u.Contacts)
	count(&models.Subscriber{}, &u.Subscribers)
	count(&models.Campaign{}, &u.Campaigns)
	count(&models.Domain{}, &u.Domains)
	count(&models.SMTPServer{}, &u.SMTPServers)
	count(&models.APIKey{}, &u.APIKeys)
	count(&models.Webhook{}, &u.Webhooks)
	return u
}

// owners resolves the owning accounts for a page of workspaces in one query.
func (h *AdminWorkspaceHandler) owners(workspaces []models.Workspace) map[uint]models.User {
	out := map[uint]models.User{}
	if len(workspaces) == 0 {
		return out
	}
	ids := make([]uint, 0, len(workspaces))
	for i := range workspaces {
		ids = append(ids, workspaces[i].OwnerID)
	}
	var rows []models.User
	if err := h.db.Select("id", "name", "email").Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return out
	}
	for i := range rows {
		out[rows[i].ID] = rows[i]
	}
	return out
}

func (h *AdminWorkspaceHandler) planNames(workspaces []models.Workspace) map[uint]string {
	out := map[uint]string{}
	ids := make([]uint, 0, len(workspaces))
	for i := range workspaces {
		if workspaces[i].PlanID != nil {
			ids = append(ids, *workspaces[i].PlanID)
		}
	}
	if len(ids) == 0 {
		return out
	}
	var rows []models.Plan
	if err := h.db.Select("id", "name").Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return out
	}
	for i := range rows {
		out[rows[i].ID] = rows[i].Name
	}
	return out
}

func (h *AdminWorkspaceHandler) memberCounts(workspaces []models.Workspace) map[uint]int64 {
	out := map[uint]int64{}
	if len(workspaces) == 0 {
		return out
	}
	ids := make([]uint, 0, len(workspaces))
	for i := range workspaces {
		ids = append(ids, workspaces[i].ID)
	}
	var rows []struct {
		WorkspaceID uint
		Count       int64
	}
	if err := h.db.Model(&models.WorkspaceMember{}).
		Select("workspace_id, COUNT(*) AS count").
		Where("workspace_id IN ?", ids).
		Group("workspace_id").
		Scan(&rows).Error; err != nil {
		return out
	}
	for _, r := range rows {
		out[r.WorkspaceID] = r.Count
	}
	return out
}
