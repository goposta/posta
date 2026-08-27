// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package workspacemigrate

import (
	"fmt"
	"testing"

	"github.com/goposta/posta/internal/models"
)

func TestPersonalSlugIsStableAndUnique(t *testing.T) {
	if got := personalSlug(42); got != "personal-42" {
		t.Fatalf("personalSlug(42) = %q, want %q", got, "personal-42")
	}
	if personalSlug(1) == personalSlug(2) {
		t.Fatal("personalSlug must differ per user")
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
