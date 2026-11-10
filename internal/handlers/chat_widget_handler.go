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

package handlers

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/goposta/posta/internal/enterprise"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/audit"
	"github.com/goposta/posta/internal/services/chat"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
	"gorm.io/gorm"
)

const metaChatWidgetID = "chat_widget_id"

type ChatWidgetHandler struct {
	edition         enterprise.EE
	quota           QuotaChecker
	db              *gorm.DB
	repo            *repositories.ChatRepository
	domains         *repositories.DomainRepository
	audit           *audit.Logger
	apiBaseURL      string
	maxMessageBytes int
}

func NewChatWidgetHandler(repo *repositories.ChatRepository, auditLogger *audit.Logger, apiBaseURL string, maxMessageBytes int) *ChatWidgetHandler {
	if maxMessageBytes <= 0 {
		maxMessageBytes = models.DefaultChatMaxMessageBytes
	}
	return &ChatWidgetHandler{
		repo:            repo,
		audit:           auditLogger,
		apiBaseURL:      strings.TrimSuffix(apiBaseURL, "/"),
		maxMessageBytes: maxMessageBytes,
	}
}

// SetEdition gates auto-assign; without it the settings are refused, as in Community.
func (h *ChatWidgetHandler) SetEdition(ee enterprise.EE) { h.edition = ee }

// requireAutoAssign accepts unchanged values so a client that saves the whole widget keeps working.
func (h *ChatWidgetHandler) requireAutoAssign(w *models.ChatWidget, s chatWidgetSettings) error {
	changed := (s.AutoAssign != nil && *s.AutoAssign != w.AutoAssign) ||
		(s.MaxOpenPerAgent != nil && *s.MaxOpenPerAgent != w.MaxOpenPerAgent)
	if !changed {
		return nil
	}
	if h.edition == nil {
		return enterprise.Community().RequireMutable()
	}
	return h.edition.RequireMutable()
}

// SetQuota enforces the plan's widget limit.
func (h *ChatWidgetHandler) SetQuota(q QuotaChecker, db *gorm.DB) { h.quota, h.db = q, db }

// SetDomains enables the verified-domain check on reply_from.
func (h *ChatWidgetHandler) SetDomains(d *repositories.DomainRepository) { h.domains = d }

// chatWidgetSettings is shared by create and update; nil leaves a setting as it is.
type chatWidgetSettings struct {
	AllowedOrigins *[]string        `json:"allowed_origins" doc:"Origins allowed to load the widget, e.g. https://example.com. Empty allows any."`
	StrictOrigin   *bool            `json:"strict_origin" doc:"Refuse requests without an Origin header. On by default: the widget only runs in browsers."`
	Title          *string          `json:"title" doc:"Panel heading"`
	Greeting       *string          `json:"greeting" doc:"First line visitors see"`
	AccentColor    *string          `json:"accent_color" doc:"#RRGGBB"`
	Position       *string          `json:"position" doc:"left or right"`
	AvatarURL      *string          `json:"avatar_url" doc:"https image shown in the panel header"`
	ShowAgentNames *bool            `json:"show_agent_names" doc:"Show visitors the name of the agent answering"`
	Home           *models.ChatHome `json:"home" doc:"Home screen and menu, replaced as a whole"`
	RequireName    *bool            `json:"require_name" doc:"Ask for a name before the first message"`
	RequireEmail   *bool            `json:"require_email" doc:"Ask for an email address before the first message. Always asked while the widget is offline."`

	BusinessHours   *models.BusinessHours `json:"business_hours" doc:"Weekly staffed hours; no days means always. Outside them the widget is offline."`
	OfflineMode     *string               `json:"offline_mode" doc:"email (take messages, answer by email) or hide"`
	OfflineGreeting *string               `json:"offline_greeting" doc:"First line visitors see while no one is online"`
	ReplyFrom       *string               `json:"reply_from" doc:"Sender of replies emailed to visitors who left; must be on a verified domain"`
	ReplyFromName   *string               `json:"reply_from_name"`

	AllowAttachments *bool `json:"allow_attachments" doc:"Let visitors and agents send images and PDFs up to 5 MB (needs blob storage)"`
	AutoAssign       *bool `json:"auto_assign" doc:"Give each new conversation to the next online agent in turn. Posta Enterprise: changing it without a license answers 402."`
	MaxOpenPerAgent  *int  `json:"max_open_per_agent" doc:"Auto-assign skips agents holding this many open conversations; 0 means no cap"`
}

