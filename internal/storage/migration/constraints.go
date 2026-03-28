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

	// Ensure template_localizations cascade-deletes when a version is removed.
	// GORM's AutoMigrate does not update existing FK constraints, so we
	// drop and recreate if the current constraint lacks ON DELETE CASCADE.
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

	// Rebuild unique indexes to include workspace_id so that
	// the same name/email can exist in different workspaces.
	rebuildUniqueIndexes(db)
}

// rebuildUniqueIndexes drops legacy two-column unique indexes and replaces
// them with expression indexes that use COALESCE(workspace_id, 0) so that
// NULL workspace_id values (personal space) are treated as equal.
func rebuildUniqueIndexes(db *gorm.DB) {
	type indexDef struct {
		table  string
		name   string
		column string // the varying column (name, email, code, domain)
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
