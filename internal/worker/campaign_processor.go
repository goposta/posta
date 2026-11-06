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

package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/email"
	"github.com/goposta/posta/internal/services/tracking"
	"github.com/goposta/posta/internal/services/webhook"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/hibiken/asynq"
	"github.com/jkaninda/logger"
	"github.com/jkaninda/okapi"
	"github.com/lib/pq"
)

type CampaignProcessor struct {
	campaignRepo     *repositories.CampaignRepository
	messageRepo      *repositories.CampaignMessageRepository
	listRepo         *repositories.SubscriberListRepository
	subscriberRepo   *repositories.SubscriberRepository
	emailRepo        *repositories.EmailRepository
	templateRepo     *repositories.TemplateRepository
	versionRepo      *repositories.TemplateVersionRepository
	localizationRepo *repositories.TemplateLocalizationRepository
	trackingService  *tracking.Service
	producer         *Producer
	dispatcher       *webhook.Dispatcher
}

func NewCampaignProcessor(
	campaignRepo *repositories.CampaignRepository,
	messageRepo *repositories.CampaignMessageRepository,
	listRepo *repositories.SubscriberListRepository,
	subscriberRepo *repositories.SubscriberRepository,
	emailRepo *repositories.EmailRepository,
	templateRepo *repositories.TemplateRepository,
	versionRepo *repositories.TemplateVersionRepository,
	localizationRepo *repositories.TemplateLocalizationRepository,
	trackingService *tracking.Service,
	producer *Producer,
	dispatcher *webhook.Dispatcher,
) *CampaignProcessor {
	return &CampaignProcessor{
		campaignRepo:     campaignRepo,
		messageRepo:      messageRepo,
		listRepo:         listRepo,
		subscriberRepo:   subscriberRepo,
		emailRepo:        emailRepo,
		templateRepo:     templateRepo,
		versionRepo:      versionRepo,
		localizationRepo: localizationRepo,
		trackingService:  trackingService,
		producer:         producer,
		dispatcher:       dispatcher,
	}
}

const campaignBatchSize = 100

func (p *CampaignProcessor) HandleCampaignStart(_ context.Context, t *asynq.Task) error {
	var payload CampaignPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal campaign start payload: %w", err)
	}

	campaign, err := p.campaignRepo.FindByID(payload.CampaignID)
	if err != nil {
		return fmt.Errorf("campaign not found: %w", err)
	}

	if campaign.Status != models.CampaignStatusSending {
		logger.Info("campaign is not in sending status, skipping", "id", campaign.ID, "status", campaign.Status)
		return nil
	}

	list, err := p.listRepo.FindByID(campaign.ListID)
	if err != nil {
		return fmt.Errorf("subscriber list not found: %w", err)
	}

	var subscribers []models.Subscriber
	scope := repositories.ResourceScope{UserID: campaign.UserID, WorkspaceID: campaign.WorkspaceID}

	if list.Type == models.SubscriberListTypeDynamic {
		subscribers, _, err = p.subscriberRepo.FindByFilterRules(scope, list.FilterRules, -1, 0)
		if err != nil {
			return fmt.Errorf("failed to resolve dynamic list: %w", err)
		}
	} else {
		subscribers, _, err = p.listRepo.ListMembers(list.ID, -1, 0)
		if err != nil {
			return fmt.Errorf("failed to list members: %w", err)
		}
	}

	if len(subscribers) == 0 {
		logger.Info("campaign has no subscribers, marking as sent", "id", campaign.ID)
		_ = p.campaignRepo.UpdateStatus(campaign.ID, models.CampaignStatusSent)
		return nil
	}

	suppressed, sErr := p.listRepo.SuppressedSubscriberIDs(campaign.ListID)
	if sErr != nil {
		logger.Warn("campaign: failed to load list suppressions, proceeding without", "list_id", campaign.ListID, "error", sErr)
		suppressed = nil
	}

	messages := make([]models.CampaignMessage, 0, len(subscribers))
	for _, sub := range subscribers {
		if sub.Status != models.SubscriberStatusSubscribed {
			continue
		}
		if _, opt := suppressed[sub.ID]; opt {
			continue
		}
		messages = append(messages, models.CampaignMessage{
			CampaignID:   campaign.ID,
			SubscriberID: sub.ID,
			Status:       models.CampaignMsgPending,
		})
	}

	if len(messages) == 0 {
		logger.Info("no eligible subscribers, marking campaign as sent", "id", campaign.ID)
		_ = p.campaignRepo.UpdateStatus(campaign.ID, models.CampaignStatusSent)
		return nil
	}

	if campaign.ABTestEnabled && len(campaign.ABTestVariants) > 0 {
		rand.Shuffle(len(messages), func(i, j int) {
			messages[i], messages[j] = messages[j], messages[i]
		})

		idx := 0
		for _, variant := range campaign.ABTestVariants {
			count := len(messages) * variant.SplitPercentage / 100
			if count == 0 {
				count = 1
			}
			end := idx + count
			if end > len(messages) {
				end = len(messages)
			}
			for i := idx; i < end; i++ {
				messages[i].Variant = variant.Name
			}
			idx = end
		}
		if idx < len(messages) {
			lastVariant := campaign.ABTestVariants[len(campaign.ABTestVariants)-1].Name
			for i := idx; i < len(messages); i++ {
				messages[i].Variant = lastVariant
			}
		}
	}

	if _, err := p.messageRepo.BulkCreate(messages); err != nil {
		return fmt.Errorf("failed to create campaign messages: %w", err)
	}

	logger.Info("campaign messages created, starting batch processing",
		"campaign_id", campaign.ID, "messages", len(messages))

	if campaign.SendAtLocalTime && campaign.ScheduledAt != nil {
		logger.Info("campaign with timezone-aware scheduling", "campaign_id", campaign.ID)
	}

	if err := p.producer.EnqueueCampaignBatch(campaign.ID, 0); err != nil {
		return err
	}
	if p.dispatcher != nil {
		p.dispatcher.DispatchCampaign(campaign.UserID, campaign.WorkspaceID, "campaign.started", campaign.ID, campaign.Name, campaign.FromEmail)
	}
	return nil
}

