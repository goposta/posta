package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/auth"
	"github.com/goposta/posta/internal/services/eventbus"
	"github.com/goposta/posta/internal/services/seeder"
	"github.com/goposta/posta/internal/storage/repositories"
	"github.com/jkaninda/logger"
	"github.com/jkaninda/okapi"
	"github.com/redis/go-redis/v9"
)

type OAuthHandler struct {
	oauthService *auth.OAuthService
	providerRepo *repositories.OAuthProviderRepository
	accountRepo  *repositories.OAuthAccountRepository
	userRepo     *repositories.UserRepository
	sessionRepo  *repositories.SessionRepository
	jwtSecret    []byte
	bus          *eventbus.EventBus
	seeder       *seeder.Seeder
	redisClient  *redis.Client
	callbackBase string // e.g. "http://localhost:9000"
	appWebURL    string // e.g. "http://localhost:9000"
}

func NewOAuthHandler(
	oauthService *auth.OAuthService,
	providerRepo *repositories.OAuthProviderRepository,
	accountRepo *repositories.OAuthAccountRepository,
	userRepo *repositories.UserRepository,
	sessionRepo *repositories.SessionRepository,
	jwtSecret string,
	seeder *seeder.Seeder,
	bus *eventbus.EventBus,
	redisClient *redis.Client,
	callbackBase string,
	appWebURL string,
) *OAuthHandler {
	return &OAuthHandler{
		oauthService: oauthService,
		providerRepo: providerRepo,
		accountRepo:  accountRepo,
		userRepo:     userRepo,
		sessionRepo:  sessionRepo,
		jwtSecret:    []byte(jwtSecret),
		seeder:       seeder,
		bus:          bus,
		redisClient:  redisClient,
		callbackBase: callbackBase,
		appWebURL:    appWebURL,
	}
}

