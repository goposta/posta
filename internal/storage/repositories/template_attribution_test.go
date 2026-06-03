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
	"testing"

	"github.com/goposta/posta/internal/models"
)

func TestTemplateAttribution(t *testing.T) {
	db := testDB(t)
	if err := db.AutoMigrate(&models.Template{}); err != nil {
		t.Skipf("skipping: cannot migrate templates: %v", err)
	}
	tx := db.Begin()
	defer tx.Rollback()

	creator := createUser(t, tx, "creator@attr.test")
	editor := createUser(t, tx, "editor@attr.test")
	repo := &TemplateRepository{db: tx}

	tmpl := &models.Template{UserID: creator, Name: "Attr", LastEditedByID: &creator}
	if err := repo.Create(tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}

	// Initially both creator and last editor are the creator.
	got, err := repo.FindByIDWithActors(tmpl.ID)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}
	if got.CreatedBy == nil || got.CreatedBy.ID != creator {
		t.Fatalf("CreatedBy = %+v, want id %d", got.CreatedBy, creator)
	}
	if got.CreatedBy.Name != "test" {
		t.Errorf("CreatedBy.Name = %q, want %q", got.CreatedBy.Name, "test")
	}
	if got.LastEditedBy == nil || got.LastEditedBy.ID != creator {
		t.Fatalf("LastEditedBy = %+v, want id %d", got.LastEditedBy, creator)
	}

	if err := repo.TouchEditor(tmpl.ID, editor); err != nil {
		t.Fatalf("touch editor: %v", err)
	}
	got, err = repo.FindByIDWithActors(tmpl.ID)
	if err != nil {
		t.Fatalf("find by id after touch: %v", err)
	}
	if got.CreatedBy.ID != creator {
		t.Errorf("CreatedBy.ID = %d, want %d (creator must not change)", got.CreatedBy.ID, creator)
	}
	if got.LastEditedBy == nil || got.LastEditedBy.ID != editor {
		t.Fatalf("LastEditedBy = %+v, want id %d", got.LastEditedBy, editor)
	}

	got.Name = "Attr Renamed"
	if err := repo.Update(got); err != nil {
		t.Fatalf("update preloaded template: %v", err)
	}
	reloaded, err := repo.FindByIDWithActors(tmpl.ID)
	if err != nil {
		t.Fatalf("find by id after update: %v", err)
	}
	if reloaded.Name != "Attr Renamed" {
		t.Errorf("Name = %q, want %q", reloaded.Name, "Attr Renamed")
	}
	if reloaded.LastEditedBy == nil || reloaded.LastEditedBy.ID != editor {
		t.Errorf("LastEditedBy after update = %+v, want id %d", reloaded.LastEditedBy, editor)
	}
}
