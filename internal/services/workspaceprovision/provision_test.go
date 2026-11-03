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

package workspaceprovision

import (
	"fmt"
	"testing"

	"github.com/goposta/posta/internal/models"
)

func TestWorkspaceNameIsStableAndUnique(t *testing.T) {
	if got := workspaceName(42); got != "workspace-42" {
		t.Fatalf("workspaceName(42) = %q, want %q", got, "workspace-42")
	}
	if workspaceName(1) == workspaceName(2) {
		t.Fatal("workspaceName must differ per user")
	}
}

func TestWorkspaceName(t *testing.T) {
	cases := map[string]string{
		"Jonas Kaninda": "Jonas's workspace",
		"Ada":           "Ada's workspace",
		"  Grace  ":     "Grace's workspace",
		"":              "My workspace",
		"   ":           "My workspace",
	}
	for in, want := range cases {
		if got := workspaceDisplayName(in); got != want {
			t.Fatalf("workspaceDisplayName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestOperationalTablesMatchPlan(t *testing.T) {
	want := []interface{}{
		&models.APIKey{}, &models.Template{}, &models.StyleSheet{}, &models.Language{},
		&models.Domain{}, &models.SMTPServer{}, &models.Webhook{}, &models.Contact{},
		&models.Subscriber{}, &models.SubscriberList{}, &models.UnsubscribeList{},
		&models.Suppression{}, &models.Bounce{}, &models.Email{}, &models.InboundEmail{},
		&models.Campaign{},
	}

	if len(operationalTables) != len(want) {
		t.Fatalf("operationalTables has %d entries, plan expects %d", len(operationalTables), len(want))
	}

	got := make(map[string]bool, len(operationalTables))
	for _, m := range operationalTables {
		got[fmt.Sprintf("%T", m)] = true
	}
	for _, m := range want {
		key := fmt.Sprintf("%T", m)
		if !got[key] {
			t.Errorf("operationalTables is missing %s", key)
		}
	}
}
