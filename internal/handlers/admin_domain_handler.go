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
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/audit"
	"github.com/goposta/posta/internal/services/domain"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
	"gorm.io/gorm"
)

type AdminDomainHandler struct {
	db    *gorm.DB
	repo  *repositories.DomainRepository
	audit *audit.Logger
}

func NewAdminDomainHandler(db *gorm.DB, repo *repositories.DomainRepository, auditLogger *audit.Logger) *AdminDomainHandler {
	return &AdminDomainHandler{db: db, repo: repo, audit: auditLogger}
}

// AdminDomainRow is one line of the platform-wide domain table.
type AdminDomainRow struct {
	ID                uint      `json:"id"`
	Domain            string    `json:"domain"`
	WorkspaceID       *uint     `json:"workspace_id,omitempty"`
	WorkspaceName     string    `json:"workspace_name"`
	OwnerID           uint      `json:"owner_id"`
	OwnerEmail        string    `json:"owner_email"`
	OwnershipVerified bool      `json:"ownership_verified"`
	SPFVerified       bool      `json:"spf_verified"`
	DKIMVerified      bool      `json:"dkim_verified"`
	DMARCVerified     bool      `json:"dmarc_verified"`
	FullyVerified     bool      `json:"fully_verified"`
	CreatedAt         time.Time `json:"created_at"`
}

// AdminDomainDetail adds what an administrator needs to act on a domain: who
// owns it, the records its owner still has to publish, and whether the same name
// is already verified somewhere else.
type AdminDomainDetail struct {
	AdminDomainRow
	VerificationToken     string             `json:"verification_token"`
	Records               *domain.DNSRecords `json:"records"`
	ConflictWorkspaceID   *uint              `json:"conflict_workspace_id,omitempty"`
	ConflictWorkspaceName string             `json:"conflict_workspace_name,omitempty"`
}

type AdminListDomainsRequest struct {
	Page      int    `query:"page" default:"0"`
	Size      int    `query:"size" default:"20"`
	Search    string `query:"search" doc:"Match part of the domain name"`
	Status    string `query:"status" enum:"verified,unverified" doc:"Filter on ownership verification"`
	Workspace int    `query:"workspace" doc:"Restrict to one workspace id"`
}

type AdminDomainIDRequest struct {
	ID int `param:"id"`
}

type AdminSetDomainVerificationRequest struct {
	ID   int `param:"id"`
	Body struct {
		OwnershipVerified bool   `json:"ownership_verified" required:"true"`
		Reason            string `json:"reason" doc:"Why the DNS check is being bypassed. Required when granting."`
	} `json:"body"`
}

func (h *AdminDomainHandler) List(c *okapi.Context, req *AdminListDomainsRequest) error {
	page, size, offset := normalizePageParams(req.Page, req.Size)

	filter := repositories.AdminDomainFilter{Search: req.Search}
	switch req.Status {
	case "verified":
		v := true
		filter.Verified = &v
	case "unverified":
		v := false
		filter.Verified = &v
	}
	if req.Workspace > 0 {
		ws := uint(req.Workspace)
		filter.WorkspaceID = &ws
	}

	domains, total, err := h.repo.FindAllFiltered(filter, size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list domains")
	}

	workspaces := h.workspaceNames(domains)
	owners := h.ownerEmails(domains)

	rows := make([]AdminDomainRow, 0, len(domains))
	for i := range domains {
		rows = append(rows, h.row(&domains[i], workspaces, owners))
	}
	return paginated(c, rows, total, page, size)
}

func (h *AdminDomainHandler) Get(c *okapi.Context, req *AdminDomainIDRequest) error {
	d, err := h.repo.FindByID(uint(req.ID))
	if err != nil {
		return c.AbortNotFound("domain not found")
	}

	detail := AdminDomainDetail{
		AdminDomainRow:    h.row(d, h.workspaceNames([]models.Domain{*d}), h.ownerEmails([]models.Domain{*d})),
		VerificationToken: d.VerificationToken,
		Records:           domain.RequiredRecords(d),
	}
	if other := h.conflictingOwner(d); other != nil {
		detail.ConflictWorkspaceID = other.WorkspaceID
		detail.ConflictWorkspaceName = h.workspaceName(other.WorkspaceID)
	}
	return ok(c, detail)
}

// Verify runs the real DNS check against a tenant's domain, so an administrator
// can answer "is their DNS actually right?" without asking them to click it.
func (h *AdminDomainHandler) Verify(c *okapi.Context, req *AdminDomainIDRequest) error {
	d, err := h.repo.FindByID(uint(req.ID))
	if err != nil {
		return c.AbortNotFound("domain not found")
	}

	result, err := domain.Verify(d)
	if err != nil {
		return c.AbortInternalServerError("DNS verification failed")
	}

	var conflict *models.Domain
	if result.OwnershipVerified && !d.OwnershipVerified {
		if conflict = h.conflictingOwner(d); conflict != nil {
			result.OwnershipVerified = false
		}
	}

	d.OwnershipVerified = result.OwnershipVerified
	d.SPFVerified = result.SPFVerified
	d.DKIMVerified = result.DKIMVerified
	d.DMARCVerified = result.DMARCVerified

	if err := h.repo.Update(d); err != nil {
		return c.AbortInternalServerError("failed to record the verification result")
	}

	h.log(c, d, "admin.domain.verified", "Ran DNS verification for "+d.Domain, map[string]any{
		"ownership_verified": d.OwnershipVerified,
		"fully_verified":     d.IsFullyVerified(),
	})

	payload := okapi.M{
		"domain":         d,
		"verification":   result,
		"fully_verified": d.IsFullyVerified(),
	}
	if conflict != nil {
		payload["conflict_workspace_id"] = conflict.WorkspaceID
		payload["conflict_workspace_name"] = h.workspaceName(conflict.WorkspaceID)
	}
	return ok(c, payload)
}

