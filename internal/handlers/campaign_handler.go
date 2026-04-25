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
	"errors"
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/goposta/posta/internal/worker"
	"github.com/jkaninda/logger"
	"github.com/jkaninda/okapi"
)

type CampaignHandler struct {
	campaignRepo   *repositories.CampaignRepository
	messageRepo    *repositories.CampaignMessageRepository
	listRepo       *repositories.SubscriberListRepository
	subscriberRepo *repositories.SubscriberRepository
	templateRepo   *repositories.TemplateRepository
	domainRepo     *repositories.DomainRepository
	producer       *worker.Producer
}

func NewCampaignHandler(
	campaignRepo *repositories.CampaignRepository,
	messageRepo *repositories.CampaignMessageRepository,
	listRepo *repositories.SubscriberListRepository,
	subscriberRepo *repositories.SubscriberRepository,
	templateRepo *repositories.TemplateRepository,
	domainRepo *repositories.DomainRepository,
	producer *worker.Producer,
) *CampaignHandler {
	return &CampaignHandler{
		campaignRepo:   campaignRepo,
		messageRepo:    messageRepo,
		listRepo:       listRepo,
		subscriberRepo: subscriberRepo,
		templateRepo:   templateRepo,
		domainRepo:     domainRepo,
		producer:       producer,
	}
}

type CreateCampaignRequest struct {
	Body struct {
		Name              string                 `json:"name" required:"true"`
		Subject           string                 `json:"subject" required:"true"`
		FromEmail         string                 `json:"from_email" required:"true"`
		FromName          string                 `json:"from_name"`
		TemplateID        uint                   `json:"template_id" required:"true"`
		TemplateVersionID *uint                  `json:"template_version_id"`
		Language          string                 `json:"language"`
		TemplateData      map[string]interface{} `json:"template_data"`
		ListID            uint                   `json:"list_id" required:"true"`
		SendRate          int                    `json:"send_rate"`
		SendAtLocalTime   bool                   `json:"send_at_local_time"`
		ABTestEnabled     bool                   `json:"ab_test_enabled"`
		ABTestVariants    []models.ABTestVariant `json:"ab_test_variants"`
		ScheduledAt       *time.Time             `json:"scheduled_at"`
	} `json:"body"`
}

type UpdateCampaignRequest struct {
	ID   int `param:"id"`
	Body struct {
		Name              string                 `json:"name"`
		Subject           string                 `json:"subject"`
		FromEmail         string                 `json:"from_email"`
		FromName          string                 `json:"from_name"`
		TemplateID        *uint                  `json:"template_id"`
		TemplateVersionID *uint                  `json:"template_version_id"`
		Language          string                 `json:"language"`
		TemplateData      map[string]interface{} `json:"template_data"`
		ListID            *uint                  `json:"list_id"`
		SendRate          *int                   `json:"send_rate"`
		SendAtLocalTime   *bool                  `json:"send_at_local_time"`
		ABTestEnabled     *bool                  `json:"ab_test_enabled"`
		ABTestVariants    []models.ABTestVariant `json:"ab_test_variants"`
		ScheduledAt       *time.Time             `json:"scheduled_at"`
	} `json:"body"`
}

type ListCampaignsRequest struct {
	Page   int    `query:"page" default:"0"`
	Size   int    `query:"size" default:"20"`
	Status string `query:"status"`
}

type CampaignActionRequest struct {
	ID int `param:"id"`
}

type ListCampaignMessagesRequest struct {
	ID     int    `param:"id"`
	Page   int    `query:"page" default:"0"`
	Size   int    `query:"size" default:"20"`
	Status string `query:"status"`
}

type CampaignStats struct {
	Total   int64 `json:"total"`
	Pending int64 `json:"pending"`
	Queued  int64 `json:"queued"`
	Sent    int64 `json:"sent"`
	Failed  int64 `json:"failed"`
	Skipped int64 `json:"skipped"`
}

type CampaignWithStats struct {
	models.Campaign
	Stats *CampaignStats `json:"stats,omitempty"`
}

