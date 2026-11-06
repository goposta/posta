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

package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/goposta/posta/internal/config"
	"github.com/goposta/posta/internal/enterprise"
	"github.com/goposta/posta/internal/handlers"
	"github.com/goposta/posta/internal/models"
	"github.com/jkaninda/okapi"
)

// chatApp mirrors production CORS: ChatPreflight ahead of a global handler that only knows the dashboard origin.
func chatApp(t *testing.T) *Router {
	t.Helper()
	app := okapi.New()
	app.Use(config.ChatPreflight)
	app.WithCORS(okapi.Cors{
		AllowedOrigins:   []string{"https://dashboard.test"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	})
	r := &Router{app: app, cfg: &config.Config{ChatEnabled: true}, v1: app.Group("/api/v1")}
	r.h.chatVisitor = new(handlers.ChatVisitorHandler)
	r.h.chatWidget = new(handlers.ChatWidgetHandler)
	r.h.chatConversation = new(handlers.ChatConversationHandler)
	r.h.chatSnippet = new(handlers.ChatSnippetHandler)
	app.Register(r.chatPublicRoutes()...)
	return r
}

// Every widget call is preflighted from unknown sites: it must succeed for any origin and never grant credentials.
func TestChatPreflightAllowsCustomerOrigins(t *testing.T) {
	r := chatApp(t)
	for _, path := range []string{
		"/api/v1/chat/pcw_abc/config",
		"/api/v1/chat/pcw_abc/conversations",
		"/api/v1/chat/pcw_abc/conversations/6f1c2a9e/messages",
		"/api/v1/chat/pcw_abc/conversations/6f1c2a9e/stream",
	} {
		req := httptest.NewRequest(http.MethodOptions, path, nil)
		req.Header.Set("Origin", "https://customer.test")
		req.Header.Set("Access-Control-Request-Method", http.MethodPost)
		req.Header.Set("Access-Control-Request-Headers", "authorization, content-type")
		rec := httptest.NewRecorder()
		r.app.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("%s: status = %d, want 204", path, rec.Code)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://customer.test" {
			t.Fatalf("%s: allow-origin = %q", path, got)
		}
		if got := rec.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(got, "Authorization") {
			t.Fatalf("%s: allow-headers = %q", path, got)
		}
		if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
			t.Fatalf("%s: credentials granted to a customer origin: %q", path, got)
		}
	}
}

// The chat exception must not leak: an unknown-origin preflight for a dashboard route is still refused.
func TestChatPreflightLeavesOtherRoutesAlone(t *testing.T) {
	r := chatApp(t)
	r.app.Register(okapi.RouteDefinition{
		Method:  http.MethodPost,
		Path:    "/workspaces/current/conversations/{id}/messages",
		Handler: func(c *okapi.Context) error { return c.NoContent() },
		Group:   r.v1,
	})
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/workspaces/current/conversations/x/messages", nil)
	req.Header.Set("Origin", "https://customer.test")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	rec := httptest.NewRecorder()
	r.app.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("dashboard route preflight allowed an unknown origin: %q", got)
	}
}

func TestChatWidgetScriptIsNotShadowedBySPA(t *testing.T) {
	o := okapi.NewTestServer(t)
	r := &Router{app: o.Okapi, cfg: &config.Config{ChatEnabled: true}, v1: o.Group("/api/v1")}
	r.h.chatVisitor = new(handlers.ChatVisitorHandler)
	o.Register(r.chatPublicRoutes()...)
	o.WebFS("/", fstest.MapFS{"dist/index.html": {Data: []byte("<!doctype html><title>SPA</title>")}}, okapi.WebConfig{Root: "dist"})

	rec := httptest.NewRecorder()
	o.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/widget/v1/posta-chat.js", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/javascript") {
		t.Fatalf("content type = %q", ct)
	}
	if strings.Contains(rec.Body.String(), "<title>SPA</title>") {
		t.Fatal("the SPA fallback served the chat widget script")
	}
}

// Handlers check the role before touching anything, so a zero handler is enough to see the viewer refusal.
func TestChatViewerCannotWrite(t *testing.T) {
	o := okapi.NewTestServer(t)
	r := &Router{app: o.Okapi, cfg: &config.Config{ChatEnabled: true}, v1: o.Group("/api/v1")}
	r.h.chatWidget = new(handlers.ChatWidgetHandler)
	r.h.chatConversation = new(handlers.ChatConversationHandler)
	r.h.chatSnippet = new(handlers.ChatSnippetHandler)
	defs := r.chatWorkspaceRoutes()

	writes := []string{
		"Create a chat widget",
		"Update a chat widget",
		"Delete a chat widget",
		"Rotate a chat widget's public key",
		"Reply in a conversation",
		"Change a conversation's state",
		"Assign a conversation",
		"Ban a conversation's visitor",
		"Lift a visitor's ban",
		"Send a file in a conversation",
		"Rotate a chat widget's identity secret",
		"Create a canned reply",
		"Update a canned reply",
		"Delete a canned reply",
	}
	for _, summary := range writes {
		def := defBySummary(t, defs, summary)
		srv := okapi.NewTestServer(t)
		srv.Register(okapi.RouteDefinition{
			Method: def.Method, Path: "/t/{id}", Handler: def.Handler,
			Middlewares: []okapi.Middleware{seedSessionRole(models.WorkspaceRoleViewer)},
		})
		body := strings.NewReader(`{"name":"x","body":"hi","state":"closed","shortcut":"x","title":"x"}`)
		req := httptest.NewRequest(def.Method, "/t/1", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s as viewer: status = %d, want 403 (%s)", summary, rec.Code, rec.Body.String())
		}
	}
}

// Chat analytics is Enterprise: in Community the route answers 402 before the handler runs.
func TestChatAnalyticsNeedsALicense(t *testing.T) {
	o := okapi.NewTestServer(t)
	r := &Router{app: o.Okapi, cfg: &config.Config{ChatEnabled: true}, v1: o.Group("/api/v1"), ee: enterprise.Community()}
	r.h.chatWidget = new(handlers.ChatWidgetHandler)
	r.h.chatConversation = new(handlers.ChatConversationHandler)
	r.h.chatSnippet = new(handlers.ChatSnippetHandler)
	def := defBySummary(t, r.chatWorkspaceRoutes(), "Chat analytics")

	srv := okapi.NewTestServer(t)
	srv.Register(okapi.RouteDefinition{
		Method: def.Method, Path: "/t", Handler: def.Handler,
		Middlewares: append([]okapi.Middleware{seedSessionRole(models.WorkspaceRoleOwner)}, def.Middlewares...),
	})
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/t", nil))
	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want 402 (%s)", rec.Code, rec.Body.String())
	}
}