func (p *CampaignProcessor) HandleCampaignBatch(_ context.Context, t *asynq.Task) error {
	var payload CampaignPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal campaign batch payload: %w", err)
	}

	campaign, err := p.campaignRepo.FindByID(payload.CampaignID)
	if err != nil {
		return fmt.Errorf("campaign not found: %w", err)
	}

	if campaign.Status != models.CampaignStatusSending {
		logger.Info("campaign is not in sending status, skipping batch", "id", campaign.ID, "status", campaign.Status)
		return nil
	}

	pendingMessages, err := p.messageRepo.FindPendingByCampaign(campaign.ID, campaignBatchSize)
	if err != nil {
		return fmt.Errorf("failed to get pending messages: %w", err)
	}

	if len(pendingMessages) == 0 {
		logger.Info("campaign sending complete", "id", campaign.ID)
		if err := p.campaignRepo.UpdateStatus(campaign.ID, models.CampaignStatusSent); err != nil {
			return err
		}
		if p.dispatcher != nil {
			p.dispatcher.DispatchCampaign(campaign.UserID, campaign.WorkspaceID, "campaign.completed", campaign.ID, campaign.Name, campaign.FromEmail)
		}
		return nil
	}

	var templateName string
	if tmpl, err := p.templateRepo.FindByID(campaign.TemplateID); err == nil && tmpl != nil {
		templateName = tmpl.Name
	}

	// Cache resolved content per language to avoid re-rendering for each subscriber
	contentCache := make(map[string]*resolvedContent)
	resolveForLang := func(lang string) *resolvedContent {
		if lang == "" {
			lang = campaign.Language
		}
		if lang == "" {
			lang = "en"
		}
		if cached, ok := contentCache[lang]; ok {
			return cached
		}
		c := p.resolveTemplateContent(campaign, lang)
		if c != nil {
			contentCache[lang] = c
		}
		return c
	}

	defaultContent := resolveForLang(campaign.Language)
	if defaultContent == nil {
		return fmt.Errorf("failed to resolve template content for campaign %d", campaign.ID)
	}

	for i := range pendingMessages {
		msg := &pendingMessages[i]

		// Resolve content for subscriber's language (falls back to campaign language)
		content := defaultContent
		if msg.Subscriber.Language != "" {
			if langContent := resolveForLang(msg.Subscriber.Language); langContent != nil {
				content = langContent
			}
		}

		if campaign.ABTestEnabled && msg.Variant != "" {
			for _, v := range campaign.ABTestVariants {
				if v.Name == msg.Variant && v.Subject != "" {
					// Copy to avoid mutating cached content
					override := *content
					override.Subject = v.Subject
					content = &override
					break
				}
			}
		}

		sender := campaign.FromEmail
		if campaign.FromName != "" {
			sender = fmt.Sprintf("%s <%s>", campaign.FromName, campaign.FromEmail)
		}

		// The UUID is assigned up front so the per-recipient web view link can be built before persisting.
		em := &models.Email{
			UUID:         uuid.NewString(),
			UserID:       campaign.UserID,
			WorkspaceID:  campaign.WorkspaceID,
			Sender:       sender,
			Recipients:   pq.StringArray{msg.Subscriber.Email},
			Subject:      content.Subject,
			TemplateName: templateName,
			HTMLBody:     content.HTMLBody,
			TextBody:     content.TextBody,
			Status:       models.EmailStatusQueued,
			Provider:     email.ClassifyProvider(msg.Subscriber.Email),
		}

		if err := p.emailRepo.Create(em); err != nil {
			_ = p.messageRepo.UpdateStatus(msg.ID, models.CampaignMsgFailed, "failed to create email record: "+err.Error())
			continue
		}

		if p.trackingService != nil {
			// ProcessHTML's click rewriter skips the generated /t/ URLs, so substitute posta_* links before it runs.
			webViewURL := p.trackingService.WebViewURL(em.UUID)
			unsubscribeURL := p.trackingService.UnsubscribeURL(msg.ID)
			em.HTMLBody = email.SubstituteSystemLinks(em.HTMLBody, webViewURL, unsubscribeURL)
			em.TextBody = email.SubstituteSystemLinks(em.TextBody, webViewURL, unsubscribeURL)
			em.Subject = email.SubstituteSystemLinks(em.Subject, webViewURL, unsubscribeURL)

			if em.HTMLBody != "" {
				em.HTMLBody = p.trackingService.ProcessHTML(em.HTMLBody, campaign.ID, msg.ID)
			}
			em.ListUnsubscribeURL = unsubscribeURL
			em.ListUnsubscribePost = true
			_ = p.emailRepo.Update(em)
		}

		_ = p.messageRepo.SetEmailID(msg.ID, em.ID)
		_ = p.messageRepo.UpdateStatus(msg.ID, models.CampaignMsgQueued, "")

		if err := p.producer.EnqueueEmailSend(em.ID, QueueBulk); err != nil {
			_ = p.messageRepo.UpdateStatus(msg.ID, models.CampaignMsgFailed, "failed to enqueue email: "+err.Error())
		}
	}

	remaining, err := p.messageRepo.CountPending(campaign.ID)
	if err != nil {
		return fmt.Errorf("failed to count remaining messages: %w", err)
	}

	if remaining > 0 {
		var delay time.Duration
		if campaign.SendRate > 0 {
			sent := len(pendingMessages)
			delay = time.Duration(float64(sent) / float64(campaign.SendRate) * float64(time.Minute))
		}
		return p.producer.EnqueueCampaignBatch(campaign.ID, delay)
	}

	logger.Info("campaign sending complete", "id", campaign.ID)
	if err := p.campaignRepo.UpdateStatus(campaign.ID, models.CampaignStatusSent); err != nil {
		return err
	}
	if p.dispatcher != nil {
		p.dispatcher.DispatchCampaign(campaign.UserID, campaign.WorkspaceID, "campaign.completed", campaign.ID, campaign.Name, campaign.FromEmail)
	}
	return nil
}

