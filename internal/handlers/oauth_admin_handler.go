package handlers

import (
	"strings"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/okapi"
)

type OAuthAdminHandler struct {
	providerRepo *repositories.OAuthProviderRepository
	ssoRepo      *repositories.WorkspaceSSORepository
}

func NewOAuthAdminHandler(providerRepo *repositories.OAuthProviderRepository, ssoRepo *repositories.WorkspaceSSORepository) *OAuthAdminHandler {
	return &OAuthAdminHandler{providerRepo: providerRepo, ssoRepo: ssoRepo}
}

type CreateOAuthProviderRequest struct {
	Body struct {
		Name           string `json:"name" required:"true"`
		Slug           string `json:"slug" required:"true"`
		Type           string `json:"type" required:"true"`
		ClientID       string `json:"client_id" required:"true"`
		ClientSecret   string `json:"client_secret" required:"true"`
		Issuer         string `json:"issuer"`
		AuthURL        string `json:"auth_url"`
		TokenURL       string `json:"token_url"`
		UserInfoURL    string `json:"userinfo_url"`
		Scopes         string `json:"scopes"`
		AutoRegister   *bool  `json:"auto_register"`
		Hidden         *bool  `json:"hidden"`
		AllowedDomains string `json:"allowed_domains"`
	} `json:"body"`
}

type UpdateOAuthProviderRequest struct {
	ID   int `param:"id"`
	Body struct {
		Name           string `json:"name"`
		ClientID       string `json:"client_id"`
		ClientSecret   string `json:"client_secret"`
		Issuer         string `json:"issuer"`
		AuthURL        string `json:"auth_url"`
		TokenURL       string `json:"token_url"`
		UserInfoURL    string `json:"userinfo_url"`
		Scopes         string `json:"scopes"`
		Enabled        *bool  `json:"enabled"`
		Hidden         *bool  `json:"hidden"`
		AutoRegister   *bool  `json:"auto_register"`
		AllowedDomains string `json:"allowed_domains"`
	} `json:"body"`
}

type DeleteOAuthProviderRequest struct {
	ID int `param:"id"`
}

type OAuthProviderResponse struct {
	ID             uint   `json:"id"`
	Name           string `json:"name"`
	Slug           string `json:"slug"`
	Type           string `json:"type"`
	Issuer         string `json:"issuer"`
	Scopes         string `json:"scopes"`
	Enabled        bool   `json:"enabled"`
	Hidden         bool   `json:"hidden"`
	AutoRegister   bool   `json:"auto_register"`
	AllowedDomains string `json:"allowed_domains"`
	CreatedAt      string `json:"created_at"`
}

type SetWorkspaceSSORequest struct {
	Body struct {
		ProviderID     uint   `json:"provider_id" required:"true"`
		EnforceSSO     bool   `json:"enforce_sso"`
		AutoProvision  bool   `json:"auto_provision"`
		AllowedDomains string `json:"allowed_domains"`
	} `json:"body"`
}

type WorkspaceSSOResponse struct {
	ProviderID     uint   `json:"provider_id"`
	ProviderName   string `json:"provider_name"`
	EnforceSSO     bool   `json:"enforce_sso"`
	AutoProvision  bool   `json:"auto_provision"`
	AllowedDomains string `json:"allowed_domains"`
}

func (h *OAuthAdminHandler) CreateProvider(c *okapi.Context, req *CreateOAuthProviderRequest) error {
	slug := strings.ToLower(strings.TrimSpace(req.Body.Slug))
	providerType := models.OAuthProviderType(req.Body.Type)
	if providerType != models.OAuthProviderGoogle && providerType != models.OAuthProviderOIDC {
		return c.AbortBadRequest("type must be 'google' or 'oidc'")
	}

	provider := &models.OAuthProvider{
		Name:           strings.TrimSpace(req.Body.Name),
		Slug:           slug,
		Type:           providerType,
		ClientID:       req.Body.ClientID,
		ClientSecret:   req.Body.ClientSecret,
		Issuer:         req.Body.Issuer,
		AuthURL:        req.Body.AuthURL,
		TokenURL:       req.Body.TokenURL,
		UserInfoURL:    req.Body.UserInfoURL,
		Scopes:         req.Body.Scopes,
		Enabled:        true,
		AutoRegister:   true,
		AllowedDomains: req.Body.AllowedDomains,
	}
	if req.Body.AutoRegister != nil {
		provider.AutoRegister = *req.Body.AutoRegister
	}
	if req.Body.Hidden != nil {
		provider.Hidden = *req.Body.Hidden
	}
	if provider.Scopes == "" {
		provider.Scopes = "openid email profile"
	}

	if err := h.providerRepo.Create(provider); err != nil {
		return c.AbortConflict("provider slug already exists")
	}

	return created(c, toProviderResponse(provider))
}

