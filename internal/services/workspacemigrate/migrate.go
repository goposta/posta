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

package workspacemigrate

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/jkaninda/logger"
	"gorm.io/gorm"
)

// operationalTables are the user-scoped resources whose rows are re-scoped to
// the personal workspace. Each has a `user_id NOT NULL` + nullable `workspace_id`.
// Order is irrelevant — every table is backfilled by user_id in one pass.
var operationalTables = []interface{}{
	&models.APIKey{},
	&models.Template{},
	&models.StyleSheet{},
	&models.Language{},
	&models.Domain{},
	&models.SMTPServer{},
	&models.Webhook{},
	&models.Contact{},
	&models.Subscriber{},
	&models.SubscriberList{},
	&models.UnsubscribeList{},
	&models.Suppression{},
	&models.Bounce{},
	&models.Email{},
	&models.InboundEmail{},
	&models.Campaign{},
}

// Seeder provisions default content (stylesheet, templates, languages) for a
// freshly-created personal workspace. Implemented by services/seeder.Seeder.
// It is only invoked on the per-user hook path (register/login/oauth), not the
// bulk backfill — existing users already own their content.
type Seeder interface {
	SeedWorkspaceDefaults(workspaceID, userID uint, userName string)
}

// Service performs personal-workspace migrations.
type Service struct {
	// planEnforcement mirrors config.PlanEnforcement. When true a default plan
	// must exist and is assigned to each personal workspace; when false (OSS
	// default) workspaces get plan_id = NULL. See plan §1.
	planEnforcement bool

	// seeder, when set, seeds default content into a newly-created personal
	// workspace after MigrateUser commits. nil on the backfill path.
	seeder Seeder
}

// New constructs a migration service.
func New(planEnforcement bool) *Service {
	return &Service{planEnforcement: planEnforcement}
}

// SetSeeder attaches a seeder so MigrateUser provisions default content for new
// personal workspaces. Folds the old fire-and-forget SeedUserDefaults call into
// the migration (§4).
func (s *Service) SetSeeder(seeder Seeder) {
	s.seeder = seeder
}

func (s *Service) MigrateUser(db *gorm.DB, userID uint) (uint, error) {
	var existing models.User
	if err := db.Select("personal_workspace_id", "name").First(&existing, userID).Error; err == nil &&
		existing.PersonalWorkspaceID != nil {
		return *existing.PersonalWorkspaceID, nil
	}

	var wsID uint
	err := db.Transaction(func(tx *gorm.DB) error {
		id, e := s.migrate(tx, userID)
		wsID = id
		return e
	})
	if err != nil {
		var u models.User
		if e := db.First(&u, userID).Error; e == nil && u.PersonalWorkspaceID != nil {
			return *u.PersonalWorkspaceID, nil
		}
		return 0, err
	}

	// Seed default content after commit so the workspace row is visible. The
	// seeder is idempotent (skips users who already own templates), so this is a
	// no-op for users who were re-scoped rather than freshly created.
	if s.seeder != nil {
		s.seeder.SeedWorkspaceDefaults(wsID, userID, existing.Name)
	}

	return wsID, nil
}

func (s *Service) MigrateAllUnmigrated(tx *gorm.DB) error {
	var ids []uint
	if err := tx.Model(&models.User{}).
		Where("personal_workspace_id IS NULL").
		Order("id").
		Pluck("id", &ids).Error; err != nil {
		return fmt.Errorf("list unmigrated users: %w", err)
	}

	logger.Info("workspace migration: backfilling personal workspaces", "users", len(ids))

	migrated := 0
	for _, id := range ids {
		err := tx.Transaction(func(utx *gorm.DB) error {
			_, e := s.migrate(utx, id)
			return e
		})
		if err != nil {
			logger.Error("workspace migration failed for user", "user_id", id, "error", err)
			if uerr := tx.Model(&models.User{}).Where("id = ?", id).
				Update("migration_error", err.Error()).Error; uerr != nil {
				return fmt.Errorf("record migration error for user %d: %w", id, uerr)
			}
			continue
		}
		migrated++
	}

	logger.Info("workspace migration: backfill complete", "migrated", migrated, "failed", len(ids)-migrated)
	return nil
}

