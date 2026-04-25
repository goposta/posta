/*
 * Copyright 2026 Jonas Kaninda
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

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
// delivery records based on platform retention settings.
type RetentionCleanupJob struct {
	emailRepo        *repositories.EmailRepository
	eventRepo        *repositories.EventRepository
	whDeliveryRepo   *repositories.WebhookDeliveryRepository
	trackingRepo     *repositories.TrackingRepository
	inboundEmailRepo *repositories.InboundEmailRepository
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

func (j *RetentionCleanupJob) Run(_ context.Context, _ *asynq.Client) error {
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
