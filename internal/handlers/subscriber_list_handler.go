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
	"errors"
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
	"gorm.io/gorm"
)

type SubscriberListHandler struct {
	repo           *repositories.SubscriberListRepository
	subscriberRepo *repositories.SubscriberRepository
}

func NewSubscriberListHandler(repo *repositories.SubscriberListRepository, subscriberRepo *repositories.SubscriberRepository) *SubscriberListHandler {
	return &SubscriberListHandler{repo: repo, subscriberRepo: subscriberRepo}
}

type CreateSubscriberListRequest struct {
	Body struct {
		Name        string              `json:"name" required:"true"`
		Description string              `json:"description"`
		Type        string              `json:"type"`
		FilterRules []models.FilterRule `json:"filter_rules"`
	} `json:"body"`
}

type UpdateSubscriberListRequest struct {
	ID   int `param:"id"`
	Body struct {
		Name        string              `json:"name"`
		Description string              `json:"description"`
		FilterRules []models.FilterRule `json:"filter_rules"`
	} `json:"body"`
}

type DeleteSubscriberListRequest struct {
	ID int `param:"id"`
}

type GetSubscriberListRequest struct {
	ID int `param:"id"`
}

type ListSubscriberListMembersRequest struct {
	ID   int `param:"id"`
	Page int `query:"page" default:"0"`
	Size int `query:"size" default:"20"`
}

type AddSubscriberToListRequest struct {
	ID   int `param:"id"`
	Body struct {
		SubscriberID uint `json:"subscriber_id" required:"true"`
	} `json:"body"`
}

type RemoveSubscriberFromListRequest struct {
	ID   int `param:"id"`
	Body struct {
		SubscriberID uint `json:"subscriber_id" required:"true"`
	} `json:"body"`
}

type PreviewSegmentRequest struct {
	Body struct {
		FilterRules []models.FilterRule `json:"filter_rules" required:"true"`
	} `json:"body"`
}

// ListUnsubscribeByEmailRequest is the API-key-accessible unsubscribe that
// takes an email address (the caller usually doesn't know the subscriber ID).
type ListUnsubscribeByEmailRequest struct {
	ID   int `param:"id"`
	Body struct {
		Email  string `json:"email" required:"true" format:"email"`
		Reason string `json:"reason"`
	} `json:"body"`
}

// ListResubscribeByEmailRequest is the inverse of ListUnsubscribeByEmailRequest.
type ListResubscribeByEmailRequest struct {
	ID   int `param:"id"`
	Body struct {
		Email string `json:"email" required:"true" format:"email"`
	} `json:"body"`
}

// ListSubscribeRequest is the API-key-accessible "subscribe this person to
// this list" endpoint. The list is identified by name (not id) and created on
// first use, so external platforms (n8n, forms) don't need a prior list-id
// lookup. Any prior per-list opt-out for the same (list, email) is cleared —
// this is the explicit inverse of /unsubscribe.
type ListSubscribeRequest struct {
	Body struct {
		Email string `json:"email" required:"true" format:"email"`
		Name  string `json:"name"`
		List  string `json:"list" required:"true"`
	} `json:"body"`
}

type ListSubscribeResponse struct {
	ListID            uint   `json:"list_id"`
	SubscriberID      uint   `json:"subscriber_id"`
	Email             string `json:"email"`
	Action            string `json:"action"` // "subscribed" | "unsubscribed" | "resubscribed" | "noop"
	ListCreated       bool   `json:"list_created,omitempty"`
	SubscriberCreated bool   `json:"subscriber_created,omitempty"`
	MemberAdded       bool   `json:"member_added,omitempty"`
}

type SubscriberListWithCount struct {
	ID          uint                      `json:"id"`
	Name        string                    `json:"name"`
	Description string                    `json:"description"`
	Type        models.SubscriberListType `json:"type"`
	FilterRules models.FilterRules        `json:"filter_rules,omitempty"`
	MemberCount int64                     `json:"member_count"`
	CreatedAt   time.Time                 `json:"created_at"`
	UpdatedAt   *time.Time                `json:"updated_at"`
}

func (h *SubscriberListHandler) Create(c *okapi.Context, req *CreateSubscriberListRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("Insufficient workspace permissions", err)
	}
	scope := getScope(c)

	listType := models.SubscriberListType(req.Body.Type)
	if listType == "" {
		listType = models.SubscriberListTypeStatic
	}

	list := &models.SubscriberList{
		UserID:      scope.UserID,
		WorkspaceID: scope.WorkspaceID,
		Name:        req.Body.Name,
		Description: req.Body.Description,
		Type:        listType,
		FilterRules: req.Body.FilterRules,
	}

	if err := h.repo.Create(list); err != nil {
		return c.AbortInternalServerError("failed to create list")
	}
	return created(c, list)
}

