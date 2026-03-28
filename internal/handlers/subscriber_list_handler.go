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
	"time"

	"github.com/jkaninda/okapi"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/storage/repositories"
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
		return err
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
		return err
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
		return err
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
		return err
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
		return err
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
