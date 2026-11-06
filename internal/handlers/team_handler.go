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
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/audit"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
)

// Any member can list teams (the inbox filters by team); changing them is tenant administration.
type TeamHandler struct {
	repo          *repositories.TeamRepository
	ruleRepo      *repositories.FormRuleRepository
	workspaceRepo *repositories.WorkspaceRepository
	audit         *audit.Logger
}

func NewTeamHandler(repo *repositories.TeamRepository, ruleRepo *repositories.FormRuleRepository, workspaceRepo *repositories.WorkspaceRepository, auditLogger *audit.Logger) *TeamHandler {
	return &TeamHandler{repo: repo, ruleRepo: ruleRepo, workspaceRepo: workspaceRepo, audit: auditLogger}
}

type TeamBody struct {
	Name    string `json:"name" required:"true"`
	Email   string `json:"email" doc:"Optional shared address notified along with the members, such as sales@example.com"`
	UserIDs []uint `json:"user_ids" doc:"Members; each must belong to the workspace"`
}

type CreateTeamRequest struct {
	Body TeamBody `json:"body"`
}

type UpdateTeamRequest struct {
	ID   int      `param:"id"`
	Body TeamBody `json:"body"`
}

type TeamIDRequest struct {
	ID int `param:"id"`
}

type TeamMemberView struct {
	UserID uint                 `json:"user_id"`
	Name   string               `json:"name"`
	Email  string               `json:"email"`
	Role   models.WorkspaceRole `json:"role"`
}

type TeamDetail struct {
	ID           uint                         `json:"id"`
	UUID         string                       `json:"uuid"`
	Name         string                       `json:"name"`
	Email        string                       `json:"email"`
	CreatedAt    time.Time                    `json:"created_at"`
	Members      []TeamMemberView             `json:"members"`
	Rules        []repositories.RuleUsingTeam `json:"rules"`
	OpenMessages int64                        `json:"open_messages" doc:"Submissions routed to the team that are new or open"`
	Messages     int64                        `json:"messages" doc:"All submissions routed to the team"`
}

// requireManage allows workspace owners and admins only: teams decide who is sent submissions.
func requireManage(c *okapi.Context) error {
	role := workspaceRole(c)
	if role != "" && !role.CanManageMembers() {
		return errForbidden
	}
	return nil
}

func (h *TeamHandler) workspace(c *okapi.Context) (uint, error) {
	scope := getScope(c)
	if scope.WorkspaceID == nil {
		return 0, c.AbortBadRequest("a workspace is required")
	}
	return *scope.WorkspaceID, nil
}

