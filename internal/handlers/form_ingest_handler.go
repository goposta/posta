// SPDX-FileCopyrightText: 2026 Jonas Kaninda
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

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/inbound"
	"github.com/goposta/posta/internal/services/messages"
	"github.com/goposta/posta/internal/storage/blob"
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
	if !messages.OriginAllowed(form, origin) {
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
		if errors.Is(err, messages.ErrRateLimited) {
			return h.respond(c, form, nil, http.StatusTooManyRequests, "rate_limited", "Too many submissions. Try again later.")
		}
		if errors.Is(err, messages.ErrTooManyFields) {
			return h.respond(c, form, nil, http.StatusBadRequest, "too_many_fields", "Submission has too many fields.")
		}
		logger.Error("form submission failed", "form_id", form.ID, "error", err)
		return h.respond(c, form, nil, http.StatusInternalServerError, "error", "Submission could not be stored.")
	}

	if redirect != "" && messages.RedirectAllowed(form, redirect) {
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
		fields := make([]models.MessageField, 0, len(raw))
		for k, v := range raw {
			fields = append(fields, models.MessageField{Key: k, Value: stringify(v)})
		}
		if len(fields) > form.FieldLimit() {
			return nil, nil, "", messages.ErrTooManyFields
		}
		return fields, nil, redirectFrom(raw), nil

	default:
		if err := h.parseFormValues(c, limit); err != nil {
			return nil, nil, "", err
		}
		fields, redirect := valuesToFields(req.PostForm)
		if len(fields) > form.FieldLimit() {
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
	if len(fields) > form.FieldLimit() {
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
	if origin == "" || !messages.OriginAllowed(form, origin) {
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

func valuesToFields(values url.Values) ([]models.MessageField, string) {
	fields := make([]models.MessageField, 0, len(values))
	redirect := ""
	for key, vals := range values {
		if len(vals) == 0 {
			continue
		}
		value := strings.Join(vals, ", ")
		if strings.EqualFold(key, "_redirect") || strings.EqualFold(key, "_next") {
			redirect = value
			continue
		}
		fields = append(fields, models.MessageField{Key: key, Value: value})
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
