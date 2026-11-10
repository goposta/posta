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
	"strings"
	"testing"

	"github.com/goposta/posta/internal/models"
)

func TestNormalizeHomeCleansAndKeepsIDs(t *testing.T) {
	in := models.ChatHome{
		Enabled:     true,
		HeaderColor: " #0F172A ",
		Headline:    "  How can\n we help?  ",
		Items: []models.ChatHomeItem{
			{ID: "chat", Kind: models.ChatHomeItemMessage, Title: "Send us a message", URL: "https://ignored.example", NewTab: true},
			{ID: "faq", Kind: models.ChatHomeItemLink, Title: "Read the FAQ", URL: "https://example.com/faq", NewTab: true},
			{Kind: models.ChatHomeItemCard, Title: "Webinars", Body: "Register\nnow", URL: "https://example.com/w", LinkLabel: "See the dates"},
		},
		HelpLinks: []models.ChatLink{{ID: "faq", Title: "Shipping", URL: "mailto:help@example.com"}},
	}
	out, err := NormalizeHome(in)
	if err != nil {
		t.Fatal(err)
	}
	if out.HeaderColor != "#0f172a" || out.Headline != "How can we help?" {
		t.Fatalf("header not cleaned: %q %q", out.HeaderColor, out.Headline)
	}
	if out.Items[0].ID != "chat" || out.Items[1].ID != "faq" || out.Items[2].ID == "" {
		t.Fatalf("item IDs: %+v", out.Items)
	}
	if out.Items[0].URL != "" || out.Items[0].NewTab {
		t.Fatal("a message item kept a URL")
	}
	if out.Items[2].Body != "Register\nnow" || out.Items[2].LinkLabel != "See the dates" {
		t.Fatalf("card: %+v", out.Items[2])
	}
	if out.HelpLinks[0].ID == "faq" || out.HelpLinks[0].ID == "" {
		t.Fatal("a duplicate ID across items and help links was kept")
	}
}

func TestNormalizeHomeRefuses(t *testing.T) {
	link := func(u string) models.ChatHome {
		return models.ChatHome{Items: []models.ChatHomeItem{{Kind: models.ChatHomeItemLink, Title: "x", URL: u}}}
	}
	many := func(n int) []models.ChatHomeItem {
		items := make([]models.ChatHomeItem, n)
		for i := range items {
			items[i] = models.ChatHomeItem{Kind: models.ChatHomeItemLink, Title: "x", URL: "https://example.com"}
		}
		return items
	}
	for name, h := range map[string]models.ChatHome{
		"javascript url":         link("javascript:alert(1)"),
		"data url":               link("data:text/html,<script>alert(1)</script>"),
		"relative url":           link("/faq"),
		"link without url":       link(""),
		"mailto without address": link("mailto:"),
		"long url":               link("https://example.com/" + strings.Repeat("a", 2000)),
		"bad colour":             {HeaderColor: "red;}body{display:none"},
		"unknown kind":           {Items: []models.ChatHomeItem{{Kind: "html", Title: "x"}}},
		"untitled link":          {Items: []models.ChatHomeItem{{Kind: models.ChatHomeItemLink, URL: "https://example.com"}}},
		"two message items":      {Items: []models.ChatHomeItem{{Kind: models.ChatHomeItemMessage}, {Kind: models.ChatHomeItemMessage}}},
		"too many items":         {Items: many(MaxHomeItems + 1)},
		"long card":              {Items: []models.ChatHomeItem{{Kind: models.ChatHomeItemCard, Title: "x", Body: strings.Repeat("a", 281)}}},
		"help link without url":  {HelpLinks: []models.ChatLink{{Title: "x"}}},
	} {
		if _, err := NormalizeHome(h); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestNormalizeHomeSizeCap(t *testing.T) {
	links := make([]models.ChatLink, MaxHelpLinks)
	for i := range links {
		links[i] = models.ChatLink{Title: "Link", Description: strings.Repeat("d", 140), URL: "https://example.com/" + strings.Repeat("p", 500)}
	}
	if _, err := NormalizeHome(models.ChatHome{HelpLinks: links}); err == nil {
		t.Fatal("a home over the size cap was accepted")
	}
}

func TestNormalizeHomeAssets(t *testing.T) {
	const id = "0b6f3b0e-4a43-4c1e-9d2a-6a1f0e7c2b11"
	out, err := NormalizeHome(models.ChatHome{
		LogoAsset: id,
		Items:     []models.ChatHomeItem{{Kind: models.ChatHomeItemCard, Title: "x", ImageAsset: id}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := out.AssetIDs(); len(got) != 2 {
		t.Fatalf("asset IDs: %v", got)
	}
	for name, h := range map[string]models.ChatHome{
		"logo path": {LogoAsset: "../../etc/passwd"},
		"image url": {Items: []models.ChatHomeItem{{Kind: models.ChatHomeItemCard, Title: "x", ImageAsset: "https://evil.example/a.png"}}},
	} {
		if _, err := NormalizeHome(h); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	pub := ToPublicHome(out)
	if pub.LogoURL != "/widget/v1/assets/"+id || pub.Items[0].ImageURL != "/widget/v1/assets/"+id {
		t.Fatalf("public home: %+v", pub)
	}
}
