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

package repositories

import (
	"encoding/json"
	"strings"

	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
)

// EraseMessagesBySender hard-deletes, in one transaction, every web form
// message one address submitted to a workspace, the thread replies on them,
// and the inbound emails those replies arrived as. Soft-deleted rows are
// included: an erasure request is not a trash can.
//
// It returns the number of messages deleted and the blob keys (attachments,
// raw .eml files) the caller must remove once the transaction has committed.
func EraseMessagesBySender(db *gorm.DB, workspaceID uint, email string) (int64, []string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var blobKeys []string
	var deleted int64
	err := db.Transaction(func(tx *gorm.DB) error {
		var msgs []models.Message
		if err := tx.Unscoped().
			Select("id", "attachments_json").
			Where("workspace_id = ? AND LOWER(sender_email) = ?", workspaceID, email).
			Find(&msgs).Error; err != nil {
			return err
		}
		if len(msgs) == 0 {
			return nil
		}
		ids := make([]uint, 0, len(msgs))
		for _, m := range msgs {
			ids = append(ids, m.ID)
			blobKeys = append(blobKeys, AttachmentBlobKeys(m.AttachmentsJSON)...)
		}

		var inboundIDs []uint
		if err := tx.Model(&models.MessageReply{}).
			Where("message_id IN ? AND inbound_email_id IS NOT NULL", ids).
			Pluck("inbound_email_id", &inboundIDs).Error; err != nil {
			return err
		}
		if len(inboundIDs) > 0 {
			var inbound []models.InboundEmail
			if err := tx.Unscoped().
				Select("id", "attachments_json", "raw_storage_key").
				Where("id IN ? AND workspace_id = ?", inboundIDs, workspaceID).
				Find(&inbound).Error; err != nil {
				return err
			}
			for _, rec := range inbound {
				blobKeys = append(blobKeys, AttachmentBlobKeys(rec.AttachmentsJSON)...)
				if rec.RawStorageKey != "" {
					blobKeys = append(blobKeys, rec.RawStorageKey)
				}
			}
			if err := tx.Unscoped().Where("id IN ? AND workspace_id = ?", inboundIDs, workspaceID).
				Delete(&models.InboundEmail{}).Error; err != nil {
				return err
			}
		}

		// Delete replies explicitly rather than relying on the cascade alone, so
		// erasure holds even where the foreign key was never created.
		if err := tx.Where("message_id IN ?", ids).Delete(&models.MessageReply{}).Error; err != nil {
			return err
		}
		result := tx.Unscoped().Where("id IN ?", ids).Delete(&models.Message{})
		if result.Error != nil {
			return result.Error
		}
		deleted = result.RowsAffected
		return nil
	})
	if err != nil {
		return 0, nil, err
	}
	return deleted, blobKeys, nil
}

// AttachmentBlobKeys returns the blob storage keys in an attachments_json
// column (a []models.InboundAttachmentMeta).
func AttachmentBlobKeys(attachmentsJSON string) []string {
	if attachmentsJSON == "" {
		return nil
	}
	var atts []models.InboundAttachmentMeta
	if err := json.Unmarshal([]byte(attachmentsJSON), &atts); err != nil {
		return nil
	}
	keys := make([]string, 0, len(atts))
	for _, a := range atts {
		if a.StorageKey != "" {
			keys = append(keys, a.StorageKey)
		}
	}
	return keys
}
