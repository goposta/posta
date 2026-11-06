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
	"errors"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TemplateRepository struct {
	db *gorm.DB
}

func NewTemplateRepository(db *gorm.DB) *TemplateRepository {
	return &TemplateRepository{db: db}
}

func (r *TemplateRepository) Create(tmpl *models.Template) error {
	return r.db.Create(tmpl).Error
}

func (r *TemplateRepository) Update(tmpl *models.Template) error {
	return r.db.Omit(clause.Associations).Save(tmpl).Error
}

func (r *TemplateRepository) Delete(id uint) error {
	return r.db.Delete(&models.Template{}, id).Error
}

func (r *TemplateRepository) FindByID(id uint) (*models.Template, error) {
	var tmpl models.Template
	if err := r.db.First(&tmpl, id).Error; err != nil {
		return nil, err
	}
	return &tmpl, nil
}

func (r *TemplateRepository) FindByIDWithActors(id uint) (*models.Template, error) {
	var tmpl models.Template
	if err := r.db.
		Preload("ActiveVersion").
		Preload("CreatedBy").
		Preload("LastEditedBy").
		First(&tmpl, id).Error; err != nil {
		return nil, err
	}
	return &tmpl, nil
}

func (r *TemplateRepository) TouchEditor(id, editorID uint) error {
	return r.db.Model(&models.Template{}).Where("id = ?", id).
		Updates(map[string]any{"last_edited_by_id": editorID, "updated_at": time.Now()}).Error
}

func (r *TemplateRepository) FindByWorkspaceID(workspaceID uint, limit, offset int) ([]models.Template, int64, error) {
	var templates []models.Template
	var total int64

	r.db.Model(&models.Template{}).Where("workspace_id = ?", workspaceID).Count(&total)

	if err := r.db.Where("workspace_id = ?", workspaceID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&templates).Error; err != nil {
		return nil, 0, err
	}
	return templates, total, nil
}

// FindByScope preloads ActiveVersion and the actor refs because the list renders all three.
func (r *TemplateRepository) FindByScope(scope ResourceScope, search string, limit, offset int) ([]models.Template, int64, error) {
	var items []models.Template
	var total int64

	filter := func(q *gorm.DB) *gorm.DB {
		q = ApplyScope(q, scope)
		if search != "" {
			q = q.Where("name ILIKE ? OR display_name ILIKE ? OR description ILIKE ?", "%"+search+"%", "%"+search+"%", "%"+search+"%")
		}
		return q
	}

	if err := filter(r.db.Model(&models.Template{})).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := filter(r.db.Model(&models.Template{})).
		Preload("ActiveVersion").
		Preload("CreatedBy").
		Preload("LastEditedBy").
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// FindByWorkspaceDisplayName serves callers still sending pre-0.18 names; the oldest wins when two share a label.
func (r *TemplateRepository) FindByWorkspaceDisplayName(workspaceID uint, displayName string) (*models.Template, error) {
	var tmpl models.Template
	if err := r.db.Where("workspace_id = ? AND display_name = ?", workspaceID, displayName).Order("id").First(&tmpl).Error; err != nil {
		return nil, err
	}
	return &tmpl, nil
}

func (r *TemplateRepository) FindByWorkspaceName(workspaceID uint, name string) (*models.Template, error) {
	var tmpl models.Template
	if err := r.db.Where("workspace_id = ? AND name = ?", workspaceID, name).First(&tmpl).Error; err != nil {
		return nil, err
	}
	return &tmpl, nil
}

// ImportLocalization is one language's content for a template being created.
type ImportLocalization struct {
	Language        string
	SubjectTemplate string
	HTMLTemplate    string
	TextTemplate    string
	BuilderJSON     string
}

// ImportVersion is one version of a template being created, with its content.
type ImportVersion struct {
	SampleData    string
	StyleSheetID  *uint
	IsActive      bool
	Localizations []ImportLocalization
}

// CreateWithVersions is atomic so no template lacks an active version; versions are numbered from 1 in given order.
func (r *TemplateRepository) CreateWithVersions(tmpl *models.Template, versions []ImportVersion) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(tmpl).Error; err != nil {
			return err
		}

		var activeID *uint
		for i, in := range versions {
			v := &models.TemplateVersion{
				TemplateID:   tmpl.ID,
				Version:      i + 1,
				SampleData:   in.SampleData,
				StyleSheetID: in.StyleSheetID,
			}
			if err := tx.Create(v).Error; err != nil {
				return err
			}
			// The first version is activated so the template is usable at once; an explicitly active one wins.
			if in.IsActive || i == 0 {
				id := v.ID
				activeID = &id
			}

			for _, l := range in.Localizations {
				loc := &models.TemplateLocalization{
					VersionID:       v.ID,
					Language:        l.Language,
					SubjectTemplate: l.SubjectTemplate,
					HTMLTemplate:    l.HTMLTemplate,
					TextTemplate:    l.TextTemplate,
					BuilderJSON:     l.BuilderJSON,
				}
				if err := tx.Create(loc).Error; err != nil {
					return err
				}
			}
		}

		if activeID == nil {
			return nil
		}
		tmpl.ActiveVersionID = activeID
		return tx.Model(tmpl).Update("active_version_id", *activeID).Error
	})
}

// IsDuplicateName reads the driver error, since gorm.ErrDuplicatedKey only exists with TranslateError.
func IsDuplicateName(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == uniqueViolationCode
	}
	return false
}

const uniqueViolationCode = "23505"

// LanguagesForActiveVersions aggregates codes instead of preloading localizations, which would pull every HTML body.
func (r *TemplateRepository) LanguagesForActiveVersions(templateIDs []uint) (map[uint][]string, error) {
	out := make(map[uint][]string, len(templateIDs))
	if len(templateIDs) == 0 {
		return out, nil
	}

	var rows []struct {
		TemplateID uint
		Language   string
	}
	err := r.db.Table("templates t").
		Select("t.id as template_id, l.language").
		Joins("JOIN template_localizations l ON l.version_id = t.active_version_id").
		Where("t.id IN ?", templateIDs).
		Order("t.id, l.language").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		out[row.TemplateID] = append(out[row.TemplateID], row.Language)
	}
	return out, nil
}
