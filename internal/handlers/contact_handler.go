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
	"github.com/jkaninda/posta/internal/models"
	"github.com/jkaninda/posta/internal/storage/repositories"
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
	userID := c.GetInt("user_id")
	page, size, offset := normalizePageParams(req.Page, req.Size)

	contacts, total, err := h.repo.FindByUserID(uint(userID), req.Search, size, offset)
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
		unsuppressed, err := h.suppressionRepo.FilterSuppressed(uint(userID), emails)
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
	userID := c.GetInt("user_id")

	contact, err := h.repo.FindByID(uint(req.ID))
	if err != nil || contact.UserID != uint(userID) {
		return c.AbortNotFound("contact not found")
	}

	// Check suppression status
	suppressed := false
	unsuppressed, err := h.suppressionRepo.FilterSuppressed(uint(userID), []string{contact.Email})
	if err == nil {
		suppressed = len(unsuppressed) == 0
	}

	return ok(c, ContactWithSuppressed{
		Contact:    *contact,
		Suppressed: suppressed,
	})
}
