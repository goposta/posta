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
	"fmt"
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/audit"
	"github.com/goposta/posta/internal/services/email"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
	"gorm.io/gorm"
)

type SMTPHandler struct {
	repo       *repositories.SMTPRepository
	domainRepo *repositories.DomainRepository
	sender     *email.SMTPSender
	audit      *audit.Logger
	quota      QuotaChecker
	db         *gorm.DB
}
type CreateSMTPRequest struct {
	Body struct {
		Host          string   `json:"host" required:"true"`
		Port          int      `json:"port" required:"true"`
		Username      string   `json:"username"`
		Password      string   `json:"password"`
		Encryption    string   `json:"encryption"`
		MaxRetries    int      `json:"max_retries"`
		AllowedEmails []string `json:"allowed_emails"`
	} `json:"body"`
}
type UpdateSMTPRequest struct {
	ID   int `param:"id"`
	Body struct {
		Host          string   `json:"host"`
		Port          int      `json:"port"`
		Username      string   `json:"username"`
		Password      string   `json:"password"`
		Encryption    string   `json:"encryption"`
		MaxRetries    *int     `json:"max_retries"`
		AllowedEmails []string `json:"allowed_emails"`
		Status        string   `json:"status"`
	} `json:"body"`
}
type GetSMTPRequest struct {
	ID int `param:"id"`
}
type DeleteSMTPRequest struct {
	ID int `param:"id"`
}
type TestSMTPRequest struct {
	ID int `param:"id"`
}

func NewSMTPHandler(repo *repositories.SMTPRepository, domainRepo *repositories.DomainRepository, audit *audit.Logger) *SMTPHandler {
	return &SMTPHandler{repo: repo, domainRepo: domainRepo, sender: email.NewSMTPSender(), audit: audit}
}

// validateAllowedEmails checks that each allowed email's domain belongs to the user's/workspace's domains.
// If no domains are configured, all emails are allowed.
func (h *SMTPHandler) validateAllowedEmails(scope repositories.ResourceScope, emails []string) error {
	if len(emails) == 0 {
		return nil
	}

	domains, _, err := h.domainRepo.FindByScope(scope, 1000, 0)
	if err != nil {
		return fmt.Errorf("failed to load domains")
	}

	// If user has no domains, skip validation
	if len(domains) == 0 {
		return nil
	}

	domainSet := make(map[string]bool, len(domains))
	for _, d := range domains {
		domainSet[strings.ToLower(d.Domain)] = true
	}

	for _, addr := range emails {
		parts := strings.SplitN(addr, "@", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid email address: %s", addr)
		}
		domain := strings.ToLower(parts[1])
		if !domainSet[domain] {
			return fmt.Errorf("domain %q is not in your verified domains", domain)
		}
	}
	return nil
}

// SetQuota sets the quota checker for plan-based resource limits.
func (h *SMTPHandler) SetQuota(q QuotaChecker, db *gorm.DB) {
	h.quota = q
	h.db = db
}

func (h *SMTPHandler) Create(c *okapi.Context, req *CreateSMTPRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("Insufficient workspace permissions", err)
	}
	scope := getScope(c)

	if h.quota != nil {
		if err := h.quota.CheckQuota(h.db, scope.UserID, scope.WorkspaceID, "smtp_servers"); err != nil {
			return c.AbortForbidden("SMTP server quota exceeded for this workspace", err)
		}
	}

	if err := h.validateAllowedEmails(scope, req.Body.AllowedEmails); err != nil {
		return c.AbortBadRequest(err.Error())
	}

	encryption := req.Body.Encryption
	if encryption == "" {
		encryption = models.EncryptionNone
	}

	server := &models.SMTPServer{
		UserID:        scope.UserID,
		WorkspaceID:   scope.WorkspaceID,
		Host:          req.Body.Host,
		Port:          req.Body.Port,
		Username:      req.Body.Username,
		Password:      req.Body.Password,
		Encryption:    encryption,
		MaxRetries:    req.Body.MaxRetries,
		AllowedEmails: req.Body.AllowedEmails,
	}

	// Validate SMTP connection before saving
	now := time.Now()
	server.ValidatedAt = &now
	if err := h.sender.TestConnection(server); err != nil {
		server.Status = models.SMTPStatusInvalid
		server.ValidationError = err.Error()
	} else {
		server.Status = models.SMTPStatusEnabled
		server.ValidationError = ""
	}

	if err := h.repo.Create(server); err != nil {
		return c.AbortInternalServerError("failed to create SMTP server", err)
	}

	h.audit.Log(scope.UserID, c.GetString("email"), c.RealIP(), "smtp.created", "SMTP server created: "+req.Body.Host, nil)

	return created(c, server)
}

func (h *SMTPHandler) Get(c *okapi.Context, req *GetSMTPRequest) error {
	server, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, server.UserID, server.WorkspaceID) {
		return c.AbortNotFound("SMTP server not found")
	}

	return ok(c, server)
}