func (h *AdminDomainHandler) SetVerification(c *okapi.Context, req *AdminSetDomainVerificationRequest) error {
	d, err := h.repo.FindByID(uint(req.ID))
	if err != nil {
		return c.AbortNotFound("domain not found")
	}

	reason := strings.TrimSpace(req.Body.Reason)
	if req.Body.OwnershipVerified && reason == "" {
		return c.AbortBadRequest("a reason is required when verifying a domain without a DNS check")
	}

	if req.Body.OwnershipVerified {
		// At most one workspace may hold a given name verified; the database
		// enforces it with a partial unique index. Catching it here turns what
		// would be an opaque 500 into an answer naming the workspace at fault.
		if other := h.conflictingOwner(d); other != nil {
			return c.AbortConflict(
				"another workspace has already verified " + d.Domain +
					"; unverify it there before granting it here")
		}
	}

	if d.OwnershipVerified == req.Body.OwnershipVerified {
		return ok(c, d)
	}

	d.OwnershipVerified = req.Body.OwnershipVerified
	if err := h.repo.Update(d); err != nil {
		return c.AbortInternalServerError("failed to update the domain")
	}

	action, message := "admin.domain.unverified", "Ownership revoked for "+d.Domain
	if req.Body.OwnershipVerified {
		action, message = "admin.domain.force_verified", "Ownership granted for "+d.Domain+" without a DNS check"
	}
	h.log(c, d, action, message, map[string]any{"reason": reason})

	return ok(c, d)
}

// conflictingOwner returns the domain row that already holds this name verified,
// when it is a different row. Nil means the name is free to verify.
func (h *AdminDomainHandler) conflictingOwner(d *models.Domain) *models.Domain {
	other, err := h.repo.FindVerifiedByName(d.Domain)
	if err != nil || other == nil || other.ID == d.ID {
		return nil
	}
	return other
}

func (h *AdminDomainHandler) row(d *models.Domain, workspaces map[uint]string, owners map[uint]string) AdminDomainRow {
	row := AdminDomainRow{
		ID:                d.ID,
		Domain:            d.Domain,
		WorkspaceID:       d.WorkspaceID,
		OwnerID:           d.UserID,
		OwnerEmail:        owners[d.UserID],
		OwnershipVerified: d.OwnershipVerified,
		SPFVerified:       d.SPFVerified,
		DKIMVerified:      d.DKIMVerified,
		DMARCVerified:     d.DMARCVerified,
		FullyVerified:     d.IsFullyVerified(),
		CreatedAt:         d.CreatedAt,
	}
	if d.WorkspaceID != nil {
		row.WorkspaceName = workspaces[*d.WorkspaceID]
	}
	return row
}

// workspaceNames resolves the names for a page of domains in one query rather
// than one per row.
func (h *AdminDomainHandler) workspaceNames(domains []models.Domain) map[uint]string {
	ids := make([]uint, 0, len(domains))
	for i := range domains {
		if domains[i].WorkspaceID != nil {
			ids = append(ids, *domains[i].WorkspaceID)
		}
	}
	out := map[uint]string{}
	if len(ids) == 0 {
		return out
	}
	var rows []models.Workspace
	if err := h.db.Select("id", "display_name").Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return out
	}
	for i := range rows {
		out[rows[i].ID] = rows[i].DisplayName
	}
	return out
}

func (h *AdminDomainHandler) workspaceName(id *uint) string {
	if id == nil {
		return ""
	}
	var ws models.Workspace
	if err := h.db.Select("id", "display_name").First(&ws, *id).Error; err != nil {
		return ""
	}
	return ws.DisplayName
}

func (h *AdminDomainHandler) ownerEmails(domains []models.Domain) map[uint]string {
	ids := make([]uint, 0, len(domains))
	for i := range domains {
		ids = append(ids, domains[i].UserID)
	}
	out := map[uint]string{}
	if len(ids) == 0 {
		return out
	}
	var rows []models.User
	if err := h.db.Select("id", "email").Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return out
	}
	for i := range rows {
		out[rows[i].ID] = rows[i].Email
	}
	return out
}

func (h *AdminDomainHandler) log(c *okapi.Context, d *models.Domain, action, message string, meta map[string]any) {
	if h.audit == nil {
		return
	}
	if meta == nil {
		meta = map[string]any{}
	}
	meta["domain_id"] = d.ID
	meta["domain"] = d.Domain
	if d.WorkspaceID != nil {
		h.audit.LogCtxScoped(c, *d.WorkspaceID, action, message, meta)
		return
	}
	h.audit.LogCtx(c, action, message, meta)
}
