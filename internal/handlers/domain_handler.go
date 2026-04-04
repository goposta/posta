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
	"crypto/rand"
	"encoding/hex"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/domain"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
	"gorm.io/gorm"
)

type DomainHandler struct {
	repo  *repositories.DomainRepository
	quota QuotaChecker
	db    *gorm.DB
}
type CreateDomainRequest struct {
	Body struct {
		Domain string `json:"domain" required:"true"`
	} `json:"body"`
}
type DomainWithRecords struct {
	models.Domain
	DNSRecords *domain.DNSRecords `json:"dns_records"`
}
type GetDomainRequest struct {
	ID int `param:"id"`
}
type DeleteDomainRequest struct {
	ID int `param:"id"`
}
type VerifyDomainRequest struct {
	ID int `param:"id"`
}

func NewDomainHandler(repo *repositories.DomainRepository) *DomainHandler {
	return &DomainHandler{repo: repo}
}

// SetQuota sets the quota checker for plan-based resource limits.
func (h *DomainHandler) SetQuota(q QuotaChecker, db *gorm.DB) {
	h.quota = q
	h.db = db
}

func (h *DomainHandler) Create(c *okapi.Context, req *CreateDomainRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	scope := getScope(c)

	if h.quota != nil {
		if err := h.quota.CheckQuota(h.db, scope.UserID, scope.WorkspaceID, "domains"); err != nil {
			return c.AbortForbidden("Domain quota exceeded for this workspace", err)
		}
	}

	token, err := generateVerificationToken()
	if err != nil {
		return c.AbortInternalServerError("failed to generate verification token")
	}

	d := &models.Domain{
		UserID:            scope.UserID,
		WorkspaceID:       scope.WorkspaceID,
		Domain:            req.Body.Domain,
		VerificationToken: token,
	}

	if err := h.repo.Create(d); err != nil {
		return c.AbortConflict("domain already exists")
	}

	return created(c, DomainWithRecords{
		Domain:     *d,
		DNSRecords: domain.RequiredRecords(d),
	})
}

func (h *DomainHandler) List(c *okapi.Context, req *ListRequest) error {
	page, size, offset := normalizePageParams(req.Page, req.Size)

	domains, total, err := h.repo.FindByScope(getScope(c), size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list domains")
	}

	return paginated(c, domains, total, page, size)
}

func (h *DomainHandler) Get(c *okapi.Context, req *GetDomainRequest) error {
	d, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, d.UserID, d.WorkspaceID) {
		return c.AbortNotFound("domain not found")
	}

	return ok(c, DomainWithRecords{
		Domain:     *d,
		DNSRecords: domain.RequiredRecords(d),
	})
}

func (h *DomainHandler) Delete(c *okapi.Context, req *DeleteDomainRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	d, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, d.UserID, d.WorkspaceID) {
		return c.AbortNotFound("domain not found")
	}

	if err := h.repo.Delete(d.ID); err != nil {
		return c.AbortInternalServerError("failed to delete domain")
	}

	return noContent(c)
}

func (h *DomainHandler) Verify(c *okapi.Context, req *VerifyDomainRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("insufficient workspace permissions", err)
	}
	d, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, d.UserID, d.WorkspaceID) {
		return c.AbortNotFound("domain not found")
	}

	result, err := domain.Verify(d)
	if err != nil {
		return c.AbortInternalServerError("DNS verification failed")
	}

	d.OwnershipVerified = result.OwnershipVerified
	d.SPFVerified = result.SPFVerified
	d.DKIMVerified = result.DKIMVerified
	d.DMARCVerified = result.DMARCVerified

	if err := h.repo.Update(d); err != nil {
		return c.AbortInternalServerError("failed to update domain")
	}

	return ok(c, okapi.M{
		"domain":         d,
		"verification":   result,
		"fully_verified": d.IsFullyVerified(),
	})
}

func generateVerificationToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
