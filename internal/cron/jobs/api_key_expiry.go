// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/notification"
	"github.com/hibiken/asynq"
	"github.com/jkaninda/logger"
	"gorm.io/gorm"
)

// APIKeyExpiryJob checks for workspace API keys expiring within 7 days and
// notifies the owning workspace's owners and admins.
type APIKeyExpiryJob struct {
	db       *gorm.DB
	notifier *notification.Service
}

func NewAPIKeyExpiryJob(db *gorm.DB, notifier *notification.Service) *APIKeyExpiryJob {
	return &APIKeyExpiryJob{db: db, notifier: notifier}
}

func (j *APIKeyExpiryJob) Name() string     { return "api-key-expiry" }
func (j *APIKeyExpiryJob) Schedule() string { return "0 8 * * *" } // daily at 08:00 UTC

type expiringKey struct {
	Name      string
	ExpiresAt string
}

func (j *APIKeyExpiryJob) Run(_ context.Context, _ *asynq.Client) error {
	if j.notifier == nil || !j.notifier.IsConfigured() {
		return nil
	}

	// Find workspace API keys expiring within the next 7 days. Personal
	// (user-scoped) keys are excluded: scoping is workspace-only.
	now := time.Now().UTC()
	deadline := now.AddDate(0, 0, 7)

	var keys []models.APIKey
	if err := j.db.Where("expires_at IS NOT NULL AND expires_at > ? AND expires_at <= ? AND revoked = ? AND workspace_id IS NOT NULL", now, deadline, false).
		Find(&keys).Error; err != nil {
		logger.Error("api-key-expiry: failed to query", "error", err)
		return err
	}

	if len(keys) == 0 {
		return nil
	}

	// Group by workspace
	workspaceKeys := make(map[uint][]expiringKey)
	for _, k := range keys {
		if k.WorkspaceID == nil {
			continue
		}
		workspaceKeys[*k.WorkspaceID] = append(workspaceKeys[*k.WorkspaceID], expiringKey{
			Name:      k.Name,
			ExpiresAt: k.ExpiresAt.Format("January 2, 2006"),
		})
	}

	sent := 0
	for workspaceID, ks := range workspaceKeys {
		subject := fmt.Sprintf("%d API key(s) expiring soon", len(ks))
		if len(ks) == 1 {
			subject = "API key expiring soon"
		}
		if err := j.notifier.SendToWorkspaceAdmins(workspaceID, subject, notification.TemplateAPIKeyExpiry, map[string]any{
			"Keys":     ks,
			"KeyCount": len(ks),
		}); err != nil {
			logger.Error("api-key-expiry: failed to send", "workspace_id", workspaceID, "error", err)
			continue
		}
		sent++
	}

	logger.Info("api-key-expiry: notifications sent", "count", sent)
	return nil
}