func (h *SMTPHandler) Update(c *okapi.Context, req *UpdateSMTPRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("Insufficient workspace permissions", err)
	}
	server, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, server.UserID, server.WorkspaceID) {
		return c.AbortNotFound("SMTP server not found")
	}

	// Track whether connection-related fields changed
	connectionChanged := false

	if req.Body.Host != "" && req.Body.Host != server.Host {
		server.Host = req.Body.Host
		connectionChanged = true
	}
	if req.Body.Port != 0 && req.Body.Port != server.Port {
		server.Port = req.Body.Port
		connectionChanged = true
	}
	if req.Body.Username != "" && req.Body.Username != server.Username {
		server.Username = req.Body.Username
		connectionChanged = true
	}
	if req.Body.Password != "" {
		server.Password = req.Body.Password
		connectionChanged = true
	}
	if req.Body.Encryption != "" && req.Body.Encryption != server.Encryption {
		server.Encryption = req.Body.Encryption
		connectionChanged = true
	}
	if req.Body.MaxRetries != nil {
		server.MaxRetries = *req.Body.MaxRetries
	}
	if req.Body.AllowedEmails != nil {
		if err := h.validateAllowedEmails(repositories.ResourceScope{UserID: server.UserID, WorkspaceID: server.WorkspaceID}, req.Body.AllowedEmails); err != nil {
			return c.AbortBadRequest(err.Error())
		}
		server.AllowedEmails = req.Body.AllowedEmails
	}

	// Handle status changes
	if req.Body.Status != "" {
		switch req.Body.Status {
		case models.SMTPStatusDisabled:
			server.Status = models.SMTPStatusDisabled
			server.ValidationError = ""
		case models.SMTPStatusEnabled:
			// Re-validate before enabling
			now := time.Now()
			server.ValidatedAt = &now
			if err := h.sender.TestConnection(server); err != nil {
				server.Status = models.SMTPStatusInvalid
				server.ValidationError = err.Error()
			} else {
				server.Status = models.SMTPStatusEnabled
				server.ValidationError = ""
			}
		}
	} else if connectionChanged {
		// Connection fields changed — re-validate automatically
		now := time.Now()
		server.ValidatedAt = &now
		if err := h.sender.TestConnection(server); err != nil {
			server.Status = models.SMTPStatusInvalid
			server.ValidationError = err.Error()
		} else {
			server.Status = models.SMTPStatusEnabled
			server.ValidationError = ""
		}
	}

	if err := h.repo.Update(server); err != nil {
		return c.AbortInternalServerError("failed to update SMTP server")
	}

	h.audit.Log(server.UserID, c.GetString("email"), c.RealIP(), "smtp.updated", "SMTP server updated: "+server.Host, nil)

	return ok(c, server)
}

func (h *SMTPHandler) List(c *okapi.Context, req *ListRequest) error {
	page, size, offset := normalizePageParams(req.Page, req.Size)

	servers, total, err := h.repo.FindByScope(getScope(c), size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list SMTP servers")
	}

	return paginated(c, servers, total, page, size)
}

func (h *SMTPHandler) Delete(c *okapi.Context, req *DeleteSMTPRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("Insufficient workspace permissions", err)
	}
	server, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, server.UserID, server.WorkspaceID) {
		return c.AbortNotFound("SMTP server not found")
	}

	if err := h.repo.Delete(server.ID); err != nil {
		return c.AbortInternalServerError("failed to delete SMTP server")
	}

	h.audit.Log(server.UserID, c.GetString("email"), c.RealIP(), "smtp.deleted", "SMTP server deleted: "+server.Host, nil)

	return noContent(c)
}

func (h *SMTPHandler) Test(c *okapi.Context, req *TestSMTPRequest) error {
	if err := requireEdit(c); err != nil {
		return c.AbortForbidden("Insufficient workspace permissions", err)
	}
	server, err := h.repo.FindByID(uint(req.ID))
	if err != nil || !ownsResource(c, server.UserID, server.WorkspaceID) {
		return c.AbortNotFound("SMTP server not found")
	}

	now := time.Now()
	if err := h.sender.TestConnection(server); err != nil {
		// Update validation state on test failure
		_ = h.repo.SetStatus(server.ID, models.SMTPStatusInvalid, err.Error())
		return ok(c, okapi.M{
			"success":      false,
			"message":      err.Error(),
			"status":       models.SMTPStatusInvalid,
			"validated_at": now,
		})
	}

	// Update validation state on test success
	_ = h.repo.SetStatus(server.ID, models.SMTPStatusEnabled, "")
	return ok(c, okapi.M{
		"success":      true,
		"message":      "connection successful",
		"status":       models.SMTPStatusEnabled,
		"validated_at": now,
	})
}
