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

package routes

import (
	"net/http"

	"github.com/jkaninda/okapi"
	"github.com/goposta/posta/internal/handlers"
)

// trackingRoutes returns public (no auth) route definitions for open/click/unsubscribe tracking.
func (r *Router) trackingRoutes() []okapi.RouteDefinition {
	return []okapi.RouteDefinition{
		{
			Method:  http.MethodGet,
			Path:    "/t/o/{message_id:int}.png",
			Handler: okapi.H(r.h.tracking.OpenPixel),
			Tags:    []string{"Tracking"},
			Summary: "Open tracking pixel",
			Options: []okapi.RouteOption{okapi.DocHide()},
		},
		{
			Method:  http.MethodGet,
			Path:    "/t/c/{message_id:int}/{hash}",
			Handler: okapi.H(r.h.tracking.ClickRedirect),
			Tags:    []string{"Tracking"},
			Summary: "Click tracking redirect",
			Options: []okapi.RouteOption{okapi.DocHide()},
		},
		{
			Method:  http.MethodGet,
			Path:    "/t/u/{token}",
			Handler: okapi.H(r.h.tracking.UnsubscribePage),
			Tags:    []string{"Tracking"},
			Summary: "Unsubscribe page",
			Options: []okapi.RouteOption{okapi.DocHide()},
		},
		{
			Method:  http.MethodPost,
			Path:    "/t/u/{token}",
			Handler: okapi.H(r.h.tracking.UnsubscribeConfirm),
			Tags:    []string{"Tracking"},
			Summary: "Confirm unsubscribe",
			Options: []okapi.RouteOption{okapi.DocHide()},
		},
	}
}

// bounceWebhookRoutes returns the bounce webhook route (authenticated via API key).
func (r *Router) bounceWebhookRoutes() []okapi.RouteDefinition {
	bounceGroup := r.app.Group("/webhooks/bounce", r.mw.apiKey).WithTags([]string{"Webhooks"})
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

// trackingAnalyticsRoutes returns the authenticated campaign analytics route.
func (r *Router) trackingAnalyticsRoutes() []okapi.RouteDefinition {
	userGroup := r.v1.Group("/users/me", r.mw.jwtAuth.Middleware, r.mw.optionalWorkspace).WithTags([]string{"Campaigns"})
	userGroup.WithBearerAuth()

	return []okapi.RouteDefinition{
		{
			Method:   http.MethodGet,
			Path:     "/campaigns/{id:int}/analytics",
			Handler:  okapi.H(r.h.tracking.CampaignAnalytics),
			Group:    userGroup,
			Summary:  "Get campaign analytics",
			Response: &handlers.CampaignAnalyticsResponse{},
			Options:  []okapi.RouteOption{okapi.DocPathParam("id", "integer", "Campaign ID")},
		},
	}
}