func (h *CampaignHandler) Create(c *okapi.Context, req *CreateCampaignRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	scope := getScope(c)

	if req.Body.Name == "" || req.Body.Subject == "" || req.Body.FromEmail == "" {
		return c.AbortBadRequest("name, subject, and from_email are required")
	}

	if req.Body.ABTestEnabled {
		if len(req.Body.ABTestVariants) < 2 {
			return c.AbortBadRequest("A/B test requires at least 2 variants")
		}
		totalSplit := 0
		for _, v := range req.Body.ABTestVariants {
			if v.Name == "" || v.SplitPercentage <= 0 {
				return c.AbortBadRequest("each variant must have a name and positive split percentage")
			}
			totalSplit += v.SplitPercentage
		}
		if totalSplit != 100 {
			return c.AbortBadRequest("variant split percentages must sum to 100")
		}
	}

	lang := req.Body.Language
	if lang == "" {
		lang = "en"
	}

	campaign := &models.Campaign{
		UserID:            scope.UserID,
		WorkspaceID:       scope.WorkspaceID,
		Name:              req.Body.Name,
		Subject:           req.Body.Subject,
		FromEmail:         req.Body.FromEmail,
		FromName:          req.Body.FromName,
		TemplateID:        req.Body.TemplateID,
		TemplateVersionID: req.Body.TemplateVersionID,
		Language:          lang,
		TemplateData:      req.Body.TemplateData,
		Status:            models.CampaignStatusDraft,
		ListID:            req.Body.ListID,
		SendRate:          req.Body.SendRate,
		SendAtLocalTime:   req.Body.SendAtLocalTime,
		ABTestEnabled:     req.Body.ABTestEnabled,
		ABTestVariants:    req.Body.ABTestVariants,
		ScheduledAt:       req.Body.ScheduledAt,
	}

	if err := h.campaignRepo.Create(campaign); err != nil {
		return c.AbortInternalServerError("failed to create campaign")
	}
	return created(c, campaign)
}