type ChatIdentitySecretResponse struct {
	Secret string `json:"secret" doc:"Shown once. Sign identities with HMAC-SHA256(secret, user_id + \".\" + email), hex."`
}

type CreateChatWidgetRequest struct {
	Body struct {
		Name        string `json:"name" required:"true" doc:"The widget's name, as typed: \"Website chat\" is stored as the identifier website-chat, with \"Website chat\" as display_name. It cannot change later."`
		DisplayName string `json:"display_name" doc:"The widget's label. Defaults to name."`
		chatWidgetSettings
	} `json:"body"`
}

type UpdateChatWidgetRequest struct {
	ID   int `param:"id"`
	Body struct {
		Name                string   `json:"name" doc:"A widget's name cannot change; only accepted when it is the current one."`
		DisplayName         string   `json:"display_name"`
		Status              string   `json:"status" enum:"active,paused,archived"`
		ScanEnabled         *bool    `json:"scan_enabled"`
		FlagThreshold       *float64 `json:"flag_threshold"`
		QuarantineThreshold *float64 `json:"quarantine_threshold"`
		RejectThreshold     *float64 `json:"reject_threshold"`
		MaxMessageBytes     *int     `json:"max_message_bytes"`
		RetentionDays       *int     `json:"retention_days" doc:"Keep closed and spam conversations this many days; 0 uses the platform setting, which this can only lower"`
		chatWidgetSettings
	} `json:"body"`
}

type ChatWidgetIDRequest struct {
	ID int `param:"id"`
}

type ChatWidgetSnippetResponse struct {
	PublicKey string `json:"public_key"`
	ScriptURL string `json:"script_url"`
	HTML      string `json:"html" doc:"The script tag to paste before </body>"`
}

func (h *ChatWidgetHandler) Create(c *okapi.Context, req *CreateChatWidgetRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	scope := getScope(c)
	if scope.WorkspaceID == nil {
		return c.AbortBadRequest("a workspace is required to create a chat widget")
	}
	name, displayName, err := resolveNames(req.Body.Name, req.Body.DisplayName, "")
	if err != nil {
		return c.AbortBadRequest(err.Error())
	}
	if h.quota != nil {
		if err := h.quota.CheckQuota(h.db, scope.UserID, scope.WorkspaceID, "chat_widgets"); err != nil {
			return c.AbortForbidden("chat widget quota exceeded for this workspace", err)
		}
	}
	if h.repo.WidgetNameExists(*scope.WorkspaceID, name, 0) {
		return c.AbortConflict("a chat widget named \"" + name + "\" already exists")
	}
	key, err := models.NewChatWidgetKey()
	if err != nil {
		return c.AbortInternalServerError("failed to generate widget key")
	}
	userID := uint(c.GetInt("user_id"))
	w := &models.ChatWidget{
		WorkspaceID:         scope.WorkspaceID,
		Name:                name,
		DisplayName:         displayName,
		PublicKey:           key,
		Status:              models.ChatWidgetStatusActive,
		StrictOrigin:        true,
		Title:               "Chat with us",
		Greeting:            "Hi! How can we help?",
		Position:            "right",
		ShowAgentNames:      true,
		Home:                models.DefaultChatHome(),
		ScanEnabled:         true,
		FlagThreshold:       3,
		QuarantineThreshold: 6,
		RejectThreshold:     10,
		MaxMessageBytes:     min(models.DefaultChatMaxMessageBytes, h.maxMessageBytes),
		LastEditedByID:      &userID,
	}
	w.OfflineMode = models.ChatOfflineEmail
	if err := h.requireAutoAssign(w, req.Body.chatWidgetSettings); err != nil {
		return licenseAbort(c, err)
	}
	if err := applyChatWidgetSettings(w, req.Body.chatWidgetSettings); err != nil {
		return c.AbortBadRequest(err.Error())
	}
	if err := h.checkReplyFrom(scope, w, req.Body.ReplyFrom); err != nil {
		return c.AbortBadRequest(err.Error())
	}
	if err := h.repo.CreateWidget(w); err != nil {
		return c.AbortConflict("failed to create chat widget")
	}
	h.log(c, "chat_widget.created", fmt.Sprintf("Chat widget %s created", w.DisplayName), map[string]any{
		metaChatWidgetID: w.ID, metaName: w.Name,
	})
	return created(c, w)
}