func (s *Service) migrate(tx *gorm.DB, userID uint) (uint, error) {
	var user models.User
	if err := tx.First(&user, userID).Error; err != nil {
		return 0, fmt.Errorf("load user: %w", err)
	}

	// Already migrated — no-op.
	if user.PersonalWorkspaceID != nil {
		return *user.PersonalWorkspaceID, nil
	}

	// Resolve the plan (NULL in OSS / non-enforcing mode).
	planID, err := s.resolvePlanID(tx)
	if err != nil {
		return 0, err
	}

	// Create the personal workspace.
	name := strings.TrimSpace(user.Name)
	if name == "" {
		name = "Personal"
	}
	ws := &models.Workspace{
		Name:       name,
		Slug:       personalSlug(user.ID),
		OwnerID:    user.ID,
		IsPersonal: true,
		PlanID:     planID,
	}
	if err := tx.Create(ws).Error; err != nil {
		return 0, fmt.Errorf("create personal workspace: %w", err)
	}

	// Owner member row
	member := &models.WorkspaceMember{
		WorkspaceID: ws.ID,
		UserID:      user.ID,
		Role:        models.WorkspaceRoleOwner,
	}
	if err := tx.Create(member).Error; err != nil {
		return 0, fmt.Errorf("create owner member: %w", err)
	}

	if err := s.copySettings(tx, user.ID, ws.ID, user.RequireVerifiedDomain); err != nil {
		return 0, err
	}

	for _, model := range operationalTables {
		if err := tx.Model(model).
			Where("user_id = ? AND workspace_id IS NULL", user.ID).
			Update("workspace_id", ws.ID).Error; err != nil {
			return 0, fmt.Errorf("backfill %T: %w", model, err)
		}
	}

	// Mark the user migrated and clear any prior error.
	if err := tx.Model(&models.User{}).Where("id = ?", user.ID).Updates(map[string]interface{}{
		"personal_workspace_id": ws.ID,
		"migrated_at":           time.Now().UTC(),
		"migration_error":       "",
	}).Error; err != nil {
		return 0, fmt.Errorf("mark user migrated: %w", err)
	}

	return ws.ID, nil
}

func (s *Service) resolvePlanID(tx *gorm.DB) (*uint, error) {
	if !s.planEnforcement {
		return nil, nil
	}
	var plan models.Plan
	if err := tx.Where("is_default = ? AND is_active = ?", true, true).First(&plan).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("plan enforcement is enabled but no default plan is configured")
		}
		return nil, fmt.Errorf("resolve default plan: %w", err)
	}
	return &plan.ID, nil
}

func (s *Service) copySettings(tx *gorm.DB, userID, workspaceID uint, requireVerifiedDomain bool) error {
	var us models.UserSetting
	err := tx.Where("user_id = ?", userID).First(&us).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		us = models.UserSetting{
			Timezone:           "UTC",
			WebhookRetryCount:  3,
			APIKeyExpiryDays:   90,
			BounceAutoSuppress: true,
		}
	} else if err != nil {
		return fmt.Errorf("load user settings: %w", err)
	}

	ws := models.WorkspaceSetting{
		WorkspaceID:           workspaceID,
		Timezone:              us.Timezone,
		DefaultSenderName:     us.DefaultSenderName,
		DefaultSenderEmail:    us.DefaultSenderEmail,
		WebhookRetryCount:     us.WebhookRetryCount,
		APIKeyExpiryDays:      us.APIKeyExpiryDays,
		BounceAutoSuppress:    us.BounceAutoSuppress,
		RequireVerifiedDomain: requireVerifiedDomain,
	}
	if err := tx.Create(&ws).Error; err != nil {
		return fmt.Errorf("create workspace settings: %w", err)
	}
	return nil
}

// personalSlug derives a stable, unique slug for a user's personal workspace.
func personalSlug(userID uint) string {
	return fmt.Sprintf("personal-%d", userID)
}
