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
	"time"

	"github.com/jkaninda/okapi"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/goposta/posta/internal/worker"
)

type CampaignHandler struct {
	campaignRepo    *repositories.CampaignRepository
	messageRepo     *repositories.CampaignMessageRepository
	listRepo        *repositories.SubscriberListRepository
	subscriberRepo  *repositories.SubscriberRepository
	templateRepo    *repositories.TemplateRepository
	producer        *worker.Producer
}

func NewCampaignHandler(
	campaignRepo *repositories.CampaignRepository,
	messageRepo *repositories.CampaignMessageRepository,
	listRepo *repositories.SubscriberListRepository,
	subscriberRepo *repositories.SubscriberRepository,
	templateRepo *repositories.TemplateRepository,
	producer *worker.Producer,
) *CampaignHandler {
	return &CampaignHandler{
		campaignRepo:   campaignRepo,
		messageRepo:    messageRepo,
		listRepo:       listRepo,
		subscriberRepo: subscriberRepo,
		templateRepo:   templateRepo,
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
		ABTestVariants    []models.ABTestVariant  `json:"ab_test_variants"`
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
		ABTestVariants    []models.ABTestVariant  `json:"ab_test_variants"`
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
		return err
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

	result := make([]CampaignWithStats, len(items))
	for i, item := range items {
		stats := h.buildStats(item.ID)
		result[i] = CampaignWithStats{Campaign: item, Stats: stats}
	}
	return paginated(c, result, total, page, size)
}

func (h *CampaignHandler) Get(c *okapi.Context, req *CampaignActionRequest) error {
	campaign, err := h.campaignRepo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, campaign.UserID, campaign.WorkspaceID) {
		return c.AbortNotFound("campaign not found")
	}
	stats := h.buildStats(campaign.ID)
	return ok(c, CampaignWithStats{Campaign: *campaign, Stats: stats})
}

func (h *CampaignHandler) Update(c *okapi.Context, req *UpdateCampaignRequest) error {
	if err := requireEdit(c); err != nil {
		return err
	}
	campaign, err := h.campaignRepo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, campaign.UserID, campaign.WorkspaceID) {
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
		return err
	}
	campaign, err := h.campaignRepo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, campaign.UserID, campaign.WorkspaceID) {
		return c.AbortNotFound("campaign not found")
	}
	if campaign.Status != models.CampaignStatusDraft && campaign.Status != models.CampaignStatusCancelled {
		return c.AbortBadRequest("can only delete draft or cancelled campaigns")
	}
	if err := h.campaignRepo.Delete(campaign.ID); err != nil {
		return c.AbortInternalServerError("failed to delete campaign")
	}
	return noContent(c)
}

func (h *CampaignHandler) Send(c *okapi.Context, req *CampaignActionRequest) error {
	if err := requireEdit(c); err != nil {
		return err
	}
	campaign, err := h.campaignRepo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, campaign.UserID, campaign.WorkspaceID) {
		return c.AbortNotFound("campaign not found")
	}
	if campaign.Status != models.CampaignStatusDraft {
		return c.AbortBadRequest("can only send draft campaigns")
	}

	if campaign.ScheduledAt != nil && campaign.ScheduledAt.After(time.Now()) {
		// Schedule for later
		if err := h.campaignRepo.UpdateStatus(campaign.ID, models.CampaignStatusScheduled); err != nil {
			return c.AbortInternalServerError("failed to schedule campaign")
		}
		campaign.Status = models.CampaignStatusScheduled
		return ok(c, campaign)
	}

	// Send immediately
	if err := h.campaignRepo.UpdateStatus(campaign.ID, models.CampaignStatusSending); err != nil {
		return c.AbortInternalServerError("failed to update campaign status")
	}
	if err := h.producer.EnqueueCampaignStart(campaign.ID); err != nil {
		return c.AbortInternalServerError("failed to enqueue campaign")
	}
	campaign.Status = models.CampaignStatusSending
	return ok(c, campaign)
}

