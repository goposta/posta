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

package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

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

// CampaignProcessor handles campaign:start and campaign:batch tasks.
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

// HandleCampaignStart processes a campaign:start task.
// It resolves the subscriber list, creates campaign_messages in bulk,
// then enqueues the first CampaignBatch task.
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

	// Resolve subscribers from the list
	list, err := p.listRepo.FindByID(campaign.ListID)
	if err != nil {
		return fmt.Errorf("subscriber list not found: %w", err)
	}

	var subscribers []models.Subscriber
	scope := repositories.ResourceScope{UserID: campaign.UserID, WorkspaceID: campaign.WorkspaceID}

	if list.Type == models.SubscriberListTypeDynamic {
		// Dynamic list: resolve via filter rules
		subscribers, _, err = p.subscriberRepo.FindByFilterRules(scope, list.FilterRules, -1, 0)
		if err != nil {
			return fmt.Errorf("failed to resolve dynamic list: %w", err)
		}
	} else {
		// Static list: get all members
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

	// Create campaign messages in bulk
	messages := make([]models.CampaignMessage, 0, len(subscribers))
	for _, sub := range subscribers {
		// Only include subscribed subscribers
		if sub.Status != models.SubscriberStatusSubscribed {
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

	// Assign A/B test variants if enabled
	if campaign.ABTestEnabled && len(campaign.ABTestVariants) > 0 {
		// Shuffle messages for random distribution
		rand.Shuffle(len(messages), func(i, j int) {
			messages[i], messages[j] = messages[j], messages[i]
		})

		// Assign variants based on split percentages
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
		// Assign remaining messages to last variant
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

	// For timezone-aware campaigns, compute initial delay per timezone group
	if campaign.SendAtLocalTime && campaign.ScheduledAt != nil {
		// Group subscribers by timezone and enqueue separate delayed batches
		// The batch processor will handle the actual sending
		logger.Info("campaign with timezone-aware scheduling", "campaign_id", campaign.ID)
	}

	// Enqueue the first batch
	if err := p.producer.EnqueueCampaignBatch(campaign.ID, 0); err != nil {
		return err
	}
	if p.dispatcher != nil {
		p.dispatcher.Dispatch(campaign.UserID, "campaign.started", fmt.Sprint(campaign.ID), campaign.FromEmail)
	}
	return nil
}

// HandleCampaignBatch processes a campaign:batch task.
// It loads a batch of pending messages, creates Email records, and enqueues them.
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

	// Get pending messages with subscribers preloaded
	pendingMessages, err := p.messageRepo.FindPendingByCampaign(campaign.ID, campaignBatchSize)
	if err != nil {
		return fmt.Errorf("failed to get pending messages: %w", err)
	}

	if len(pendingMessages) == 0 {
		// No more pending messages: mark campaign as sent
		logger.Info("campaign sending complete", "id", campaign.ID)
		if err := p.campaignRepo.UpdateStatus(campaign.ID, models.CampaignStatusSent); err != nil {
			return err
		}
		if p.dispatcher != nil {
			p.dispatcher.Dispatch(campaign.UserID, "campaign.completed", fmt.Sprint(campaign.ID), campaign.FromEmail)
		}
		return nil
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

	// Pre-resolve default content for the campaign language
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

		// A/B variant subject override
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

		// Format sender with display name
		sender := campaign.FromEmail
		if campaign.FromName != "" {
			sender = fmt.Sprintf("%s <%s>", campaign.FromName, campaign.FromEmail)
		}

		// Create email record
		em := &models.Email{
			UserID:      campaign.UserID,
			WorkspaceID: campaign.WorkspaceID,
			Sender:      sender,
			Recipients:  pq.StringArray{msg.Subscriber.Email},
			Subject:     content.Subject,
			HTMLBody:    content.HTMLBody,
			TextBody:    content.TextBody,
			Status:      models.EmailStatusQueued,
		}

		if err := p.emailRepo.Create(em); err != nil {
			_ = p.messageRepo.UpdateStatus(msg.ID, models.CampaignMsgFailed, "failed to create email record: "+err.Error())
			continue
		}

		// Rewrite links for click tracking and inject open pixel
		if p.trackingService != nil && em.HTMLBody != "" {
			em.HTMLBody = p.trackingService.ProcessHTML(em.HTMLBody, campaign.ID, msg.ID)
			em.ListUnsubscribeURL = p.trackingService.UnsubscribeURL(msg.ID)
			em.ListUnsubscribePost = true
			_ = p.emailRepo.Update(em)
		}

		// Link email to campaign message
		_ = p.messageRepo.SetEmailID(msg.ID, em.ID)
		_ = p.messageRepo.UpdateStatus(msg.ID, models.CampaignMsgQueued, "")

		// Enqueue email for sending
		if err := p.producer.EnqueueEmailSend(em.ID, QueueBulk); err != nil {
			_ = p.messageRepo.UpdateStatus(msg.ID, models.CampaignMsgFailed, "failed to enqueue email: "+err.Error())
		}
	}

	// Check if there are more pending messages
	remaining, err := p.messageRepo.CountPending(campaign.ID)
	if err != nil {
		return fmt.Errorf("failed to count remaining messages: %w", err)
	}

	if remaining > 0 {
		// Compute rate limiting delay
		var delay time.Duration
		if campaign.SendRate > 0 {
			// SendRate is messages per minute
			delay = time.Duration(float64(time.Minute) / float64(campaign.SendRate) * float64(campaignBatchSize))
		}
		return p.producer.EnqueueCampaignBatch(campaign.ID, delay)
	}

	// All done
	logger.Info("campaign sending complete", "id", campaign.ID)
	if err := p.campaignRepo.UpdateStatus(campaign.ID, models.CampaignStatusSent); err != nil {
		return err
	}
	if p.dispatcher != nil {
		p.dispatcher.Dispatch(campaign.UserID, "campaign.completed", fmt.Sprint(campaign.ID), campaign.FromEmail)
	}
	return nil
}

// resolvedContent holds the rendered template content for a campaign email.
type resolvedContent struct {
	Subject  string
	HTMLBody string
	TextBody string
}

// resolveTemplateContent looks up the template, renders it with campaign data, and injects CSS.
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

	// Get CSS from stylesheet
	var css string
	if v.StyleSheet != nil {
		css = v.StyleSheet.CSS
	}

	// Render templates with campaign data
	renderer := email.NewTemplateRenderer()
	renderer.MissingKeyBehavior = "zero" // don't fail on missing keys in campaigns

	var data okapi.M
	if campaign.TemplateData != nil {
		data = okapi.M(campaign.TemplateData)
	}

	rendered, err := renderer.Render(&email.RenderInput{
		SubjectTemplate: loc.SubjectTemplate,
		HTMLTemplate:    loc.HTMLTemplate,
		TextTemplate:    loc.TextTemplate,
		CSS:             css,
	}, data)
	if err != nil {
		logger.Error("campaign: template render failed", "error", err, "campaign_id", campaign.ID)
		// Fall back to raw content
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
