// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package upgrade

import (
	"fmt"

	"gorm.io/gorm"
)

func applyDedupeContacts(tx *gorm.DB) error {

	if err := tx.Exec(`
		UPDATE contacts c
		SET sent_count = agg.sent_count,
			fail_count = agg.fail_count,
			last_sent_at = agg.last_sent_at
		FROM (
			SELECT workspace_id, email,
				MIN(id) AS keep_id,
				SUM(sent_count) AS sent_count,
				SUM(fail_count) AS fail_count,
				MAX(last_sent_at) AS last_sent_at
			FROM contacts
			WHERE workspace_id IS NOT NULL
			GROUP BY workspace_id, email
			HAVING COUNT(*) > 1
		) agg
		WHERE c.id = agg.keep_id`).Error; err != nil {
		return fmt.Errorf("merge duplicate contacts: %w", err)
	}

	if err := tx.Exec(`
		DELETE FROM contacts c
		USING (
			SELECT workspace_id, email, MIN(id) AS keep_id
			FROM contacts
			WHERE workspace_id IS NOT NULL
			GROUP BY workspace_id, email
			HAVING COUNT(*) > 1
		) dup
		WHERE c.workspace_id = dup.workspace_id
		  AND c.email = dup.email
		  AND c.id <> dup.keep_id`).Error; err != nil {
		return fmt.Errorf("delete duplicate contacts: %w", err)
	}

	if err := tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_workspace_email
		ON contacts (workspace_id, email) WHERE workspace_id IS NOT NULL`).Error; err != nil {
		return fmt.Errorf("create contacts unique index: %w", err)
	}
	return nil
}
