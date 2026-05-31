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

	// Composite index for fast Message-ID dedup lookups on inbound_emails, scoped
	// to the workspace (workspace-only migration — replaces the legacy user_id one).
	db.Exec(`DO $$ BEGIN
		DROP INDEX IF EXISTS idx_inbound_user_message_id;
		CREATE INDEX IF NOT EXISTS idx_inbound_workspace_message_id ON inbound_emails (workspace_id, message_id) WHERE message_id <> '';
	EXCEPTION WHEN others THEN NULL;
	END $$`)

	db.Exec(`DO $$ BEGIN
		CREATE UNIQUE INDEX IF NOT EXISTS one_personal_per_user ON workspaces (owner_id) WHERE is_personal;
	EXCEPTION WHEN others THEN NULL;
	END $$`)
}

// rebuildUniqueIndexes re-scopes the per-tenant uniqueness constraints to
// workspace-only.
func rebuildUniqueIndexes(db *gorm.DB) {
	type indexDef struct {
		table   string
		oldName string // legacy user-scoped index, dropped
		newName string // workspace-scoped index, created
		column  string
	}

	indexes := []indexDef{
		{"templates", "idx_user_template", "idx_workspace_template", "name"},
		{"style_sheets", "idx_user_stylesheet", "idx_workspace_stylesheet", "name"},
		{"contacts", "idx_user_email", "idx_workspace_email", "email"},
		{"domains", "idx_user_domain", "idx_workspace_domain", "domain"},
		{"languages", "idx_user_language", "idx_workspace_language", "code"},

		{"suppressions", "idx_user_suppression", "idx_workspace_suppression", "email, COALESCE(list_id, 0)"},
		{"unsubscribe_lists", "idx_user_unsub_list", "idx_workspace_unsub_list", "name"},
	}

	for _, idx := range indexes {
		db.Exec(fmt.Sprintf(`
			DO $$ BEGIN
				DROP INDEX IF EXISTS %s;
				DROP INDEX IF EXISTS %s;
				CREATE UNIQUE INDEX %s ON %s (workspace_id, %s) WHERE workspace_id IS NOT NULL;
			EXCEPTION WHEN others THEN NULL;
			END $$`,
			idx.oldName, idx.newName, idx.newName, idx.table, idx.column,
		))
	}

	// Subscribers carry their unique index from a GORM tag (idx_sub_scope_email,
	// historically on user_id, workspace_id, email). Re-scope it to workspace-only
	// here so both fresh and existing databases converge. Recreated under the same
	// name so AutoMigrate won't re-add the legacy definition.
	db.Exec(`DO $$ BEGIN
		DROP INDEX IF EXISTS idx_sub_scope_email;
		CREATE UNIQUE INDEX idx_sub_scope_email ON subscribers (workspace_id, email) WHERE workspace_id IS NOT NULL;
	EXCEPTION WHEN others THEN NULL;
	END $$`)
}
