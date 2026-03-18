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
	"net/http"
	"time"

	"github.com/jkaninda/okapi"
	"github.com/jkaninda/posta/internal/dto"
	"github.com/jkaninda/posta/internal/models"
	"github.com/jkaninda/posta/internal/services/audit"
	"github.com/jkaninda/posta/internal/services/email"
	"github.com/jkaninda/posta/internal/storage/repositories"
)

// ServerHandler handles admin management of shared SMTP servers.
type ServerHandler struct {
	repo   *repositories.ServerRepository
	sender *email.SMTPSender
	audit  *audit.Logger
}

func NewServerHandler(repo *repositories.ServerRepository, audit *audit.Logger) *ServerHandler {
	return &ServerHandler{repo: repo, sender: email.NewSMTPSender(), audit: audit}
}

// --- Request types ---

type CreateServerRequest struct {
	Body struct {
		Name           string   `json:"name" required:"true"`
		Host           string   `json:"host" required:"true"`
		Port           int      `json:"port" required:"true"`
		Username       string   `json:"username"`
		Password       string   `json:"password"`
		Encryption     string   `json:"encryption"`
		MaxRetries     int      `json:"max_retries"`
		AllowedDomains []string `json:"allowed_domains"`
		SecurityMode   string   `json:"security_mode" enum:"permissive,strict"`
	} `json:"body"`
}

type UpdateServerRequest struct {
	ID   int `param:"id"`
	Body struct {
		Name           string   `json:"name"`
		Host           string   `json:"host"`
		Port           int      `json:"port"`
		Username       string   `json:"username"`
		Password       string   `json:"password"`
		Encryption     string   `json:"encryption"`
		MaxRetries     *int     `json:"max_retries"`
		Status         string   `json:"status"`
		AllowedDomains []string `json:"allowed_domains"`
		SecurityMode   string   `json:"security_mode" enum:"permissive,strict"`
	} `json:"body"`
}

type DeleteServerRequest struct {
	ID int `param:"id"`
}

type ServerIDRequest struct {
	ID int `param:"id"`
}

// --- Handlers ---

func (h *ServerHandler) Create(c *okapi.Context, req *CreateServerRequest) error {
	userID := uint(c.GetInt("user_id"))

	encryption := req.Body.Encryption
	if encryption == "" {
		encryption = models.EncryptionNone
	}

	securityMode := req.Body.SecurityMode
	if securityMode != models.ServerSecurityModeStrict {
		securityMode = models.ServerSecurityModePermissive
	}

	server := &models.Server{
		Name:           req.Body.Name,
		Host:           req.Body.Host,
		Port:           req.Body.Port,
		Username:       req.Body.Username,
		Password:       req.Body.Password,
		Encryption:     encryption,
		MaxRetries:     req.Body.MaxRetries,
		AllowedDomains: req.Body.AllowedDomains,
		SecurityMode:   securityMode,
	}

	// Validate SMTP connection before saving
	now := time.Now()
	server.ValidatedAt = &now
	smtpTest := server.ToSMTPServer()
	smtpTest.Password = server.Password // ensure raw password is used for test
	if err := h.sender.TestConnection(smtpTest); err != nil {
		server.Status = models.SMTPStatusInvalid
		server.ValidationError = err.Error()
	} else {
		server.Status = models.SMTPStatusEnabled
		server.ValidationError = ""
	}

	if err := h.repo.Create(server); err != nil {
		return c.AbortInternalServerError("failed to create server", err)
	}

	h.audit.Log(userID, c.GetString("email"), c.RealIP(), "server.created", "Shared SMTP server created: "+req.Body.Host, nil)

	return created(c, server)
}

func (h *ServerHandler) Get(c *okapi.Context, req *ServerIDRequest) error {
	server, err := h.repo.FindByID(uint(req.ID))
	if err != nil {
		return c.AbortNotFound("server not found")
	}
	return ok(c, server)
}

func (h *ServerHandler) List(c *okapi.Context, req *ListRequest) error {
	page, size, offset := normalizePageParams(req.Page, req.Size)

	servers, total, err := h.repo.FindAll(size, offset)
	if err != nil {
		return c.AbortInternalServerError("failed to list servers")
	}

	return paginated(c, servers, total, page, size)
}

