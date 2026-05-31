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
	"errors"
	"os"
	"testing"

	"github.com/goposta/posta/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		dsn = "host=localhost user=posta password=posta dbname=posta port=5432 sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Skipf("skipping: no test database available: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.StyleSheet{}); err != nil {
		t.Skipf("skipping: cannot migrate test schema: %v", err)
	}
	return db
}

func uintPtr(v uint) *uint { return &v }

func createUser(t *testing.T, tx *gorm.DB, email string) uint {
	t.Helper()
	u := &models.User{Name: "test", Email: email, PasswordHash: "x"}
	if err := tx.Create(u).Error; err != nil {
		t.Fatalf("create user %s: %v", email, err)
	}
	return u.ID
}

func TestFindByIDInScope_RejectsForeignWorkspace(t *testing.T) {
	db := testDB(t)

	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })

	repo := NewStyleSheetRepository(tx)

	userA := createUser(t, tx, "scope-a@test.local")
	userB := createUser(t, tx, "scope-b@test.local")

	const wsA, wsB uint = 90001, 90002
	owned := &models.StyleSheet{UserID: userA, WorkspaceID: uintPtr(wsA), Name: "owned", CSS: "body{color:red}"}
	foreign := &models.StyleSheet{UserID: userB, WorkspaceID: uintPtr(wsB), Name: "foreign", CSS: "body{color:blue}"}
	if err := repo.Create(owned); err != nil {
		t.Fatalf("create owned: %v", err)
	}
	if err := repo.Create(foreign); err != nil {
		t.Fatalf("create foreign: %v", err)
	}

	scopeA := ResourceScope{UserID: userA, WorkspaceID: uintPtr(wsA)}

	got, err := repo.FindByIDInScope(scopeA, owned.ID)
	if err != nil {
		t.Fatalf("expected in-scope stylesheet to resolve, got error: %v", err)
	}
	if got.ID != owned.ID {
		t.Fatalf("expected stylesheet %d, got %d", owned.ID, got.ID)
	}

	if _, err := repo.FindByIDInScope(scopeA, foreign.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected ErrRecordNotFound for foreign stylesheet, got: %v", err)
	}
}

func TestFindByIDInScope_PersonalScopeIsolated(t *testing.T) {
	db := testDB(t)

	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })

	repo := NewStyleSheetRepository(tx)

	userID := createUser(t, tx, "personal-scope@test.local")
	wsOwned := &models.StyleSheet{UserID: userID, WorkspaceID: uintPtr(90003), Name: "ws", CSS: "a{}"}
	if err := repo.Create(wsOwned); err != nil {
		t.Fatalf("create: %v", err)
	}

	personal := ResourceScope{UserID: userID}
	if _, err := repo.FindByIDInScope(personal, wsOwned.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected personal scope to not resolve workspace stylesheet, got: %v", err)
	}
}
