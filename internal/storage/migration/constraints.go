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

package migration

import (
	"fmt"

	"gorm.io/gorm"
)

func runConstraints(db *gorm.DB) {
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

	// Analytics queries filter a workspace by date range; the single-column index would read every email it ever sent.
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_emails_workspace_created
		ON emails (workspace_id, created_at)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_bounces_workspace_created
		ON bounces (workspace_id, created_at)`)

	// At most one unresolved notification per user and condition, so a recurring one updates its row even under races.
	db.Exec(`DO $$ BEGIN
		CREATE UNIQUE INDEX IF NOT EXISTS idx_notification_open
			ON notifications (user_id, workspace_id, dedup_key) WHERE resolved_at IS NULL;
	EXCEPTION WHEN others THEN NULL;
	END $$`)

	// At most one ownership-verified row per domain (case-insensitive), so two tenants cannot both verify it.
	db.Exec(`DO $$ BEGIN
		DROP INDEX IF EXISTS idx_verified_domain;
		CREATE UNIQUE INDEX idx_verified_domain ON domains (LOWER(domain)) WHERE ownership_verified = true;
	EXCEPTION WHEN others THEN NULL;
	END $$`)

	// Workspace-scoped Message-ID dedup lookups on inbound_emails; replaces the legacy user_id index.
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
		DROP INDEX IF EXISTS idx_workspace_form_slug;
		CREATE UNIQUE INDEX IF NOT EXISTS idx_workspace_form_name ON forms (workspace_id, name) WHERE workspace_id IS NOT NULL AND deleted_at IS NULL;
	EXCEPTION WHEN others THEN NULL;
	END $$`)

	db.Exec(`DO $$ BEGIN
		CREATE INDEX IF NOT EXISTS idx_messages_ws_created ON messages (workspace_id, created_at DESC) WHERE deleted_at IS NULL;
		CREATE INDEX IF NOT EXISTS idx_messages_form_status ON messages (form_id, status, created_at DESC) WHERE deleted_at IS NULL;
		CREATE INDEX IF NOT EXISTS idx_messages_dedup ON messages (form_id, dedup_hash, created_at DESC) WHERE dedup_hash <> '';
		CREATE INDEX IF NOT EXISTS idx_messages_inbox ON messages (workspace_id, state, created_at DESC) WHERE deleted_at IS NULL AND status IN ('received','flagged');
	EXCEPTION WHEN others THEN NULL;
	END $$`)

	// Form submission field filters are containment queries (data @> '{"department":"sales"}'), served by jsonb_path_ops.
	db.Exec(`DO $$ BEGIN
		CREATE INDEX IF NOT EXISTS idx_messages_data ON messages USING GIN (data jsonb_path_ops) WHERE data IS NOT NULL;
	EXCEPTION WHEN others THEN NULL;
	END $$`)

	// Tag filters are array containment (tags @> '{vip}').
	db.Exec(`DO $$ BEGIN
		CREATE INDEX IF NOT EXISTS idx_messages_tags ON messages USING GIN (tags) WHERE tags IS NOT NULL;
	EXCEPTION WHEN others THEN NULL;
	END $$`)

	// Routing: a form's rules in order, team names unique per workspace, and the "My teams" inbox view.
	db.Exec(`DO $$ BEGIN
		CREATE INDEX IF NOT EXISTS idx_form_rules_order ON form_rules (form_id, position);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_workspace_team_name ON teams (workspace_id, LOWER(name)) WHERE deleted_at IS NULL;
		CREATE INDEX IF NOT EXISTS idx_messages_team ON messages (team_id, created_at DESC) WHERE team_id IS NOT NULL AND deleted_at IS NULL;
	EXCEPTION WHEN others THEN NULL;
	END $$`)

	// Live chat: the seq index orders messages, the client index dedups retries; separate blocks so one failure skips neither.
	db.Exec(`DO $$ BEGIN
		CREATE UNIQUE INDEX IF NOT EXISTS idx_chat_msg_seq ON chat_messages (conversation_id, seq);
	EXCEPTION WHEN others THEN NULL;
	END $$`)
	db.Exec(`DO $$ BEGIN
		CREATE UNIQUE INDEX IF NOT EXISTS idx_chat_msg_client ON chat_messages (conversation_id, client_msg_id) WHERE client_msg_id <> '';
	EXCEPTION WHEN others THEN NULL;
	END $$`)
	db.Exec(`DO $$ BEGIN
		CREATE UNIQUE INDEX IF NOT EXISTS idx_workspace_chat_widget_name ON chat_widgets (workspace_id, name) WHERE workspace_id IS NOT NULL AND deleted_at IS NULL;
		CREATE INDEX IF NOT EXISTS idx_chat_conv_inbox ON chat_conversations (workspace_id, state, last_message_at DESC) WHERE deleted_at IS NULL;
		CREATE INDEX IF NOT EXISTS idx_chat_conv_assignee ON chat_conversations (assigned_to_id, state) WHERE deleted_at IS NULL;
		CREATE INDEX IF NOT EXISTS idx_chat_visitor_identity ON chat_visitors (widget_id, external_id) WHERE email_verified AND external_id <> '';
		CREATE UNIQUE INDEX IF NOT EXISTS idx_workspace_chat_snippet ON chat_snippets (workspace_id, shortcut) WHERE deleted_at IS NULL;
	EXCEPTION WHEN others THEN NULL;
	END $$`)
	// Erasing a conversation takes its messages with it.
	db.Exec(`DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_chat_messages_conversation') THEN
			ALTER TABLE chat_messages ADD CONSTRAINT fk_chat_messages_conversation
				FOREIGN KEY (conversation_id) REFERENCES chat_conversations(id) ON DELETE CASCADE;
		END IF;
	EXCEPTION WHEN others THEN NULL;
	END $$`)

	// One reply per inbound email so a retried parse cannot attach it twice; own block so a failure skips no others.
	db.Exec(`DO $$ BEGIN
		CREATE UNIQUE INDEX IF NOT EXISTS idx_message_replies_inbound ON message_replies (inbound_email_id) WHERE inbound_email_id IS NOT NULL;
	EXCEPTION WHEN others THEN NULL;
	END $$`)

	// Older AutoMigrate made fk_messages_form/replies without ON DELETE CASCADE, breaking deletes; replace them in place.
	db.Exec(`DO $$ BEGIN
		IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_messages_replies' AND confdeltype <> 'c') THEN
			ALTER TABLE message_replies DROP CONSTRAINT fk_messages_replies;
			ALTER TABLE message_replies ADD CONSTRAINT fk_messages_replies
				FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE;
		END IF;
		IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_messages_form' AND confdeltype <> 'c') THEN
			ALTER TABLE messages DROP CONSTRAINT fk_messages_form;
			ALTER TABLE messages ADD CONSTRAINT fk_messages_form
				FOREIGN KEY (form_id) REFERENCES forms(id) ON DELETE CASCADE;
		END IF;
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

// rebuildUniqueIndexes re-scopes the per-tenant uniqueness constraints to workspace-only.
func rebuildUniqueIndexes(db *gorm.DB) {
	type indexDef struct {
		table   string
		oldName string // legacy user-scoped index, dropped
		newName string // workspace-scoped index, created
		column  string
	}

	const colName = "name"

	indexes := []indexDef{
		{"templates", "idx_user_template", "idx_workspace_template", colName},
		{"style_sheets", "idx_user_stylesheet", "idx_workspace_stylesheet", colName},
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
