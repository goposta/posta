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
	ChatAuthorVisitor = "visitor"
	ChatAuthorAgent   = "agent"
	ChatAuthorSystem  = "system"
)

// ChatMessage bodies are plain text, never rendered as HTML; Seq lets a reconnecting client ask for what it missed.
type ChatMessage struct {
	ID             uint   `json:"-" gorm:"primaryKey"`
	UUID           string `json:"id" gorm:"type:uuid;default:gen_random_uuid();uniqueIndex;not null"`
	WorkspaceID    *uint  `json:"-" gorm:"index;not null"`
	ConversationID uint   `json:"-" gorm:"not null"`
	Seq            int64  `json:"seq" gorm:"not null"`
	AuthorKind     string `json:"author_kind" gorm:"type:varchar(8);not null"`
	// AuthorID is the agent's user ID; nil for visitor and system messages.
	AuthorID    *uint  `json:"author_id,omitempty"`
	Body        string `json:"body" gorm:"type:text;not null"`
	ClientMsgID string `json:"client_msg_id,omitempty" gorm:"size:36;default:'';not null"`
	// Channel is email for an emailed answer; EmailUUID is the email that carried an agent's message to a visitor who left.
	Channel     string          `json:"channel" gorm:"type:varchar(8);default:'chat';not null"`
	EmailUUID   string          `json:"email_uuid,omitempty" gorm:"index"`
	Attachments ChatAttachments `json:"attachments,omitempty" gorm:"type:jsonb"`
	CreatedAt   time.Time       `json:"created_at"`

	Author *ActorRef `json:"author,omitempty" gorm:"->;foreignKey:AuthorID;references:ID;constraint:-"`
}

// ChatAttachment.Key is the blob store key and never leaves the server; clients fetch by message and index.
type ChatAttachment struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	Key         string `json:"-"`
}

// chatAttachmentStored is the stored shape, which keeps Key.
type chatAttachmentStored struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	Key         string `json:"key"`
}

type ChatAttachments []ChatAttachment

func (a ChatAttachments) Value() (driver.Value, error) {
	if len(a) == 0 {
		return nil, nil
	}
	stored := make([]chatAttachmentStored, len(a))
	for i, x := range a {
		stored[i] = chatAttachmentStored(x)
	}
	return json.Marshal(stored)
}

func (a *ChatAttachments) Scan(value any) error {
	*a = nil
	var raw []byte
	switch v := value.(type) {
	case nil:
		return nil
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("chat attachments: unsupported type %T", value)
	}
	var stored []chatAttachmentStored
	if err := json.Unmarshal(raw, &stored); err != nil {
		return err
	}
	for _, x := range stored {
		*a = append(*a, ChatAttachment(x))
	}
	return nil
}