func (h *TeamHandler) apply(c *okapi.Context, workspaceID uint, team *models.Team, b TeamBody) ([]uint, error) {
	name := strings.Join(strings.Fields(b.Name), " ")
	if name == "" || len([]rune(name)) > 80 {
		return nil, c.AbortBadRequest("a team needs a name of at most 80 characters")
	}
	if h.repo.NameTaken(workspaceID, name, team.ID) {
		return nil, c.AbortConflict("a team with that name already exists")
	}
	email := strings.ToLower(strings.TrimSpace(b.Email))
	if email != "" {
		if addr, err := mail.ParseAddress(email); err != nil || addr.Address != email {
			return nil, c.AbortBadRequest(fmt.Sprintf("%q is not an email address", b.Email))
		}
	}
	members, err := h.workspaceRepo.ListMembers(workspaceID)
	if err != nil {
		return nil, c.AbortInternalServerError("failed to load members")
	}
	inWorkspace := map[uint]bool{}
	for _, m := range members {
		inWorkspace[m.UserID] = true
	}
	seen := map[uint]bool{}
	ids := make([]uint, 0, len(b.UserIDs))
	for _, id := range b.UserIDs {
		if !inWorkspace[id] {
			return nil, c.AbortBadRequest(fmt.Sprintf("user %d is not a member of this workspace", id))
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	team.Name, team.Email = name, email
	return ids, nil
}

func (h *TeamHandler) List(c *okapi.Context) error {
	ws, err := h.workspace(c)
	if ws == 0 {
		return err
	}
	teams, err := h.repo.ListByWorkspace(ws)
	if err != nil {
		return c.AbortInternalServerError("failed to list teams")
	}
	if teams == nil {
		teams = []models.Team{}
	}
	return ok(c, teams)
}

// Any workspace member may read a team.
func (h *TeamHandler) Get(c *okapi.Context, req *TeamIDRequest) error {
	ws, err := h.workspace(c)
	if ws == 0 {
		return err
	}
	team, err := h.repo.FindForWorkspace(ws, uint(req.ID))
	if err != nil {
		return c.AbortNotFound("team not found")
	}
	members, err := h.workspaceRepo.ListMembers(ws)
	if err != nil {
		return c.AbortInternalServerError("failed to load members")
	}
	byUser := map[uint]models.WorkspaceMember{}
	for _, m := range members {
		byUser[m.UserID] = m
	}
	detail := TeamDetail{
		ID: team.ID, UUID: team.UUID, Name: team.Name, Email: team.Email, CreatedAt: team.CreatedAt,
		Members: []TeamMemberView{}, Rules: []repositories.RuleUsingTeam{},
	}
	for _, tm := range team.Members {
		m, ok := byUser[tm.UserID]
		if !ok {
			// A member who left the workspace no longer receives anything.
			continue
		}
		detail.Members = append(detail.Members, TeamMemberView{UserID: m.UserID, Name: m.User.Name, Email: m.User.Email, Role: m.Role})
	}
	if rules, err := h.ruleRepo.RulesUsingTeam(ws, team.ID); err == nil && rules != nil {
		detail.Rules = rules
	}
	if detail.OpenMessages, detail.Messages, err = h.repo.MessageCounts(team.ID); err != nil {
		return c.AbortInternalServerError("failed to count messages")
	}
	return ok(c, detail)
}

func (h *TeamHandler) Create(c *okapi.Context, req *CreateTeamRequest) error {
	if err := requireManage(c); err != nil {
		return c.AbortForbidden("only workspace owners and admins can manage teams", err)
	}
	ws, err := h.workspace(c)
	if ws == 0 {
		return err
	}
	team := &models.Team{WorkspaceID: &ws}
	ids, err := h.apply(c, ws, team, req.Body)
	if ids == nil {
		return err
	}
	if err := h.repo.Create(team); err != nil {
		return c.AbortInternalServerError("failed to create team")
	}
	if err := h.repo.SetMembers(team.ID, ids); err != nil {
		return c.AbortInternalServerError("failed to set team members")
	}
	h.log(c, ws, "team.created", fmt.Sprintf("Team %s created", team.Name), team, len(ids))
	saved, _ := h.repo.FindForWorkspace(ws, team.ID)
	return created(c, saved)
}

func (h *TeamHandler) Update(c *okapi.Context, req *UpdateTeamRequest) error {
	if err := requireManage(c); err != nil {
		return c.AbortForbidden("only workspace owners and admins can manage teams", err)
	}
	ws, err := h.workspace(c)
	if ws == 0 {
		return err
	}
	team, err := h.repo.FindForWorkspace(ws, uint(req.ID))
	if err != nil {
		return c.AbortNotFound("team not found")
	}
	ids, err := h.apply(c, ws, team, req.Body)
	if ids == nil {
		return err
	}
	now := time.Now()
	team.UpdatedAt = &now
	if err := h.repo.Update(team); err != nil {
		return c.AbortInternalServerError("failed to update team")
	}
	if err := h.repo.SetMembers(team.ID, ids); err != nil {
		return c.AbortInternalServerError("failed to set team members")
	}
	h.log(c, ws, "team.updated", fmt.Sprintf("Team %s updated", team.Name), team, len(ids))
	saved, _ := h.repo.FindForWorkspace(ws, team.ID)
	return ok(c, saved)
}

// Delete refuses a team a rule points at: that rule would silently stop notifying anyone.
func (h *TeamHandler) Delete(c *okapi.Context, req *TeamIDRequest) error {
	if err := requireManage(c); err != nil {
		return c.AbortForbidden("only workspace owners and admins can manage teams", err)
	}
	ws, err := h.workspace(c)
	if ws == 0 {
		return err
	}
	team, err := h.repo.FindForWorkspace(ws, uint(req.ID))
	if err != nil {
		return c.AbortNotFound("team not found")
	}
	if names, err := h.ruleRepo.UsingTeam(ws, team.ID); err == nil && len(names) > 0 {
		return c.AbortConflict(fmt.Sprintf("routing rules use this team: %s. Change them first.", strings.Join(names, ", ")))
	}
	if err := h.repo.Delete(team.ID); err != nil {
		return c.AbortInternalServerError("failed to delete team")
	}
	h.log(c, ws, "team.deleted", fmt.Sprintf("Team %s deleted", team.Name), team, -1)
	return noContent(c)
}

func (h *TeamHandler) log(c *okapi.Context, ws uint, action, message string, team *models.Team, members int) {
	if h.audit == nil {
		return
	}
	meta := map[string]any{"team_id": team.ID, "team": team.Name}
	if members >= 0 {
		meta["members"] = members
	}
	h.audit.LogCtxScoped(c, ws, action, message, meta)
}