func (h *ChatWidgetHandler) List(c *okapi.Context, req *ListRequest) error {
	page, size := req.Page, req.Size
	if size <= 0 || size > 100 {
		size = 20
	}
	items, total, err := h.repo.ListWidgets(getScope(c), size, page*size)
	if err != nil {
		return c.AbortInternalServerError("failed to list chat widgets")
	}
	return paginated(c, items, total, page, size)
}

func (h *ChatWidgetHandler) Get(c *okapi.Context, req *ChatWidgetIDRequest) error {
	w, err := h.repo.FindWidgetForScope(getScope(c), uint(req.ID))
	if err != nil {
		return c.AbortNotFound("chat widget not found")
	}
	return ok(c, w)
}

func (h *ChatWidgetHandler) Update(c *okapi.Context, req *UpdateChatWidgetRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	w, err := h.repo.FindWidgetForScope(getScope(c), uint(req.ID))
	if err != nil {
		return c.AbortNotFound("chat widget not found")
	}
	b := req.Body
	displayName, err := resolveRename(b.Name, b.DisplayName, "", w.Name, "chat widget")
	if err != nil {
		return c.AbortBadRequest(err.Error())
	}
	if displayName = strings.TrimSpace(displayName); displayName != "" {
		w.DisplayName = displayName
	}
	switch models.ChatWidgetStatus(b.Status) {
	case "":
	case models.ChatWidgetStatusActive, models.ChatWidgetStatusPaused, models.ChatWidgetStatusArchived:
		w.Status = models.ChatWidgetStatus(b.Status)
	default:
		return c.AbortBadRequest("status must be active, paused or archived")
	}
	if err := h.requireAutoAssign(w, b.chatWidgetSettings); err != nil {
		return licenseAbort(c, err)
	}
	if err := applyChatWidgetSettings(w, b.chatWidgetSettings); err != nil {
		return c.AbortBadRequest(err.Error())
	}
	if err := h.checkReplyFrom(getScope(c), w, b.ReplyFrom); err != nil {
		return c.AbortBadRequest(err.Error())
	}
	if b.ScanEnabled != nil {
		w.ScanEnabled = *b.ScanEnabled
	}
	for _, t := range []struct {
		v   *float64
		dst *float64
	}{{b.FlagThreshold, &w.FlagThreshold}, {b.QuarantineThreshold, &w.QuarantineThreshold}, {b.RejectThreshold, &w.RejectThreshold}} {
		if t.v == nil {
			continue
		}
		if *t.v < 0 || *t.v > 100 {
			return c.AbortBadRequest("scan thresholds must be between 0 and 100")
		}
		*t.dst = *t.v
	}
	if b.MaxMessageBytes != nil {
		if *b.MaxMessageBytes < 256 || *b.MaxMessageBytes > h.maxMessageBytes {
			return c.AbortBadRequest(fmt.Sprintf("max_message_bytes must be between 256 and %d", h.maxMessageBytes))
		}
		w.MaxMessageBytes = *b.MaxMessageBytes
	}
	if b.RetentionDays != nil {
		if *b.RetentionDays < 0 || *b.RetentionDays > 3650 {
			return c.AbortBadRequest("retention_days must be between 0 and 3650")
		}
		w.RetentionDays = *b.RetentionDays
	}
	userID := uint(c.GetInt("user_id"))
	now := time.Now()
	w.LastEditedByID, w.UpdatedAt = &userID, &now
	if err := h.repo.UpdateWidget(w); err != nil {
		return c.AbortInternalServerError("failed to update chat widget")
	}
	h.log(c, "chat_widget.updated", fmt.Sprintf("Chat widget %s updated", w.DisplayName), map[string]any{metaChatWidgetID: w.ID})
	return ok(c, w)
}

