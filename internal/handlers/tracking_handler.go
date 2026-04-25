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
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/tracking"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
)

type TrackingHandler struct {
	trackingRepo    *repositories.TrackingRepository
	messageRepo     *repositories.CampaignMessageRepository
	campaignRepo    *repositories.CampaignRepository
	subRepo         *repositories.SubscriberRepository
	listRepo        *repositories.SubscriberListRepository
	emailRepo       *repositories.EmailRepository
	suppressionRepo *repositories.SuppressionRepository
	trackingService *tracking.Service
}

func NewTrackingHandler(
	trackingRepo *repositories.TrackingRepository,
	messageRepo *repositories.CampaignMessageRepository,
	campaignRepo *repositories.CampaignRepository,
	subRepo *repositories.SubscriberRepository,
	listRepo *repositories.SubscriberListRepository,
	emailRepo *repositories.EmailRepository,
	suppressionRepo *repositories.SuppressionRepository,
	trackingService *tracking.Service,
) *TrackingHandler {
	return &TrackingHandler{
		trackingRepo:    trackingRepo,
		messageRepo:     messageRepo,
		campaignRepo:    campaignRepo,
		subRepo:         subRepo,
		listRepo:        listRepo,
		emailRepo:       emailRepo,
		suppressionRepo: suppressionRepo,
		trackingService: trackingService,
	}
}

type TrackingOpenRequest struct {
	MessageID int    `param:"message_id"`
	Sig       string `query:"sig"`
}

type TrackingClickRequest struct {
	MessageID int    `param:"message_id"`
	Hash      string `param:"hash"`
	Sig       string `query:"sig"`
}

type TrackingUnsubscribeRequest struct {
	Token string `param:"token"`
}

// OpenPixel serves a 1x1 transparent GIF and records the open event.
// Signature is mandatory; unsigned or bad-sig requests get 404.
func (h *TrackingHandler) OpenPixel(c *okapi.Context, req *TrackingOpenRequest) error {
	if req.Sig == "" || !h.trackingService.VerifyOpenSig(uint(req.MessageID), req.Sig) {
		return c.AbortNotFound("not found")
	}

	ua := c.Request().UserAgent()
	if !isBotUA(ua) {
		go h.recordOpen(uint(req.MessageID), c.RealIP(), ua)
	}

	c.ResponseWriter().Header().Set("Content-Type", "image/gif")
	c.ResponseWriter().Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	c.ResponseWriter().WriteHeader(http.StatusOK)
	_, _ = c.ResponseWriter().Write(transparentPixel)
	return nil
}

// ClickRedirect records the click and redirects to the original URL.
func (h *TrackingHandler) ClickRedirect(c *okapi.Context, req *TrackingClickRequest) error {
	if req.Sig == "" || !h.trackingService.VerifyClickSig(uint(req.MessageID), req.Hash, req.Sig) {
		return c.AbortNotFound("not found")
	}

	link, err := h.trackingRepo.FindLinkByHash(req.Hash)
	if err != nil {
		return c.AbortNotFound("link not found")
	}

	// Validate redirect URL to prevent SSRF
	if !strings.HasPrefix(link.OriginalURL, "http://") && !strings.HasPrefix(link.OriginalURL, "https://") {
		return c.AbortBadRequest("invalid redirect URL")
	}

	ua := c.Request().UserAgent()
	if !isBotUA(ua) {
		go h.recordClick(uint(req.MessageID), link.ID, c.RealIP(), ua)
	}

	c.Redirect(http.StatusFound, link.OriginalURL)
	return nil
}