type OAuthProviderInfo struct {
	ID   uint   `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type OAuthLinkedAccount struct {
	ID           uint   `json:"id"`
	ProviderID   uint   `json:"provider_id"`
	ProviderName string `json:"provider_name"`
	ProviderType string `json:"provider_type"`
	Email        string `json:"email"`
	CreatedAt    string `json:"created_at"`
}

type UnlinkOAuthRequest struct {
	ProviderID int `param:"provider_id"`
}

type DiscoverSSORequest struct {
	Body struct {
		Email string `json:"email" required:"true"`
	} `json:"body"`
}

type DiscoverSSOResponse struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// ListProviders returns enabled OAuth providers for the login page
func (h *OAuthHandler) ListProviders(c *okapi.Context) error {
	providers, err := h.providerRepo.FindEnabled()
	if err != nil {
		return ok(c, okapi.M{"providers": []OAuthProviderInfo{}, "sso_available": false})
	}

	var result []OAuthProviderInfo
	for _, p := range providers {
		result = append(result, OAuthProviderInfo{
			ID:   p.ID,
			Slug: p.Slug,
			Name: p.Name,
			Type: string(p.Type),
		})
	}

	ssoAvailable, _ := h.providerRepo.HasEnabledHidden()

	return ok(c, okapi.M{"providers": result, "sso_available": ssoAvailable})
}

// DiscoverSSO looks up an enabled OAuth provider whose AllowedDomains matches the
// email's domain.
func (h *OAuthHandler) DiscoverSSO(c *okapi.Context, req *DiscoverSSORequest) error {
	email := strings.TrimSpace(req.Body.Email)
	at := strings.LastIndex(email, "@")
	if at < 0 || at == len(email)-1 {
		return c.AbortBadRequest("invalid email")
	}
	domain := email[at+1:]

	provider, err := h.providerRepo.FindEnabledByDomain(domain)
	if err != nil {
		return c.AbortNotFound("no SSO provider for this email")
	}

	return ok(c, DiscoverSSOResponse{
		Slug: provider.Slug,
		Name: provider.Name,
		Type: string(provider.Type),
	})
}

// Authorize redirects the user to the OAuth provider's authorization page.
func (h *OAuthHandler) Authorize(c *okapi.Context) error {
	providerSlug := c.Param("provider")
	provider, err := h.providerRepo.FindBySlug(providerSlug)
	if err != nil {
		return c.AbortNotFound("OAuth provider not found")
	}

	// Generate state for CSRF protection
	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		return c.AbortInternalServerError("failed to generate state")
	}
	state := hex.EncodeToString(stateBytes)

	// Store state in Redis with 10-minute TTL
	stateData, _ := json.Marshal(map[string]string{
		"provider_slug": provider.Slug,
		"created_at":    time.Now().UTC().Format(time.RFC3339),
	})
	h.redisClient.Set(c.Request().Context(), "oauth:state:"+state, string(stateData), 10*time.Minute)

	redirectURI := fmt.Sprintf("%s/api/v1/auth/oauth/%s/callback", h.callbackBase, provider.Slug)
	oauthCfg, err := h.oauthService.GetOAuthConfig(provider, redirectURI)
	if err != nil {
		return c.AbortBadRequest("OAuth provider misconfigured: " + err.Error())
	}
	authURL := oauthCfg.AuthCodeURL(state)

	c.Redirect(http.StatusFound, authURL)
	return nil
}

// Callback handles the OAuth provider's redirect with authorization code.
func (h *OAuthHandler) Callback(c *okapi.Context) error {
	providerSlug := c.Param("provider")
	code := c.Query("code")
	state := c.Query("state")
	oauthError := c.Query("error")

	redirect := func(errMsg string) error {
		c.Redirect(http.StatusFound, h.appWebURL+"/login?error="+errMsg)
		return nil
	}

	if oauthError != "" {
		return redirect(oauthError)
	}
	if code == "" || state == "" {
		return redirect("missing_params")
	}

	// Validate state
	ctx := c.Request().Context()
	stateKey := "oauth:state:" + state
	stateVal := h.redisClient.Get(ctx, stateKey)
	if stateVal.Err() != nil {
		return redirect("invalid_state")
	}
	h.redisClient.Del(ctx, stateKey)

	// Verify state matches provider
	var stateData map[string]string
	if err := json.Unmarshal([]byte(stateVal.Val()), &stateData); err != nil || stateData["provider_slug"] != providerSlug {
		return redirect("state_mismatch")
	}

	provider, err := h.providerRepo.FindBySlug(providerSlug)
	if err != nil {
		return redirect("provider_not_found")
	}

	redirectURI := fmt.Sprintf("%s/api/v1/auth/oauth/%s/callback", h.callbackBase, provider.Slug)

	// Exchange code for tokens + user info
	userInfo, token, err := h.oauthService.ExchangeCode(ctx, provider, code, redirectURI)
	if err != nil {
		logger.Error("OAuth exchange failed for provider %s: %v", provider.Slug, err)
		return redirect("exchange_failed")
	}

	// Find or create user
	user, isNew, err := h.oauthService.FindOrCreateUser(provider, userInfo, token)
	if err != nil {
		logger.Error("OAuth account resolution failed for provider %s: %v", provider.Slug, err)
		return redirect("account_error")
	}

	if !user.Active {
		return redirect("account_disabled")
	}

	// Seed defaults for new users
	if isNew && h.seeder != nil {
		go h.seeder.SeedUserDefaults(user.ID, user.Name)
	}

	// Generate JWT
	jwtToken, err := h.generateToken(c, user)
	if err != nil {
		return redirect("token_generation_failed")
	}

	// Publish event
	if h.bus != nil {
		action := "user.oauth_login"
		if isNew {
			action = "user.oauth_registered"
		}
		h.bus.PublishSimple(models.EventCategoryUser, action, &user.ID, user.Email, c.RealIP(),
			fmt.Sprintf("User %q authenticated via %s", user.Email, provider.Name), nil)
	}

	// Update last login
	now := time.Now()
	user.LastLoginAt = &now
	_ = h.userRepo.Update(user)

	// Redirect to frontend with token
	c.Redirect(http.StatusFound, fmt.Sprintf("%s/auth/oauth/callback?token=%s", h.appWebURL, jwtToken))
	return nil
}

// ListLinkedAccounts returns OAuth accounts linked to the current user.
func (h *OAuthHandler) ListLinkedAccounts(c *okapi.Context) error {
	userID := c.GetInt("user_id")

	accounts, err := h.accountRepo.FindByUserID(uint(userID))
	if err != nil {
		return c.AbortInternalServerError("failed to list linked accounts")
	}

	var result []OAuthLinkedAccount
	for _, a := range accounts {
		result = append(result, OAuthLinkedAccount{
			ID:           a.ID,
			ProviderID:   a.ProviderID,
			ProviderName: a.Provider.Name,
			ProviderType: string(a.Provider.Type),
			Email:        a.Email,
			CreatedAt:    a.CreatedAt.Format(time.RFC3339),
		})
	}

	return ok(c, result)
}

// UnlinkAccount removes an OAuth provider link from the current user.
func (h *OAuthHandler) UnlinkAccount(c *okapi.Context, req *UnlinkOAuthRequest) error {
	userID := c.GetInt("user_id")

	// Check the user has another auth method before unlinking
	user, err := h.userRepo.FindByID(uint(userID))
	if err != nil {
		return c.AbortNotFound("user not found")
	}

	account, err := h.accountRepo.FindByUserAndProvider(uint(userID), uint(req.ProviderID))
	if err != nil {
		return c.AbortNotFound("linked account not found")
	}

	// Count remaining OAuth links
	count, _ := h.accountRepo.CountByUserID(uint(userID))
	hasPassword := user.PasswordHash != "" && user.AuthMethod != "oauth"

	if count <= 1 && !hasPassword {
		return c.AbortBadRequest("cannot unlink the last authentication method — set a password first")
	}

	if err := h.accountRepo.Delete(account.ID); err != nil {
		return c.AbortInternalServerError("failed to unlink account")
	}

	// Update auth method if no more OAuth links
	if count <= 1 && hasPassword {
		user.AuthMethod = "password"
		_ = h.userRepo.Update(user)
	}

	return ok(c, okapi.M{"message": "account unlinked"})
}

// generateToken creates a JWT token for the given user.
func (h *OAuthHandler) generateToken(c *okapi.Context, user *models.User) (string, error) {
	jti := uuid.NewString()

	token, err := okapi.GenerateJwtToken(h.jwtSecret, map[string]any{
		"sub":   user.ID,
		"email": user.Email,
		"role":  string(user.Role),
		"aud":   "posta",
		"jti":   jti,
	}, 24*time.Hour)
	if err != nil {
		return "", err
	}

	// Track session
	if h.sessionRepo != nil {
		session := &models.Session{
			UserID:    user.ID,
			JTI:       jti,
			IPAddress: c.RealIP(),
			UserAgent: c.Request().UserAgent(),
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}
		_ = h.sessionRepo.Create(session)
	}

	return token, nil
}
