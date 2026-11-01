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
package upgrade

import (
	"context"
	"os"
	"testing"

	"github.com/goposta/posta/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// A dev binary never writes app.version, so the database it created looks
// unversioned on the next boot. That boot must not seal the registry again.
func TestDevBinaryRestartsOnItsOwnDatabase(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		dsn = "host=localhost user=posta password=posta dbname=posta port=5432 sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Skipf("skipping: no test database available: %v", err)
	}
	if err := db.AutoMigrate(&models.Setting{}, &models.UpgradeStep{}); err != nil {
		t.Skipf("skipping: cannot migrate test schema: %v", err)
	}
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })

	if err := tx.Exec("DELETE FROM upgrade_steps").Error; err != nil {
		t.Fatalf("clear steps: %v", err)
	}
	if err := tx.Where("key = ?", KeyAppVersion).Delete(&models.Setting{}).Error; err != nil {
		t.Fatalf("clear version: %v", err)
	}

	for boot := 1; boot <= 2; boot++ {
		if err := runLocked(context.Background(), tx, devVersion, Options{}); err != nil {
			t.Fatalf("boot %d: %v", boot, err)
		}
	}

	var steps int64
	if err := tx.Model(&models.UpgradeStep{}).Count(&steps).Error; err != nil {
		t.Fatalf("count steps: %v", err)
	}
	if steps != int64(len(registry)) {
		t.Fatalf("recorded %d steps, want %d", steps, len(registry))
	}
	if _, fresh, err := readVersion(tx); err != nil || !fresh {
		t.Fatalf("dev boots wrote app.version (fresh=%v, err=%v); the regression needs it absent", fresh, err)
	}
}
