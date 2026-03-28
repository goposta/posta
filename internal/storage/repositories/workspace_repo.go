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

package repositories

import (
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

func (r *WorkspaceRepository) FindBySlug(slug string) (*models.Workspace, error) {
	var ws models.Workspace
	if err := r.db.Where("slug = ?", slug).First(&ws).Error; err != nil {
		return nil, err
	}
	return &ws, nil
}

// FindByUserID returns all workspaces the user is a member of.
func (r *WorkspaceRepository) FindByUserID(userID uint) ([]models.Workspace, error) {
	var workspaces []models.Workspace
	if err := r.db.
		Joins("JOIN workspace_members ON workspace_members.workspace_id = workspaces.id").
		Where("workspace_members.user_id = ?", userID).
		Order("workspaces.created_at DESC").
		Find(&workspaces).Error; err != nil {
		return nil, err
	}
	return workspaces, nil
}

// AddMember adds a user as a member of a workspace.
func (r *WorkspaceRepository) AddMember(member *models.WorkspaceMember) error {
	return r.db.Create(member).Error
}

// RemoveMember removes a user from a workspace.
func (r *WorkspaceRepository) RemoveMember(workspaceID, userID uint) error {
	return r.db.Where("workspace_id = ? AND user_id = ?", workspaceID, userID).
		Delete(&models.WorkspaceMember{}).Error
}

// UpdateMemberRole updates a member's role in a workspace.
func (r *WorkspaceRepository) UpdateMemberRole(workspaceID, userID uint, role models.WorkspaceRole) error {
	return r.db.Model(&models.WorkspaceMember{}).
		Where("workspace_id = ? AND user_id = ?", workspaceID, userID).
		Update("role", role).Error
}

// FindMember returns the membership record for a user in a workspace.
func (r *WorkspaceRepository) FindMember(workspaceID, userID uint) (*models.WorkspaceMember, error) {
	var member models.WorkspaceMember
	if err := r.db.Where("workspace_id = ? AND user_id = ?", workspaceID, userID).
		First(&member).Error; err != nil {
		return nil, err
	}
	return &member, nil
}

// ListMembers returns all members of a workspace with their user details.
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

// CountMembers returns the number of members in a workspace.
func (r *WorkspaceRepository) CountMembers(workspaceID uint) (int64, error) {
	var count int64
	if err := r.db.Model(&models.WorkspaceMember{}).
		Where("workspace_id = ?", workspaceID).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CreateInvitation creates a new workspace invitation.
func (r *WorkspaceRepository) CreateInvitation(inv *models.WorkspaceInvitation) error {
	return r.db.Create(inv).Error
}

// FindInvitationByID returns an invitation by its ID.
func (r *WorkspaceRepository) FindInvitationByID(id uint) (*models.WorkspaceInvitation, error) {
	var inv models.WorkspaceInvitation
	if err := r.db.Preload("Workspace").First(&inv, id).Error; err != nil {
		return nil, err
	}
	return &inv, nil
}

// FindInvitationByToken returns an invitation by its token.
func (r *WorkspaceRepository) FindInvitationByToken(token string) (*models.WorkspaceInvitation, error) {
	var inv models.WorkspaceInvitation
	if err := r.db.Preload("Workspace").Where("token = ?", token).First(&inv).Error; err != nil {
		return nil, err
	}
	return &inv, nil
}

// UpdateInvitation updates an invitation.
func (r *WorkspaceRepository) UpdateInvitation(inv *models.WorkspaceInvitation) error {
	return r.db.Save(inv).Error
}

// ListPendingInvitations returns pending invitations for a workspace.
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

// FindPendingInvitationsByEmail returns pending invitations for a given email.
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

// DeleteInvitation deletes an invitation.
func (r *WorkspaceRepository) DeleteInvitation(id uint) error {
	return r.db.Delete(&models.WorkspaceInvitation{}, id).Error
}

// CreateDefaultWorkspace creates a default workspace for a user during registration.
func (r *WorkspaceRepository) CreateDefaultWorkspace(userID uint, name, slug string) (*models.Workspace, error) {
	ws := &models.Workspace{
		Name:    name,
		Slug:    slug,
		OwnerID: userID,
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
