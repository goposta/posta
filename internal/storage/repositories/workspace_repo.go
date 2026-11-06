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

package repositories

import (
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
)

type WorkspaceRepository struct {
	db *gorm.DB
}

func NewWorkspaceRepository(db *gorm.DB) *WorkspaceRepository {
	return &WorkspaceRepository{db: db}
}

func (r *WorkspaceRepository) Create(ws *models.Workspace) error {
	return r.db.Create(ws).Error
}

func (r *WorkspaceRepository) Update(ws *models.Workspace) error {
	return r.db.Save(ws).Error
}

func (r *WorkspaceRepository) Delete(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("workspace_id = ?", id).Delete(&models.WorkspaceInvitation{}).Error; err != nil {
			return err
		}
		if err := tx.Where("workspace_id = ?", id).Delete(&models.WorkspaceMember{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.Workspace{}, id).Error
	})
}

func (r *WorkspaceRepository) FindByID(id uint) (*models.Workspace, error) {
	var ws models.Workspace
	if err := r.db.First(&ws, id).Error; err != nil {
		return nil, err
	}
	return &ws, nil
}

func (r *WorkspaceRepository) FindByName(name string) (*models.Workspace, error) {
	var ws models.Workspace
	if err := r.db.Where("name = ?", name).First(&ws).Error; err != nil {
		return nil, err
	}
	return &ws, nil
}

// FindAll returns every workspace oldest first, for platform cron jobs that span all workspaces.
func (r *WorkspaceRepository) FindAll() ([]models.Workspace, error) {
	var workspaces []models.Workspace
	if err := r.db.Order("id ASC").Find(&workspaces).Error; err != nil {
		return nil, err
	}
	return workspaces, nil
}

type AdminWorkspaceFilter struct {
	Search  string
	OwnerID *uint
	PlanID  *uint
}

// FindAllFiltered sorts the system workspace first so it is easy to find, then newest first.
func (r *WorkspaceRepository) FindAllFiltered(f AdminWorkspaceFilter, limit, offset int) ([]models.Workspace, int64, error) {
	var items []models.Workspace
	var total int64

	apply := func(q *gorm.DB) *gorm.DB {
		if s := strings.TrimSpace(f.Search); s != "" {
			like := "%" + strings.ToLower(s) + "%"
			q = q.Where("LOWER(display_name) LIKE ? OR LOWER(name) LIKE ?", like, like)
		}
		if f.OwnerID != nil {
			q = q.Where("owner_id = ?", *f.OwnerID)
		}
		if f.PlanID != nil {
			q = q.Where("plan_id = ?", *f.PlanID)
		}
		return q
	}

	if err := apply(r.db.Model(&models.Workspace{})).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := apply(r.db).
		Order("system DESC, created_at DESC").
		Limit(limit).Offset(offset).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// FindByUserID sorts the system workspace last so it never displaces a real one at the top of a list.
func (r *WorkspaceRepository) FindByUserID(userID uint) ([]models.Workspace, error) {
	var workspaces []models.Workspace
	if err := r.db.
		Joins("JOIN workspace_members ON workspace_members.workspace_id = workspaces.id").
		Where("workspace_members.user_id = ?", userID).
		Order("workspaces.system ASC, workspaces.created_at DESC").
		Find(&workspaces).Error; err != nil {
		return nil, err
	}
	return workspaces, nil
}

func (r *WorkspaceRepository) FindSystem() (*models.Workspace, error) {
	var ws models.Workspace
	if err := r.db.Where("system = ?", true).First(&ws).Error; err != nil {
		return nil, err
	}
	return &ws, nil
}

// NameExists reports whether name is taken by a workspace other than excludeID.
func (r *WorkspaceRepository) NameExists(name string, excludeID uint) bool {
	var count int64
	q := r.db.Model(&models.Workspace{}).Where("name = ?", name)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	q.Count(&count)
	return count > 0
}

// CountNonSystemMemberships is used to refuse deleting somebody's last workspace.
func (r *WorkspaceRepository) CountNonSystemMemberships(userID uint) int64 {
	var count int64
	r.db.Model(&models.WorkspaceMember{}).
		Joins("JOIN workspaces ON workspaces.id = workspace_members.workspace_id").
		Where("workspace_members.user_id = ? AND workspaces.system = ?", userID, false).
		Count(&count)
	return count
}

func (r *WorkspaceRepository) AddMember(member *models.WorkspaceMember) error {
	return r.db.Create(member).Error
}

// RemoveMember takes a user out of a workspace, and off its teams with it.
func (r *WorkspaceRepository) RemoveMember(workspaceID, userID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ? AND team_id IN (?)", userID,
			tx.Model(&models.Team{}).Select("id").Where("workspace_id = ?", workspaceID)).
			Delete(&models.TeamMember{}).Error; err != nil {
			return err
		}
		return tx.Where("workspace_id = ? AND user_id = ?", workspaceID, userID).
			Delete(&models.WorkspaceMember{}).Error
	})
}