func (h *ChatWidgetHandler) Delete(c *okapi.Context, req *ChatWidgetIDRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	w, err := h.repo.FindWidgetForScope(getScope(c), uint(req.ID))
	if err != nil {
		return c.AbortNotFound("chat widget not found")
	}
	if err := h.repo.DeleteWidget(w.ID); err != nil {
		return c.AbortInternalServerError("failed to delete chat widget")
	}
	h.log(c, "chat_widget.deleted", fmt.Sprintf("Chat widget %s deleted", w.DisplayName), map[string]any{metaChatWidgetID: w.ID})
	return noContent(c)
}

func (h *ChatWidgetHandler) RotateKey(c *okapi.Context, req *ChatWidgetIDRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	w, err := h.repo.FindWidgetForScope(getScope(c), uint(req.ID))
	if err != nil {
		return c.AbortNotFound("chat widget not found")
	}
	key, err := models.NewChatWidgetKey()
	if err != nil {
		return c.AbortInternalServerError("failed to generate widget key")
	}
	now := time.Now()
	w.PublicKey, w.UpdatedAt = key, &now
	if err := h.repo.UpdateWidget(w); err != nil {
		return c.AbortInternalServerError("failed to rotate widget key")
	}
	h.log(c, "chat_widget.key_rotated", fmt.Sprintf("Chat widget %s key rotated", w.DisplayName), map[string]any{metaChatWidgetID: w.ID})
	return ok(c, w)
}

// RotateIdentitySecret returns the new secret once; visitors signed with the old one carry on anonymously.
func (h *ChatWidgetHandler) RotateIdentitySecret(c *okapi.Context, req *ChatWidgetIDRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	w, err := h.repo.FindWidgetForScope(getScope(c), uint(req.ID))
	if err != nil {
		return c.AbortNotFound("chat widget not found")
	}
	secret, stored, err := chat.NewIdentitySecret()
	if err != nil {
		return c.AbortInternalServerError("failed to create identity secret")
	}
	now := time.Now()
	w.IdentitySecretEnc, w.UpdatedAt = stored, &now
	if err := h.repo.UpdateWidget(w); err != nil {
		return c.AbortInternalServerError("failed to store identity secret")
	}
	h.log(c, "chat_widget.identity_secret_rotated", fmt.Sprintf("Chat widget %s identity secret rotated", w.DisplayName),
		map[string]any{metaChatWidgetID: w.ID})
	return ok(c, ChatIdentitySecretResponse{Secret: secret})
}

func (h *ChatWidgetHandler) Snippet(c *okapi.Context, req *ChatWidgetIDRequest) error {
	w, err := h.repo.FindWidgetForScope(getScope(c), uint(req.ID))
	if err != nil {
		return c.AbortNotFound("chat widget not found")
	}
	script := h.apiBaseURL + "/widget/v1/posta-chat.js"
	return ok(c, ChatWidgetSnippetResponse{
		PublicKey: w.PublicKey,
		ScriptURL: script,
		HTML: fmt.Sprintf(`<script src="%s" data-posta-widget="%s" async></script>`,
			html.EscapeString(script), html.EscapeString(w.PublicKey)),
	})
}