func (h *CampaignHandler) Pause(c *okapi.Context, req *CampaignActionRequest) error {
	if err := requireEdit(c); err != nil {
		return err
	}
	campaign, err := h.campaignRepo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, campaign.UserID, campaign.WorkspaceID) {
		return c.AbortNotFound("campaign not found")
	}
	if campaign.Status != models.CampaignStatusSending {
		return c.AbortBadRequest("can only pause sending campaigns")
	}
	if err := h.campaignRepo.UpdateStatus(campaign.ID, models.CampaignStatusPaused); err != nil {
		return c.AbortInternalServerError("failed to pause campaign")
	}
	campaign.Status = models.CampaignStatusPaused
	return ok(c, campaign)
}

func (h *CampaignHandler) Resume(c *okapi.Context, req *CampaignActionRequest) error {
	if err := requireEdit(c); err != nil {
		return err
	}
	campaign, err := h.campaignRepo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, campaign.UserID, campaign.WorkspaceID) {
		return c.AbortNotFound("campaign not found")
	}
	if campaign.Status != models.CampaignStatusPaused {
		return c.AbortBadRequest("can only resume paused campaigns")
	}
	if err := h.campaignRepo.UpdateStatus(campaign.ID, models.CampaignStatusSending); err != nil {
		return c.AbortInternalServerError("failed to resume campaign")
	}
	if err := h.producer.EnqueueCampaignBatch(campaign.ID, 0); err != nil {
		return c.AbortInternalServerError("failed to enqueue campaign batch")
	}
	campaign.Status = models.CampaignStatusSending
	return ok(c, campaign)
}

func (h *CampaignHandler) Cancel(c *okapi.Context, req *CampaignActionRequest) error {
	if err := requireEdit(c); err != nil {
		return err
	}
	campaign, err := h.campaignRepo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, campaign.UserID, campaign.WorkspaceID) {
		return c.AbortNotFound("campaign not found")
	}
	if campaign.Status != models.CampaignStatusSending &&
		campaign.Status != models.CampaignStatusPaused &&
		campaign.Status != models.CampaignStatusScheduled {
		return c.AbortBadRequest("can only cancel sending, paused, or scheduled campaigns")
	}
	if err := h.campaignRepo.UpdateStatus(campaign.ID, models.CampaignStatusCancelled); err != nil {
		return c.AbortInternalServerError("failed to cancel campaign")
	}
	campaign.Status = models.CampaignStatusCancelled
	return ok(c, campaign)
}

func (h *CampaignHandler) Duplicate(c *okapi.Context, req *CampaignActionRequest) error {
	if err := requireEdit(c); err != nil {
		return err
	}
	campaign, err := h.campaignRepo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, campaign.UserID, campaign.WorkspaceID) {
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
		TemplateData:      campaign.TemplateData,
		Status:            models.CampaignStatusDraft,
		ListID:            campaign.ListID,
		SendRate:          campaign.SendRate,
		SendAtLocalTime:   campaign.SendAtLocalTime,
		ABTestEnabled:     campaign.ABTestEnabled,
		ABTestVariants:    campaign.ABTestVariants,
	}

	if err := h.campaignRepo.Create(clone); err != nil {
		return c.AbortInternalServerError("failed to duplicate campaign")
	}
	return created(c, clone)
}

func (h *CampaignHandler) ListMessages(c *okapi.Context, req *ListCampaignMessagesRequest) error {
	campaign, err := h.campaignRepo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, campaign.UserID, campaign.WorkspaceID) {
		return c.AbortNotFound("campaign not found")
	}
	page, size, offset := normalizePageParams(req.Page, req.Size)
	items, total, err := h.messageRepo.FindByCampaign(campaign.ID, req.Status, size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list campaign messages")
	}
	return paginated(c, items, total, page, size)
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
