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

package jobs

import (
	"context"
	"encoding/json"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/settings"
	"github.com/goposta/posta/internal/storage/blob"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/hibiken/asynq"
	"github.com/jkaninda/logger"
)

// RetentionCleanupJob purges old email logs, audit events, and webhook
// delivery records based on platform retention settings. It also scrubs email
// bodies and attachments off records that outlive their (shorter) content windows.
type RetentionCleanupJob struct {
	emailRepo        *repositories.EmailRepository
	eventRepo        *repositories.EventRepository
	whDeliveryRepo   *repositories.WebhookDeliveryRepository
	trackingRepo     *repositories.TrackingRepository
	inboundEmailRepo *repositories.InboundEmailRepository
	messageRepo      *repositories.MessageRepository
	formRepo         *repositories.FormRepository
	notificationRepo *repositories.NotificationRepository
	blobStore        blob.Store
	settings         *settings.Provider
}

func NewRetentionCleanupJob(
	emailRepo *repositories.EmailRepository,
	eventRepo *repositories.EventRepository,
	whDeliveryRepo *repositories.WebhookDeliveryRepository,
	trackingRepo *repositories.TrackingRepository,
	sp *settings.Provider,
) *RetentionCleanupJob {
	return &RetentionCleanupJob{
		emailRepo:      emailRepo,
		eventRepo:      eventRepo,
		whDeliveryRepo: whDeliveryRepo,
		trackingRepo:   trackingRepo,
		settings:       sp,
	}
}

// SetInboundEmailRepo configures the inbound email repository for cleanup.
// When nil, inbound emails are not subject to retention cleanup.
func (j *RetentionCleanupJob) SetInboundEmailRepo(r *repositories.InboundEmailRepository) {
	j.inboundEmailRepo = r
}

func (j *RetentionCleanupJob) SetMessageRepo(r *repositories.MessageRepository) {
	j.messageRepo = r
}

// SetFormRepo enables per-form message retention (Form.RetentionDays).
func (j *RetentionCleanupJob) SetFormRepo(r *repositories.FormRepository) {
	j.formRepo = r
}

// SetNotificationRepo enables pruning of the in-app inbox. Only settled items
// are dropped; an unresolved, undismissed notification is still true however old
// it is, so age alone never removes it.
func (j *RetentionCleanupJob) SetNotificationRepo(r *repositories.NotificationRepository) {
	j.notificationRepo = r
}

// SetBlobStore configures the blob store so retention cleanup can also purge
// attachment bytes (and raw .eml) before dropping DB rows. Nil disables blob
// cleanup — DB rows are still purged, but blobs will leak.
func (j *RetentionCleanupJob) SetBlobStore(bs blob.Store) {
	j.blobStore = bs
}

// deleteOutboundAttachmentBlobs enumerates StorageKeys inside each attachments_json
// payload and deletes them from blob storage. Returns the number of keys deleted.
func (j *RetentionCleanupJob) deleteOutboundAttachmentBlobs(jsons []string) int {
	if j.blobStore == nil {
		return 0
	}
	ctx := context.Background()
	deleted := 0
	for _, js := range jsons {
		if js == "" {
			continue
		}
		var atts []models.Attachment
		if err := json.Unmarshal([]byte(js), &atts); err != nil {
			continue
		}
		for _, a := range atts {
			if a.StorageKey == "" {
				continue
			}
			if err := j.blobStore.Delete(ctx, a.StorageKey); err != nil {
				logger.Warn("retention cleanup: failed to delete attachment blob", "key", a.StorageKey, "error", err)
				continue
			}
			deleted++
		}
	}
	return deleted
}

// deleteInboundAttachmentBlobs does the same for inbound attachment metadata.
func (j *RetentionCleanupJob) deleteInboundAttachmentBlobs(jsons []string) int {
	if j.blobStore == nil {
		return 0
	}
	ctx := context.Background()
	deleted := 0
	for _, js := range jsons {
		if js == "" {
			continue
		}
		var atts []models.InboundAttachmentMeta
		if err := json.Unmarshal([]byte(js), &atts); err != nil {
			continue
		}
		for _, a := range atts {
			if a.StorageKey == "" {
				continue
			}
			if err := j.blobStore.Delete(ctx, a.StorageKey); err != nil {
				logger.Warn("retention cleanup: failed to delete inbound attachment blob", "key", a.StorageKey, "error", err)
				continue
			}
			deleted++
		}
	}
	return deleted
}

