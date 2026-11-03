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

	"github.com/jkaninda/logger"
	"gorm.io/gorm"
)

// renameIdentifiers moves workspaces, forms, OAuth providers, templates and
// stylesheets to one naming scheme before AutoMigrate sees the new models:
// name is the identifier used in the API, display_name the label people
// read. It runs before AutoMigrate because AutoMigrate would otherwise add
// an empty display_name and build the unique index on name while name still
// holds labels, which need not be unique. Each table is moved once; on a
// fresh database there is nothing to move.
func renameIdentifiers(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		// name was the label and slug the identifier.
		for _, table := range []string{"workspaces", "forms", "o_auth_providers"} {
			if !columnExists(tx, table, "slug") {
				continue
			}
			for _, stmt := range []string{
				fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS display_name text NOT NULL DEFAULT ''", table),
				fmt.Sprintf("UPDATE %s SET display_name = name, name = slug", table),
				fmt.Sprintf("ALTER TABLE %s DROP COLUMN slug", table),
			} {
				if err := tx.Exec(stmt).Error; err != nil {
					return fmt.Errorf("rename %s identifiers: %w", table, err)
				}
			}
			logger.Info("identifiers renamed: slug is now name, name is now display_name", "table", table)
		}

		// name was a free-form label that also identified the row. It becomes
		// the label, and name becomes a slug of it; a second row with the
		// same slug in a workspace gets its id appended. The unique index
		// is dropped while names change and rebuilt by runConstraints.
		for _, t := range []struct{ table, index, fallback string }{
			{"templates", "idx_workspace_template", "template"},
			{"style_sheets", "idx_workspace_stylesheet", "stylesheet"},
		} {
			if !tableExists(tx, t.table) || columnExists(tx, t.table, "display_name") {
				continue
			}
			for _, stmt := range []string{
				fmt.Sprintf("DROP INDEX IF EXISTS %s", t.index),
				fmt.Sprintf("ALTER TABLE %s ADD COLUMN display_name text NOT NULL DEFAULT ''", t.table),
				fmt.Sprintf("UPDATE %s SET display_name = name", t.table),
				fmt.Sprintf(`
					WITH s AS (
						SELECT id, workspace_id,
							COALESCE(NULLIF(trim(both '-' from regexp_replace(lower(name), '[^a-z0-9]+', '-', 'g')), ''), '%s') AS base
						FROM %s
					), r AS (
						SELECT id, base, row_number() OVER (PARTITION BY workspace_id, base ORDER BY id) AS n FROM s
					)
					UPDATE %s t SET name = CASE WHEN r.n = 1 THEN r.base ELSE r.base || '-' || t.id END
					FROM r WHERE r.id = t.id`, t.fallback, t.table, t.table),
			} {
				if err := tx.Exec(stmt).Error; err != nil {
					return fmt.Errorf("rename %s identifiers: %w", t.table, err)
				}
			}
			logger.Info("identifiers renamed: name is now display_name, name is a slug of it", "table", t.table)
		}
		return nil
	})
}

func tableExists(tx *gorm.DB, table string) bool {
	var n int64
	tx.Raw("SELECT count(*) FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = ?", table).Scan(&n)
	return n > 0
}

func columnExists(tx *gorm.DB, table, column string) bool {
	var n int64
	tx.Raw("SELECT count(*) FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?", table, column).Scan(&n)
	return n > 0
}
