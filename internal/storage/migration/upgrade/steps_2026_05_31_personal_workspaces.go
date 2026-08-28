// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package upgrade

import (
	"github.com/goposta/posta/internal/services/workspaceprovision"
	"gorm.io/gorm"
)

// applyPersonalWorkspaces is the Apply func for the one-shot personal-workspace
// backfill.
func applyPersonalWorkspaces(tx *gorm.DB) error {
	svc := workspaceprovision.New(runOptions.PlanEnforcement)
	return svc.BackfillMissingWorkspaces(tx)
}
