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
	"time"
)

const (
	ChatLauncherIconChat = "chat"
	ChatLauncherIconHelp = "help"
	ChatLauncherIconMail = "mail"
)

type ChatLauncher struct {
	Label        string `json:"label,omitempty" doc:"Text shown next to the closed button, e.g. Need help?"`
	Icon         string `json:"icon,omitempty" enum:"chat,help,mail"`
	OffsetX      int    `json:"offset_x,omitempty" doc:"Pixels from the side edge, 0-200; 0 keeps the default"`
	OffsetY      int    `json:"offset_y,omitempty" doc:"Pixels from the bottom edge, 0-200; 0 keeps the default"`
	HideOnMobile bool   `json:"hide_on_mobile,omitempty" doc:"Hide the button on narrow screens; PostaChat.open() and data-posta-chat-open still work"`
}

func (l ChatLauncher) Value() (driver.Value, error) { return json.Marshal(l) }

func (l *ChatLauncher) Scan(value any) error { return scanJSON(value, l, "chat launcher") }

// ChatTeaser is a bubble over the closed launcher, offered on some pages after a delay (Posta Enterprise).
type ChatTeaser struct {
	Enabled      bool     `json:"enabled"`
	Text         string   `json:"text,omitempty" doc:"At most 140 characters, plain text"`
	DelaySeconds int      `json:"delay_seconds" doc:"Seconds on the page before the teaser shows, 0-120"`
	Paths        []string `json:"paths,omitempty" doc:"Page paths it shows on, such as /pricing or /docs/*; empty means every page"`
}

func (t ChatTeaser) Value() (driver.Value, error) { return json.Marshal(t) }

func (t *ChatTeaser) Scan(value any) error { return scanJSON(value, t, "chat teaser") }

func scanJSON(value, dst any, what string) error {
	switch v := value.(type) {
	case nil:
		return nil
	case []byte:
		return json.Unmarshal(v, dst)
	case string:
		return json.Unmarshal([]byte(v), dst)
	}
	return fmt.Errorf("%s: unsupported type %T", what, value)
}

// ChatWidgetClick counts anonymous clicks on one home item or help link per day; nothing identifies the visitor.
type ChatWidgetClick struct {
	ID          uint      `json:"-" gorm:"primaryKey"`
	WorkspaceID uint      `json:"-" gorm:"index;not null"`
	WidgetID    uint      `json:"widget_id" gorm:"uniqueIndex:idx_chat_click_day;not null"`
	ItemID      string    `json:"item_id" gorm:"type:varchar(16);uniqueIndex:idx_chat_click_day;not null"`
	Day         time.Time `json:"day" gorm:"type:date;uniqueIndex:idx_chat_click_day;not null"`
	Count       int64     `json:"count" gorm:"not null;default:0"`
}
