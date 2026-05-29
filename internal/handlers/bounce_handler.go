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

package handlers

import (
	"encoding/json"
	"time"

	"github.com/goposta/posta/internal/metrics"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/webhook"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
)

const (
	bounceTypeHard      = "hard"
	bounceTypeSoft      = "soft"
	bounceTypeComplaint = "complaint"
)

type BounceHandler struct {
	bounceRepo      *repositories.BounceRepository
	suppressionRepo *repositories.SuppressionRepository
	emailRepo       *repositories.EmailRepository
	dispatcher      *webhook.Dispatcher
}
type RecordBounceRequest struct {
	Body struct {
		EmailID   string `json:"email_id" required:"true"`
		Recipient string `json:"recipient" required:"true" format:"email"`
		Type      string `json:"type" required:"true"`
		Reason    string `json:"reason"`
	} `json:"body"`
}

func NewBounceHandler(bounceRepo *repositories.BounceRepository, suppressionRepo *repositories.SuppressionRepository, emailRepo *repositories.EmailRepository, dispatcher *webhook.Dispatcher) *BounceHandler {
	return &BounceHandler{bounceRepo: bounceRepo, suppressionRepo: suppressionRepo, emailRepo: emailRepo, dispatcher: dispatcher}
}

func (h *BounceHandler) Record(c *okapi.Context, req *RecordBounceRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	scope := getScope(c)

	validTypes := map[string]bool{bounceTypeHard: true, bounceTypeSoft: true, bounceTypeComplaint: true}
	if !validTypes[req.Body.Type] {
		return c.AbortBadRequest("invalid bounce type. Valid types: hard, soft, complaint")
	}

	em, err := h.emailRepo.FindByUUID(req.Body.EmailID)
	if err != nil {
		return c.AbortNotFound("email not found")
	}

	bounce := &models.Bounce{
		UserID:      scope.UserID,
		WorkspaceID: scope.WorkspaceID,
		EmailID:     em.ID,
		Recipient:   req.Body.Recipient,
		Type:        models.BounceType(req.Body.Type),
		Reason:      req.Body.Reason,
	}

	if err := h.bounceRepo.Create(bounce); err != nil {
		return c.AbortInternalServerError("failed to record bounce")
	}

	metrics.IncrementBounce(req.Body.Type)

	// Auto-suppress on hard bounce or complaint. Kind is set explicitly so the
	// row is distinguishable from a manual block in the suppression table.
	if req.Body.Type == bounceTypeHard || req.Body.Type == bounceTypeComplaint {
		kind := models.SuppressionKindBounce
		if req.Body.Type == bounceTypeComplaint {
			kind = models.SuppressionKindComplaint
		}
		suppression := &models.Suppression{
			UserID:      scope.UserID,
			WorkspaceID: scope.WorkspaceID,
			Email:       req.Body.Recipient,
			Kind:        kind,
			Reason:      "auto-suppressed: " + req.Body.Type + " bounce",
		}
		// Ignore error if already suppressed
		if err := h.suppressionRepo.Create(suppression); err == nil {
			metrics.IncrementSuppression()
		}
	}

	// Complaint webhook: high-value signal for senders to mirror into their own
	// CRM. Mirrors the email.unsubscribed payload shape.
	if req.Body.Type == bounceTypeComplaint {
		h.emitComplained(em, req.Body.Recipient)
	}

	return created(c, bounce)
}

// emitComplained fires the email.complained webhook for a recorded complaint.
func (h *BounceHandler) emitComplained(em *models.Email, addr string) {
	if h.dispatcher == nil {
		return
	}
	payload := struct {
		Event     string `json:"event"`
		EmailUUID string `json:"email_uuid"`
		Email     string `json:"email"`
		Timestamp string `json:"timestamp"`
	}{
		Event:     "email.complained",
		EmailUUID: em.UUID,
		Email:     addr,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	h.dispatcher.DispatchJSON(em.UserID, em.WorkspaceID, "email.complained", body, em.Sender)
}

func (h *BounceHandler) List(c *okapi.Context, req *ListRequest) error {
	page, size, offset := normalizePageParams(req.Page, req.Size)

	bounces, total, err := h.bounceRepo.FindByScope(getScope(c), size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list bounces")
	}

	return paginated(c, bounces, total, page, size)
}
