// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

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

	db.Exec(`DO $$ BEGIN
		IF EXISTS (
			SELECT 1 FROM pg_constraint c
			JOIN pg_class t ON c.conrelid = t.oid
			WHERE t.relname = 'emails'
			  AND c.conname = 'fk_emails_api_key'
			  AND c.confdeltype <> 'n'
		) THEN
			ALTER TABLE emails DROP CONSTRAINT fk_emails_api_key;
			ALTER TABLE emails ADD CONSTRAINT fk_emails_api_key
				FOREIGN KEY (api_key_id) REFERENCES api_keys(id) ON DELETE SET NULL;
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
		CREATE UNIQUE INDEX IF NOT EXISTS one_system_workspace ON workspaces ((true)) WHERE system;
	EXCEPTION WHEN others THEN NULL;
	END $$`)

	db.Exec(`DO $$ BEGIN
		CREATE UNIQUE INDEX IF NOT EXISTS idx_workspace_form_slug ON forms (workspace_id, slug) WHERE workspace_id IS NOT NULL AND deleted_at IS NULL;
	EXCEPTION WHEN others THEN NULL;
	END $$`)

	db.Exec(`DO $$ BEGIN
		CREATE INDEX IF NOT EXISTS idx_messages_ws_created ON messages (workspace_id, created_at DESC) WHERE deleted_at IS NULL;
		CREATE INDEX IF NOT EXISTS idx_messages_form_status ON messages (form_id, status, created_at DESC) WHERE deleted_at IS NULL;
		CREATE INDEX IF NOT EXISTS idx_messages_dedup ON messages (form_id, dedup_hash, created_at DESC) WHERE dedup_hash <> '';
		CREATE INDEX IF NOT EXISTS idx_messages_inbox ON messages (workspace_id, state, created_at DESC) WHERE deleted_at IS NULL AND status IN ('received','flagged');
	EXCEPTION WHEN others THEN NULL;
	END $$`)

	db.Exec(`DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_messages_form') THEN
			ALTER TABLE messages ADD CONSTRAINT fk_messages_form
				FOREIGN KEY (form_id) REFERENCES forms(id) ON DELETE CASCADE;
		END IF;
	END $$`)

	db.Exec(`DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_message_replies_message') THEN
			ALTER TABLE message_replies ADD CONSTRAINT fk_message_replies_message
				FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE;
		END IF;
	END $$`)

	db.Exec(`DO $$ BEGIN
		CREATE INDEX IF NOT EXISTS idx_message_filters_lookup ON message_filters (workspace_id, enabled);
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

	db.Exec(`DO $$ BEGIN
		DROP INDEX IF EXISTS idx_sub_scope_email;
		CREATE UNIQUE INDEX idx_sub_scope_email ON subscribers (workspace_id, email) WHERE workspace_id IS NOT NULL;
	EXCEPTION WHEN others THEN NULL;
	END $$`)
}