// UnsubscribePage shows a simple unsubscribe confirmation page.
func (h *TrackingHandler) UnsubscribePage(c *okapi.Context, req *TrackingUnsubscribeRequest) error {
	messageID, err := h.trackingService.VerifyUnsubscribeToken(req.Token)
	if err != nil {
		return c.HTMLView(http.StatusNotFound, "Invalid or expired unsubscribe link", okapi.M{})
	}

	msg, err := h.messageRepo.FindByCampaignMessageID(messageID)
	if err != nil {
		return c.HTMLView(http.StatusNotFound, "Message not found", okapi.M{})

	}

	sub, err := h.subRepo.FindByID(msg.SubscriberID)
	if err != nil {
		return c.HTMLView(http.StatusNotFound, "Subscriber not found", okapi.M{})
	}

	html := fmt.Sprintf(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>Unsubscribe</title>
<style>body{font-family:sans-serif;display:flex;align-items:center;justify-content:center;min-height:100vh;margin:0;background:#f9fafb}
.card{background:#fff;border-radius:12px;padding:40px;max-width:420px;text-align:center;box-shadow:0 2px 8px rgba(0,0,0,0.08)}
h1{font-size:20px;margin-bottom:8px}p{color:#6b7280;font-size:14px;margin-bottom:20px}
button{background:#9333ea;color:#fff;border:none;padding:12px 32px;border-radius:8px;font-size:15px;cursor:pointer}
button:hover{background:#7e22ce}.done{color:#16a34a;font-weight:600}</style></head><body>
<div class="card"><h1>Unsubscribe</h1><p>%s</p>
<form method="POST"><button type="submit">Confirm Unsubscribe</button></form></div></body></html>`, html.EscapeString(sub.Email))

	return c.HTMLView(http.StatusOK, html, okapi.M{})
}

// TxUnsubscribePage renders a confirmation page for a transactional one-click
// unsubscribe link. The link is RFC 8058 compliant: the POST variant will
// opt the recipient out without any further interaction.
func (h *TrackingHandler) TxUnsubscribePage(c *okapi.Context, req *TrackingUnsubscribeRequest) error {
	emailID, err := h.trackingService.VerifyTxUnsubscribeToken(req.Token)
	if err != nil {
		return c.HTMLView(http.StatusNotFound, "Invalid or expired unsubscribe link", okapi.M{})
	}
	em, err := h.emailRepo.FindByID(emailID)
	if err != nil || em == nil {
		return c.HTMLView(http.StatusNotFound, "Message not found", okapi.M{})
	}
	shown := ""
	if len(em.Recipients) > 0 {
		shown = em.Recipients[0]
	}
	page := fmt.Sprintf(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>Unsubscribe</title>
<style>body{font-family:sans-serif;display:flex;align-items:center;justify-content:center;min-height:100vh;margin:0;background:#f9fafb}
.card{background:#fff;border-radius:12px;padding:40px;max-width:420px;text-align:center;box-shadow:0 2px 8px rgba(0,0,0,0.08)}
h1{font-size:20px;margin-bottom:8px}p{color:#6b7280;font-size:14px;margin-bottom:20px}
button{background:#9333ea;color:#fff;border:none;padding:12px 32px;border-radius:8px;font-size:15px;cursor:pointer}
button:hover{background:#7e22ce}</style></head><body>
<div class="card"><h1>Unsubscribe</h1><p>%s</p>
<form method="POST"><button type="submit">Confirm Unsubscribe</button></form></div></body></html>`, html.EscapeString(shown))

	return c.HTMLView(http.StatusOK, page, okapi.M{})
}

// TxUnsubscribeConfirm processes a POST to the transactional unsubscribe link.
// It is the RFC 8058 one-click endpoint — idempotent and requires no session.
// All recipients on the email are added to the scoped suppression list.
func (h *TrackingHandler) TxUnsubscribeConfirm(c *okapi.Context, req *TrackingUnsubscribeRequest) error {
	emailID, err := h.trackingService.VerifyTxUnsubscribeToken(req.Token)
	if err != nil {
		return c.HTMLView(http.StatusNotFound, "Invalid or expired unsubscribe link", okapi.M{})
	}
	em, err := h.emailRepo.FindByID(emailID)
	if err != nil || em == nil {
		return c.HTMLView(http.StatusNotFound, "Message not found", okapi.M{})
	}

	if h.suppressionRepo != nil {
		for _, addr := range em.Recipients {
			if addr == "" {
				continue
			}
			_ = h.suppressionRepo.Upsert(&models.Suppression{
				UserID:      em.UserID,
				WorkspaceID: em.WorkspaceID,
				Email:       addr,
				Reason:      "one_click_unsubscribe",
			})
		}
	}

	confirmHTML := `<!DOCTYPE html><html><head><meta charset="utf-8"><title>Unsubscribed</title>
<style>body{font-family:sans-serif;display:flex;align-items:center;justify-content:center;min-height:100vh;margin:0;background:#f9fafb}
.card{background:#fff;border-radius:12px;padding:40px;max-width:420px;text-align:center;box-shadow:0 2px 8px rgba(0,0,0,0.08)}
h1{font-size:20px;color:#16a34a}p{color:#6b7280;font-size:14px}</style></head><body>
<div class="card"><h1>Unsubscribed</h1><p>You will no longer receive emails of this type from the sender.</p></div></body></html>`

	return c.HTMLView(http.StatusOK, confirmHTML, okapi.M{})
}

// UnsubscribeConfirm processes the unsubscribe action.
func (h *TrackingHandler) UnsubscribeConfirm(c *okapi.Context, req *TrackingUnsubscribeRequest) error {
	messageID, err := h.trackingService.VerifyUnsubscribeToken(req.Token)
	if err != nil {
		return c.HTMLView(http.StatusNotFound, "Invalid or expired unsubscribe link", okapi.M{})
	}

	msg, err := h.messageRepo.FindByCampaignMessageID(messageID)
	if err != nil {
		return c.HTMLView(http.StatusNotFound, "Message not found", okapi.M{})
	}

	camp, err := h.campaignRepo.FindByID(msg.CampaignID)
	if err != nil {
		return c.HTMLView(http.StatusNotFound, "Campaign not found", okapi.M{})
	}

	// Suppress this subscriber on the campaign's list
	if h.listRepo != nil {
		_ = h.listRepo.SuppressMember(camp.ListID, msg.SubscriberID, "user_unsubscribed")
	}

	// Mark the campaign message itself.
	_ = h.messageRepo.UpdateUnsubscribedAt(msg.ID)

	// Record event for analytics.
	_ = h.trackingRepo.CreateEvent(&models.TrackingEvent{
		CampaignMessageID: msg.ID,
		EventType:         models.TrackingEventUnsubscribe,
		IP:                c.RealIP(),
		UserAgent:         c.Request().UserAgent(),
	})

	confirmHTML := `<!DOCTYPE html><html><head><meta charset="utf-8"><title>Unsubscribed</title>
<style>body{font-family:sans-serif;display:flex;align-items:center;justify-content:center;min-height:100vh;margin:0;background:#f9fafb}
.card{background:#fff;border-radius:12px;padding:40px;max-width:420px;text-align:center;box-shadow:0 2px 8px rgba(0,0,0,0.08)}
h1{font-size:20px;color:#16a34a}p{color:#6b7280;font-size:14px}</style></head><body>
<div class="card"><h1>Unsubscribed</h1><p>You have been removed from this mailing list. Other lists you are subscribed to are unaffected.</p></div></body></html>`

	return c.HTMLView(http.StatusOK, confirmHTML, okapi.M{})
}

type CampaignAnalyticsRequest struct {
	ID int `param:"id"`
}

type CampaignAnalyticsResponse struct {
	Analytics        *repositories.CampaignAnalytics            `json:"analytics"`
	VariantAnalytics map[string]*repositories.CampaignAnalytics `json:"variant_analytics,omitempty"`
	Links            []models.TrackedLink                       `json:"links"`
	OpenSeries       []repositories.TimeSeriesPoint             `json:"open_series"`
	ClickSeries      []repositories.TimeSeriesPoint             `json:"click_series"`
}

func (h *TrackingHandler) CampaignAnalytics(c *okapi.Context, req *CampaignAnalyticsRequest) error {
	campaign, err := h.campaignRepo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, campaign.UserID, campaign.WorkspaceID) {
		return c.AbortNotFound("campaign not found")
	}

	analytics, err := h.trackingRepo.CampaignAnalytics(campaign.ID)
	if err != nil {
		return c.AbortInternalServerError("failed to load analytics")
	}

	variantAnalytics, _ := h.trackingRepo.CampaignAnalyticsByVariant(campaign.ID)

	links, _ := h.trackingRepo.FindLinksByCampaign(campaign.ID)
	openSeries, _ := h.trackingRepo.EventTimeSeries(campaign.ID, models.TrackingEventOpen)
	clickSeries, _ := h.trackingRepo.EventTimeSeries(campaign.ID, models.TrackingEventClick)

	return ok(c, CampaignAnalyticsResponse{
		Analytics:        analytics,
		VariantAnalytics: variantAnalytics,
		Links:            links,
		OpenSeries:       openSeries,
		ClickSeries:      clickSeries,
	})
}

func (h *TrackingHandler) recordOpen(messageID uint, ip, userAgent string) {
	msg, err := h.messageRepo.FindByCampaignMessageID(messageID)
	if err != nil {
		return
	}

	// Record first open on the campaign message
	if msg.OpenedAt == nil {
		now := time.Now()
		msg.OpenedAt = &now
		_ = h.messageRepo.UpdateOpenedAt(msg.ID)
	}

	// Always record the event (for total open tracking)
	_ = h.trackingRepo.CreateEvent(&models.TrackingEvent{
		CampaignMessageID: msg.ID,
		EventType:         models.TrackingEventOpen,
		IP:                ip,
		UserAgent:         userAgent,
	})
}

func (h *TrackingHandler) recordClick(messageID uint, linkID uint, ip, userAgent string) {
	msg, err := h.messageRepo.FindByCampaignMessageID(messageID)
	if err != nil {
		return
	}

	// Record first click on the campaign message
	if msg.ClickedAt == nil {
		now := time.Now()
		msg.ClickedAt = &now
		_ = h.messageRepo.UpdateClickedAt(msg.ID)
	}

	// Increment link click count
	h.trackingRepo.IncrementLinkClickCount(linkID)

	// Record event (unique per link per message for deduplication stats)
	if !h.trackingRepo.HasClickEvent(msg.ID, linkID) {
		_ = h.trackingRepo.CreateEvent(&models.TrackingEvent{
			CampaignMessageID: msg.ID,
			EventType:         models.TrackingEventClick,
			TrackedLinkID:     &linkID,
			IP:                ip,
			UserAgent:         userAgent,
		})
	}
}
