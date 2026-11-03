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

	tmpl := &models.Template{UserID: creator, Name: "attr", LastEditedByID: &creator}
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

	got.DisplayName = "Attr Renamed"
	if err := repo.Update(got); err != nil {
		t.Fatalf("update preloaded template: %v", err)
	}
	reloaded, err := repo.FindByIDWithActors(tmpl.ID)
	if err != nil {
		t.Fatalf("find by id after update: %v", err)
	}
	if reloaded.DisplayName != "Attr Renamed" {
		t.Errorf("Name = %q, want %q", reloaded.DisplayName, "Attr Renamed")
	}
	if reloaded.LastEditedBy == nil || reloaded.LastEditedBy.ID != editor {
		t.Errorf("LastEditedBy after update = %+v, want id %d", reloaded.LastEditedBy, editor)
	}
}