// deleteRawKeys removes each raw .eml blob for the given storage keys.
func (j *RetentionCleanupJob) deleteRawKeys(keys []string) int {
	if j.blobStore == nil {
		return 0
	}
	ctx := context.Background()
	deleted := 0
	for _, k := range keys {
		if k == "" {
			continue
		}
		if err := j.blobStore.Delete(ctx, k); err != nil {
			logger.Warn("retention cleanup: failed to delete raw eml blob", "key", k, "error", err)
			continue
		}
		deleted++
	}
	return deleted
}

func (j *RetentionCleanupJob) Name() string     { return "retention-cleanup" }
func (j *RetentionCleanupJob) Schedule() string { return "0 3 * * *" } // daily at 03:00 UTC

// capDays caps a content window at the record window. A value of 0 means "keep
// forever" on either side, so it is never treated as a cap or capped.
func capDays(v, limit int) int {
	if limit <= 0 || v <= 0 {
		return v
	}
	if v > limit {
		return limit
	}
	return v
}

// minDays returns the shorter of two windows, treating 0 ("forever") as no bound.
func minDays(a, b int) int {
	if a <= 0 {
		return b
	}
	if b <= 0 {
		return a
	}
	if a < b {
		return a
	}
	return b
}

// notificationRetentionDays is how long a read-and-settled inbox item is kept.
// It is not operator-configurable: the inbox is a UI convenience, not a record,
// and the audit log is where history is meant to live.
const notificationRetentionDays = 90