func (h *ServerHandler) Update(c *okapi.Context, req *UpdateServerRequest) error {
	userID := uint(c.GetInt("user_id"))

	server, err := h.repo.FindByID(uint(req.ID))
	if err != nil {
		return c.AbortNotFound("server not found")
	}

	connectionChanged := false

	if req.Body.Name != "" {
		server.Name = req.Body.Name
	}
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
	if req.Body.AllowedDomains != nil {
		server.AllowedDomains = req.Body.AllowedDomains
	}
	switch req.Body.SecurityMode {
	case models.ServerSecurityModeStrict:
		server.SecurityMode = models.ServerSecurityModeStrict
	case models.ServerSecurityModePermissive:
		server.SecurityMode = models.ServerSecurityModePermissive
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
			if err := h.sender.TestConnection(server.ToSMTPServer()); err != nil {
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
		if err := h.sender.TestConnection(server.ToSMTPServer()); err != nil {
			server.Status = models.SMTPStatusInvalid
			server.ValidationError = err.Error()
		} else {
			server.Status = models.SMTPStatusEnabled
			server.ValidationError = ""
		}
	}

	if err := h.repo.Update(server); err != nil {
		return c.AbortInternalServerError("failed to update server")
	}

	h.audit.Log(userID, c.GetString("email"), c.RealIP(), "server.updated", "Shared SMTP server updated: "+server.Host, nil)

	return ok(c, server)
}

func (h *ServerHandler) Delete(c *okapi.Context, req *DeleteServerRequest) error {
	userID := uint(c.GetInt("user_id"))

	server, err := h.repo.FindByID(uint(req.ID))
	if err != nil {
		return c.AbortNotFound("server not found")
	}

	if err := h.repo.Delete(server.ID); err != nil {
		return c.AbortInternalServerError("failed to delete server")
	}

	h.audit.Log(userID, c.GetString("email"), c.RealIP(), "server.deleted", "Shared SMTP server deleted: "+server.Host, nil)

	return noContent(c)
}

func (h *ServerHandler) Enable(c *okapi.Context, req *ServerIDRequest) error {
	userID := uint(c.GetInt("user_id"))

	server, err := h.repo.FindByID(uint(req.ID))
	if err != nil {
		return c.AbortNotFound("server not found")
	}

	// Re-validate before enabling
	now := time.Now()
	if err := h.sender.TestConnection(server.ToSMTPServer()); err != nil {
		_ = h.repo.SetStatus(server.ID, models.SMTPStatusInvalid, err.Error())
		server.Status = models.SMTPStatusInvalid
		server.ValidationError = err.Error()
		server.ValidatedAt = &now
		return ok(c, server)
	}

	_ = h.repo.SetStatus(server.ID, models.SMTPStatusEnabled, "")
	h.audit.Log(userID, c.GetString("email"), c.RealIP(), "server.enabled", "Shared SMTP server enabled: "+server.Host, nil)

	server.Status = models.SMTPStatusEnabled
	server.ValidationError = ""
	server.ValidatedAt = &now
	return ok(c, server)
}

func (h *ServerHandler) Disable(c *okapi.Context, req *ServerIDRequest) error {
	userID := uint(c.GetInt("user_id"))

	server, err := h.repo.FindByID(uint(req.ID))
	if err != nil {
		return c.AbortNotFound("server not found")
	}

	if err := h.repo.SetStatus(server.ID, models.SMTPStatusDisabled, ""); err != nil {
		return c.AbortInternalServerError("failed to disable server")
	}

	h.audit.Log(userID, c.GetString("email"), c.RealIP(), "server.disabled", "Shared SMTP server disabled: "+server.Host, nil)

	server.Status = models.SMTPStatusDisabled
	server.ValidationError = ""
	return ok(c, server)
}

func (h *ServerHandler) Test(c *okapi.Context, req *ServerIDRequest) error {
	server, err := h.repo.FindByID(uint(req.ID))
	if err != nil {
		return c.AbortNotFound("server not found")
	}

	now := time.Now()
	if err := h.sender.TestConnection(server.ToSMTPServer()); err != nil {
		_ = h.repo.SetStatus(server.ID, models.SMTPStatusInvalid, err.Error())
		return c.JSON(http.StatusOK, dto.Response[okapi.M]{
			Success: false,
			Data: okapi.M{
				"message":      err.Error(),
				"status":       models.SMTPStatusInvalid,
				"validated_at": now,
			},
		})
	}

	// Only set to enabled if not manually disabled
	if server.Status != models.SMTPStatusDisabled {
		_ = h.repo.SetStatus(server.ID, models.SMTPStatusEnabled, "")
	} else {
		// Still clear validation error even if disabled
		_ = h.repo.SetStatus(server.ID, models.SMTPStatusDisabled, "")
	}

	return ok(c, okapi.M{
		"message":      "connection successful",
		"status":       server.Status,
		"validated_at": now,
	})
}
