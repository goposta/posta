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

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/goposta/posta/internal/config"
	"github.com/goposta/posta/internal/models"
	workspacesvc "github.com/goposta/posta/internal/services/workspace"
	"gorm.io/gorm"
)

// A row with a NULL workspace_id becomes unreachable once repositories.ApplyScope is enforced, so this is checked.
var scopedTables = []string{
	"api_keys", "templates", "style_sheets", "languages", "domains",
	"smtp_servers", "webhooks", "contacts", "subscribers", "subscriber_lists",
	"unsubscribe_lists", "suppressions", "bounces", "emails", "inbound_emails",
	"campaigns",
}

type check struct {
	name   string
	detail string
	count  int64
	fatal  bool
	fix    string
}

func runDoctorWorkspaces() error {
	cfg := config.New()
	cfg.InitStorage()
	db := cfg.Database.DB

	checks := []check{}

	checks = append(checks, unscopedRowChecks(db)...)

	var noWorkspace int64
	db.Model(&models.User{}).
		Where("active = ? AND NOT EXISTS (SELECT 1 FROM workspace_members m WHERE m.user_id = users.id)", true).
		Count(&noWorkspace)
	checks = append(checks, check{
		name:   "users without a workspace",
		detail: "these users land on an empty dashboard until one is provisioned",
		count:  noWorkspace,
		fatal:  false,
		fix:    "they are provisioned on next sign-in; POST /api/v1/admin/users/{id}/migrate forces it",
	})

	if hasColumn(db, "users", "migration_error") {
		var failed int64
		db.Model(&models.User{}).Where("migration_error <> ''").Count(&failed)
		checks = append(checks, check{
			name:   "users with a recorded provisioning error",
			detail: "the earlier workspace backfill did not finish for these users",
			count:  failed,
			fatal:  true,
			fix:    "resolve the recorded error, then re-run provisioning before upgrading",
		})
	}

	var systems int64
	db.Model(&models.Workspace{}).Where("system = ?", true).Count(&systems)

	fmt.Println("Posta workspace check")
	fmt.Println(strings.Repeat("-", 60))

	blocked := false
	for _, c := range checks {
		status := "ok"
		if c.count > 0 {
			if c.fatal {
				status = "BLOCKING"
				blocked = true
			} else {
				status = "warn"
			}
		}
		fmt.Printf("%-10s %-45s %d\n", status, c.name, c.count)
		if c.count > 0 {
			fmt.Printf("           %s\n           fix: %s\n", c.detail, c.fix)
		}
	}

	switch systems {
	case 0:
		fmt.Printf("%-10s %-45s %d\n", "warn", "system workspace", systems)
		fmt.Println("           not created yet; the server creates it on next boot")
	case 1:
		fmt.Printf("%-10s %-45s %d\n", "ok", "system workspace", systems)
	default:
		fmt.Printf("%-10s %-45s %d\n", "BLOCKING", "system workspace", systems)
		fmt.Println("           more than one exists; keep the oldest and clear the flag on the rest")
		blocked = true
	}

	fmt.Println(strings.Repeat("-", 60))
	if blocked {
		fmt.Println("NOT SAFE TO UPGRADE — resolve the blocking rows above first.")
		fmt.Printf("Reserved system workspace name: %q\n", workspacesvc.SystemName)
		os.Exit(1)
	}
	fmt.Println("Safe to upgrade.")
	return nil
}

func unscopedRowChecks(db *gorm.DB) []check {
	out := make([]check, 0, len(scopedTables))
	for _, table := range scopedTables {
		if !hasColumn(db, table, "workspace_id") {
			continue
		}
		var count int64
		db.Table(table).Where("workspace_id IS NULL").Count(&count)
		out = append(out, check{
			name:   table + " rows with no workspace",
			detail: "these rows become unreachable once scoping is enforced",
			count:  count,
			fatal:  true,
			fix:    "run the earlier workspace backfill to completion before upgrading",
		})
	}
	return out
}

func hasColumn(db *gorm.DB, table, column string) bool {
	var count int64
	db.Raw(`SELECT count(*) FROM information_schema.columns WHERE table_name = ? AND column_name = ?`,
		table, column).Scan(&count)
	return count > 0
}
