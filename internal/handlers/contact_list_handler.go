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
	"github.com/jkaninda/okapi"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/storage/repositories"
)

type ContactListHandler struct {
	repo *repositories.ContactListRepository
}

func NewContactListHandler(repo *repositories.ContactListRepository) *ContactListHandler {
	return &ContactListHandler{repo: repo}
}

type CreateContactListRequest struct {
	Body struct {
		Name        string `json:"name" required:"true"`
		Description string `json:"description"`
	} `json:"body"`
}

type UpdateContactListRequest struct {
	ID   int `param:"id"`
	Body struct {
		Name        string `json:"name" required:"true"`
		Description string `json:"description"`
	} `json:"body"`
}

type AddMemberRequest struct {
	ID   int `param:"id"`
	Body struct {
		Email string `json:"email" required:"true" format:"email"`
		Name  string `json:"name"`
	} `json:"body"`
}

type RemoveMemberRequest struct {
	ID   int `param:"id"`
	Body struct {
		Email string `json:"email" required:"true" format:"email"`
	} `json:"body"`
}

type ListMembersRequest struct {
	ID   int `param:"id"`
	Page int `query:"page" default:"0"`
	Size int `query:"size" default:"20"`
}

type ContactListWithCount struct {
	models.ContactList
	MemberCount int64 `json:"member_count"`
}

func (h *ContactListHandler) Create(c *okapi.Context, req *CreateContactListRequest) error {
	if err := requireEdit(c); err != nil {
		return err
	}
	scope := getScope(c)
	list := &models.ContactList{
		UserID:      scope.UserID,
		WorkspaceID: scope.WorkspaceID,
		Name:        req.Body.Name,
		Description: req.Body.Description,
	}
	if err := h.repo.Create(list); err != nil {
		return c.AbortInternalServerError("failed to create contact list")
	}
	return created(c, list)
}

func (h *ContactListHandler) List(c *okapi.Context, req *ListRequest) error {
	page, size, offset := normalizePageParams(req.Page, req.Size)
	lists, total, err := h.repo.FindByScope(getScope(c), size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list contact lists")
	}

	// Add member counts
	result := make([]ContactListWithCount, len(lists))
	for i, l := range lists {
		result[i] = ContactListWithCount{
			ContactList: l,
			MemberCount: h.repo.MemberCount(l.ID),
		}
	}

	return paginated(c, result, total, page, size)
}

func (h *ContactListHandler) Update(c *okapi.Context, req *UpdateContactListRequest) error {
	if err := requireEdit(c); err != nil {
		return err
	}
	list, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, list.UserID, list.WorkspaceID) {
		return c.AbortNotFound("contact list not found")
	}
	list.Name = req.Body.Name
	list.Description = req.Body.Description
	if err := h.repo.Update(list); err != nil {
		return c.AbortInternalServerError("failed to update contact list")
	}
	return ok(c, list)
}

func (h *ContactListHandler) Delete(c *okapi.Context, req *GetByIDRequest) error {
	if err := requireEdit(c); err != nil {
		return err
	}
	list, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, list.UserID, list.WorkspaceID) {
		return c.AbortNotFound("contact list not found")
	}
	if err := h.repo.Delete(list.ID); err != nil {
		return c.AbortInternalServerError("failed to delete contact list")
	}
	return noContent(c)
}

func (h *ContactListHandler) AddMember(c *okapi.Context, req *AddMemberRequest) error {
	if err := requireEdit(c); err != nil {
		return err
	}
	list, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, list.UserID, list.WorkspaceID) {
		return c.AbortNotFound("contact list not found")
	}
	member := &models.ContactListMember{
		ListID: list.ID,
		Email:  req.Body.Email,
		Name:   req.Body.Name,
	}
	if err := h.repo.AddMember(member); err != nil {
		return c.AbortConflict("member already exists in list")
	}
	return created(c, member)
}

func (h *ContactListHandler) RemoveMember(c *okapi.Context, req *RemoveMemberRequest) error {
	if err := requireEdit(c); err != nil {
		return err
	}
	list, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, list.UserID, list.WorkspaceID) {
		return c.AbortNotFound("contact list not found")
	}
	if err := h.repo.RemoveMember(list.ID, req.Body.Email); err != nil {
		return c.AbortInternalServerError("failed to remove member")
	}
	return noContent(c)
}

func (h *ContactListHandler) ListMembers(c *okapi.Context, req *ListMembersRequest) error {
	list, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, list.UserID, list.WorkspaceID) {
		return c.AbortNotFound("contact list not found")
	}
	page, size, offset := normalizePageParams(req.Page, req.Size)
	members, total, err := h.repo.ListMembers(list.ID, size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list members")
	}
	return paginated(c, members, total, page, size)
}