func (h *SubscriberListHandler) List(c *okapi.Context, req *ListRequest) error {
	page, size, offset := normalizePageParams(req.Page, req.Size)
	lists, total, err := h.repo.FindByScope(getScope(c), size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list subscriber lists")
	}

	// Enrich with member counts
	var result []SubscriberListWithCount
	for _, l := range lists {
		count := h.repo.MemberCount(l.ID)
		if l.Type == models.SubscriberListTypeDynamic && l.FilterRules != nil {
			dynCount, _ := h.subscriberRepo.CountByFilterRules(getScope(c), l.FilterRules)
			count = dynCount
		}
		result = append(result, SubscriberListWithCount{
			ID:          l.ID,
			Name:        l.Name,
			Description: l.Description,
			Type:        l.Type,
			FilterRules: l.FilterRules,
			MemberCount: count,
			CreatedAt:   l.CreatedAt,
			UpdatedAt:   l.UpdatedAt,
		})
	}

	return paginated(c, result, total, page, size)
}

func (h *SubscriberListHandler) Get(c *okapi.Context, req *GetSubscriberListRequest) error {
	list, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, list.UserID, list.WorkspaceID) {
		return c.AbortNotFound("list not found")
	}

	count := h.repo.MemberCount(list.ID)
	if list.Type == models.SubscriberListTypeDynamic && list.FilterRules != nil {
		dynCount, _ := h.subscriberRepo.CountByFilterRules(getScope(c), list.FilterRules)
		count = dynCount
	}

	return ok(c, SubscriberListWithCount{
		ID:          list.ID,
		Name:        list.Name,
		Description: list.Description,
		Type:        list.Type,
		FilterRules: list.FilterRules,
		MemberCount: count,
		CreatedAt:   list.CreatedAt,
		UpdatedAt:   list.UpdatedAt,
	})
}

func (h *SubscriberListHandler) Update(c *okapi.Context, req *UpdateSubscriberListRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("Insufficient workspace permissions", err)
	}
	list, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, list.UserID, list.WorkspaceID) {
		return c.AbortNotFound("list not found")
	}

	if req.Body.Name != "" {
		list.Name = req.Body.Name
	}
	if req.Body.Description != "" {
		list.Description = req.Body.Description
	}
	if req.Body.FilterRules != nil {
		list.FilterRules = req.Body.FilterRules
	}
	now := time.Now()
	list.UpdatedAt = &now

	if err := h.repo.Update(list); err != nil {
		return c.AbortInternalServerError("failed to update list")
	}
	return ok(c, list)
}

func (h *SubscriberListHandler) Delete(c *okapi.Context, req *DeleteSubscriberListRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("Insufficient workspace permissions", err)
	}
	list, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, list.UserID, list.WorkspaceID) {
		return c.AbortNotFound("list not found")
	}
	if err := h.repo.Delete(list.ID); err != nil {
		return c.AbortInternalServerError("failed to delete list")
	}
	return noContent(c)
}

func (h *SubscriberListHandler) AddMember(c *okapi.Context, req *AddSubscriberToListRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("Insufficient workspace permissions", err)
	}
	list, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, list.UserID, list.WorkspaceID) {
		return c.AbortNotFound("list not found")
	}
	if list.Type != models.SubscriberListTypeStatic {
		return c.AbortBadRequest("cannot manually add members to a dynamic list")
	}

	// Verify subscriber belongs to the same scope
	sub, err := h.subscriberRepo.FindByID(req.Body.SubscriberID)
	if err != nil || !ownsResource(c, sub.UserID, sub.WorkspaceID) {
		return c.AbortNotFound("subscriber not found")
	}

	member := &models.SubscriberListMember{
		ListID:       list.ID,
		SubscriberID: sub.ID,
	}
	if err := h.repo.AddMember(member); err != nil {
		return c.AbortConflict("subscriber already in list")
	}
	return ok(c, okapi.M{"message": "subscriber added to list"})
}

func (h *SubscriberListHandler) RemoveMember(c *okapi.Context, req *RemoveSubscriberFromListRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("Insufficient workspace permissions", err)
	}
	list, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, list.UserID, list.WorkspaceID) {
		return c.AbortNotFound("list not found")
	}
	if err := h.repo.RemoveMember(list.ID, req.Body.SubscriberID); err != nil {
		return c.AbortInternalServerError("failed to remove subscriber")
	}
	return ok(c, okapi.M{"message": "subscriber removed from list"})
}

func (h *SubscriberListHandler) ListMembers(c *okapi.Context, req *ListSubscriberListMembersRequest) error {
	list, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, list.UserID, list.WorkspaceID) {
		return c.AbortNotFound("list not found")
	}

	page, size, offset := normalizePageParams(req.Page, req.Size)

	if list.Type == models.SubscriberListTypeDynamic && list.FilterRules != nil {
		items, total, err := h.subscriberRepo.FindByFilterRules(getScope(c), list.FilterRules, size, offset)
		if err != nil {
			return c.AbortInternalServerError("failed to evaluate segment")
		}
		return paginated(c, items, total, page, size)
	}

	items, total, err := h.repo.ListMembers(list.ID, size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list members")
	}
	return paginated(c, items, total, page, size)
}

