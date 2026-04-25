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

package migration

import (
	"fmt"

	"gorm.io/gorm"
)

func runConstraints(db *gorm.DB) {
	// Add FK constraints
	db.Exec(`DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_template_versions_template') THEN
			ALTER TABLE template_versions ADD CONSTRAINT fk_template_versions_template
				FOREIGN KEY (template_id) REFERENCES templates(id) ON DELETE CASCADE;
		END IF;
	END $$`)

	db.Exec(`DO $$ BEGIN
		IF EXISTS (
			SELECT 1 FROM pg_constraint c
			JOIN pg_class t ON c.conrelid = t.oid
			WHERE t.relname = 'template_localizations'
			  AND c.conname = 'fk_template_versions_localizations'
			  AND c.confdeltype <> 'c'
		) THEN
			ALTER TABLE template_localizations DROP CONSTRAINT fk_template_versions_localizations;
			ALTER TABLE template_localizations ADD CONSTRAINT fk_template_versions_localizations
				FOREIGN KEY (version_id) REFERENCES template_versions(id) ON DELETE CASCADE;
		END IF;
	END $$`)

	rebuildUniqueIndexes(db)

	// Partial unique index: at most one ownership-verified row per domain name
	// (case-insensitive). Prevents two tenants from both verifying the same domain.
	db.Exec(`DO $$ BEGIN
		DROP INDEX IF EXISTS idx_verified_domain;
		CREATE UNIQUE INDEX idx_verified_domain ON domains (LOWER(domain)) WHERE ownership_verified = true;
	EXCEPTION WHEN others THEN NULL;
	END $$`)

	// Composite index for fast Message-ID dedup lookups on inbound_emails.
	db.Exec(`DO $$ BEGIN
		CREATE INDEX IF NOT EXISTS idx_inbound_user_message_id ON inbound_emails (user_id, message_id) WHERE message_id <> '';
	EXCEPTION WHEN others THEN NULL;
	END $$`)
}

func rebuildUniqueIndexes(db *gorm.DB) {
	type indexDef struct {
		table  string
		name   string
		column string
	}

	indexes := []indexDef{
		{"templates", "idx_user_template", "name"},
		{"style_sheets", "idx_user_stylesheet", "name"},
		{"contacts", "idx_user_email", "email"},
		{"domains", "idx_user_domain", "domain"},
		{"languages", "idx_user_language", "code"},
		{"suppressions", "idx_user_suppression", "email"},
	}

	for _, idx := range indexes {
		db.Exec(fmt.Sprintf(`
			DO $$ BEGIN
				DROP INDEX IF EXISTS %s;
				CREATE UNIQUE INDEX %s ON %s (user_id, COALESCE(workspace_id, 0), %s);
			EXCEPTION WHEN others THEN NULL;
			END $$`,
			idx.name, idx.name, idx.table, idx.column,
		))
	}
}
