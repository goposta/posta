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
	"fmt"
	"slices"
	"strings"

	"github.com/goposta/posta/internal/enterprise"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/audit"
	"github.com/goposta/posta/internal/services/crypto"
	"github.com/goposta/posta/internal/services/directory"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
)

// The directory is reached only through the enterprise seam; reads need a license, changes an unexpired one.
type LDAPAdminHandler struct {
	repo       *repositories.LDAPRepository
	workspaces *repositories.WorkspaceRepository
	ee         enterprise.EE
	audit      *audit.Logger
}

func NewLDAPAdminHandler(repo *repositories.LDAPRepository, workspaces *repositories.WorkspaceRepository, ee enterprise.EE, auditLogger *audit.Logger) *LDAPAdminHandler {
	return &LDAPAdminHandler{repo: repo, workspaces: workspaces, ee: ee, audit: auditLogger}
}

type LDAPConfigBody struct {
	DisplayName     string `json:"display_name" required:"true" doc:"Shown to admins, such as Corporate AD"`
	Host            string `json:"host" required:"true"`
	Port            int    `json:"port" doc:"Default 389, or 636 for ldaps"`
	TLSMode         string `json:"tls_mode" enum:"none,starttls,ldaps" doc:"Default starttls"`
	CACertPEM       string `json:"ca_cert_pem" doc:"A private CA to trust, in PEM"`
	InsecureSkipTLS bool   `json:"insecure_skip_tls" doc:"Skip certificate checks. Testing only."`
	TimeoutSeconds  int    `json:"timeout_seconds" doc:"Default 10"`
	BindDN          string `json:"bind_dn" doc:"Service account that searches for users; empty binds anonymously"`
	BindPassword    string `json:"bind_password" doc:"Service account password. Empty on update keeps the stored one."`
	UserBaseDN      string `json:"user_base_dn" required:"true"`
	UserFilter      string `json:"user_filter" doc:"%s is the sign-in email. Default (mail=%s)."`
	AttrEmail       string `json:"attr_email" doc:"Default mail"`
	AttrName        string `json:"attr_name" doc:"Default displayName"`
	GroupBaseDN     string `json:"group_base_dn"`
	GroupFilter     string `json:"group_filter" doc:"%s is the user's DN, such as (member=%s)"`
	MemberAttr      string `json:"member_attr" doc:"Default memberOf"`
	NestedGroups    bool   `json:"nested_groups" doc:"Active Directory: include nested groups"`
	Enabled         *bool  `json:"enabled" doc:"Default true"`
}

type CreateLDAPConfigRequest struct {
	Body LDAPConfigBody `json:"body"`
}

type UpdateLDAPConfigRequest struct {
	ID   int            `param:"id"`
	Body LDAPConfigBody `json:"body"`
}

type LDAPConfigIDRequest struct {
	ID int `param:"id"`
}

type CreateLDAPMappingRequest struct {
	ID   int `param:"id"`
	Body struct {
		GroupDN       string               `json:"group_dn" required:"true" doc:"A group DN, or a bare CN such as posta-admins"`
		PlatformAdmin bool                 `json:"platform_admin" doc:"Members become platform admins"`
		WorkspaceID   *uint                `json:"workspace_id" doc:"Members join this workspace"`
		WorkspaceRole models.WorkspaceRole `json:"workspace_role" enum:"admin,editor,viewer" doc:"Their role there"`
	} `json:"body"`
}

type LDAPTestResponse = enterprise.LDAPTestResult

type DeleteLDAPMappingRequest struct {
	ID        int `param:"id"`
	MappingID int `param:"mapping_id"`
}

func (h *LDAPAdminHandler) List(c *okapi.Context) error {
	cfgs, err := h.repo.FindAll()
	if err != nil {
		return c.AbortInternalServerError("failed to list directories")
	}
	if cfgs == nil {
		cfgs = []models.LDAPConfig{}
	}
	return ok(c, cfgs)
}

