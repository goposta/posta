// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package handlers

import (
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
)

type ContactHandler struct {
	repo            *repositories.ContactRepository
	suppressionRepo *repositories.SuppressionRepository
}

func NewContactHandler(repo *repositories.ContactRepository, suppressionRepo *repositories.SuppressionRepository) *ContactHandler {
	return &ContactHandler{repo: repo, suppressionRepo: suppressionRepo}
}

type ContactWithSuppressed struct {
	models.Contact
	Suppressed bool `json:"suppressed"`
}

type ListContactsRequest struct {
	Page   int    `query:"page" default:"0"`
	Size   int    `query:"size" default:"20"`
	Search string `query:"search"`
}

func (h *ContactHandler) List(c *okapi.Context, req *ListContactsRequest) error {
	scope := getScope(c)
	page, size, offset := normalizePageParams(req.Page, req.Size)

	contacts, total, err := h.repo.FindByScope(scope, req.Search, size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list contacts")
	}

	// Collect emails to batch-check suppression status
	emails := make([]string, len(contacts))
	for i, ct := range contacts {
		emails[i] = ct.Email
	}

	suppressedSet := make(map[string]bool)
	if len(emails) > 0 {
		unsuppressed, err := h.suppressionRepo.FilterSuppressed(scope, emails)
		if err == nil {
			unsuppressedSet := make(map[string]bool, len(unsuppressed))
			for _, e := range unsuppressed {
				unsuppressedSet[e] = true
			}
			for _, e := range emails {
				if !unsuppressedSet[e] {
					suppressedSet[e] = true
				}
			}
		}
	}

	result := make([]ContactWithSuppressed, len(contacts))
	for i, ct := range contacts {
		result[i] = ContactWithSuppressed{
			Contact:    ct,
			Suppressed: suppressedSet[ct.Email],
		}
	}

	return paginated(c, result, total, page, size)
}

func (h *ContactHandler) Get(c *okapi.Context, req *GetByIDRequest) error {
	contact, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, contact.UserID, contact.WorkspaceID) {
		return c.AbortNotFound("contact not found")
	}

	// Check suppression status using the contact's own scope
	contactScope := repositories.ResourceScope{UserID: contact.UserID, WorkspaceID: contact.WorkspaceID}
	suppressed := false
	unsuppressed, err := h.suppressionRepo.FilterSuppressed(contactScope, []string{contact.Email})
	if err == nil {
		suppressed = len(unsuppressed) == 0
	}

	return ok(c, ContactWithSuppressed{
		Contact:    *contact,
		Suppressed: suppressed,
	})
}