func (h *CampaignHandler) List(c *okapi.Context, req *ListCampaignsRequest) error {
	page, size, offset := normalizePageParams(req.Page, req.Size)
	items, total, err := h.campaignRepo.FindByScope(getScope(c), req.Status, size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list campaigns")
	}

	// Single grouped query instead of N per-campaign stats calls.
	ids := make([]uint, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	countsByID, cerr := h.messageRepo.CountByStatusForCampaigns(ids)
	if cerr != nil {
		// Stats are best-effort; still return the list.
		countsByID = nil
	}

	result := make([]CampaignWithStats, len(items))
	for i, item := range items {
		result[i] = CampaignWithStats{Campaign: item, Stats: statsFromCounts(countsByID[item.ID])}
	}
	return paginated(c, result, total, page, size)
}

func (h *CampaignHandler) Get(c *okapi.Context, req *CampaignActionRequest) error {
	campaign, err := h.findScoped(c, uint(req.ID))
	if err != nil {
		return c.AbortNotFound("campaign not found")
	}
	stats := h.buildStats(campaign.ID)
	return ok(c, CampaignWithStats{Campaign: *campaign, Stats: stats})
}

func (h *CampaignHandler) Update(c *okapi.Context, req *UpdateCampaignRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	campaign, err := h.findScoped(c, uint(req.ID))
	if err != nil {
		return c.AbortNotFound("campaign not found")
	}
	if campaign.Status != models.CampaignStatusDraft {
		return c.AbortBadRequest("can only update draft campaigns")
	}

	if req.Body.Name != "" {
		campaign.Name = req.Body.Name
	}
	if req.Body.Subject != "" {
		campaign.Subject = req.Body.Subject
	}
	if req.Body.FromEmail != "" {
		campaign.FromEmail = req.Body.FromEmail
	}
	if req.Body.FromName != "" {
		campaign.FromName = req.Body.FromName
	}
	if req.Body.TemplateID != nil {
		campaign.TemplateID = *req.Body.TemplateID
	}
	if req.Body.TemplateVersionID != nil {
		campaign.TemplateVersionID = req.Body.TemplateVersionID
	}
	if req.Body.Language != "" {
		campaign.Language = req.Body.Language
	}
	if req.Body.TemplateData != nil {
		campaign.TemplateData = req.Body.TemplateData
	}
	if req.Body.ListID != nil {
		campaign.ListID = *req.Body.ListID
	}
	if req.Body.SendRate != nil {
		campaign.SendRate = *req.Body.SendRate
	}
	if req.Body.SendAtLocalTime != nil {
		campaign.SendAtLocalTime = *req.Body.SendAtLocalTime
	}
	if req.Body.ABTestEnabled != nil {
		campaign.ABTestEnabled = *req.Body.ABTestEnabled
	}
	if req.Body.ABTestVariants != nil {
		campaign.ABTestVariants = req.Body.ABTestVariants
	}
	if req.Body.ScheduledAt != nil {
		campaign.ScheduledAt = req.Body.ScheduledAt
	}
	now := time.Now()
	campaign.UpdatedAt = &now

	if err := h.campaignRepo.Update(campaign); err != nil {
		return c.AbortInternalServerError("failed to update campaign")
	}
	return ok(c, campaign)
}

func (h *CampaignHandler) Delete(c *okapi.Context, req *CampaignActionRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	campaign, err := h.findScoped(c, uint(req.ID))
	if err != nil {
		return c.AbortNotFound("campaign not found")
	}
	// Block deletion mid-flight so the worker doesn't race against row removal.
	// Callers should Cancel first, then Delete.
	if campaign.Status == models.CampaignStatusSending {
		return c.AbortBadRequest("cancel the campaign before deleting it")
	}
	// Snapshot stats so post-deletion analytics survive.
	snapshot := h.snapshotFor(campaign)
	if err := h.campaignRepo.Delete(campaign.ID, snapshot); err != nil {
		return c.AbortInternalServerError("failed to delete campaign")
	}
	return noContent(c)
}

func (h *CampaignHandler) Send(c *okapi.Context, req *CampaignActionRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	campaign, err := h.findScoped(c, uint(req.ID))
	if err != nil {
		return c.AbortNotFound("campaign not found")
	}
	if campaign.Status != models.CampaignStatusDraft {
		return c.AbortBadRequest("can only send draft campaigns")
	}

	// Deliverability check: surface unverified sender domains as a warning in
	// logs rather than blocking, since the SMTP-server/shared-server layer has
	// its own policy and legitimate setups exist where ownership isn't yet
	// proven (DNS propagation, shared-server tenants, etc.).
	h.warnSenderDeliverable(campaign)

	if campaign.ScheduledAt != nil && campaign.ScheduledAt.After(time.Now()) {
		if err := h.campaignRepo.TransitionStatus(
			campaign.ID,
			[]models.CampaignStatus{models.CampaignStatusDraft},
			models.CampaignStatusScheduled,
		); err != nil {
			return translateTransitionErr(c, err, "failed to schedule campaign")
		}
		campaign.Status = models.CampaignStatusScheduled
		return ok(c, campaign)
	}

	if h.producer == nil {
		return c.AbortInternalServerError("background worker is not configured; cannot send campaigns")
	}

	// Atomically claim the draft -> sending transition. Losing the race returns 409
	// so a double-click or concurrent Send doesn't double-enqueue.
	if err := h.campaignRepo.TransitionStatus(
		campaign.ID,
		[]models.CampaignStatus{models.CampaignStatusDraft},
		models.CampaignStatusSending,
	); err != nil {
		return translateTransitionErr(c, err, "failed to update campaign status")
	}

	// Idempotent enqueue: TaskID dedupes retries of the same campaign start.
	if err := h.producer.EnqueueCampaignStart(campaign.ID); err != nil {
		// Roll back so the campaign is retryable, not stuck.
		_ = h.campaignRepo.TransitionStatus(
			campaign.ID,
			[]models.CampaignStatus{models.CampaignStatusSending},
			models.CampaignStatusDraft,
		)
		return c.AbortInternalServerError("failed to enqueue campaign")
	}
	campaign.Status = models.CampaignStatusSending
	return ok(c, campaign)
}

func (h *CampaignHandler) Pause(c *okapi.Context, req *CampaignActionRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	campaign, err := h.findScoped(c, uint(req.ID))
	if err != nil {
		return c.AbortNotFound("campaign not found")
	}
	if err := h.campaignRepo.TransitionStatus(
		campaign.ID,
		[]models.CampaignStatus{models.CampaignStatusSending},
		models.CampaignStatusPaused,
	); err != nil {
		return translateTransitionErr(c, err, "failed to pause campaign")
	}
	campaign.Status = models.CampaignStatusPaused
	return ok(c, campaign)
}

func (h *CampaignHandler) Resume(c *okapi.Context, req *CampaignActionRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	campaign, err := h.findScoped(c, uint(req.ID))
	if err != nil {
		return c.AbortNotFound("campaign not found")
	}
	if h.producer == nil {
		return c.AbortInternalServerError("background worker is not configured; cannot resume campaigns")
	}
	if err := h.campaignRepo.TransitionStatus(
		campaign.ID,
		[]models.CampaignStatus{models.CampaignStatusPaused},
		models.CampaignStatusSending,
	); err != nil {
		return translateTransitionErr(c, err, "failed to resume campaign")
	}
	if err := h.producer.EnqueueCampaignBatch(campaign.ID, 0); err != nil {
		_ = h.campaignRepo.TransitionStatus(
			campaign.ID,
			[]models.CampaignStatus{models.CampaignStatusSending},
			models.CampaignStatusPaused,
		)
		return c.AbortInternalServerError("failed to enqueue campaign batch")
	}
	campaign.Status = models.CampaignStatusSending
	return ok(c, campaign)
}

func (h *CampaignHandler) Cancel(c *okapi.Context, req *CampaignActionRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	campaign, err := h.findScoped(c, uint(req.ID))
	if err != nil {
		return c.AbortNotFound("campaign not found")
	}
	if err := h.campaignRepo.TransitionStatus(
		campaign.ID,
		[]models.CampaignStatus{
			models.CampaignStatusSending,
			models.CampaignStatusPaused,
			models.CampaignStatusScheduled,
		},
		models.CampaignStatusCancelled,
	); err != nil {
		return translateTransitionErr(c, err, "failed to cancel campaign")
	}
	// Drain: mark still-pending messages as skipped. Already-queued emails are
	// filtered at dispatch time by EmailSendHandler checking the campaign status.
	_, _ = h.messageRepo.SkipPendingForCampaign(campaign.ID, "campaign cancelled")
	campaign.Status = models.CampaignStatusCancelled
	return ok(c, campaign)
}

func (h *CampaignHandler) Duplicate(c *okapi.Context, req *CampaignActionRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	campaign, err := h.findScoped(c, uint(req.ID))
	if err != nil {
		return c.AbortNotFound("campaign not found")
	}
	scope := getScope(c)

	clone := &models.Campaign{
		UserID:            scope.UserID,
		WorkspaceID:       scope.WorkspaceID,
		Name:              campaign.Name + " (copy)",
		Subject:           campaign.Subject,
		FromEmail:         campaign.FromEmail,
		FromName:          campaign.FromName,
		TemplateID:        campaign.TemplateID,
		TemplateVersionID: campaign.TemplateVersionID,
		Language:          campaign.Language,
		TemplateData:      cloneTemplateData(campaign.TemplateData),
		Status:            models.CampaignStatusDraft,
		ListID:            campaign.ListID,
		SendRate:          campaign.SendRate,
		SendAtLocalTime:   campaign.SendAtLocalTime,
		ABTestEnabled:     campaign.ABTestEnabled,
		ABTestVariants:    append(models.ABTestVariants(nil), campaign.ABTestVariants...),
	}

	if err := h.campaignRepo.Create(clone); err != nil {
		return c.AbortInternalServerError("failed to duplicate campaign")
	}
	return created(c, clone)
}

func (h *CampaignHandler) ListMessages(c *okapi.Context, req *ListCampaignMessagesRequest) error {
	campaign, err := h.findScoped(c, uint(req.ID))
	if err != nil {
		return c.AbortNotFound("campaign not found")
	}
	page, size, offset := normalizePageParams(req.Page, req.Size)
	items, total, err := h.messageRepo.FindByCampaign(campaign.ID, req.Status, size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list campaign messages")
	}
	return paginated(c, items, total, page, size)
}

// findScoped centralizes the "find + workspace ownership check" pattern so it
// can't be forgotten at a callsite.
func (h *CampaignHandler) findScoped(c *okapi.Context, id uint) (*models.Campaign, error) {
	return h.campaignRepo.FindByIDForScope(id, getScope(c))
}

// warnSenderDeliverable logs (but does not block) when the sender domain is
// registered for this user/workspace but lacks ownership/SPF/DKIM verification.
// Blocking belongs to the SMTP-server layer, which already enforces per-server
// policy — here we just give the operator a hint that their bounce rate is
// about to spike.
func (h *CampaignHandler) warnSenderDeliverable(campaign *models.Campaign) {
	if h.domainRepo == nil {
		return
	}
	at := strings.LastIndexByte(campaign.FromEmail, '@')
	if at < 0 || at == len(campaign.FromEmail)-1 {
		return
	}
	domain := strings.ToLower(campaign.FromEmail[at+1:])
	d, err := h.domainRepo.FindByUserIDAndDomain(campaign.UserID, domain)
	if err != nil || d == nil {
		return
	}
	if !d.OwnershipVerified || !d.SPFVerified || !d.DKIMVerified {
		logger.Warn("campaign: sending from domain with incomplete DNS verification",
			"campaign_id", campaign.ID,
			"domain", domain,
			"ownership_verified", d.OwnershipVerified,
			"spf_verified", d.SPFVerified,
			"dkim_verified", d.DKIMVerified,
		)
	}
}

// snapshotFor captures the pre-deletion stats of a campaign so reports survive.
func (h *CampaignHandler) snapshotFor(campaign *models.Campaign) models.TemplateData {
	stats := h.buildStats(campaign.ID)
	if stats == nil {
		return nil
	}
	return models.TemplateData{
		"total":       stats.Total,
		"pending":     stats.Pending,
		"queued":      stats.Queued,
		"sent":        stats.Sent,
		"failed":      stats.Failed,
		"skipped":     stats.Skipped,
		"snapshot_at": time.Now().UTC().Format(time.RFC3339),
		"from_email":  campaign.FromEmail,
		"list_id":     campaign.ListID,
		"template_id": campaign.TemplateID,
	}
}

// cloneTemplateData is a shallow copy at the top level; campaign template data
// is JSON so shared references below the top level would only matter if a
// caller mutated them, which is not a pattern we use.
func cloneTemplateData(src models.TemplateData) models.TemplateData {
	if src == nil {
		return nil
	}
	dst := make(models.TemplateData, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// translateTransitionErr maps an atomic transition failure to either 409 (on
// conflict) or 500 (on any other DB error).
func translateTransitionErr(c *okapi.Context, err error, genericMsg string) error {
	if errors.Is(err, repositories.ErrStatusConflict) {
		return c.AbortConflict("campaign is not in a state that allows this action")
	}
	return c.AbortInternalServerError(genericMsg)
}

// statsFromCounts projects a status->count map into the CampaignStats shape.
func statsFromCounts(counts map[models.CampaignMessageStatus]int64) *CampaignStats {
	if counts == nil {
		return nil
	}
	stats := &CampaignStats{}
	for status, count := range counts {
		stats.Total += count
		switch status {
		case models.CampaignMsgPending:
			stats.Pending = count
		case models.CampaignMsgQueued:
			stats.Queued = count
		case models.CampaignMsgSent:
			stats.Sent = count
		case models.CampaignMsgFailed:
			stats.Failed = count
		case models.CampaignMsgSkipped:
			stats.Skipped = count
		}
	}
	return stats
}

// buildStats computes stats for a campaign from the message counts.
func (h *CampaignHandler) buildStats(campaignID uint) *CampaignStats {
	counts, err := h.messageRepo.CountByStatus(campaignID)
	if err != nil {
		return nil
	}
	stats := &CampaignStats{}
	for status, count := range counts {
		stats.Total += count
		switch status {
		case models.CampaignMsgPending:
			stats.Pending = count
		case models.CampaignMsgQueued:
			stats.Queued = count
		case models.CampaignMsgSent:
			stats.Sent = count
		case models.CampaignMsgFailed:
			stats.Failed = count
		case models.CampaignMsgSkipped:
			stats.Skipped = count
		}
	}
	return stats
}