func (h *LDAPAdminHandler) Create(c *okapi.Context, req *CreateLDAPConfigRequest) error {
	cfg := &models.LDAPConfig{Enabled: true}
	if err := h.apply(c, cfg, req.Body); err != nil {
		return err
	}
	cfg.Name = h.uniqueName(cfg.DisplayName)
	if err := h.repo.Create(cfg); err != nil {
		return c.AbortInternalServerError("failed to create the directory")
	}
	h.log(c, "ldap.created", "LDAP directory "+cfg.DisplayName+" added", cfg)
	saved, _ := h.repo.FindByID(cfg.ID)
	return created(c, saved)
}

func (h *LDAPAdminHandler) Update(c *okapi.Context, req *UpdateLDAPConfigRequest) error {
	cfg, err := h.repo.FindByID(uint(req.ID))
	if err != nil {
		return c.AbortNotFound("directory not found")
	}
	if err := h.apply(c, cfg, req.Body); err != nil {
		return err
	}
	if err := h.repo.Update(cfg); err != nil {
		return c.AbortInternalServerError("failed to update the directory")
	}
	h.log(c, "ldap.updated", "LDAP directory "+cfg.DisplayName+" updated", cfg)
	saved, _ := h.repo.FindByID(cfg.ID)
	return ok(c, saved)
}

func (h *LDAPAdminHandler) Delete(c *okapi.Context, req *LDAPConfigIDRequest) error {
	cfg, err := h.repo.FindByID(uint(req.ID))
	if err != nil {
		return c.AbortNotFound("directory not found")
	}
	if err := h.repo.Delete(cfg.ID); err != nil {
		return c.AbortInternalServerError("failed to delete the directory")
	}
	h.log(c, "ldap.deleted", "LDAP directory "+cfg.DisplayName+" deleted", cfg)
	return noContent(c)
}

// Test binds as the service account and reads the user base DN.
func (h *LDAPAdminHandler) Test(c *okapi.Context, req *LDAPConfigIDRequest) error {
	a := h.ee.LDAP()
	if a == nil {
		return licenseAbort(c, h.ee.Require())
	}
	cfg, err := h.repo.FindByID(uint(req.ID))
	if err != nil {
		return c.AbortNotFound("directory not found")
	}
	return ok(c, a.TestConnection(c.Request().Context(), cfg))
}

func (h *LDAPAdminHandler) CreateMapping(c *okapi.Context, req *CreateLDAPMappingRequest) error {
	cfg, err := h.repo.FindByID(uint(req.ID))
	if err != nil {
		return c.AbortNotFound("directory not found")
	}
	b := req.Body
	group := strings.TrimSpace(b.GroupDN)
	switch {
	case group == "":
		return c.AbortBadRequest("group_dn is required")
	case !b.PlatformAdmin && b.WorkspaceID == nil:
		return c.AbortBadRequest("a mapping grants the platform admin role, a workspace, or both")
	case b.WorkspaceID != nil && !slices.Contains(directory.MappableRoles, b.WorkspaceRole):
		return c.AbortBadRequest("workspace_role must be admin, editor or viewer")
	case b.WorkspaceID == nil && b.WorkspaceRole != "":
		return c.AbortBadRequest("workspace_role needs a workspace_id")
	}
	if b.WorkspaceID != nil {
		if _, err := h.workspaces.FindByID(*b.WorkspaceID); err != nil {
			return c.AbortBadRequest(fmt.Sprintf("workspace %d does not exist", *b.WorkspaceID))
		}
	}
	m := &models.LDAPGroupMapping{
		LDAPConfigID: cfg.ID, GroupDN: group,
		PlatformAdmin: b.PlatformAdmin, WorkspaceID: b.WorkspaceID, WorkspaceRole: b.WorkspaceRole,
	}
	if err := h.repo.CreateMapping(m); err != nil {
		return c.AbortInternalServerError("failed to add the mapping")
	}
	h.log(c, "ldap.mapping_added", "Group "+group+" mapped on LDAP directory "+cfg.DisplayName, cfg)
	return created(c, m)
}

