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
	"testing"

	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
)

func templateDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := testDB(t)
	if err := db.AutoMigrate(
		&models.User{}, &models.Workspace{}, &models.Template{},
		&models.TemplateVersion{}, &models.TemplateLocalization{},
	); err != nil {
		t.Skipf("skipping: cannot migrate schema: %v", err)
	}
	return db
}

func seedTemplateWorkspace(t *testing.T, tx *gorm.DB, slug string) (uint, uint) {
	t.Helper()
	userID := createUser(t, tx, slug+"@test.local")
	ws := &models.Workspace{DisplayName: "Acme", Name: slug, OwnerID: userID}
	if err := tx.Create(ws).Error; err != nil {
		t.Fatalf("workspace: %v", err)
	}
	return userID, ws.ID
}

// The list renders the active version number. Without the preload it had
// nothing to read and printed "v?" against every template that had one.
func TestFindByScopePreloadsActiveVersion(t *testing.T) {
	db := templateDB(t)
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })

	userID, wsID := seedTemplateWorkspace(t, tx, "tmpl-preload")
	repo := &TemplateRepository{db: tx}

	tmpl := &models.Template{UserID: userID, WorkspaceID: &wsID, Name: "welcome", DefaultLanguage: "en"}
	if err := repo.CreateWithVersions(tmpl, []ImportVersion{
		{SampleData: "{}"},
		{SampleData: "{}", IsActive: true},
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	items, total, err := repo.FindByScope(ResourceScope{UserID: userID, WorkspaceID: &wsID}, "", 20, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("expected 1 template, got %d (total %d)", len(items), total)
	}
	if items[0].ActiveVersion == nil {
		t.Fatal("active version was not preloaded; the list can only render \"v?\"")
	}
	if items[0].ActiveVersion.Version != 2 {
		t.Errorf("expected the explicitly active version 2, got v%d", items[0].ActiveVersion.Version)
	}
}

// A template with no active version can never be sent. Creating the rows
// separately left exactly that behind whenever a later step failed.
func TestCreateWithVersionsIsAtomic(t *testing.T) {
	db := templateDB(t)
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })

	userID, wsID := seedTemplateWorkspace(t, tx, "tmpl-atomic")
	repo := &TemplateRepository{db: tx}

	// Two localizations for the same language violate idx_version_language, so
	// the second version fails after the template and first version are written.
	tmpl := &models.Template{UserID: userID, WorkspaceID: &wsID, Name: "broken", DefaultLanguage: "en"}
	err := repo.CreateWithVersions(tmpl, []ImportVersion{
		{Localizations: []ImportLocalization{
			{Language: "en", SubjectTemplate: "Hi"},
			{Language: "en", SubjectTemplate: "Hi again"},
		}},
	})
	if err == nil {
		t.Fatal("expected the duplicate language to be rejected")
	}

	var templates int64
	tx.Model(&models.Template{}).Where("workspace_id = ?", wsID).Count(&templates)
	if templates != 0 {
		t.Errorf("a failed create left %d template(s) behind", templates)
	}
	var versions int64
	tx.Model(&models.TemplateVersion{}).Where("template_id = ?", tmpl.ID).Count(&versions)
	if versions != 0 {
		t.Errorf("a failed create left %d version(s) behind", versions)
	}
	var locs int64
	tx.Model(&models.TemplateLocalization{}).
		Where("version_id IN (SELECT id FROM template_versions WHERE template_id = ?)", tmpl.ID).
		Count(&locs)
	if locs != 0 {
		t.Errorf("a failed create left %d localization(s) behind", locs)
	}
}

// Every successful create must leave the template immediately usable.
func TestCreateAlwaysActivatesAVersion(t *testing.T) {
	db := templateDB(t)
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })

	userID, wsID := seedTemplateWorkspace(t, tx, "tmpl-active")
	repo := &TemplateRepository{db: tx}

	// No version is marked active, as an older export would be.
	tmpl := &models.Template{UserID: userID, WorkspaceID: &wsID, Name: "fallback", DefaultLanguage: "en"}
	if err := repo.CreateWithVersions(tmpl, []ImportVersion{{SampleData: "{}"}, {SampleData: "{}"}}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if tmpl.ActiveVersionID == nil {
		t.Fatal("no version was activated; the template could never be sent")
	}

	stored, err := repo.FindByIDWithActors(tmpl.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.ActiveVersionID == nil || *stored.ActiveVersionID != *tmpl.ActiveVersionID {
		t.Error("the activation pointer was not persisted")
	}
	if stored.ActiveVersion == nil || stored.ActiveVersion.Version != 1 {
		t.Error("expected the first version to be activated by default")
	}
}

// Versions are numbered from 1 in source order, so an import reproduces the
// ordering it was given.
func TestImportedVersionsAreNumberedInOrder(t *testing.T) {
	db := templateDB(t)
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })

	userID, wsID := seedTemplateWorkspace(t, tx, "tmpl-order")
	repo := &TemplateRepository{db: tx}

	tmpl := &models.Template{UserID: userID, WorkspaceID: &wsID, Name: "ordered", DefaultLanguage: "en"}
	if err := repo.CreateWithVersions(tmpl, []ImportVersion{{}, {}, {IsActive: true}}); err != nil {
		t.Fatalf("create: %v", err)
	}

	var versions []models.TemplateVersion
	tx.Where("template_id = ?", tmpl.ID).Order("version ASC").Find(&versions)
	if len(versions) != 3 {
		t.Fatalf("expected 3 versions, got %d", len(versions))
	}
	for i, v := range versions {
		if v.Version != i+1 {
			t.Errorf("version %d numbered %d", i, v.Version)
		}
	}
	if tmpl.ActiveVersionID == nil || *tmpl.ActiveVersionID != versions[2].ID {
		t.Error("the explicitly active version should win over the first")
	}
}