func (j *RetentionCleanupJob) Run(_ context.Context, _ *asynq.Client) error {
	if j.notificationRepo != nil {
		before := time.Now().AddDate(0, 0, -notificationRetentionDays)
		if deleted, err := j.notificationRepo.Prune(before); err != nil {
			logger.Error("retention cleanup: failed to prune notifications", "error", err)
		} else if deleted > 0 {
			logger.Info("retention cleanup: pruned notifications", "count", deleted)
		}
	}

	// Clean up email logs (and any blob-stored attachments).
	emailRetention := j.settings.RetentionDays()
	if emailRetention > 0 {
		before := time.Now().AddDate(0, 0, -emailRetention)

		// Outbound attachments
		if j.blobStore != nil {
			if jsons, err := j.emailRepo.AttachmentsJSONOlderThan(before); err == nil {
				n := j.deleteOutboundAttachmentBlobs(jsons)
				if n > 0 {
					logger.Info("retention cleanup: deleted outbound attachment blobs", "count", n)
				}
			} else {
				logger.Error("retention cleanup: failed to enumerate outbound attachments", "error", err)
			}
		}

		deleted, err := j.emailRepo.DeleteOlderThan(before)
		if err != nil {
			logger.Error("retention cleanup: failed to delete old emails", "error", err)
		} else if deleted > 0 {
			logger.Info("retention cleanup: deleted old emails", "count", deleted, "older_than_days", emailRetention)
		}

		if j.inboundEmailRepo != nil {
			// Inbound attachments + raw .eml
			if j.blobStore != nil {
				if jsons, rawKeys, err := j.inboundEmailRepo.InboundBlobKeysOlderThan(before); err == nil {
					n := j.deleteInboundAttachmentBlobs(jsons)
					r := j.deleteRawKeys(rawKeys)
					if n+r > 0 {
						logger.Info("retention cleanup: deleted inbound blobs", "attachments", n, "raw_eml", r)
					}
				} else {
					logger.Error("retention cleanup: failed to enumerate inbound blobs", "error", err)
				}
			}

			deleted, err := j.inboundEmailRepo.DeleteOlderThan(before)
			if err != nil {
				logger.Error("retention cleanup: failed to delete old inbound emails", "error", err)
			} else if deleted > 0 {
				logger.Info("retention cleanup: deleted old inbound emails", "count", deleted, "older_than_days", emailRetention)
			}
		}
	}

	j.cleanupMessages()

	// Content windows can never usefully exceed the record window — the row (and
	// all its content) is deleted first — so cap them at the email log retention.
	bodyRetention := capDays(j.settings.EmailBodyRetentionDays(), emailRetention)
	attachmentRetention := capDays(j.settings.EmailAttachmentRetentionDays(), emailRetention)

	// Scrub attachments off records that outlive the attachment window (keeps the row).
	if attachmentRetention > 0 {
		before := time.Now().AddDate(0, 0, -attachmentRetention)

		if j.blobStore != nil {
			if jsons, err := j.emailRepo.AttachmentsJSONOlderThan(before); err == nil {
				j.deleteOutboundAttachmentBlobs(jsons)
			} else {
				logger.Error("retention cleanup: failed to enumerate outbound attachments for scrub", "error", err)
			}
		}
		if scrubbed, err := j.emailRepo.ScrubAttachmentsOlderThan(before); err != nil {
			logger.Error("retention cleanup: failed to scrub outbound attachments", "error", err)
		} else if scrubbed > 0 {
			logger.Info("retention cleanup: scrubbed outbound attachments", "count", scrubbed, "older_than_days", attachmentRetention)
		}

		if j.inboundEmailRepo != nil {
			if j.blobStore != nil {
				if jsons, _, err := j.inboundEmailRepo.InboundBlobKeysOlderThan(before); err == nil {
					j.deleteInboundAttachmentBlobs(jsons)
				} else {
					logger.Error("retention cleanup: failed to enumerate inbound attachments for scrub", "error", err)
				}
			}
			if scrubbed, err := j.inboundEmailRepo.ScrubAttachmentsOlderThan(before); err != nil {
				logger.Error("retention cleanup: failed to scrub inbound attachments", "error", err)
			} else if scrubbed > 0 {
				logger.Info("retention cleanup: scrubbed inbound attachments", "count", scrubbed, "older_than_days", attachmentRetention)
			}
		}
	}

	// Scrub bodies off records that outlive the body window (keeps the row).
	if bodyRetention > 0 {
		before := time.Now().AddDate(0, 0, -bodyRetention)

		if scrubbed, err := j.emailRepo.ScrubBodiesOlderThan(before); err != nil {
			logger.Error("retention cleanup: failed to scrub outbound bodies", "error", err)
		} else if scrubbed > 0 {
			logger.Info("retention cleanup: scrubbed outbound bodies", "count", scrubbed, "older_than_days", bodyRetention)
		}

		if j.inboundEmailRepo != nil {
			if scrubbed, err := j.inboundEmailRepo.ScrubBodiesOlderThan(before); err != nil {
				logger.Error("retention cleanup: failed to scrub inbound bodies", "error", err)
			} else if scrubbed > 0 {
				logger.Info("retention cleanup: scrubbed inbound bodies", "count", scrubbed, "older_than_days", bodyRetention)
			}
		}
	}

	// The raw .eml holds both body and attachments, so purge it at the shorter of
	// the two windows — it must never outlive either content type.
	if rawRetention := minDays(bodyRetention, attachmentRetention); rawRetention > 0 && j.inboundEmailRepo != nil {
		before := time.Now().AddDate(0, 0, -rawRetention)

		if j.blobStore != nil {
			if _, rawKeys, err := j.inboundEmailRepo.InboundBlobKeysOlderThan(before); err == nil {
				j.deleteRawKeys(rawKeys)
			} else {
				logger.Error("retention cleanup: failed to enumerate inbound raw blobs for scrub", "error", err)
			}
		}
		if scrubbed, err := j.inboundEmailRepo.ScrubRawOlderThan(before); err != nil {
			logger.Error("retention cleanup: failed to scrub inbound raw messages", "error", err)
		} else if scrubbed > 0 {
			logger.Info("retention cleanup: scrubbed inbound raw messages", "count", scrubbed, "older_than_days", rawRetention)
		}
	}

	// Clean up audit/event logs
	auditRetention := j.settings.AuditLogRetentionDays()
	if auditRetention > 0 {
		before := time.Now().AddDate(0, 0, -auditRetention)
		deleted, err := j.eventRepo.DeleteOlderThan(before)
		if err != nil {
			logger.Error("retention cleanup: failed to delete old events", "error", err)
		} else if deleted > 0 {
			logger.Info("retention cleanup: deleted old events", "count", deleted, "older_than_days", auditRetention)
		}
	}

	// Clean up webhook delivery logs
	whRetention := j.settings.WebhookDeliveryRetentionDays()
	if whRetention > 0 {
		before := time.Now().AddDate(0, 0, -whRetention)
		deleted, err := j.whDeliveryRepo.DeleteOlderThan(before)
		if err != nil {
			logger.Error("retention cleanup: failed to delete old webhook deliveries", "error", err)
		} else if deleted > 0 {
			logger.Info("retention cleanup: deleted old webhook deliveries", "count", deleted, "older_than_days", whRetention)
		}
	}

	// Clean up old tracking events
	trackingRetention := j.settings.TrackingEventRetentionDays()
	if trackingRetention > 0 {
		before := time.Now().AddDate(0, 0, -trackingRetention)
		deleted, err := j.trackingRepo.DeleteOlderThan(before)
		if err != nil {
			logger.Error("retention cleanup: failed to delete old tracking events", "error", err)
		} else if deleted > 0 {
			logger.Info("retention cleanup: deleted old tracking events", "count", deleted, "older_than_days", trackingRetention)
		}
	}

	return nil
}