func (h *SubscriberListHandler) PreviewSegment(c *okapi.Context, req *PreviewSegmentRequest) error {
	count, err := h.subscriberRepo.CountByFilterRules(getScope(c), req.Body.FilterRules)
	if err != nil {
		return c.AbortInternalServerError("failed to evaluate segment")
	}
	return ok(c, okapi.M{"count": count})
}

// Subscribe adds an email to a named list via API key.
func (h *SubscriberListHandler) Subscribe(c *okapi.Context, req *ListSubscribeRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	scope := getScope(c)

	listName := strings.TrimSpace(req.Body.List)
	if listName == "" {
		return c.AbortBadRequest("list is required")
	}
	email := strings.ToLower(strings.TrimSpace(req.Body.Email))
	if email == "" {
		return c.AbortBadRequest("email is required")
	}

	// Find-or-create list in scope.
	listCreated := false
	list, err := h.repo.FindByNameForScope(scope, listName)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return c.AbortInternalServerError("list lookup failed")
		}
		list = &models.SubscriberList{
			UserID:      scope.UserID,
			WorkspaceID: scope.WorkspaceID,
			Name:        listName,
			Type:        models.SubscriberListTypeStatic,
		}
		if err := h.repo.Create(list); err != nil {
			return c.AbortInternalServerError("failed to create list")
		}
		listCreated = true
	}
	if list.Type == models.SubscriberListTypeDynamic {
		return c.AbortBadRequest("cannot subscribe to a dynamic list; adjust filter rules instead")
	}

	// Find-or-create subscriber.
	subscriberCreated := false
	sub, err := h.subscriberRepo.FindByEmail(scope, email)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return c.AbortInternalServerError("subscriber lookup failed")
		}
		now := time.Now()
		sub = &models.Subscriber{
			UserID:       scope.UserID,
			WorkspaceID:  scope.WorkspaceID,
			Email:        email,
			Name:         strings.TrimSpace(req.Body.Name),
			Status:       models.SubscriberStatusSubscribed,
			SubscribedAt: &now,
		}
		if err := h.subscriberRepo.Create(sub); err != nil {
			return c.AbortInternalServerError("failed to create subscriber")
		}
		subscriberCreated = true
	} else if req.Body.Name != "" && sub.Name == "" {
		// Fill in a name we didn't have before; don't overwrite a set one.
		sub.Name = strings.TrimSpace(req.Body.Name)
		_ = h.subscriberRepo.Update(sub)
	}

	_ = h.repo.UnsuppressMember(list.ID, sub.ID)

	memberAdded := !h.repo.IsMember(list.ID, sub.ID)
	if err := h.repo.AddMember(&models.SubscriberListMember{
		ListID:       list.ID,
		SubscriberID: sub.ID,
	}); err != nil {
		return c.AbortInternalServerError("failed to add to list")
	}

	return ok(c, ListSubscribeResponse{
		ListID:            list.ID,
		SubscriberID:      sub.ID,
		Email:             sub.Email,
		Action:            "subscribed",
		ListCreated:       listCreated,
		SubscriberCreated: subscriberCreated,
		MemberAdded:       memberAdded,
	})
}

// UnsubscribeByEmail opts an email out of a specific list.
func (h *SubscriberListHandler) UnsubscribeByEmail(c *okapi.Context, req *ListUnsubscribeByEmailRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	list, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, list.UserID, list.WorkspaceID) {
		return c.AbortNotFound("list not found")
	}
	sub, err := h.subscriberRepo.FindByEmail(getScope(c), req.Body.Email)
	if err != nil || sub == nil {
		return c.AbortNotFound("subscriber not found for this email")
	}
	reason := req.Body.Reason
	if reason == "" {
		reason = "api"
	}
	if err := h.repo.SuppressMember(list.ID, sub.ID, reason); err != nil {
		return c.AbortInternalServerError("failed to unsubscribe")
	}
	return ok(c, ListSubscribeResponse{
		ListID:       list.ID,
		SubscriberID: sub.ID,
		Email:        sub.Email,
		Action:       "unsubscribed",
	})
}

// ResubscribeByEmail removes the list-scoped opt-out AND re-adds the
// subscriber to the list (static lists only).
func (h *SubscriberListHandler) ResubscribeByEmail(c *okapi.Context, req *ListResubscribeByEmailRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	list, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, list.UserID, list.WorkspaceID) {
		return c.AbortNotFound("list not found")
	}
	sub, err := h.subscriberRepo.FindByEmail(getScope(c), req.Body.Email)
	if err != nil || sub == nil {
		return c.AbortNotFound("subscriber not found for this email")
	}
	if err := h.repo.UnsuppressMember(list.ID, sub.ID); err != nil {
		return c.AbortInternalServerError("failed to resubscribe")
	}
	if list.Type == models.SubscriberListTypeStatic {
		_ = h.repo.AddMember(&models.SubscriberListMember{
			ListID:       list.ID,
			SubscriberID: sub.ID,
		})
	}
	return ok(c, ListSubscribeResponse{
		ListID:       list.ID,
		SubscriberID: sub.ID,
		Email:        sub.Email,
		Action:       "resubscribed",
	})
}
