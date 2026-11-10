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

package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

const (
	ChatHomeItemMessage = "message"
	ChatHomeItemLink    = "link"
	ChatHomeItemCard    = "card"
)

// ChatHome is the widget's home screen and menu; all text is plain and rendered as text nodes on the customer's page.
type ChatHome struct {
	Enabled     bool           `json:"enabled" doc:"Show a home screen with a tab bar; off keeps the single chat thread"`
	HeaderColor string         `json:"header_color,omitempty" doc:"#RRGGBB background of the home header; the text colour is picked for contrast"`
	Headline    string         `json:"headline,omitempty" doc:"Second line of the home header, under the greeting" example:"How can we help?"`
	Items       []ChatHomeItem `json:"items,omitempty" doc:"Home screen items in order; at most 12, and at most one message item"`
	HelpTitle   string         `json:"help_title,omitempty" doc:"Heading of the Help tab"`
	HelpLinks   []ChatLink     `json:"help_links,omitempty" doc:"Links listed on the Help tab; at most 30. No links hides the tab."`
}

type ChatHomeItem struct {
	ID        string `json:"id" doc:"Stable short ID; generated when empty"`
	Kind      string `json:"kind" enum:"message,link,card"`
	Title     string `json:"title" doc:"At most 80 characters; required for link and card items"`
	Body      string `json:"body,omitempty" doc:"Card text, at most 280 characters"`
	URL       string `json:"url,omitempty" doc:"https, http, mailto or tel URL; required for link items"`
	LinkLabel string `json:"link_label,omitempty" doc:"Card link text, e.g. See the dates"`
	NewTab    bool   `json:"new_tab" doc:"Open the link in a new tab"`
}

type ChatLink struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	URL         string `json:"url"`
}

func (h ChatHome) Value() (driver.Value, error) {
	return json.Marshal(h)
}

func (h *ChatHome) Scan(value any) error {
	*h = ChatHome{}
	switch v := value.(type) {
	case nil:
		return nil
	case []byte:
		return json.Unmarshal(v, h)
	case string:
		return json.Unmarshal([]byte(v), h)
	}
	return fmt.Errorf("chat home: unsupported type %T", value)
}

func DefaultChatHome() ChatHome {
	return ChatHome{
		Enabled:  true,
		Headline: "How can we help?",
		Items:    []ChatHomeItem{{ID: "message", Kind: ChatHomeItemMessage, Title: "Send us a message"}},
	}
}
