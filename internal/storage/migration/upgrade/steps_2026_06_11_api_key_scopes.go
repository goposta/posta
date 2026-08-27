// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package upgrade

import (
	"fmt"

	"gorm.io/gorm"
)

func applyAPIKeyScopes(tx *gorm.DB) error {
	if err := tx.Exec(
		`UPDATE api_keys SET scopes = '{send}' WHERE scopes IS NULL OR scopes = '{}'`,
	).Error; err != nil {
		return fmt.Errorf("backfill api_key scopes: %w", err)
	}
	return nil
}
