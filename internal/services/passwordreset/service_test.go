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

package passwordreset

import (
	"os"
	"testing"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/storage/repositories"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setup(t *testing.T) (*Service, *repositories.UserRepository, *repositories.PasswordResetRepository) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		dsn = "host=localhost user=posta password=posta dbname=posta port=5432 sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Skipf("skipping: no test database available: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.PasswordResetToken{}); err != nil {
		t.Skipf("skipping: cannot migrate schema: %v", err)
	}
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })
	userRepo := repositories.NewUserRepository(tx)
	tokenRepo := repositories.NewPasswordResetRepository(tx)
	return NewService(userRepo, tokenRepo, nil, "https://app.test"), userRepo, tokenRepo
}

func mkUser(t *testing.T, r *repositories.UserRepository, email string) *models.User {
	t.Helper()
	hash, _ := bcrypt.GenerateFromPassword([]byte("oldpassword"), bcrypt.DefaultCost)
	u := &models.User{Name: "T", Email: email, PasswordHash: string(hash)}
	if err := r.Create(u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u
}

// issue mints a token row directly and returns the raw token.
func issue(t *testing.T, r *repositories.PasswordResetRepository, userID uint, expires time.Time) string {
	t.Helper()
	raw, hash, err := newToken()
	if err != nil {
		t.Fatalf("newToken: %v", err)
	}
	if err := r.Create(&models.PasswordResetToken{UserID: userID, TokenHash: hash, ExpiresAt: expires, CreatedAt: time.Now()}); err != nil {
		t.Fatalf("create token: %v", err)
	}
	return raw
}

func TestRedeemSetsNewPasswordAndBurnsToken(t *testing.T) {
	svc, userRepo, tokenRepo := setup(t)
	user := mkUser(t, userRepo, "reset@pr.test")
	raw := issue(t, tokenRepo, user.ID, time.Now().Add(time.Hour))

	got, err := svc.Redeem(raw, "brand-new-password")
	if err != nil {
		t.Fatalf("Redeem: %v", err)
	}
	if got.ID != user.ID {
		t.Fatalf("Redeem returned user %d, want %d", got.ID, user.ID)
	}

	reloaded, _ := userRepo.FindByID(user.ID)
	if bcrypt.CompareHashAndPassword([]byte(reloaded.PasswordHash), []byte("brand-new-password")) != nil {
		t.Error("password was not updated to the new value")
	}

	// Single-use: the same token can't be redeemed again.
	if _, err := svc.Redeem(raw, "another-password"); err == nil {
		t.Error("expected reused token to be rejected")
	}
}

func TestRedeemRejectsExpiredAndInvalidTokens(t *testing.T) {
	svc, userRepo, tokenRepo := setup(t)
	user := mkUser(t, userRepo, "expired@pr.test")

	expired := issue(t, tokenRepo, user.ID, time.Now().Add(-time.Minute))
	if _, err := svc.Redeem(expired, "whatever12"); err == nil {
		t.Error("expected expired token to be rejected")
	}
	if _, err := svc.Redeem("not-a-real-token", "whatever12"); err == nil {
		t.Error("expected unknown token to be rejected")
	}
	if _, err := svc.Redeem("", "whatever12"); err == nil {
		t.Error("expected empty token to be rejected")
	}
}