type resolvedContent struct {
	Subject  string
	HTMLBody string
	TextBody string
}

func (p *CampaignProcessor) resolveTemplateContent(campaign *models.Campaign, language string) *resolvedContent {
	tmpl, err := p.templateRepo.FindByID(campaign.TemplateID)
	if err != nil {
		logger.Error("campaign: template not found", "template_id", campaign.TemplateID)
		return nil
	}

	versionID := campaign.TemplateVersionID
	if versionID == nil {
		versionID = tmpl.ActiveVersionID
	}
	if versionID == nil {
		logger.Error("campaign: no active version", "template_id", campaign.TemplateID)
		return nil
	}

	v, err := p.versionRepo.FindByID(*versionID)
	if err != nil {
		logger.Error("campaign: version not found", "version_id", *versionID)
		return nil
	}

	lang := language
	if lang == "" {
		lang = campaign.Language
	}
	if lang == "" {
		lang = tmpl.DefaultLanguage
	}

	loc, err := p.localizationRepo.FindByVersionAndLanguage(*versionID, lang)
	if err != nil {
		if lang != tmpl.DefaultLanguage {
			loc, err = p.localizationRepo.FindByVersionAndLanguage(*versionID, tmpl.DefaultLanguage)
			if err != nil {
				logger.Error("campaign: localization not found", "version_id", *versionID, "language", lang)
				return nil
			}
		} else {
			return nil
		}
	}

	var css string
	if v.StyleSheet != nil {
		css = v.StyleSheet.CSS
	}

	renderer := email.NewTemplateRenderer()
	renderer.MissingKeyBehavior = "zero" // don't fail on missing keys in campaigns

	var data okapi.M
	if campaign.TemplateData != nil {
		data = okapi.M(campaign.TemplateData)
	}
	// Sentinels only: the render is shared, so per-recipient posta_* URLs are substituted in the message loop.
	data = okapi.M(email.WithSystemVars(data))

	rendered, err := renderer.Render(&email.RenderInput{
		SubjectTemplate: loc.SubjectTemplate,
		HTMLTemplate:    loc.HTMLTemplate,
		TextTemplate:    loc.TextTemplate,
		CSS:             css,
	}, data)
	if err != nil {
		logger.Error("campaign: template render failed", "error", err, "campaign_id", campaign.ID)
		return &resolvedContent{
			Subject:  campaign.Subject,
			HTMLBody: loc.HTMLTemplate,
			TextBody: loc.TextTemplate,
		}
	}

	// Use rendered subject, but fall back to campaign.Subject if localization subject was empty
	subject := rendered.Subject
	if subject == "" {
		subject = campaign.Subject
	}

	return &resolvedContent{
		Subject:  subject,
		HTMLBody: rendered.HTML,
		TextBody: rendered.Text,
	}
}
