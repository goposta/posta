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

package seeder

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/messages"
	"github.com/goposta/posta/internal/storage/migration"
	"github.com/goposta/posta/internal/storage/repositories"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestDefaultFormsAreValid checks each example form the way the API checks a
// form it creates, and that none of them uses branching: the seed must not
// hand Posta Community an Enterprise feature.
func TestDefaultFormsAreValid(t *testing.T) {
	defs := defaultFormDefs()
	if len(defs) == 0 {
		t.Fatal("no default forms defined")
	}
	slugs := map[string]bool{}
	var sawCollect, sawSections bool
	for _, def := range defs {
		t.Run(def.Name, func(t *testing.T) {
			if slugs[def.Name] {
				t.Fatalf("slug %q is used twice", def.Name)
			}
			slugs[def.Name] = true

			fields := append(models.FieldDefs{}, def.Fields...)
			if err := messages.CheckSchema(fields, nil, models.DefaultHoneypotField); err != nil {
				t.Fatalf("schema: %v", err)
			}
			if b := messages.BranchTargets(fields); len(b) > 0 {
				t.Fatalf("uses branching (Enterprise): %v", b)
			}
			if n := len(fields.Active().Questions()); n+3 > models.DefaultMaxFields {
				t.Fatalf("%d questions exceed max_fields %d", n, models.DefaultMaxFields)
			}
			sections := 0
			for _, f := range fields {
				if f.IsSection() {
					sections++
				}
			}
			if def.Mode == models.FormModeCollect {
				sawCollect = true
			}
			if sections > 1 {
				sawSections = true
			}
		})
	}
	if !sawCollect {
		t.Error("no collect-mode example")
	}
	if !sawSections {
		t.Error("no example with several sections")
	}
}

// TestSeedWorkspaceDefaultsCreatesForms seeds a new workspace, in a
// transaction that is rolled back, and seeds it again: the example forms
// are created once.
func TestSeedWorkspaceDefaultsCreatesForms(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		dsn = "host=localhost user=posta password=posta dbname=posta port=5432 sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Skipf("skipping: no test database available: %v", err)
	}
	tx := db.Begin()
	defer tx.Rollback()

	if err := migration.Run(tx); err != nil {
		t.Skipf("skipping: cannot migrate test schema: %v", err)
	}

	suffix := time.Now().UnixNano()
	user := &models.User{Name: "seed test", Email: fmt.Sprintf("seed-%d@example.test", suffix), PasswordHash: "x"}
	if err := tx.Create(user).Error; err != nil {
		t.Skipf("skipping: cannot create user: %v", err)
	}
	ws := &models.Workspace{DisplayName: "seed test", Name: fmt.Sprintf("seed-%d", suffix), OwnerID: user.ID}
	if err := tx.Create(ws).Error; err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	formRepo := repositories.NewFormRepository(tx)
	s := New(
		repositories.NewTemplateRepository(tx),
		repositories.NewStyleSheetRepository(tx),
		repositories.NewTemplateVersionRepository(tx),
		repositories.NewTemplateLocalizationRepository(tx),
		repositories.NewLanguageRepository(tx),
		formRepo,
	)
	scope := repositories.ResourceScope{UserID: user.ID, WorkspaceID: &ws.ID}
	for run := 1; run <= 2; run++ {
		s.SeedWorkspaceDefaults(ws.ID, user.ID, user.Name)
		forms, total, err := formRepo.FindByScope(scope, 10, 0)
		if err != nil {
			t.Fatalf("run %d: list forms: %v", run, err)
		}
		if int(total) != len(defaultFormDefs()) {
			t.Fatalf("run %d: %d forms, want %d", run, total, len(defaultFormDefs()))
		}
		for _, f := range forms {
			if len(f.PublicKey) != models.FormPublicKeyLength || len(f.Fields) == 0 || f.Status != models.FormStatusActive {
				t.Fatalf("run %d: form %q seeded incompletely: %+v", run, f.Name, f)
			}
		}
	}
}