func (j *RetentionCleanupJob) cleanupMessages() {
	if j.messageRepo == nil {
		return
	}

	if days := j.settings.MessageSpamRetentionDays(); days > 0 {
		before := time.Now().AddDate(0, 0, -days)
		if j.blobStore != nil {
			if jsons, err := j.messageRepo.SpamAttachmentKeysOlderThan(before); err == nil {
				if n := j.deleteInboundAttachmentBlobs(jsons); n > 0 {
					logger.Info("retention cleanup: deleted spam message attachment blobs", "count", n)
				}
			} else {
				logger.Error("retention cleanup: failed to enumerate spam message attachments", "error", err)
			}
		}
		if deleted, err := j.messageRepo.DeleteSpamOlderThan(before); err != nil {
			logger.Error("retention cleanup: failed to delete spam messages", "error", err)
		} else if deleted > 0 {
			logger.Info("retention cleanup: deleted spam messages", "count", deleted, "older_than_days", days)
		}
	}

	days := j.settings.MessageRetentionDays()
	j.cleanupFormMessages(days)
	if days <= 0 {
		return
	}
	before := time.Now().AddDate(0, 0, -days)

	if j.blobStore != nil {
		if jsons, err := j.messageRepo.AttachmentKeysOlderThan(before); err == nil {
			if n := j.deleteInboundAttachmentBlobs(jsons); n > 0 {
				logger.Info("retention cleanup: deleted message attachment blobs", "count", n)
			}
		} else {
			logger.Error("retention cleanup: failed to enumerate message attachments", "error", err)
		}
	}

	if deleted, err := j.messageRepo.DeleteOlderThan(before); err != nil {
		logger.Error("retention cleanup: failed to delete old messages", "error", err)
	} else if deleted > 0 {
		logger.Info("retention cleanup: deleted old messages", "count", deleted, "older_than_days", days)
	}
}

// formRetentionDays is the window that applies to one form's messages. A form
// can shorten retention below the platform setting but never extend it past
// that ceiling; 0 means the form sets no window of its own.
func formRetentionDays(form, platform int) int {
	if form <= 0 {
		return 0
	}
	return minDays(form, platform)
}

// cleanupFormMessages applies each form's own retention window before the
// platform-wide pass.
func (j *RetentionCleanupJob) cleanupFormMessages(platformDays int) {
	if j.formRepo == nil {
		return
	}
	forms, err := j.formRepo.FindWithRetention()
	if err != nil {
		logger.Error("retention cleanup: failed to list forms with retention", "error", err)
		return
	}
	for _, form := range forms {
		days := formRetentionDays(form.RetentionDays, platformDays)
		if days <= 0 {
			continue
		}
		before := time.Now().AddDate(0, 0, -days)
		if j.blobStore != nil {
			if jsons, err := j.messageRepo.AttachmentKeysByFormOlderThan(form.ID, before); err == nil {
				j.deleteInboundAttachmentBlobs(jsons)
			} else {
				logger.Error("retention cleanup: failed to enumerate form message attachments", "form_id", form.ID, "error", err)
				continue
			}
		}
		if deleted, err := j.messageRepo.DeleteByFormOlderThan(form.ID, before); err != nil {
			logger.Error("retention cleanup: failed to delete form messages", "form_id", form.ID, "error", err)
		} else if deleted > 0 {
			logger.Info("retention cleanup: deleted form messages", "form_id", form.ID, "count", deleted, "older_than_days", days)
		}
	}
}