func (r *WorkspaceRepository) UpdateMemberRole(workspaceID, userID uint, role models.WorkspaceRole) error {
	return r.db.Model(&models.WorkspaceMember{}).
		Where("workspace_id = ? AND user_id = ?", workspaceID, userID).
		Update("role", role).Error
}

func (r *WorkspaceRepository) FindMember(workspaceID, userID uint) (*models.WorkspaceMember, error) {
	var member models.WorkspaceMember
	if err := r.db.Where("workspace_id = ? AND user_id = ?", workspaceID, userID).
		First(&member).Error; err != nil {
		return nil, err
	}
	return &member, nil
}

func (r *WorkspaceRepository) ListMembers(workspaceID uint) ([]models.WorkspaceMember, error) {
	var members []models.WorkspaceMember
	if err := r.db.Preload("User").
		Where("workspace_id = ?", workspaceID).
		Order("created_at ASC").
		Find(&members).Error; err != nil {
		return nil, err
	}
	return members, nil
}

func (r *WorkspaceRepository) CountMembers(workspaceID uint) (int64, error) {
	var count int64
	if err := r.db.Model(&models.WorkspaceMember{}).
		Where("workspace_id = ?", workspaceID).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *WorkspaceRepository) CreateInvitation(inv *models.WorkspaceInvitation) error {
	return r.db.Create(inv).Error
}

func (r *WorkspaceRepository) FindInvitationByID(id uint) (*models.WorkspaceInvitation, error) {
	var inv models.WorkspaceInvitation
	if err := r.db.Preload("Workspace").First(&inv, id).Error; err != nil {
		return nil, err
	}
	return &inv, nil
}

func (r *WorkspaceRepository) FindInvitationByToken(token string) (*models.WorkspaceInvitation, error) {
	var inv models.WorkspaceInvitation
	if err := r.db.Preload("Workspace").Where("token = ?", token).First(&inv).Error; err != nil {
		return nil, err
	}
	return &inv, nil
}

func (r *WorkspaceRepository) UpdateInvitation(inv *models.WorkspaceInvitation) error {
	return r.db.Save(inv).Error
}

func (r *WorkspaceRepository) ListPendingInvitations(workspaceID uint) ([]models.WorkspaceInvitation, error) {
	var invitations []models.WorkspaceInvitation
	if err := r.db.Where("workspace_id = ? AND status = ? AND expires_at > ?",
		workspaceID, models.InvitationStatusPending, time.Now()).
		Order("created_at DESC").
		Find(&invitations).Error; err != nil {
		return nil, err
	}
	return invitations, nil
}

func (r *WorkspaceRepository) FindPendingInvitationsByEmail(email string) ([]models.WorkspaceInvitation, error) {
	var invitations []models.WorkspaceInvitation
	if err := r.db.Preload("Workspace").
		Where("email = ? AND status = ? AND expires_at > ?",
			email, models.InvitationStatusPending, time.Now()).
		Order("created_at DESC").
		Find(&invitations).Error; err != nil {
		return nil, err
	}
	return invitations, nil
}

func (r *WorkspaceRepository) DeleteInvitation(id uint) error {
	return r.db.Delete(&models.WorkspaceInvitation{}, id).Error
}

func (r *WorkspaceRepository) CreateDefaultWorkspace(userID uint, displayName, name string) (*models.Workspace, error) {
	ws := &models.Workspace{
		Name:        name,
		DisplayName: displayName,
		OwnerID:     userID,
	}
	if err := r.db.Create(ws).Error; err != nil {
		return nil, err
	}

	member := &models.WorkspaceMember{
		WorkspaceID: ws.ID,
		UserID:      userID,
		Role:        models.WorkspaceRoleOwner,
	}
	if err := r.db.Create(member).Error; err != nil {
		return nil, err
	}

	return ws, nil
}
