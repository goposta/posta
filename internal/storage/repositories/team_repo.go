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
	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TeamRepository struct {
	db *gorm.DB
}

func NewTeamRepository(db *gorm.DB) *TeamRepository {
	return &TeamRepository{db: db}
}

func (r *TeamRepository) ListByWorkspace(workspaceID uint) ([]models.Team, error) {
	var teams []models.Team
	err := r.db.Preload("Members.User").Where("workspace_id = ?", workspaceID).Order("LOWER(name) ASC").Find(&teams).Error
	return teams, err
}

func (r *TeamRepository) FindForWorkspace(workspaceID, id uint) (*models.Team, error) {
	var team models.Team
	if err := r.db.Preload("Members.User").Where("workspace_id = ?", workspaceID).First(&team, id).Error; err != nil {
		return nil, err
	}
	return &team, nil
}

// FindByID loads a team with its member IDs, whatever the workspace; the
// worker checks the workspace itself.
func (r *TeamRepository) FindByID(id uint) (*models.Team, error) {
	var team models.Team
	if err := r.db.Preload("Members").First(&team, id).Error; err != nil {
		return nil, err
	}
	return &team, nil
}

func (r *TeamRepository) NameTaken(workspaceID uint, name string, exceptID uint) bool {
	var count int64
	r.db.Model(&models.Team{}).Where("workspace_id = ? AND LOWER(name) = LOWER(?) AND id <> ?", workspaceID, name, exceptID).Count(&count)
	return count > 0
}

func (r *TeamRepository) Create(team *models.Team) error {
	return r.db.Omit(clause.Associations).Create(team).Error
}

func (r *TeamRepository) Update(team *models.Team) error {
	return r.db.Omit(clause.Associations).Save(team).Error
}

// Delete removes a team and its memberships. Submissions routed to it stay
// where they are, without a team.
func (r *TeamRepository) Delete(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("team_id = ?", id).Delete(&models.TeamMember{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.Message{}).Where("team_id = ?", id).Update("team_id", nil).Error; err != nil {
			return err
		}
		return tx.Delete(&models.Team{}, id).Error
	})
}

// SetMembers replaces a team's members.
func (r *TeamRepository) SetMembers(teamID uint, userIDs []uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("team_id = ?", teamID).Delete(&models.TeamMember{}).Error; err != nil {
			return err
		}
		for _, id := range userIDs {
			if err := tx.Create(&models.TeamMember{TeamID: teamID, UserID: id}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// IDsForUser lists the workspace's teams the user is on.
func (r *TeamRepository) IDsForUser(workspaceID, userID uint) ([]uint, error) {
	var ids []uint
	err := r.db.Model(&models.TeamMember{}).
		Joins("JOIN teams ON teams.id = team_members.team_id AND teams.deleted_at IS NULL").
		Where("teams.workspace_id = ? AND team_members.user_id = ?", workspaceID, userID).
		Pluck("team_members.team_id", &ids).Error
	return ids, err
}

// IDs lists the workspace's team IDs, for checking rule destinations.
func (r *TeamRepository) IDs(workspaceID uint) ([]uint, error) {
	var ids []uint
	err := r.db.Model(&models.Team{}).Where("workspace_id = ?", workspaceID).Pluck("id", &ids).Error
	return ids, err
}

// RoutingDirectory answers the lookups routing rules expand recipients
// with (formrules.Directory).
type RoutingDirectory struct {
	Workspaces *WorkspaceRepository
	Teams      *TeamRepository
}

func (d RoutingDirectory) Members(workspaceID uint) (map[uint]models.WorkspaceRole, error) {
	members, err := d.Workspaces.ListMembers(workspaceID)
	if err != nil {
		return nil, err
	}
	out := make(map[uint]models.WorkspaceRole, len(members))
	for _, m := range members {
		out[m.UserID] = m.Role
	}
	return out, nil
}

func (d RoutingDirectory) Team(id uint) (*models.Team, error) { return d.Teams.FindByID(id) }
