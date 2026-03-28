package routes

import (
	"net/http"

	"github.com/jkaninda/okapi"
	"github.com/goposta/posta/internal/dto"
	"github.com/goposta/posta/internal/handlers"
)

// oauthRoutes returns route definitions for OAuth authentication.
func (r *Router) oauthRoutes() []okapi.RouteDefinition {
	// Public OAuth routes (no auth required)
	oauthPublic := r.v1.Group("/auth/oauth").WithTags([]string{"OAuth"})

	routes := []okapi.RouteDefinition{
		{
			Method:      http.MethodGet,
			Path:        "/providers",
			Handler:     r.h.oauth.ListProviders,
			Group:       oauthPublic,
			Summary:     "List OAuth providers",
			Description: "Returns enabled OAuth providers for the login page",
			Response:    &dto.Response[okapi.M]{},
		},
		{
			Method:      http.MethodGet,
			Path:        "/{provider}/authorize",
			Handler:     r.h.oauth.Authorize,
			Group:       oauthPublic,
			Summary:     "Initiate OAuth flow",
			Description: "Redirects to the OAuth provider's authorization page",
			Options: []okapi.RouteOption{
				okapi.DocPathParam("provider", "string", "Provider slug"),
			},
		},
		{
			Method:      http.MethodGet,
			Path:        "/{provider}/callback",
			Handler:     r.h.oauth.Callback,
			Group:       oauthPublic,
			Summary:     "OAuth callback",
			Description: "Handles the OAuth provider's redirect after authorization",
			Options: []okapi.RouteOption{
				okapi.DocPathParam("provider", "string", "Provider slug"),
				okapi.DocHide(),
			},
		},
	}

	// Authenticated OAuth routes (linked accounts)
	oauthUser := r.v1.Group("/users/me/oauth", r.mw.jwtAuth.Middleware).WithTags([]string{"OAuth"})
	oauthUser.WithBearerAuth()

	routes = append(routes, []okapi.RouteDefinition{
		{
			Method:   http.MethodGet,
			Path:     "",
			Handler:  r.h.oauth.ListLinkedAccounts,
			Group:    oauthUser,
			Summary:  "List linked OAuth accounts",
			Response: &dto.Response[[]handlers.OAuthLinkedAccount]{},
		},
		{
			Method:  http.MethodDelete,
			Path:    "/{provider_id:int}",
			Handler: okapi.H(r.h.oauth.UnlinkAccount),
			Group:   oauthUser,
			Summary: "Unlink OAuth account",
			Options: []okapi.RouteOption{
				okapi.DocPathParam("provider_id", "integer", "Provider ID"),
			},
			Response: &dto.Response[okapi.M]{},
		},
	}...)

	// Admin OAuth provider management
	oauthAdmin := r.v1.Group("/admin/oauth/providers", r.mw.jwtAdminAuth.Middleware).WithTags([]string{"Admin", "OAuth"})
	oauthAdmin.WithBearerAuth()

	routes = append(routes, []okapi.RouteDefinition{
		{
			Method:   http.MethodGet,
			Path:     "",
			Handler:  r.h.oauthAdmin.ListProviders,
			Group:    oauthAdmin,
			Summary:  "List all OAuth providers (admin)",
			Response: &dto.Response[[]handlers.OAuthProviderResponse]{},
		},
		{
			Method:  http.MethodPost,
			Path:    "",
			Handler: okapi.H(r.h.oauthAdmin.CreateProvider),
			Group:   oauthAdmin,
			Summary: "Create OAuth provider",
			Request: &handlers.CreateOAuthProviderRequest{},
			Options: []okapi.RouteOption{
				okapi.DocResponse(201, &dto.Response[handlers.OAuthProviderResponse]{}),
			},
		},
		{
			Method:   http.MethodPut,
			Path:     "/{id:int}",
			Handler:  okapi.H(r.h.oauthAdmin.UpdateProvider),
			Group:    oauthAdmin,
			Summary:  "Update OAuth provider",
			Request:  &handlers.UpdateOAuthProviderRequest{},
			Response: &dto.Response[handlers.OAuthProviderResponse]{},
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Provider ID"),
			},
		},
		{
			Method:  http.MethodDelete,
			Path:    "/{id:int}",
			Handler: okapi.H(r.h.oauthAdmin.DeleteProvider),
			Group:   oauthAdmin,
			Summary: "Delete OAuth provider",
			Options: []okapi.RouteOption{
				okapi.DocPathParam("id", "integer", "Provider ID"),
				okapi.DocResponse(204, nil),
			},
		},
	}...)

	// Workspace SSO config
	wsSSO := r.v1.Group("/workspaces/current/sso", r.mw.jwtAuth.Middleware, r.mw.workspace).WithTags([]string{"Workspaces", "OAuth"})
	wsSSO.WithBearerAuth()

	routes = append(routes, []okapi.RouteDefinition{
		{
			Method:   http.MethodGet,
			Path:     "",
			Handler:  r.h.oauthAdmin.GetWorkspaceSSO,
			Group:    wsSSO,
			Summary:  "Get workspace SSO config",
			Response: &dto.Response[handlers.WorkspaceSSOResponse]{},
			Options:  []okapi.RouteOption{workspaceHeaderRequired},
		},
		{
			Method:   http.MethodPut,
			Path:     "",
			Handler:  okapi.H(r.h.oauthAdmin.SetWorkspaceSSO),
			Group:    wsSSO,
			Summary:  "Set workspace SSO config",
			Request:  &handlers.SetWorkspaceSSORequest{},
			Response: &dto.Response[okapi.M]{},
			Options:  []okapi.RouteOption{workspaceHeaderRequired},
		},
		{
			Method:   http.MethodDelete,
			Path:     "",
			Handler:  r.h.oauthAdmin.DeleteWorkspaceSSO,
			Group:    wsSSO,
			Summary:  "Delete workspace SSO config",
			Response: &dto.Response[okapi.M]{},
			Options:  []okapi.RouteOption{workspaceHeaderRequired},
		},
	}...)

	return routes
}
