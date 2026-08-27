// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package handlers

import (
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/messagescan"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
)

type MessageFilterHandler struct {
	repo        *repositories.MessageFilterRepository
	messageRepo *repositories.MessageRepository
	formRepo    *repositories.FormRepository
	scanner     *messagescan.Scanner
}

func NewMessageFilterHandler(
	repo *repositories.MessageFilterRepository,
	messageRepo *repositories.MessageRepository,
	formRepo *repositories.FormRepository,
	scanner *messagescan.Scanner,
) *MessageFilterHandler {
	return &MessageFilterHandler{repo: repo, messageRepo: messageRepo, formRepo: formRepo, scanner: scanner}
}

type CreateMessageFilterRequest struct {
	Body struct {
		FormID        *uint    `json:"form_id"`
		Kind          string   `json:"kind" required:"true" enum:"keyword,phrase,regex,email,domain,ip"`
		Pattern       string   `json:"pattern" required:"true"`
		Action        string   `json:"action" enum:"score,flag,quarantine,reject,allowlist"`
		Score         *float64 `json:"score"`
		Fields        []string `json:"fields"`
		CaseSensitive bool     `json:"case_sensitive"`
		Note          string   `json:"note"`
	} `json:"body"`
}

type UpdateMessageFilterRequest struct {
	ID   int `param:"id"`
	Body struct {
		Pattern       string    `json:"pattern"`
		Action        string    `json:"action" enum:"score,flag,quarantine,reject,allowlist"`
		Score         *float64  `json:"score"`
		Fields        *[]string `json:"fields"`
		CaseSensitive *bool     `json:"case_sensitive"`
		Enabled       *bool     `json:"enabled"`
		Note          *string   `json:"note"`
	} `json:"body"`
}

type MessageFilterIDRequest struct {
	ID int `param:"id"`
}

type TestMessageFilterRequest struct {
	Body struct {
		Kind          string `json:"kind" required:"true" enum:"keyword,phrase,regex,email,domain,ip"`
		Pattern       string `json:"pattern" required:"true"`
		CaseSensitive bool   `json:"case_sensitive"`
		Limit         int    `json:"limit"`
	} `json:"body"`
}

type FilterMatch struct {
	MessageUUID string `json:"message_uuid"`
	SenderEmail string `json:"sender_email"`
	Subject     string `json:"subject"`
	Excerpt     string `json:"excerpt"`
}

type TestMessageFilterResponse struct {
	Scanned int           `json:"scanned"`
	Matched int           `json:"matched"`
	Samples []FilterMatch `json:"samples"`
}

func (h *MessageFilterHandler) Create(c *okapi.Context, req *CreateMessageFilterRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	scope := getScope(c)
	if scope.WorkspaceID == nil {
		return c.AbortBadRequest("a workspace is required")
	}

	kind := models.FilterKind(strings.ToLower(strings.TrimSpace(req.Body.Kind)))
	if !models.ValidFilterKinds[kind] {
		return c.AbortBadRequest("invalid filter kind")
	}
	pattern := strings.TrimSpace(req.Body.Pattern)
	if err := messagescan.ValidatePattern(kind, pattern); err != nil {
		return c.AbortBadRequest(err.Error())
	}

	action := models.FilterAction(strings.ToLower(strings.TrimSpace(req.Body.Action)))
	if action == "" {
		action = models.FilterActionScore
	}
	if !models.ValidFilterActions[action] {
		return c.AbortBadRequest("invalid filter action")
	}

	if req.Body.FormID != nil {
		if _, err := h.formRepo.FindByIDForScope(scope, *req.Body.FormID); err != nil {
			return c.AbortBadRequest("form not found in this workspace")
		}
	}

	score := 3.0
	if req.Body.Score != nil {
		score = *req.Body.Score
	}

	filter := &models.MessageFilter{
		WorkspaceID:   scope.WorkspaceID,
		FormID:        req.Body.FormID,
		Kind:          kind,
		Pattern:       pattern,
		Action:        action,
		Score:         score,
		Fields:        normalizeFilterFields(req.Body.Fields),
		CaseSensitive: req.Body.CaseSensitive,
		Enabled:       true,
		Note:          strings.TrimSpace(req.Body.Note),
	}
	if err := h.repo.Create(filter); err != nil {
		return c.AbortInternalServerError("failed to create filter")
	}
	return created(c, filter)
}

