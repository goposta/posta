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
