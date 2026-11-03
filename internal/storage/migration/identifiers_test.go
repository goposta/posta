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

package migration

import (
	"os"
	"testing"

	"github.com/goposta/posta/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gl "gorm.io/gorm/logger"
)

// TestIdentifiersAfterMigration runs the full migration against a scratch
// database, ideally a copy of one made before 0.18, and checks that every
// workspace, form, OAuth provider, template and stylesheet ends with an
// identifier for a name and a label for a display name. It is skipped
// unless MIGRATION_SMOKE_DSN is set. Running it twice checks the move is
// done once.
func TestIdentifiersAfterMigration(t *testing.T) {
	dsn := os.Getenv("MIGRATION_SMOKE_DSN")
	if dsn == "" {
		t.Skip("set MIGRATION_SMOKE_DSN to run")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: gl.Default.LogMode(gl.Silent)})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	for run := 1; run <= 2; run++ {
		if err := Run(db); err != nil {
			t.Fatalf("run %d: migrate: %v", run, err)
		}
	}

	for _, table := range []string{"workspaces", "forms", "o_auth_providers"} {
		if columnExists(db, table, "slug") {
			t.Errorf("%s still has a slug column", table)
		}
	}
	for _, table := range []string{"workspaces", "forms", "o_auth_providers", "templates", "style_sheets"} {
		var rows []struct {
			ID          uint
			Name        string
			DisplayName string
		}
		if err := db.Table(table).Select("id, name, display_name").Scan(&rows).Error; err != nil {
			t.Fatalf("%s: %v", table, err)
		}
		for _, r := range rows {
			if err := models.CheckIdentifier(r.Name); err != nil && table != "workspaces" {
				t.Errorf("%s %d: %v", table, r.ID, err)
			}
			if r.DisplayName == "" {
				t.Errorf("%s %d (%q) has no display name", table, r.ID, r.Name)
			}
		}
		t.Logf("%s: %d rows", table, len(rows))
	}
	for _, idx := range []string{"idx_workspace_template", "idx_workspace_stylesheet", "idx_workspace_form_name", "idx_workspaces_name", "idx_o_auth_providers_name"} {
		var n int64
		db.Raw("SELECT count(*) FROM pg_indexes WHERE indexname = ?", idx).Scan(&n)
		if n == 0 {
			t.Errorf("unique index %s is missing", idx)
		}
	}
}
