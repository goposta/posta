package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/storage/repositories"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// OAuthUserInfo represents the user info extracted from an OAuth provider.
type OAuthUserInfo struct {
	Sub       string
	Email     string
	Name      string
	AvatarURL string
}

// OAuthService handles OAuth authentication flows.
type OAuthService struct {
	providerRepo *repositories.OAuthProviderRepository
	accountRepo  *repositories.OAuthAccountRepository
	userRepo     *repositories.UserRepository
}

func NewOAuthService(
	providerRepo *repositories.OAuthProviderRepository,
	accountRepo *repositories.OAuthAccountRepository,
	userRepo *repositories.UserRepository,
) *OAuthService {
	return &OAuthService{
		providerRepo: providerRepo,
		accountRepo:  accountRepo,
		userRepo:     userRepo,
	}
}

// GetOAuthConfig builds an oauth2.Config for the given provider.
func (s *OAuthService) GetOAuthConfig(provider *models.OAuthProvider, redirectURI string) (*oauth2.Config, error) {
	scopes := strings.Split(provider.Scopes, " ")
	if len(scopes) == 0 || scopes[0] == "" {
		scopes = []string{"openid", "email", "profile"}
	}

	cfg := &oauth2.Config{
		ClientID:     provider.ClientID,
		ClientSecret: provider.ClientSecret,
		Scopes:       scopes,
		RedirectURL:  redirectURI,
	}

	if provider.Type == models.OAuthProviderGoogle {
		cfg.Endpoint = google.Endpoint
		return cfg, nil
	}

	// OIDC: try auto-discovery first, fall back to explicit URLs
	if provider.Issuer != "" {
		oidcProvider, err := oidc.NewProvider(context.Background(), provider.Issuer)
		if err == nil {
			cfg.Endpoint = oidcProvider.Endpoint()
			return cfg, nil
		}
		// Fall through to explicit URLs if discovery fails
	}

	if provider.AuthURL == "" || provider.TokenURL == "" {
		return nil, fmt.Errorf("OIDC provider %q has no issuer for auto-discovery and no explicit auth_url/token_url configured", provider.Name)
	}

	cfg.Endpoint = oauth2.Endpoint{
		AuthURL:  provider.AuthURL,
		TokenURL: provider.TokenURL,
	}
	return cfg, nil
}

// ExchangeCode exchanges an authorization code for tokens and user info.
func (s *OAuthService) ExchangeCode(ctx context.Context, provider *models.OAuthProvider, code, redirectURI string) (*OAuthUserInfo, *oauth2.Token, error) {
	cfg, err := s.GetOAuthConfig(provider, redirectURI)
	if err != nil {
		return nil, nil, err
	}

	token, err := cfg.Exchange(ctx, code)
	if err != nil {
		return nil, nil, fmt.Errorf("token exchange failed (token_url=%s, redirect_uri=%s): %w", cfg.Endpoint.TokenURL, cfg.RedirectURL, err)
	}

	if rawIDToken, ok := token.Extra("id_token").(string); ok && rawIDToken != "" {
		info, idErr := s.parseIDToken(ctx, provider, rawIDToken)
		if idErr == nil {
			return info, token, nil
		}
		fmt.Printf("[oauth] ID token parse failed for %s: %v, falling back to userinfo\n", provider.Slug, idErr)
	}

	// Fall back to userinfo endpoint (use OIDC discovery for correct URL)
	info, err := s.fetchUserInfo(ctx, provider, token)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get user info: %w", err)
	}

	return info, token, nil
}

// parseIDToken validates and extracts claims from an OIDC ID token.
func (s *OAuthService) parseIDToken(ctx context.Context, provider *models.OAuthProvider, rawIDToken string) (*OAuthUserInfo, error) {
	var issuer string
	if provider.Type == models.OAuthProviderGoogle {
		issuer = "https://accounts.google.com"
	} else {
		issuer = provider.Issuer
	}

	if issuer == "" {
		return nil, fmt.Errorf("no issuer configured")
	}

	oidcProvider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("OIDC discovery failed: %w", err)
	}

	verifier := oidcProvider.Verifier(&oidc.Config{ClientID: provider.ClientID})
	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("ID token verification failed: %w", err)
	}

	var claims struct {
		Sub     string `json:"sub"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to parse claims: %w", err)
	}

	return &OAuthUserInfo{
		Sub:       claims.Sub,
		Email:     claims.Email,
		Name:      claims.Name,
		AvatarURL: claims.Picture,
	}, nil
}

func (s *OAuthService) fetchUserInfo(ctx context.Context, provider *models.OAuthProvider, token *oauth2.Token) (*OAuthUserInfo, error) {
	userInfoURL := provider.UserInfoURL

	if userInfoURL == "" && provider.Issuer != "" {
		oidcProvider, err := oidc.NewProvider(ctx, provider.Issuer)
		if err == nil {
			var claims struct {
				UserInfoURL string `json:"userinfo_endpoint"`
			}
			if err := oidcProvider.Claims(&claims); err == nil && claims.UserInfoURL != "" {
				userInfoURL = claims.UserInfoURL
			}
		}
	}

	if userInfoURL == "" && provider.Type == models.OAuthProviderGoogle {
		userInfoURL = "https://www.googleapis.com/oauth2/v3/userinfo"
	}
	if userInfoURL == "" {
		return nil, fmt.Errorf("no userinfo URL available (set issuer for auto-discovery or configure userinfo_url)")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", userInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("userinfo request failed: %d %s", resp.StatusCode, string(body))
	}

	var claims struct {
		Sub               string `json:"sub"`
		Email             string `json:"email"`
		Name              string `json:"name"`
		PreferredUsername string `json:"preferred_username"`
		GivenName         string `json:"given_name"`
		FamilyName        string `json:"family_name"`
		Picture           string `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&claims); err != nil {
		return nil, fmt.Errorf("failed to decode userinfo: %w", err)
	}

	name := claims.Name
	if name == "" {
		name = strings.TrimSpace(claims.GivenName + " " + claims.FamilyName)
	}
	if name == "" {
		name = claims.PreferredUsername
	}

	return &OAuthUserInfo{
		Sub:       claims.Sub,
		Email:     claims.Email,
		Name:      name,
		AvatarURL: claims.Picture,
	}, nil
}