// The builder document has to survive creation, or a template designed visually
// comes back editable only as raw HTML.
func TestBuilderJSONSurvivesCreate(t *testing.T) {
	db := templateDB(t)
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })

	userID, wsID := seedTemplateWorkspace(t, tx, "tmpl-builder")
	repo := &TemplateRepository{db: tx}

	const doc = `{"pages":[{"component":"hero"}]}`
	tmpl := &models.Template{UserID: userID, WorkspaceID: &wsID, Name: "designed", DefaultLanguage: "en"}
	if err := repo.CreateWithVersions(tmpl, []ImportVersion{{
		IsActive: true,
		Localizations: []ImportLocalization{
			{Language: "en", SubjectTemplate: "Hello", HTMLTemplate: "<p>hi</p>", BuilderJSON: doc},
		},
	}}); err != nil {
		t.Fatalf("create: %v", err)
	}

	var loc models.TemplateLocalization
	if err := tx.Where("version_id = ?", *tmpl.ActiveVersionID).First(&loc).Error; err != nil {
		t.Fatalf("localization: %v", err)
	}
	if loc.BuilderJSON != doc {
		t.Errorf("builder document lost: got %q", loc.BuilderJSON)
	}
}

// A duplicate name has to be recognisable so the API can answer with a conflict
// naming the problem instead of a generic failure.
func TestDuplicateNameIsDetectable(t *testing.T) {
	db := templateDB(t)
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })

	userID, wsID := seedTemplateWorkspace(t, tx, "tmpl-dupe")
	repo := &TemplateRepository{db: tx}

	first := &models.Template{UserID: userID, WorkspaceID: &wsID, Name: "welcome", DefaultLanguage: "en"}
	if err := repo.CreateWithVersions(first, []ImportVersion{{IsActive: true}}); err != nil {
		t.Fatalf("first create: %v", err)
	}

	// The unique index lives in constraints.go, not the model, so AutoMigrate
	// alone does not create it; add it here to exercise the real behaviour.
	if err := tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_workspace_template
		ON templates (workspace_id, name) WHERE workspace_id IS NOT NULL`).Error; err != nil {
		t.Skipf("cannot create the unique index: %v", err)
	}

	second := &models.Template{UserID: userID, WorkspaceID: &wsID, Name: "welcome", DefaultLanguage: "en"}
	err := repo.CreateWithVersions(second, []ImportVersion{{IsActive: true}})
	if err == nil {
		t.Fatal("expected the duplicate name to be rejected")
	}
	if !IsDuplicateName(err) {
		t.Errorf("IsDuplicateName did not recognise the violation: %v", err)
	}
}

// The list shows which languages a template is actually translated into. It has
// to read the active version only: languages from an older version would claim
// coverage the template no longer sends.
func TestLanguagesForActiveVersions(t *testing.T) {
	db := templateDB(t)
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })

	userID, wsID := seedTemplateWorkspace(t, tx, "tmpl-langs")
	repo := &TemplateRepository{db: tx}

	tmpl := &models.Template{UserID: userID, WorkspaceID: &wsID, Name: "multi", DefaultLanguage: "en"}
	if err := repo.CreateWithVersions(tmpl, []ImportVersion{
		{Localizations: []ImportLocalization{{Language: "ja", SubjectTemplate: "old"}}},
		{IsActive: true, Localizations: []ImportLocalization{
			{Language: "fr", SubjectTemplate: "Bonjour"},
			{Language: "en", SubjectTemplate: "Hello"},
		}},
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	empty := &models.Template{UserID: userID, WorkspaceID: &wsID, Name: "empty", DefaultLanguage: "en"}
	if err := repo.CreateWithVersions(empty, []ImportVersion{{IsActive: true}}); err != nil {
		t.Fatalf("create empty: %v", err)
	}

	got, err := repo.LanguagesForActiveVersions([]uint{tmpl.ID, empty.ID})
	if err != nil {
		t.Fatalf("languages: %v", err)
	}

	langs := got[tmpl.ID]
	if len(langs) != 2 || langs[0] != "en" || langs[1] != "fr" {
		t.Errorf("expected [en fr] sorted, got %v", langs)
	}
	if _, ok := got[empty.ID]; ok {
		t.Errorf("a template with no localizations should have no languages, got %v", got[empty.ID])
	}
}

func TestLanguagesForNoTemplates(t *testing.T) {
	db := templateDB(t)
	repo := &TemplateRepository{db: db}
	got, err := repo.LanguagesForActiveVersions(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected an empty map, got %v", got)
	}
}
