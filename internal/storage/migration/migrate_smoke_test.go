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

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gl "gorm.io/gorm/logger"
)

// TestMigrateSmoke is skipped unless MIGRATION_SMOKE_DSN is set.
func TestMigrateSmoke(t *testing.T) {
	dsn := os.Getenv("MIGRATION_SMOKE_DSN")
	if dsn == "" {
		t.Skip("set MIGRATION_SMOKE_DSN to run")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: gl.Default.LogMode(gl.Silent)})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := Run(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	var idx string
	if err := db.Raw(`SELECT indexdef FROM pg_indexes WHERE indexname = 'idx_notification_open'`).
		Scan(&idx).Error; err != nil || idx == "" {
		t.Fatalf("partial unique index missing: %v (%q)", err, idx)
	}
	t.Logf("index: %s", idx)
}