// FindOrCreateUser implements the account linking logic.
func (s *OAuthService) FindOrCreateUser(provider *models.OAuthProvider, info *OAuthUserInfo, token *oauth2.Token) (*models.User, bool, error) {
	// 1. Check if OAuth account already exists
	account, err := s.accountRepo.FindByProviderAndExternalID(provider.ID, info.Sub)
	if err == nil && account != nil {
		account.AccessToken = token.AccessToken
		account.RefreshToken = token.RefreshToken
		if !token.Expiry.IsZero() {
			account.TokenExpiresAt = &token.Expiry
		}
		account.Email = info.Email
		account.Name = info.Name
		account.AvatarURL = info.AvatarURL
		account.UpdatedAt = time.Now()
		_ = s.accountRepo.Update(account)

		user, err := s.userRepo.FindByID(account.UserID)
		if err != nil {
			return nil, false, fmt.Errorf("linked user not found")
		}
		return user, false, nil
	}

	// 2. Check if user exists by email
	email := strings.ToLower(strings.TrimSpace(info.Email))
	if email == "" {
		return nil, false, fmt.Errorf("OAuth provider did not return an email")
	}

	// Check allowed domains
	if provider.AllowedDomains != "" {
		parts := strings.SplitN(email, "@", 2)
		if len(parts) == 2 {
			domain := parts[1]
			allowed := false
			for _, d := range strings.Split(provider.AllowedDomains, ",") {
				if strings.TrimSpace(d) == domain {
					allowed = true
					break
				}
			}
			if !allowed {
				return nil, false, fmt.Errorf("email domain %q is not allowed for this provider", domain)
			}
		}
	}

	existingUser, err := s.userRepo.FindByEmail(email)
	if err == nil && existingUser != nil {
		// User exists — link OAuth account
		oauthAccount := &models.OAuthAccount{
			UserID:         existingUser.ID,
			ProviderID:     provider.ID,
			ProviderUserID: info.Sub,
			Email:          info.Email,
			Name:           info.Name,
			AvatarURL:      info.AvatarURL,
			AccessToken:    token.AccessToken,
			RefreshToken:   token.RefreshToken,
		}
		if !token.Expiry.IsZero() {
			oauthAccount.TokenExpiresAt = &token.Expiry
		}
		if err := s.accountRepo.Create(oauthAccount); err != nil {
			return nil, false, fmt.Errorf("failed to link account: %w", err)
		}

		// Update auth method
		if existingUser.AuthMethod == "password" {
			existingUser.AuthMethod = "both"
		}
		if existingUser.AvatarURL == "" {
			existingUser.AvatarURL = info.AvatarURL
		}
		_ = s.userRepo.Update(existingUser)

		return existingUser, false, nil
	}

	// 3. No user found — auto-register if allowed
	if !provider.AutoRegister {
		return nil, false, fmt.Errorf("account not found and auto-registration is disabled for this provider")
	}

	name := info.Name
	if name == "" {
		name = strings.SplitN(email, "@", 2)[0]
	}

	newUser := &models.User{
		Email:      email,
		Name:       name,
		Role:       models.UserRoleUser,
		AuthMethod: "oauth",
		AvatarURL:  info.AvatarURL,
		Active:     true,
	}
	if err := s.userRepo.Create(newUser); err != nil {
		return nil, false, fmt.Errorf("failed to create user: %w", err)
	}

	oauthAccount := &models.OAuthAccount{
		UserID:         newUser.ID,
		ProviderID:     provider.ID,
		ProviderUserID: info.Sub,
		Email:          info.Email,
		Name:           info.Name,
		AvatarURL:      info.AvatarURL,
		AccessToken:    token.AccessToken,
		RefreshToken:   token.RefreshToken,
	}
	if !token.Expiry.IsZero() {
		oauthAccount.TokenExpiresAt = &token.Expiry
	}
	_ = s.accountRepo.Create(oauthAccount)

	return newUser, true, nil
}
