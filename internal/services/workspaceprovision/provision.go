// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package workspaceprovision

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/jkaninda/logger"
	"gorm.io/gorm"
)

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

type Seeder interface {
	SeedWorkspaceDefaults(workspaceID, userID uint, userName string)
}

// Service provisions a user's first workspace, and backfills workspaces for
// users that predate workspace scoping.
type Service struct {
	planEnforcement bool

	seeder Seeder
}

// New constructs a provisioning service.
func New(planEnforcement bool) *Service {
	return &Service{planEnforcement: planEnforcement}
}

func (s *Service) SetSeeder(seeder Seeder) {
	s.seeder = seeder
}

// EnsureWorkspace gives userID a workspace if they have none, and returns the
// id of the workspace they should land in. Idempotent.
func (s *Service) EnsureWorkspace(db *gorm.DB, userID uint) (uint, error) {
	var existing models.User
	if err := db.Select("default_workspace_id", "name").First(&existing, userID).Error; err == nil &&
		existing.DefaultWorkspaceID != nil {
		return *existing.DefaultWorkspaceID, nil
	}

	var wsID uint
	err := db.Transaction(func(tx *gorm.DB) error {
		id, e := s.provision(tx, userID)
		wsID = id
		return e
	})
	if err != nil {
		var u models.User
		if e := db.First(&u, userID).Error; e == nil && u.DefaultWorkspaceID != nil {
			return *u.DefaultWorkspaceID, nil
		}
		return 0, err
	}

	if s.seeder != nil {
		s.seeder.SeedWorkspaceDefaults(wsID, userID, existing.Name)
	}

	return wsID, nil
}

// BackfillMissingWorkspaces gives every workspace-less user a workspace and
// moves their legacy unscoped rows into it. Retained for installs upgrading from
// before workspace scoping; a no-op once every user has one.
func (s *Service) BackfillMissingWorkspaces(tx *gorm.DB) error {
	var ids []uint
	if err := tx.Model(&models.User{}).
		Where("default_workspace_id IS NULL").
		Order("id").
		Pluck("id", &ids).Error; err != nil {
		return fmt.Errorf("list unmigrated users: %w", err)
	}

	logger.Info("workspace backfill: provisioning workspaces", "users", len(ids))

	migrated := 0
	for _, id := range ids {
		err := tx.Transaction(func(utx *gorm.DB) error {
			_, e := s.provision(utx, id)
			return e
		})
		if err != nil {
			logger.Error("workspace provisioning failed for user", "user_id", id, "error", err)
			if uerr := tx.Model(&models.User{}).Where("id = ?", id).
				Update("migration_error", err.Error()).Error; uerr != nil {
				return fmt.Errorf("record migration error for user %d: %w", id, uerr)
			}
			continue
		}
		migrated++
	}

	logger.Info("workspace backfill complete", "migrated", migrated, "failed", len(ids)-migrated)
	return nil
}

func (s *Service) provision(tx *gorm.DB, userID uint) (uint, error) {
	var user models.User
	if err := tx.First(&user, userID).Error; err != nil {
		return 0, fmt.Errorf("load user: %w", err)
	}

	// Already has one — no-op.
	if user.DefaultWorkspaceID != nil {
		return *user.DefaultWorkspaceID, nil
	}

	// Resolve the plan (NULL in OSS / non-enforcing mode).
	planID, err := s.resolvePlanID(tx)
	if err != nil {
		return 0, err
	}

	ws := &models.Workspace{
		Name:    workspaceName(user.Name),
		Slug:    workspaceSlug(user.ID),
		OwnerID: user.ID,
		PlanID:  planID,
	}
	if err := tx.Create(ws).Error; err != nil {
		return 0, fmt.Errorf("create workspace: %w", err)
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

	if err := tx.Model(&models.User{}).Where("id = ?", user.ID).Updates(map[string]interface{}{
		"default_workspace_id": ws.ID,
		"migrated_at":          time.Now().UTC(),
		"migration_error":      "",
	}).Error; err != nil {
		return 0, fmt.Errorf("record default workspace: %w", err)
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

// workspaceName labels a user's first workspace after them, so a switcher with
// several entries reads as names rather than as "Workspace 1, Workspace 2".
func workspaceName(userName string) string {
	first := strings.TrimSpace(userName)
	if idx := strings.IndexAny(first, " \t"); idx > 0 {
		first = first[:idx]
	}
	if first == "" {
		return "My workspace"
	}
	return first + "'s workspace"
}

// workspaceSlug derives a stable, unique slug for a user's first workspace.
func workspaceSlug(userID uint) string {
	return fmt.Sprintf("workspace-%d", userID)
}
