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
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/inbound"
	"github.com/goposta/posta/internal/services/messages"
	"github.com/goposta/posta/internal/storage/blob"
	"github.com/goposta/posta/internal/web/widget"
	"github.com/jkaninda/logger"
	"github.com/jkaninda/okapi"
)

const genericAcceptMessage = "Thanks — your message has been received."

type FormIngestHandler struct {
	svc       *messages.Service
	blobStore blob.Store
	limiter   *inbound.IPRateLimiter
	maxAttach int64
}

func NewFormIngestHandler(svc *messages.Service, limiter *inbound.IPRateLimiter, maxAttach int64) *FormIngestHandler {
	return &FormIngestHandler{svc: svc, limiter: limiter, maxAttach: maxAttach}
}

func (h *FormIngestHandler) SetBlobStore(bs blob.Store) { h.blobStore = bs }

type ingestResponse struct {
	Success bool           `json:"success"`
	Data    ingestRespData `json:"data"`
}

type ingestRespData struct {
	ID      string `json:"id,omitempty"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

func (h *FormIngestHandler) Nonce(c *okapi.Context) error {
	key := c.Param("key")
	form, err := h.svc.FindFormByKey(key)
	if err != nil || !form.IsActive() {
		return c.AbortNotFound("form not found")
	}
	h.applyCORS(c, form)

	nonce, err := h.svc.IssueNonce(c.Request().Context(), key)
	if err != nil {
		return c.AbortInternalServerError("failed to issue nonce")
	}
	return ok(c, nonce)
}

// Widget config reasons. "unavailable" covers unknown, paused and archived
// forms alike, so the endpoint does not reveal which keys exist.
const (
	widgetReasonUnavailable      = "unavailable"
	widgetReasonOriginNotAllowed = "origin_not_allowed"
)

// WidgetConfig is what the website widget needs before it renders a form.
type WidgetConfig struct {
	Enabled        bool            `json:"enabled"`
	Reason         string          `json:"reason,omitempty" doc:"Why the form cannot be used here: unavailable or origin_not_allowed"`
	HoneypotField  string          `json:"honeypot_field,omitempty"`
	RequireNonce   bool            `json:"require_nonce,omitempty"`
	MinFillSeconds int             `json:"min_fill_seconds,omitempty" doc:"Seconds that must pass between issuing the nonce and submitting; the widget waits them out instead of letting the submission be rejected"`
	Nonce          *messages.Nonce `json:"nonce,omitempty" doc:"Submission nonce, present only when require_nonce is true"`
	Fields         []WidgetField   `json:"fields,omitempty" doc:"The form's questions, when it has a schema. Without them the widget shows its built-in contact fields."`
}

// WidgetField is the public view of a schema field: what a browser needs to
// render and pre-check it, without internal flags such as PII or roles.
// Defaults are resolved, so the widget does not repeat them.
type WidgetField struct {
	Key         string               `json:"key"`
	Label       string               `json:"label"`
	Type        models.FieldType     `json:"type"`
	Required    bool                 `json:"required,omitempty"`
	MinLen      *int                 `json:"min_len,omitempty"`
	MaxLen      *int                 `json:"max_len,omitempty"`
	Min         *float64             `json:"min,omitempty"`
	Max         *float64             `json:"max,omitempty"`
	Pattern     string               `json:"pattern,omitempty"`
	Options     []models.FieldOption `json:"options,omitempty"`
	Display     models.FieldDisplay  `json:"display,omitempty"`
	MinLabel    string               `json:"min_label,omitempty"`
	MaxLabel    string               `json:"max_label,omitempty"`
	Placeholder string               `json:"placeholder,omitempty"`
	Help        string               `json:"help,omitempty"`
}

func widgetFields(defs models.FieldDefs) []WidgetField {
	active := defs.Active()
	out := make([]WidgetField, 0, len(active))
	for i := range active {
		f := &active[i]
		wf := WidgetField{
			Key: f.Key, Label: f.Label, Type: f.Type, Required: f.Required,
			MinLen: f.MinLen, Min: f.Min, Max: f.Max, Pattern: f.Pattern,
			Options: f.Options, Display: f.Display, MinLabel: f.MinLabel, MaxLabel: f.MaxLabel,
			Placeholder: f.Placeholder, Help: f.Help,
		}
		switch f.Type {
		case models.FieldText, models.FieldTextarea, models.FieldHidden:
			maxLen := messages.MaxLen(f)
			wf.MaxLen = &maxLen
		case models.FieldScale:
			lo, hi := messages.ScaleRange(f)
			low, high := float64(lo), float64(hi)
			wf.Min, wf.Max = &low, &high
		case models.FieldRating:
			stars := float64(messages.RatingStars(f))
			wf.Min, wf.Max = nil, &stars
		}
		out = append(out, wf)
	}
	return out
}

// widgetConfig decides what the widget on origin may see of form. A nil form
// is an unknown key.
func widgetConfig(form *models.Form, origin string) WidgetConfig {
	if form == nil || !form.IsActive() {
		return WidgetConfig{Reason: widgetReasonUnavailable}
	}
	allowlist, strict := form.OriginPolicy()
	if !messages.OriginAllowed(allowlist, strict, origin) {
		return WidgetConfig{Reason: widgetReasonOriginNotAllowed}
	}
	cfg := WidgetConfig{
		Enabled:       true,
		HoneypotField: form.Honeypot(),
		RequireNonce:  form.RequireNonce,
	}
	if form.RequireNonce {
		cfg.MinFillSeconds = form.MinFillSeconds
	}
	if form.HasSchema() {
		cfg.Fields = widgetFields(form.Fields)
	}
	return cfg
}

// WidgetConfig answers 200 in every case so the widget can always read why a
// form is not usable.
func (h *FormIngestHandler) WidgetConfig(c *okapi.Context) error {
	key := c.Param("key")
	origin := c.Request().Header.Get("Origin")
	c.ResponseWriter().Header().Set("Cache-Control", "no-store")

	form, err := h.svc.FindFormByKey(key)
	if err != nil {
		form = nil
	}
	cfg := widgetConfig(form, origin)
	if !cfg.Enabled {
		// A disabled answer carries no form data, so any origin may read it.
		if origin != "" {
			w := c.ResponseWriter()
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		return ok(c, cfg)
	}
	h.applyCORS(c, form)

	if cfg.RequireNonce {
		nonce, err := h.svc.IssueNonce(c.Request().Context(), key)
		if err != nil {
			return c.AbortInternalServerError("failed to issue nonce")
		}
		cfg.Nonce = nonce
	}
	return ok(c, cfg)
}

// FormScript serves the website form widget.
func (h *FormIngestHandler) FormScript(c *okapi.Context) error {
	w := c.ResponseWriter()
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("ETag", widget.FormScriptETag)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
	http.ServeContent(w, c.Request(), "posta-form.js", time.Time{}, bytes.NewReader(widget.FormScript))
	return nil
}

func (h *FormIngestHandler) Submit(c *okapi.Context) error {
	req := c.Request()
	key := c.Param("key")
	clientIP := c.RealIP()

	if h.limiter != nil && !h.limiter.Allow(clientIP) {
		return h.respond(c, nil, nil, http.StatusTooManyRequests, "rate_limited", "Too many submissions. Try again later.")
	}

	form, err := h.svc.FindFormByKey(key)
	if err != nil || !form.IsActive() {
		return h.respond(c, nil, nil, http.StatusAccepted, "received", genericAcceptMessage)
	}

	origin := req.Header.Get("Origin")
	allowlist, strict := form.OriginPolicy()
	if !messages.OriginAllowed(allowlist, strict, origin) {
		return h.respond(c, form, nil, http.StatusForbidden, "origin_not_allowed", "This origin is not allowed to submit to this form.")
	}
	h.applyCORS(c, form)

	fields, attachments, redirect, err := h.parseBody(c, form)
	if err != nil {
		switch {
		case errors.Is(err, messages.ErrPayloadTooLarge):
			return h.respond(c, form, nil, http.StatusRequestEntityTooLarge, "too_large", "Submission is too large.")
		case errors.Is(err, messages.ErrTooManyFields):
			return h.respond(c, form, nil, http.StatusBadRequest, "too_many_fields", "Submission has too many fields.")
		}
		return h.respond(c, form, nil, http.StatusBadRequest, "invalid_payload", "Submission could not be read.")
	}

	honeypot := false
	nonceFailed := false
	var nonceToken string
	for _, f := range fields {
		switch strings.ToLower(f.Key) {
		case strings.ToLower(form.Honeypot()):
			if strings.TrimSpace(f.Value) != "" {
				honeypot = true
			}
		case "_nonce":
			nonceToken = strings.TrimSpace(f.Value)
		}
	}

	if form.RequireNonce {
		if err := h.svc.VerifyNonce(req.Context(), key, nonceToken, form.MinFillSeconds); err != nil {
			nonceFailed = true
		}
	}

	result, err := h.svc.Submit(req.Context(), messages.SubmitRequest{
		Form:        form,
		Fields:      fields,
		Attachments: attachments,
		ClientIP:    clientIP,
		UserAgent:   req.UserAgent(),
		Referer:     req.Referer(),
		Origin:      origin,
		HoneypotHit: honeypot,
		NonceFailed: nonceFailed,
	})
	if err != nil {
		var invalid *messages.ValidationError
		if errors.As(err, &invalid) {
			return h.respondInvalid(c, form, invalid)
		}
		if errors.Is(err, messages.ErrRateLimited) {
			return h.respond(c, form, nil, http.StatusTooManyRequests, "rate_limited", "Too many submissions. Try again later.")
		}
		if errors.Is(err, messages.ErrTooManyFields) {
			return h.respond(c, form, nil, http.StatusBadRequest, "too_many_fields", "Submission has too many fields.")
		}
		logger.Error("form submission failed", "form_id", form.ID, "error", err)
		return h.respond(c, form, nil, http.StatusInternalServerError, "error", "Submission could not be stored.")
	}

	if redirect != "" && messages.RedirectAllowed(form.AllowedOrigins, form.RedirectURL, redirect) {
		c.Redirect(http.StatusSeeOther, redirect)
		return nil
	}
	if redirect == "" && form.RedirectURL != "" && !wantsJSON(req) {
		c.Redirect(http.StatusSeeOther, form.RedirectURL)
		return nil
	}

	return h.respond(c, form, result.Message, http.StatusAccepted, "received", genericAcceptMessage)
}

func (h *FormIngestHandler) parseBody(c *okapi.Context, form *models.Form) ([]models.MessageField, []models.InboundAttachmentMeta, string, error) {
	req := c.Request()
	limit := form.BodyLimit()
	contentType, _, _ := mime.ParseMediaType(req.Header.Get("Content-Type"))

	switch contentType {
	case "multipart/form-data":
		return h.parseMultipart(c, form, limit)

	case "application/json", "text/plain":
		body, err := io.ReadAll(io.LimitReader(req.Body, limit+1))
		if err != nil {
			return nil, nil, "", err
		}
		if int64(len(body)) > limit {
			return nil, nil, "", messages.ErrPayloadTooLarge
		}
		var raw map[string]any
		if err := json.Unmarshal(body, &raw); err != nil {
			return nil, nil, "", err
		}
		if len(raw) > form.FieldLimit() {
			return nil, nil, "", messages.ErrTooManyFields
		}
		fields := make([]models.MessageField, 0, len(raw))
		for k, v := range raw {
			// An array is a multi-value answer: keep its values apart so a
			// multi-select can be validated value by value.
			if items, isArray := v.([]any); isArray && len(items) > 0 {
				for _, item := range items {
					fields = append(fields, models.MessageField{Key: k, Value: stringify(item)})
				}
				continue
			}
			fields = append(fields, models.MessageField{Key: k, Value: stringify(v)})
		}
		if messages.DistinctKeys(fields) > form.FieldLimit() {
			return nil, nil, "", messages.ErrTooManyFields
		}
		return fields, nil, redirectFrom(raw), nil

	default:
		if err := h.parseFormValues(c, limit); err != nil {
			return nil, nil, "", err
		}
		fields, redirect := valuesToFields(req.PostForm)
		if messages.DistinctKeys(fields) > form.FieldLimit() {
			return nil, nil, "", messages.ErrTooManyFields
		}
		return fields, nil, redirect, nil
	}
}

func (h *FormIngestHandler) parseFormValues(c *okapi.Context, limit int64) error {
	req := c.Request()
	req.Body = http.MaxBytesReader(c.ResponseWriter(), req.Body, limit)
	if err := req.ParseForm(); err != nil {
		return messages.ErrPayloadTooLarge
	}
	return nil
}

func (h *FormIngestHandler) parseMultipart(c *okapi.Context, form *models.Form, limit int64) ([]models.MessageField, []models.InboundAttachmentMeta, string, error) {
	req := c.Request()
	maxMemory := limit + h.maxAttach
	req.Body = http.MaxBytesReader(c.ResponseWriter(), req.Body, maxMemory+1024)
	if err := req.ParseMultipartForm(limit); err != nil {
		return nil, nil, "", messages.ErrPayloadTooLarge
	}

	fields, redirect := valuesToFields(req.MultipartForm.Value)
	if messages.DistinctKeys(fields) > form.FieldLimit() {
		return nil, nil, "", messages.ErrTooManyFields
	}

	var attachments []models.InboundAttachmentMeta
	if !form.AllowAttachments {
		return fields, nil, redirect, nil
	}
	for _, headers := range req.MultipartForm.File {
		for _, fh := range headers {
			if len(attachments) >= 5 {
				break
			}
			if fh.Size > h.maxAttach {
				continue
			}
			file, err := fh.Open()
			if err != nil {
				continue
			}
			data, err := io.ReadAll(io.LimitReader(file, h.maxAttach))
			_ = file.Close()
			if err != nil {
				continue
			}
			meta := models.InboundAttachmentMeta{
				Filename:    messages.SanitizeHeaderValue(fh.Filename),
				ContentType: fh.Header.Get("Content-Type"),
				Size:        int64(len(data)),
			}
			if h.blobStore != nil {
				key := fmt.Sprintf("messages/%s/%d_%s", form.UUID, len(attachments), safeFilename(meta.Filename))
				if err := h.blobStore.Put(c.Request().Context(), key, bytes.NewReader(data), meta.ContentType); err == nil {
					meta.StorageKey = key
				}
			}
			if meta.StorageKey == "" {
				meta.Content = base64.StdEncoding.EncodeToString(data)
			}
			attachments = append(attachments, meta)
		}
	}

	return fields, attachments, redirect, nil
}

func (h *FormIngestHandler) applyCORS(c *okapi.Context, form *models.Form) {
	origin := c.Request().Header.Get("Origin")
	allowlist, strict := form.OriginPolicy()
	if origin == "" || !messages.OriginAllowed(allowlist, strict, origin) {
		return
	}
	w := c.ResponseWriter()
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Vary", "Origin")
}

func (h *FormIngestHandler) respond(c *okapi.Context, form *models.Form, msg *models.Message, status int, code, message string) error {
	if form != nil {
		h.applyCORS(c, form)
	}

	if !wantsJSON(c.Request()) && status < 400 {
		return trackingNotice(c, status, "Message sent", message, "success")
	}
	if !wantsJSON(c.Request()) {
		return trackingNotice(c, status, "Message not sent", message, "error")
	}

	data := ingestRespData{Status: code, Message: message}
	if msg != nil && status < 400 {
		data.ID = msg.UUID
	}
	return c.JSON(status, ingestResponse{Success: status < 400, Data: data})
}

// respondInvalid answers a submission that does not match the form's schema:
// 422 with per-field errors for scripts, or for a plain HTML form a redirect
// to the form's error page or a minimal page listing the errors.
func (h *FormIngestHandler) respondInvalid(c *okapi.Context, form *models.Form, invalid *messages.ValidationError) error {
	h.applyCORS(c, form)
	if wantsJSON(c.Request()) {
		return c.JSON(http.StatusUnprocessableEntity, validationResponse{
			Success: false,
			Error: validationError{
				Code:    "validation_failed",
				Message: "Some fields need attention.",
				Fields:  invalid.Fields,
			},
		})
	}
	if form.ErrorRedirectURL != "" {
		// The URL is set by the form's admin, and the query only carries a
		// fixed code and a schema key, never a submitted value.
		target, err := url.Parse(form.ErrorRedirectURL)
		if err == nil {
			q := target.Query()
			q.Set("posta_error", "validation_failed")
			q.Set("field", invalid.Fields[0].Key)
			target.RawQuery = q.Encode()
			c.Redirect(http.StatusSeeOther, target.String())
			return nil
		}
	}
	lines := make([]string, 0, len(invalid.Fields))
	for _, f := range invalid.Fields {
		name := f.Label
		if name == "" {
			name = f.Key
		}
		lines = append(lines, name+": "+f.Message)
	}
	return trackingNotice(c, http.StatusUnprocessableEntity, "Message not sent",
		strings.Join(lines, " ")+" Go back to correct the form and send it again.", "error")
}

type validationResponse struct {
	Success bool            `json:"success"`
	Error   validationError `json:"error"`
}

type validationError struct {
	Code    string                `json:"code"`
	Message string                `json:"message"`
	Fields  []messages.FieldError `json:"fields"`
}

func wantsJSON(req *http.Request) bool {
	contentType := req.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") || strings.Contains(contentType, "text/plain") {
		return true
	}
	accept := req.Header.Get("Accept")
	if strings.Contains(accept, "application/json") {
		return true
	}
	return !strings.Contains(accept, "text/html") && accept != ""
}

// valuesToFields keeps every value of a repeated key as its own field, which
// is how checkboxes sharing a name submit a multi-select.
func valuesToFields(values url.Values) ([]models.MessageField, string) {
	fields := make([]models.MessageField, 0, len(values))
	redirect := ""
	for key, vals := range values {
		if len(vals) == 0 {
			continue
		}
		if strings.EqualFold(key, "_redirect") || strings.EqualFold(key, "_next") {
			redirect = strings.Join(vals, ", ")
			continue
		}
		for _, v := range vals {
			fields = append(fields, models.MessageField{Key: key, Value: v})
		}
	}
	return fields, redirect
}

func redirectFrom(raw map[string]any) string {
	for _, key := range []string{"_redirect", "_next"} {
		if v, exists := raw[key]; exists {
			return stringify(v)
		}
	}
	return ""
}

func stringify(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		if t {
			return "true"
		}
		return "false"
	case float64:
		if t == float64(int64(t)) {
			return strings.TrimSuffix(strings.TrimRight(strings.TrimRight(formatFloat(t), "0"), "."), ".")
		}
		return formatFloat(t)
	case []any:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			parts = append(parts, stringify(item))
		}
		return strings.Join(parts, ", ")
	default:
		encoded, err := json.Marshal(t)
		if err != nil {
			return ""
		}
		return string(encoded)
	}
}

func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func safeFilename(name string) string {
	name = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '.', r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}, name)
	if name == "" {
		return "attachment"
	}
	if len(name) > 100 {
		name = name[:100]
	}
	return name
}
