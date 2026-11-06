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

package jobs

import (
	"context"

	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/hibiken/asynq"
	"github.com/jkaninda/logger"
)

// AccountCleanupJob permanently deletes user accounts whose scheduled deletion date has passed.
type AccountCleanupJob struct {
	userRepo *repositories.UserRepository
}

func NewAccountCleanupJob(userRepo *repositories.UserRepository) *AccountCleanupJob {
	return &AccountCleanupJob{userRepo: userRepo}
}

func (j *AccountCleanupJob) Name() string     { return "account-cleanup" }
func (j *AccountCleanupJob) Schedule() string { return "0 2 * * *" }

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
