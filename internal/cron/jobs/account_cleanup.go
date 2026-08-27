// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package jobs

import (
	"context"

	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/hibiken/asynq"
	"github.com/jkaninda/logger"
)

// AccountCleanupJob permanently deletes user accounts whose scheduled
// deletion date has passed.
type AccountCleanupJob struct {
	userRepo *repositories.UserRepository
}

func NewAccountCleanupJob(userRepo *repositories.UserRepository) *AccountCleanupJob {
	return &AccountCleanupJob{userRepo: userRepo}
}

func (j *AccountCleanupJob) Name() string     { return "account-cleanup" }
func (j *AccountCleanupJob) Schedule() string { return "0 2 * * *" } // daily at 02:00 UTC

func (j *AccountCleanupJob) Run(_ context.Context, _ *asynq.Client) error {
	users, err := j.userRepo.FindScheduledForDeletion()
	if err != nil {
		logger.Error("account cleanup: failed to find users scheduled for deletion", "error", err)
		return err
	}

	if len(users) == 0 {
		return nil
	}

	deleted := 0
	for _, user := range users {
		if err := j.userRepo.DeleteAllUserData(user.ID); err != nil {
			logger.Error("account cleanup: failed to delete user", "user_id", user.ID, "email", user.Email, "error", err)
			continue
		}
		logger.Info("account cleanup: permanently deleted user", "user_id", user.ID, "email", user.Email)
		deleted++
	}

	logger.Info("account cleanup: completed", "deleted", deleted, "total", len(users))
	return nil
}
