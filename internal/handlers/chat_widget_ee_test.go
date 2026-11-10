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

package handlers

import (
	"errors"
	"testing"

	"github.com/goposta/posta/internal/enterprise"
	"github.com/goposta/posta/internal/models"
)

// Community may save a widget as long as it leaves the Enterprise auto-assign settings unchanged.
func TestChatAutoAssignNeedsALicenseToChange(t *testing.T) {
	h := &ChatWidgetHandler{edition: enterprise.Community()}
	w := &models.ChatWidget{AutoAssign: false, MaxOpenPerAgent: 0}
	on, off, five, zero := true, false, 5, 0

	var gate enterprise.GateError
	if err := h.requireEnterpriseSettings(w, chatWidgetSettings{AutoAssign: &on}); !errors.As(err, &gate) || gate.HTTPStatus() != 402 {
		t.Fatalf("turning auto-assign on in Community: err = %v, want a 402 gate error", err)
	}
	if err := h.requireEnterpriseSettings(w, chatWidgetSettings{MaxOpenPerAgent: &five}); err == nil {
		t.Fatal("changing the per-agent cap in Community was allowed")
	}
	if err := h.requireEnterpriseSettings(w, chatWidgetSettings{AutoAssign: &off, MaxOpenPerAgent: &zero}); err != nil {
		t.Fatalf("resending the stored values was refused: %v", err)
	}
	if err := h.requireEnterpriseSettings(w, chatWidgetSettings{}); err != nil {
		t.Fatalf("a save without auto-assign fields was refused: %v", err)
	}
	// No edition wired at all behaves as Community.
	if err := (&ChatWidgetHandler{}).requireEnterpriseSettings(w, chatWidgetSettings{AutoAssign: &on}); err == nil {
		t.Fatal("a handler without an edition allowed auto-assign")
	}
}

func TestChatTeaserNeedsALicenseToChange(t *testing.T) {
	h := &ChatWidgetHandler{edition: enterprise.Community()}
	stored := models.ChatTeaser{Enabled: true, Text: "Questions about pricing?", DelaySeconds: 5, Paths: []string{"/pricing"}}
	w := &models.ChatWidget{Teaser: stored}
	same := stored
	if err := h.requireEnterpriseSettings(w, chatWidgetSettings{Teaser: &same}); err != nil {
		t.Fatalf("resending the stored teaser was refused: %v", err)
	}
	changed := stored
	changed.Text = "Need help?"
	var gate enterprise.GateError
	if err := h.requireEnterpriseSettings(w, chatWidgetSettings{Teaser: &changed}); !errors.As(err, &gate) || gate.HTTPStatus() != 402 {
		t.Fatalf("changing the teaser in Community: err = %v, want a 402 gate error", err)
	}
}
