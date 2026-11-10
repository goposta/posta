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

package chat

import (
	"testing"

	"github.com/goposta/posta/internal/models"
)

func TestNormalizeLauncher(t *testing.T) {
	out, err := NormalizeLauncher(models.ChatLauncher{Label: "  Need\nhelp? ", Icon: "chat", OffsetX: 24, OffsetY: 80, HideOnMobile: true})
	if err != nil || out.Label != "Need help?" || out.Icon != "" || out.OffsetY != 80 || !out.HideOnMobile {
		t.Fatalf("got %+v, %v", out, err)
	}
	for name, l := range map[string]models.ChatLauncher{
		"unknown icon":    {Icon: "skull"},
		"negative offset": {OffsetX: -1},
		"huge offset":     {OffsetY: 201},
	} {
		if _, err := NormalizeLauncher(l); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestNormalizeTeaser(t *testing.T) {
	out, err := NormalizeTeaser(models.ChatTeaser{Enabled: true, Text: "Questions?", DelaySeconds: 10, Paths: []string{" /pricing ", "", "/docs/*"}})
	if err != nil || len(out.Paths) != 2 || out.Paths[0] != "/pricing" {
		t.Fatalf("got %+v, %v", out, err)
	}
	for name, tz := range map[string]models.ChatTeaser{
		"on without text": {Enabled: true},
		"long delay":      {DelaySeconds: 121},
		"relative path":   {Paths: []string{"pricing"}},
		"star in middle":  {Paths: []string{"/a/*/b"}},
		"full url":        {Paths: []string{"https://example.com/pricing"}},
		"query":           {Paths: []string{"/pricing?x=1"}},
	} {
		if _, err := NormalizeTeaser(tz); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestHomeItemTitle(t *testing.T) {
	h := models.ChatHome{
		Items:     []models.ChatHomeItem{{ID: "message", Kind: models.ChatHomeItemMessage}, {ID: "faq", Kind: models.ChatHomeItemLink, Title: "FAQ"}},
		HelpLinks: []models.ChatLink{{ID: "ship", Title: "Shipping"}},
	}
	for id, want := range map[string]string{"message": "Send us a message", "faq": "FAQ", "ship": "Shipping"} {
		if got, ok := HomeItemTitle(h, id); !ok || got != want {
			t.Errorf("%s: got %q %v", id, got, ok)
		}
	}
	if _, ok := HomeItemTitle(h, "nope"); ok {
		t.Error("an unknown ID was accepted")
	}
}
