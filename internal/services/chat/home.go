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
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/goposta/posta/internal/models"
)

const (
	MaxHomeItems     = 12
	MaxHelpLinks     = 30
	MaxHomeBytes     = 16 << 10
	homeTitleMax     = 80
	homeBodyMax      = 280
	homeLabelMax     = 40
	homeHeadlineMax  = 120
	helpDescMax      = 140
	homeLinkURLMax   = 2000
	homeItemIDLength = 16
)

var homeItemID = regexp.MustCompile(`^[a-z0-9-]{1,16}$`)

// NormalizeHome cleans and validates a home screen before it is stored; it ends up on customers' pages.
func NormalizeHome(h models.ChatHome) (models.ChatHome, error) {
	out := models.ChatHome{Enabled: h.Enabled}
	color := strings.ToLower(strings.TrimSpace(h.HeaderColor))
	if !ValidAccentColor(color) {
		return out, fmt.Errorf("home.header_color must be a #RRGGBB colour")
	}
	out.HeaderColor = color
	out.Headline = CleanLine(h.Headline, homeHeadlineMax)
	out.HelpTitle = CleanLine(h.HelpTitle, homeTitleMax)

	if len(h.Items) > MaxHomeItems {
		return out, fmt.Errorf("home can have at most %d items", MaxHomeItems)
	}
	if len(h.HelpLinks) > MaxHelpLinks {
		return out, fmt.Errorf("home can have at most %d help links", MaxHelpLinks)
	}
	seen := map[string]bool{}
	messages := 0
	for i, it := range h.Items {
		item, err := normalizeHomeItem(it)
		if err != nil {
			return out, fmt.Errorf("home.items[%d]: %w", i, err)
		}
		if item.Kind == models.ChatHomeItemMessage {
			if messages++; messages > 1 {
				return out, fmt.Errorf("home can have only one message item")
			}
		}
		item.ID = uniqueItemID(item.ID, seen)
		out.Items = append(out.Items, item)
	}
	for i, l := range h.HelpLinks {
		link := models.ChatLink{
			Title:       CleanLine(l.Title, homeTitleMax),
			Description: CleanLine(l.Description, helpDescMax),
		}
		if link.Title == "" {
			return out, fmt.Errorf("home.help_links[%d]: a title is required", i)
		}
		u, err := CleanHomeURL(l.URL)
		if err != nil || u == "" {
			return out, fmt.Errorf("home.help_links[%d]: an https, http, mailto or tel URL is required", i)
		}
		link.URL = u
		link.ID = uniqueItemID(strings.TrimSpace(l.ID), seen)
		out.HelpLinks = append(out.HelpLinks, link)
	}
	if raw, err := json.Marshal(out); err != nil || len(raw) > MaxHomeBytes {
		return out, fmt.Errorf("home is too large; keep it under %d KiB", MaxHomeBytes>>10)
	}
	return out, nil
}

func normalizeHomeItem(it models.ChatHomeItem) (models.ChatHomeItem, error) {
	item := models.ChatHomeItem{
		ID:     strings.TrimSpace(it.ID),
		Kind:   it.Kind,
		Title:  CleanLine(it.Title, homeTitleMax),
		NewTab: it.NewTab,
	}
	switch it.Kind {
	case models.ChatHomeItemMessage:
		item.NewTab = false
		return item, nil
	case models.ChatHomeItemLink, models.ChatHomeItemCard:
	default:
		return item, fmt.Errorf("kind must be message, link or card")
	}
	if item.Title == "" {
		return item, fmt.Errorf("a title is required")
	}
	u, err := CleanHomeURL(it.URL)
	if err != nil {
		return item, err
	}
	item.URL = u
	if it.Kind == models.ChatHomeItemLink {
		if u == "" {
			return item, fmt.Errorf("a link needs a URL")
		}
		return item, nil
	}
	body := CleanBody(it.Body)
	if len([]rune(body)) > homeBodyMax {
		return item, fmt.Errorf("card text must be at most %d characters", homeBodyMax)
	}
	item.Body = body
	if u != "" {
		item.LinkLabel = CleanLine(it.LinkLabel, homeLabelMax)
	}
	return item, nil
}

// CleanHomeURL allows only schemes that cannot run script on the customer's page.
func CleanHomeURL(raw string) (string, error) {
	u := strings.TrimSpace(raw)
	if u == "" {
		return "", nil
	}
	if len(u) > homeLinkURLMax {
		return "", fmt.Errorf("URLs can be at most %d characters", homeLinkURLMax)
	}
	parsed, err := url.Parse(u)
	if err != nil {
		return "", fmt.Errorf("the URL is not valid")
	}
	switch strings.ToLower(parsed.Scheme) {
	case schemeHTTPS, schemeHTTP:
		if parsed.Host == "" {
			return "", fmt.Errorf("the URL needs a host")
		}
	case "mailto", "tel":
		if parsed.Opaque == "" {
			return "", fmt.Errorf("the URL needs an address")
		}
	default:
		return "", fmt.Errorf("URLs must use https, http, mailto or tel")
	}
	return u, nil
}

// uniqueItemID keeps a well-formed ID the editor chose so analytics stay stable, and makes one up otherwise.
func uniqueItemID(id string, seen map[string]bool) string {
	if !homeItemID.MatchString(id) || seen[id] {
		id = newItemID()
		for seen[id] {
			id = newItemID()
		}
	}
	seen[id] = true
	return id
}

func newItemID() string {
	b := make([]byte, homeItemIDLength/4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