// checkReplyFrom verifies a newly set reply sender.
func (h *ChatWidgetHandler) checkReplyFrom(scope repositories.ResourceScope, w *models.ChatWidget, set *string) error {
	if set == nil || w.ReplyFrom == "" {
		return nil
	}
	return checkVerifiedSender(h.domains, scope, w.ReplyFrom)
}

func (h *ChatWidgetHandler) log(c *okapi.Context, action, message string, meta map[string]any) {
	if h.audit != nil {
		h.audit.LogCtx(c, action, message, meta)
	}
}

func applyChatWidgetSettings(w *models.ChatWidget, s chatWidgetSettings) error {
	if s.AllowedOrigins != nil {
		origins, err := normalizeOrigins(*s.AllowedOrigins)
		if err != nil {
			return err
		}
		w.AllowedOrigins = origins
	}
	if s.StrictOrigin != nil {
		w.StrictOrigin = *s.StrictOrigin
	}
	if s.Title != nil {
		w.Title = chat.CleanLine(*s.Title, 80)
	}
	if s.Greeting != nil {
		greeting := chat.CleanBody(*s.Greeting)
		if len([]rune(greeting)) > 500 {
			return fmt.Errorf("greeting must be at most 500 characters")
		}
		w.Greeting = greeting
	}
	if s.AccentColor != nil {
		accent := strings.TrimSpace(*s.AccentColor)
		if !chat.ValidAccentColor(accent) {
			return fmt.Errorf("accent_color must be a #RRGGBB colour")
		}
		w.AccentColor = strings.ToLower(accent)
	}
	if s.Position != nil {
		switch *s.Position {
		case "left", "right":
			w.Position = *s.Position
		default:
			return fmt.Errorf("position must be left or right")
		}
	}
	if s.AvatarURL != nil {
		avatar := strings.TrimSpace(*s.AvatarURL)
		if !chat.ValidAvatarURL(avatar) {
			return fmt.Errorf("avatar_url must be an https URL")
		}
		w.AvatarURL = avatar
	}
	if s.ShowAgentNames != nil {
		w.ShowAgentNames = *s.ShowAgentNames
	}
	if s.Home != nil {
		home, err := chat.NormalizeHome(*s.Home)
		if err != nil {
			return err
		}
		w.Home = home
	}
	if s.RequireName != nil {
		w.RequireName = *s.RequireName
	}
	if s.RequireEmail != nil {
		w.RequireEmail = *s.RequireEmail
	}
	if s.BusinessHours != nil {
		if err := s.BusinessHours.Validate(); err != nil {
			return err
		}
		w.BusinessHours = *s.BusinessHours
	}
	if s.OfflineMode != nil {
		switch *s.OfflineMode {
		case models.ChatOfflineEmail, models.ChatOfflineHide:
			w.OfflineMode = *s.OfflineMode
		default:
			return fmt.Errorf("offline_mode must be email or hide")
		}
	}
	if s.OfflineGreeting != nil {
		greeting := chat.CleanBody(*s.OfflineGreeting)
		if len([]rune(greeting)) > 500 {
			return fmt.Errorf("offline_greeting must be at most 500 characters")
		}
		w.OfflineGreeting = greeting
	}
	if s.ReplyFrom != nil {
		w.ReplyFrom = strings.ToLower(strings.TrimSpace(*s.ReplyFrom))
	}
	if s.ReplyFromName != nil {
		w.ReplyFromName = chat.CleanLine(*s.ReplyFromName, 80)
	}
	if s.AllowAttachments != nil {
		w.AllowAttachments = *s.AllowAttachments
	}
	if s.AutoAssign != nil {
		w.AutoAssign = *s.AutoAssign
	}
	if s.MaxOpenPerAgent != nil {
		if *s.MaxOpenPerAgent < 0 || *s.MaxOpenPerAgent > 1000 {
			return fmt.Errorf("max_open_per_agent must be between 0 and 1000")
		}
		w.MaxOpenPerAgent = *s.MaxOpenPerAgent
	}
	return nil
}
