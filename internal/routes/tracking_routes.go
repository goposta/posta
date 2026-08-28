// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package routes

import (
	"net/http"

	"github.com/goposta/posta/internal/handlers"
	"github.com/jkaninda/okapi"
)

// trackingRoutes returns public (no auth) route definitions for open/click/unsubscribe tracking.
func (r *Router) trackingRoutes() []okapi.RouteDefinition {
	return []okapi.RouteDefinition{
		{
			Method:  http.MethodGet,
			Path:    "/t/o/{message_id:int}.gif",
			Handler: okapi.H(r.h.tracking.OpenPixel),
			Tags:    []string{tagTracking},
			Summary: "Open tracking pixel",
			Options: []okapi.RouteOption{okapi.DocHide()},
		},
		{
			Method:  http.MethodGet,
			Path:    "/t/c/{message_id:int}/{hash}",
			Handler: okapi.H(r.h.tracking.ClickRedirect),
			Tags:    []string{tagTracking},
			Summary: "Click tracking redirect",
			Options: []okapi.RouteOption{okapi.DocHide()},
		},
		{
			Method:  http.MethodGet,
			Path:    "/t/u/{token}",
			Handler: okapi.H(r.h.tracking.UnsubscribePage),
			Tags:    []string{tagTracking},
			Summary: "Unsubscribe page",
			Options: []okapi.RouteOption{okapi.DocHide()},
		},
		{
			Method:  http.MethodPost,
			Path:    "/t/u/{token}",
			Handler: okapi.H(r.h.tracking.UnsubscribeConfirm),
			Tags:    []string{tagTracking},
			Summary: "Confirm unsubscribe",
			Options: []okapi.RouteOption{okapi.DocHide()},
		},
		{
			Method:  http.MethodGet,
			Path:    "/t/u/tx/{token}",
			Handler: okapi.H(r.h.tracking.TxUnsubscribePage),
			Tags:    []string{tagTracking},
			Summary: "Transactional unsubscribe page",
			Options: []okapi.RouteOption{okapi.DocHide()},
		},
		{
			Method:  http.MethodPost,
			Path:    "/t/u/tx/{token}",
			Handler: okapi.H(r.h.tracking.TxUnsubscribeConfirm),
			Tags:    []string{tagTracking},
			Summary: "Transactional one-click unsubscribe (RFC 8058)",
			Options: []okapi.RouteOption{okapi.DocHide()},
		},
		{
			Method:  http.MethodGet,
			Path:    "/t/v/{token}",
			Handler: okapi.H(r.h.tracking.WebView),
			Tags:    []string{tagTracking},
			Summary: "View email in browser",
			Options: []okapi.RouteOption{okapi.DocHide()},
		},
	}
}

// bounceWebhookRoutes returns the bounce webhook route (authenticated via API key).
func (r *Router) bounceWebhookRoutes() []okapi.RouteDefinition {
	bounceGroup := r.v1.Group("/webhooks/bounce", r.mw.apiKey).WithTagInfo(okapi.GroupTag{
		Name:        "Webhooks",
		Description: "Inbound webhook endpoints that receive bounce and complaint notifications from upstream mail providers. Authenticated with an API key.",
	}).WithSecurity([]map[string][]string{{"ApiKeyAuth": {}}})

	return []okapi.RouteDefinition{
		{
			Method:   http.MethodPost,
			Path:     "",
			Handler:  okapi.H(r.h.bounceWebhook.HandleBounce),
			Group:    bounceGroup,
			Summary:  "Bounce notification webhook",
			Request:  &handlers.BounceNotification{},
			Response: &handlers.BounceResponse{},
		},
	}
}

// trackingAnalyticsRoutes returns the authenticated campaign analytics route,
// scoped to the active workspace (workspace-only migration §7).
func (r *Router) trackingAnalyticsRoutes() []okapi.RouteDefinition {
	wsGroup := r.v1.Group("/workspaces/current", r.mw.auth, r.mw.workspace).WithTagInfo(okapi.GroupTag{
		Name:        tagCampaigns,
		Description: "Email campaign analytics — open, click, bounce, and engagement metrics for campaigns in the active workspace.",
	})
	wsGroup.WithBearerAuth()

	return []okapi.RouteDefinition{
		{
			Method:   http.MethodGet,
			Path:     "/campaigns/{id:int}/analytics",
			Handler:  okapi.H(r.h.tracking.CampaignAnalytics),
			Group:    wsGroup,
			Summary:  "Get campaign analytics",
			Response: &handlers.CampaignAnalyticsResponse{},
			Options:  []okapi.RouteOption{okapi.DocPathParam("id", "integer", "Campaign ID")},
		},
	}
}
