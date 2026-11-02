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

package directory

import (
	"testing"

	"github.com/goposta/posta/internal/models"
)

func ws(id uint) *uint { return &id }

func TestPlan(t *testing.T) {
	mappings := []models.LDAPGroupMapping{
		{GroupDN: "cn=posta-admins,ou=groups,dc=corp,dc=test", PlatformAdmin: true},
		{GroupDN: "sales", WorkspaceID: ws(1), WorkspaceRole: models.WorkspaceRoleViewer},
		{GroupDN: "sales-leads", WorkspaceID: ws(1), WorkspaceRole: models.WorkspaceRoleAdmin},
		{GroupDN: "support", WorkspaceID: ws(2), WorkspaceRole: models.WorkspaceRoleEditor},
	}

	a := Plan(mappings, []string{"CN=Sales,OU=Groups,DC=corp,DC=test", "cn=sales-leads,ou=groups,dc=corp,dc=test"})
	if a.Admin || !a.AdminManaged {
		t.Errorf("admin = %v, managed = %v", a.Admin, a.AdminManaged)
	}
	if a.Roles[1] != models.WorkspaceRoleAdmin {
		t.Errorf("workspace 1 role = %q, want the highest granted, admin", a.Roles[1])
	}
	if _, ok := a.Roles[2]; ok || !a.Managed[2] {
		t.Errorf("workspace 2: no group grants it, but a mapping governs it: %+v", a)
	}

	a = Plan(mappings, []string{"cn=posta-admins,ou=groups,dc=corp,dc=test"})
	if !a.Admin || len(a.Roles) != 0 {
		t.Errorf("full DN match: %+v", a)
	}

	a = Plan([]models.LDAPGroupMapping{{GroupDN: "sales", WorkspaceID: ws(1), WorkspaceRole: models.WorkspaceRoleEditor}}, nil)
	if a.AdminManaged {
		t.Error("no mapping grants admin, so the admin role must be left alone")
	}
}

func TestGroupMatches(t *testing.T) {
	have := normalizeGroups([]string{"cn=Engineering,ou=groups,dc=corp,dc=test", " "})
	cases := map[string]bool{
		"engineering": true,
		"CN=ENGINEERING,OU=GROUPS,DC=CORP,DC=TEST": true,
		"cn=engineering,ou=other,dc=corp,dc=test":  false,
		"eng": false,
		"":    false,
	}
	for group, want := range cases {
		if got := groupMatches(have, group); got != want {
			t.Errorf("groupMatches(%q) = %v, want %v", group, got, want)
		}
	}
}

func TestCnOf(t *testing.T) {
	for dn, want := range map[string]string{
		"cn=admins,ou=groups,dc=x": "admins",
		"cn= spaced ,dc=x":         "spaced",
		"plain":                    "plain",
	} {
		if got := cnOf(dn); got != want {
			t.Errorf("cnOf(%q) = %q, want %q", dn, got, want)
		}
	}
}

func TestOwnerIsNotMappable(t *testing.T) {
	for _, r := range MappableRoles {
		if r == models.WorkspaceRoleOwner {
			t.Fatal("a directory group must never grant workspace ownership")
		}
	}
}