func (h *MessageFilterHandler) List(c *okapi.Context, req *ListRequest) error {
	page, size, offset := normalizePageParams(req.Page, req.Size)
	items, total, err := h.repo.FindByScope(getScope(c), size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list filters")
	}
	return paginated(c, items, total, page, size)
}

func (h *MessageFilterHandler) Update(c *okapi.Context, req *UpdateMessageFilterRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	filter, err := h.repo.FindByIDForScope(getScope(c), uint(req.ID))
	if err != nil {
		return c.AbortNotFound("filter not found")
	}

	if p := strings.TrimSpace(req.Body.Pattern); p != "" {
		if err := messagescan.ValidatePattern(filter.Kind, p); err != nil {
			return c.AbortBadRequest(err.Error())
		}
		filter.Pattern = p
	}
	if a := strings.TrimSpace(req.Body.Action); a != "" {
		action := models.FilterAction(strings.ToLower(a))
		if !models.ValidFilterActions[action] {
			return c.AbortBadRequest("invalid filter action")
		}
		filter.Action = action
	}
	if req.Body.Score != nil {
		filter.Score = *req.Body.Score
	}
	if req.Body.Fields != nil {
		filter.Fields = normalizeFilterFields(*req.Body.Fields)
	}
	if req.Body.CaseSensitive != nil {
		filter.CaseSensitive = *req.Body.CaseSensitive
	}
	if req.Body.Enabled != nil {
		filter.Enabled = *req.Body.Enabled
	}
	if req.Body.Note != nil {
		filter.Note = strings.TrimSpace(*req.Body.Note)
	}

	now := time.Now()
	filter.UpdatedAt = &now
	if err := h.repo.Update(filter); err != nil {
		return c.AbortInternalServerError("failed to update filter")
	}
	return ok(c, filter)
}

func (h *MessageFilterHandler) Delete(c *okapi.Context, req *MessageFilterIDRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	filter, err := h.repo.FindByIDForScope(getScope(c), uint(req.ID))
	if err != nil {
		return c.AbortNotFound("filter not found")
	}
	if err := h.repo.Delete(filter.ID); err != nil {
		return c.AbortInternalServerError("failed to delete filter")
	}
	return noContent(c)
}

func (h *MessageFilterHandler) Test(c *okapi.Context, req *TestMessageFilterRequest) error {
	scope := getScope(c)
	if scope.WorkspaceID == nil {
		return c.AbortBadRequest("a workspace is required")
	}

	kind := models.FilterKind(strings.ToLower(strings.TrimSpace(req.Body.Kind)))
	if !models.ValidFilterKinds[kind] {
		return c.AbortBadRequest("invalid filter kind")
	}
	pattern := strings.TrimSpace(req.Body.Pattern)
	if err := messagescan.ValidatePattern(kind, pattern); err != nil {
		return c.AbortBadRequest(err.Error())
	}

	limit := req.Body.Limit
	if limit <= 0 || limit > 500 {
		limit = 200
	}

	candidates, _, err := h.messageRepo.FindByScopeFiltered(scope, repositories.MessageFilterQuery{}, limit, 0)
	if err != nil {
		return c.AbortInternalServerError("failed to load messages")
	}

	probe := &models.MessageFilter{
		Kind:          kind,
		Pattern:       pattern,
		Action:        models.FilterActionScore,
		CaseSensitive: req.Body.CaseSensitive,
	}

	resp := TestMessageFilterResponse{Scanned: len(candidates), Samples: []FilterMatch{}}
	for i := range candidates {
		m := &candidates[i]
		hydrate(m)
		if !h.scanner.MatchesFilter(probe, messagescan.Input{
			SenderEmail: m.SenderEmail,
			SenderName:  m.SenderName,
			Subject:     m.Subject,
			Body:        m.Body,
			Fields:      m.Fields,
			ClientIP:    m.ClientIP,
		}) {
			continue
		}
		resp.Matched++
		if len(resp.Samples) < 10 {
			resp.Samples = append(resp.Samples, FilterMatch{
				MessageUUID: m.UUID,
				SenderEmail: m.SenderEmail,
				Subject:     m.Subject,
				Excerpt:     truncateBody(m.Body, 160),
			})
		}
	}
	return ok(c, resp)
}

func normalizeFilterFields(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}
