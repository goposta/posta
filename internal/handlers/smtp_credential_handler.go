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
	"github.com/goposta/posta/internal/config"
	"github.com/goposta/posta/internal/services/audit"
	"github.com/goposta/posta/internal/services/smtprelay"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
)

type SMTPCredentialHandler struct {
	service *smtprelay.CredentialService
	repo    *repositories.SMTPCredentialRepository
	cfg     *config.Config
	audit   *audit.Logger
}

func NewSMTPCredentialHandler(service *smtprelay.CredentialService, repo *repositories.SMTPCredentialRepository, cfg *config.Config, audit *audit.Logger) *SMTPCredentialHandler {
	return &SMTPCredentialHandler{
		service: service,
		repo:    repo,
		cfg:     cfg,
		audit:   audit,
	}
}

type CreateSMTPCredentialRequest struct {
	Body struct {
		Name       string   `json:"name" required:"true"`
		AllowedIPs []string `json:"allowed_ips"`
	} `json:"body"`
}

type GetSMTPCredentialRequest struct {
	ID int `param:"id" doc:"SMTP credential ID"`
}

type RevokeSMTPCredentialRequest struct {
	ID int `param:"id"`
}

type DeleteSMTPCredentialRequest struct {
	ID int `param:"id"`
}

func (h *SMTPCredentialHandler) Create(c *okapi.Context, req *CreateSMTPCredentialRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("Insufficient workspace permissions", err)
	}
	scope := getScope(c)
	if scope.WorkspaceID == nil {
		return c.AbortBadRequest("a workspace is required to create SMTP credentials")
	}

	username, password, cred, err := h.service.GenerateCredential(scope.UserID, *scope.WorkspaceID, req.Body.Name, req.Body.AllowedIPs)
	if err != nil {
		return c.AbortInternalServerError("failed to create SMTP credential", err)
	}

	h.audit.LogCtx(c, "smtprelay.created", "SMTP relay credential created: "+cred.Name, nil)

	return created(c, okapi.M{
		"id":         cred.ID,
		"name":       cred.Name,
		"username":   username,
		"password":   password,
		"host":       h.cfg.SMTPRelayHostname,
		"port":       h.cfg.SMTPRelayPort,
		"created_at": cred.CreatedAt,
		"message":    "Save this password securely. It will not be shown again.",
	})
}

func (h *SMTPCredentialHandler) List(c *okapi.Context, req *ListRequest) error {
	page, size, offset := normalizePageParams(req.Page, req.Size)

	scope := getScope(c)
	if scope.WorkspaceID == nil {
		return c.AbortBadRequest("a workspace is required to list SMTP credentials")
	}

	creds, total, err := h.repo.FindByWorkspaceID(*scope.WorkspaceID, size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list SMTP credentials")
	}

	return paginated(c, creds, total, page, size)
}

func (h *SMTPCredentialHandler) Get(c *okapi.Context, req *GetSMTPCredentialRequest) error {
	cred, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, cred.UserID, &cred.WorkspaceID) {
		return c.AbortNotFound("SMTP credential not found")
	}
	return ok(c, cred)
}

func (h *SMTPCredentialHandler) Revoke(c *okapi.Context, req *RevokeSMTPCredentialRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("Insufficient workspace permissions", err)
	}
	cred, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, cred.UserID, &cred.WorkspaceID) {
		return c.AbortNotFound("SMTP credential not found")
	}

	if err := h.repo.Revoke(cred.ID); err != nil {
		return c.AbortInternalServerError("failed to revoke SMTP credential")
	}

	h.audit.LogCtx(c, "smtprelay.revoked", "SMTP relay credential revoked: "+cred.Name, nil)

	return ok(c, okapi.M{"message": "SMTP credential revoked"})
}

func (h *SMTPCredentialHandler) Delete(c *okapi.Context, req *DeleteSMTPCredentialRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("Insufficient workspace permissions", err)
	}
	cred, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, cred.UserID, &cred.WorkspaceID) {
		return c.AbortNotFound("SMTP credential not found")
	}

	if err := h.repo.Delete(cred.ID); err != nil {
		return c.AbortInternalServerError("failed to delete SMTP credential")
	}

	h.audit.LogCtx(c, "smtprelay.deleted", "SMTP relay credential deleted: "+cred.Name, nil)

	return ok(c, okapi.M{"message": "SMTP credential deleted"})
}
