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
	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
)

type PlanRepository struct {
	db *gorm.DB
}

func NewPlanRepository(db *gorm.DB) *PlanRepository {
	return &PlanRepository{db: db}
}

func (r *PlanRepository) Create(plan *models.Plan) error {
	return r.db.Create(plan).Error
}

func (r *PlanRepository) Update(plan *models.Plan) error {
	return r.db.Save(plan).Error
}

func (r *PlanRepository) Delete(id uint) error {
	return r.db.Delete(&models.Plan{}, id).Error
}

func (r *PlanRepository) FindByID(id uint) (*models.Plan, error) {
	var plan models.Plan
	if err := r.db.First(&plan, id).Error; err != nil {
		return nil, err
	}
	return &plan, nil
}

func (r *PlanRepository) FindAll(limit, offset int) ([]models.Plan, int64, error) {
	var plans []models.Plan
	var total int64

	r.db.Model(&models.Plan{}).Count(&total)

	if err := r.db.Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&plans).Error; err != nil {
		return nil, 0, err
	}
	return plans, total, nil
}

// FindDefault returns the active default plan, or gorm.ErrRecordNotFound if none exists.
func (r *PlanRepository) FindDefault() (*models.Plan, error) {
	var plan models.Plan
	if err := r.db.Where("is_default = ? AND is_active = ?", true, true).First(&plan).Error; err != nil {
		return nil, err
	}
	return &plan, nil
}

// FindByWorkspaceID returns the plan assigned to the given workspace.
func (r *PlanRepository) FindByWorkspaceID(workspaceID uint) (*models.Plan, error) {
	var ws models.Workspace
	if err := r.db.First(&ws, workspaceID).Error; err != nil {
		return nil, err
	}
	if ws.PlanID == nil {
		return nil, gorm.ErrRecordNotFound
	}
	var plan models.Plan
	if err := r.db.First(&plan, *ws.PlanID).Error; err != nil {
		return nil, err
	}
	return &plan, nil
}

// ClearDefault unsets is_default on all plans. Should be called within a transaction
// before setting a new default.
func (r *PlanRepository) ClearDefault() error {
	return r.db.Model(&models.Plan{}).Where("is_default = ?", true).Update("is_default", false).Error
}

// ClearDefaultTx unsets is_default on all plans within the given transaction.
func (r *PlanRepository) ClearDefaultTx(tx *gorm.DB) error {
	return tx.Model(&models.Plan{}).Where("is_default = ?", true).Update("is_default", false).Error
}

// CountWorkspaces returns the number of workspaces assigned to the given plan.
func (r *PlanRepository) CountWorkspaces(planID uint) (int64, error) {
	var count int64
	if err := r.db.Model(&models.Workspace{}).Where("plan_id = ?", planID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// AssignToWorkspace sets the plan_id on the given workspace.
func (r *PlanRepository) AssignToWorkspace(workspaceID, planID uint) error {
	return r.db.Model(&models.Workspace{}).Where("id = ?", workspaceID).Update("plan_id", planID).Error
}

// UnassignFromWorkspace removes the plan assignment from the given workspace.
func (r *PlanRepository) UnassignFromWorkspace(workspaceID uint) error {
	return r.db.Model(&models.Workspace{}).Where("id = ?", workspaceID).Update("plan_id", nil).Error
}

// UnassignAllFromPlan removes plan assignment from all workspaces using the given plan.
func (r *PlanRepository) UnassignAllFromPlan(planID uint) error {
	return r.db.Model(&models.Workspace{}).Where("plan_id = ?", planID).Update("plan_id", nil).Error
}

// DB returns the underlying database connection for use in transactions.
func (r *PlanRepository) DB() *gorm.DB {
	return r.db
}
