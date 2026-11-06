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

package inbox

import (
	"testing"

	"github.com/goposta/posta/internal/models"
)

func keys(rules []rule) map[string]rule {
	out := make(map[string]rule, len(rules))
	for _, r := range rules {
		out[r.dedupKey] = r
	}
	return out
}

func TestRatesNeedVolume(t *testing.T) {
	// A workspace with three sends and one bounce is at 33%, which means nothing.
	quiet := Snapshot{TotalEmails: 3, BounceRate: 33, FailureRate: 33}
	if got := keys(evaluate(quiet)); len(got) != 0 {
		t.Fatalf("expected no rate alerts below the volume floor, got %v", got)
	}

	busy := Snapshot{TotalEmails: minVolumeForRates, BounceRate: 33, FailureRate: 33}
	got := keys(evaluate(busy))
	if _, ok := got["deliverability:bounce-rate"]; !ok {
		t.Fatal("expected a bounce rate alert once there is enough volume")
	}
	if _, ok := got["deliverability:failure-rate"]; !ok {
		t.Fatal("expected a failure rate alert once there is enough volume")
	}
}

// A dismissal must survive a rate wobbling inside its band; crossing into the next band is new information.
func TestRateFingerprintIsBanded(t *testing.T) {
	at := func(rate float64) string {
		r := keys(evaluate(Snapshot{TotalEmails: 100, BounceRate: rate}))["deliverability:bounce-rate"]
		return r.fingerprint
	}

	if at(5.1) != at(5.9) {
		t.Fatalf("noise inside a band must not resurface a dismissal: %s vs %s", at(5.1), at(5.9))
	}
	if at(5.1) == at(12.0) {
		t.Fatal("crossing a band must resurface a dismissal")
	}
}

func TestCountFingerprintTracksTheCount(t *testing.T) {
	three := keys(evaluate(Snapshot{UnverifiedDomains: 3}))["domains:unverified"]
	four := keys(evaluate(Snapshot{UnverifiedDomains: 4}))["domains:unverified"]

	if three.fingerprint == four.fingerprint {
		t.Fatal("one more unverified domain is new information and must resurface")
	}
	if three.title != "3 domains not verified" {
		t.Fatalf("unexpected title %q", three.title)
	}
}

func TestMessagesRuleRespectsTheFeatureFlag(t *testing.T) {
	off := keys(evaluate(Snapshot{UnreadMessages: 5, MessagesEnabled: false}))
	if _, ok := off["messages:unread"]; ok {
		t.Fatal("the messages rule must not fire when the feature is disabled")
	}
	on := keys(evaluate(Snapshot{UnreadMessages: 5, MessagesEnabled: true}))
	if _, ok := on["messages:unread"]; !ok {
		t.Fatal("the messages rule should fire when the feature is enabled")
	}
}

func TestSingularTitles(t *testing.T) {
	one := keys(evaluate(Snapshot{UnverifiedDomains: 1, ExpiringAPIKeys: 1}))
	if one["domains:unverified"].title != "1 domain not verified" {
		t.Fatalf("unexpected title %q", one["domains:unverified"].title)
	}
	if one["security:expiring-api-keys"].title != "1 API key expiring within 7 days" {
		t.Fatalf("unexpected title %q", one["security:expiring-api-keys"].title)
	}
}

// Viewers cannot verify a domain or rotate a key, so actionable rules are gated above them.
func TestRoleGating(t *testing.T) {
	all := evaluate(Snapshot{
		UnverifiedDomains: 1,
		ExpiringAPIKeys:   1,
		UnreadMessages:    1,
		MessagesEnabled:   true,
		TotalEmails:       100,
		BounceRate:        9,
	})

	visible := func(role models.WorkspaceRole) map[string]bool {
		out := map[string]bool{}
		for _, r := range all {
			if roleRank(role) >= roleRank(r.minRole) {
				out[r.dedupKey] = true
			}
		}
		return out
	}

	viewer := visible(models.WorkspaceRoleViewer)
	if !viewer["messages:unread"] {
		t.Fatal("a viewer should still be told about unread messages")
	}
	if viewer["domains:unverified"] || viewer["security:expiring-api-keys"] {
		t.Fatal("a viewer cannot act on domains or API keys and should not be alerted")
	}

	editor := visible(models.WorkspaceRoleEditor)
	if !editor["domains:unverified"] || !editor["deliverability:bounce-rate"] {
		t.Fatal("an editor should see the domain and deliverability alerts")
	}
	if editor["security:expiring-api-keys"] {
		t.Fatal("API key expiry is an admin concern")
	}

	owner := visible(models.WorkspaceRoleOwner)
	if len(owner) != len(all) {
		t.Fatalf("an owner should see everything: %d of %d", len(owner), len(all))
	}
}
