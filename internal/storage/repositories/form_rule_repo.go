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
	"fmt"
	"time"

	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type FormRuleRepository struct {
	db *gorm.DB
}

func NewFormRuleRepository(db *gorm.DB) *FormRuleRepository {
	return &FormRuleRepository{db: db}
}

// ListByForm returns a form's rules in evaluation order, disabled ones too.
func (r *FormRuleRepository) ListByForm(formID uint) ([]models.FormRule, error) {
	var rules []models.FormRule
	err := r.db.Where("form_id = ?", formID).Order("position ASC, id ASC").Find(&rules).Error
	return rules, err
}

func (r *FormRuleRepository) FindForForm(formID, id uint) (*models.FormRule, error) {
	var rule models.FormRule
	if err := r.db.Where("form_id = ?", formID).First(&rule, id).Error; err != nil {
		return nil, err
	}
	return &rule, nil
}

// Create appends the rule after the form's last one.
func (r *FormRuleRepository) Create(rule *models.FormRule) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var last struct{ Max *int }
		if err := tx.Model(&models.FormRule{}).Select("MAX(position) AS max").Where("form_id = ?", rule.FormID).Scan(&last).Error; err != nil {
			return err
		}
		rule.Position = 0
		if last.Max != nil {
			rule.Position = *last.Max + 1
		}
		return tx.Create(rule).Error
	})
}

func (r *FormRuleRepository) Update(rule *models.FormRule) error {
	return r.db.Omit(clause.Associations, "hit_count", "last_hit_at", "position").Save(rule).Error
}

func (r *FormRuleRepository) Delete(id uint) error {
	return r.db.Delete(&models.FormRule{}, id).Error
}

// Reorder sets the evaluation order to ids, which must be exactly the form's
// rules.
func (r *FormRuleRepository) Reorder(formID uint, ids []uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&models.FormRule{}).Where("form_id = ? AND id IN ?", formID, ids).Count(&count).Error; err != nil {
			return err
		}
		var total int64
		tx.Model(&models.FormRule{}).Where("form_id = ?", formID).Count(&total)
		if int(count) != len(ids) || count != total {
			return fmt.Errorf("the order must list each of the form's rules once")
		}
		for pos, id := range ids {
			if err := tx.Model(&models.FormRule{}).Where("id = ?", id).Update("position", pos).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// RecordHits counts a match for each rule, by UUID.
func (r *FormRuleRepository) RecordHits(uuids []string, at time.Time) error {
	if len(uuids) == 0 {
		return nil
	}
	return r.db.Model(&models.FormRule{}).Where("uuid IN ?", uuids).
		Updates(map[string]any{"hit_count": gorm.Expr("hit_count + 1"), "last_hit_at": at}).Error
}

// NamesByUUID names rules, for "Routed by" on a submission.
func (r *FormRuleRepository) NamesByUUID(uuids []string) (map[string]string, error) {
	out := map[string]string{}
	if len(uuids) == 0 {
		return out, nil
	}
	var rows []models.FormRule
	if err := r.db.Select("uuid", "name").Where("uuid IN ?", uuids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.UUID] = row.Name
	}
	return out, nil
}

// RuleUsingTeam is a rule that notifies or assigns a team, with its form.
type RuleUsingTeam struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	Enabled  bool   `json:"enabled"`
	FormID   uint   `json:"form_id"`
	FormName string `json:"form_name"`
}

// RulesUsingTeam lists a workspace's rules that notify or assign team teamID,
// by form then rule order.
func (r *FormRuleRepository) RulesUsingTeam(workspaceID, teamID uint) ([]RuleUsingTeam, error) {
	var out []RuleUsingTeam
	err := r.db.Table("form_rules").
		Select("form_rules.id, form_rules.name, form_rules.enabled, form_rules.form_id, forms.display_name AS form_name").
		Joins("JOIN forms ON forms.id = form_rules.form_id AND forms.deleted_at IS NULL").
		Where("form_rules.workspace_id = ? AND form_rules.actions @> ?::jsonb", workspaceID, fmt.Sprintf(`[{"team_id":%d}]`, teamID)).
		Order("LOWER(forms.display_name), form_rules.position").
		Scan(&out).Error
	return out, err
}

// UsingTeam names a workspace's rules that notify or assign team teamID.
// Rules of a deleted form never run and can no longer be edited, so they do
// not count.
func (r *FormRuleRepository) UsingTeam(workspaceID, teamID uint) ([]string, error) {
	rules, err := r.RulesUsingTeam(workspaceID, teamID)
	names := make([]string, 0, len(rules))
	for _, rule := range rules {
		names = append(names, rule.Name)
	}
	return names, err
}