func (h *LDAPAdminHandler) DeleteMapping(c *okapi.Context, req *DeleteLDAPMappingRequest) error {
	cfg, err := h.repo.FindByID(uint(req.ID))
	if err != nil {
		return c.AbortNotFound("directory not found")
	}
	deleted, err := h.repo.DeleteMapping(cfg.ID, uint(req.MappingID))
	if err != nil {
		return c.AbortInternalServerError("failed to delete the mapping")
	}
	if !deleted {
		return c.AbortNotFound("mapping not found")
	}
	h.log(c, "ldap.mapping_removed", "Group mapping removed from LDAP directory "+cfg.DisplayName, cfg)
	return noContent(c)
}

func (h *LDAPAdminHandler) apply(c *okapi.Context, cfg *models.LDAPConfig, b LDAPConfigBody) error {
	cfg.DisplayName = strings.TrimSpace(b.DisplayName)
	cfg.Host = strings.TrimSpace(b.Host)
	cfg.UserBaseDN = strings.TrimSpace(b.UserBaseDN)
	tlsMode := b.TLSMode
	if tlsMode == "" {
		tlsMode = models.LDAPTLSStartTLS
	}
	switch {
	case cfg.DisplayName == "":
		return c.AbortBadRequest("display_name is required")
	case cfg.Host == "":
		return c.AbortBadRequest("host is required")
	case cfg.UserBaseDN == "":
		return c.AbortBadRequest("user_base_dn is required")
	case tlsMode != models.LDAPTLSNone && tlsMode != models.LDAPTLSStartTLS && tlsMode != models.LDAPTLSLDAPS:
		return c.AbortBadRequest("tls_mode must be none, starttls or ldaps")
	case b.Port < 0 || b.Port > 65535:
		return c.AbortBadRequest("port must be between 1 and 65535")
	case b.TimeoutSeconds < 0 || b.TimeoutSeconds > 120:
		return c.AbortBadRequest("timeout_seconds must be between 1 and 120")
	case b.UserFilter != "" && !strings.Contains(b.UserFilter, "%s"):
		return c.AbortBadRequest("user_filter must contain %s, the sign-in email")
	}
	cfg.TLSMode = tlsMode
	cfg.Port = b.Port
	if cfg.Port == 0 {
		cfg.Port = 389
		if tlsMode == models.LDAPTLSLDAPS {
			cfg.Port = 636
		}
	}
	cfg.TimeoutSeconds = b.TimeoutSeconds
	if cfg.TimeoutSeconds == 0 {
		cfg.TimeoutSeconds = 10
	}
	cfg.CACertPEM = strings.TrimSpace(b.CACertPEM)
	cfg.InsecureSkipTLS = b.InsecureSkipTLS
	cfg.BindDN = strings.TrimSpace(b.BindDN)
	cfg.UserFilter = strings.TrimSpace(b.UserFilter)
	cfg.AttrEmail = strings.TrimSpace(b.AttrEmail)
	cfg.AttrName = strings.TrimSpace(b.AttrName)
	cfg.GroupBaseDN = strings.TrimSpace(b.GroupBaseDN)
	cfg.GroupFilter = strings.TrimSpace(b.GroupFilter)
	cfg.MemberAttr = strings.TrimSpace(b.MemberAttr)
	cfg.NestedGroups = b.NestedGroups
	if b.Enabled != nil {
		cfg.Enabled = *b.Enabled
	}
	if cfg.BindDN == "" {
		cfg.BindPasswordEnc = ""
	} else if b.BindPassword != "" {
		enc, err := crypto.Encrypt(b.BindPassword)
		if err != nil {
			return c.AbortInternalServerError("failed to encrypt the bind password")
		}
		cfg.BindPasswordEnc = enc
	}
	return nil
}

func (h *LDAPAdminHandler) uniqueName(displayName string) string {
	base := models.ToIdentifier(displayName)
	if base == "" {
		base = "directory"
	}
	name := base
	for i := 2; h.repo.NameTaken(name, 0); i++ {
		name = fmt.Sprintf("%s-%d", base, i)
	}
	return name
}

func (h *LDAPAdminHandler) log(c *okapi.Context, action, message string, cfg *models.LDAPConfig) {
	if h.audit == nil {
		return
	}
	h.audit.LogCtx(c, action, message, map[string]any{"ldap_config_id": cfg.ID, "host": cfg.Host})
}