func (h *OAuthAdminHandler) ListProviders(c *okapi.Context) error {
	providers, err := h.providerRepo.FindAll()
	if err != nil {
		return c.AbortInternalServerError("failed to list providers")
	}

	var result []OAuthProviderResponse
	for _, p := range providers {
		result = append(result, toProviderResponse(&p))
	}
	return ok(c, result)
}

func (h *OAuthAdminHandler) UpdateProvider(c *okapi.Context, req *UpdateOAuthProviderRequest) error {
	provider, err := h.providerRepo.FindByID(uint(req.ID))
	if err != nil {
		return c.AbortNotFound("provider not found")
	}

	if req.Body.Name != "" {
		provider.Name = req.Body.Name
	}
	if req.Body.ClientID != "" {
		provider.ClientID = req.Body.ClientID
	}
	if req.Body.ClientSecret != "" {
		provider.ClientSecret = req.Body.ClientSecret
	}
	if req.Body.Issuer != "" {
		provider.Issuer = req.Body.Issuer
	}
	if req.Body.AuthURL != "" {
		provider.AuthURL = req.Body.AuthURL
	}
	if req.Body.TokenURL != "" {
		provider.TokenURL = req.Body.TokenURL
	}
	if req.Body.UserInfoURL != "" {
		provider.UserInfoURL = req.Body.UserInfoURL
	}
	if req.Body.Scopes != "" {
		provider.Scopes = req.Body.Scopes
	}
	if req.Body.AllowedDomains != "" {
		provider.AllowedDomains = req.Body.AllowedDomains
	}
	if req.Body.Enabled != nil {
		provider.Enabled = *req.Body.Enabled
	}
	if req.Body.Hidden != nil {
		provider.Hidden = *req.Body.Hidden
	}
	if req.Body.AutoRegister != nil {
		provider.AutoRegister = *req.Body.AutoRegister
	}
	provider.UpdatedAt = time.Now()

	if err := h.providerRepo.Update(provider); err != nil {
		return c.AbortInternalServerError("failed to update provider")
	}

	return ok(c, toProviderResponse(provider))
}

func (h *OAuthAdminHandler) DeleteProvider(c *okapi.Context, req *DeleteOAuthProviderRequest) error {
	if err := h.providerRepo.Delete(uint(req.ID)); err != nil {
		return c.AbortNotFound("provider not found")
	}
	return noContent(c)
}

func (h *OAuthAdminHandler) GetWorkspaceSSO(c *okapi.Context) error {
	wsID := c.GetInt("workspace_id")
	if wsID == 0 {
		return c.AbortBadRequest("workspace context required")
	}

	config, err := h.ssoRepo.FindByWorkspaceID(uint(wsID))
	if err != nil {
		return ok(c, (*WorkspaceSSOResponse)(nil))
	}

	return ok(c, WorkspaceSSOResponse{
		ProviderID:     config.ProviderID,
		ProviderName:   config.Provider.Name,
		EnforceSSO:     config.EnforceSSO,
		AutoProvision:  config.AutoProvision,
		AllowedDomains: config.AllowedDomains,
	})
}

func (h *OAuthAdminHandler) SetWorkspaceSSO(c *okapi.Context, req *SetWorkspaceSSORequest) error {
	wsID := c.GetInt("workspace_id")
	if wsID == 0 {
		return c.AbortBadRequest("workspace context required")
	}

	config := &models.WorkspaceSSOConfig{
		WorkspaceID:    uint(wsID),
		ProviderID:     req.Body.ProviderID,
		EnforceSSO:     req.Body.EnforceSSO,
		AutoProvision:  req.Body.AutoProvision,
		AllowedDomains: req.Body.AllowedDomains,
	}

	if err := h.ssoRepo.Upsert(config); err != nil {
		return c.AbortInternalServerError("failed to save SSO config")
	}

	return ok(c, okapi.M{"message": "SSO configuration saved"})
}

func (h *OAuthAdminHandler) DeleteWorkspaceSSO(c *okapi.Context) error {
	wsID := c.GetInt("workspace_id")
	if wsID == 0 {
		return c.AbortBadRequest("workspace context required")
	}

	if err := h.ssoRepo.Delete(uint(wsID)); err != nil {
		return c.AbortInternalServerError("failed to delete SSO config")
	}

	return ok(c, okapi.M{"message": "SSO configuration removed"})
}

func toProviderResponse(p *models.OAuthProvider) OAuthProviderResponse {
	return OAuthProviderResponse{
		ID:             p.ID,
		Name:           p.Name,
		Slug:           p.Slug,
		Type:           string(p.Type),
		Issuer:         p.Issuer,
		Scopes:         p.Scopes,
		Enabled:        p.Enabled,
		Hidden:         p.Hidden,
		AutoRegister:   p.AutoRegister,
		AllowedDomains: p.AllowedDomains,
		CreatedAt:      p.CreatedAt.Format(time.RFC3339),
	}
}
