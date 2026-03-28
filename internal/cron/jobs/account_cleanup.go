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

package jobs

import (
	"context"

	"github.com/hibiken/asynq"
	"github.com/jkaninda/logger"
	"github.com/goposta/posta/internal/storage/repositories"
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
