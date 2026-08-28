// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package workspaceprovision

import (
	"fmt"
	"testing"

	"github.com/goposta/posta/internal/models"
)

func TestWorkspaceSlugIsStableAndUnique(t *testing.T) {
	if got := workspaceSlug(42); got != "workspace-42" {
		t.Fatalf("workspaceSlug(42) = %q, want %q", got, "workspace-42")
	}
	if workspaceSlug(1) == workspaceSlug(2) {
		t.Fatal("workspaceSlug must differ per user")
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
		if got := workspaceName(in); got != want {
			t.Fatalf("workspaceName(%q) = %q, want %q", in, got, want)
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
