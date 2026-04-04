---
sidebar_position: 6
title: OAuth Providers
description: Configure OAuth / SSO providers for user authentication
---

# OAuth Providers

Administrators can configure OAuth 2.0 / OpenID Connect providers to allow users to log in via single sign-on. Any OIDC-compliant identity provider is supported.

:::tip
For the full request/response schema, see the interactive [API Reference](https://app.goposta.dev/docs).
:::

## Supported Provider Types

| Type | Description |
|------|-------------|
| `google` | Google OAuth 2.0 (pre-configured endpoints) |
| `oidc` | Generic OpenID Connect (custom endpoints) |

## Creating a Provider

```
POST /api/v1/admin/oauth/providers
```

### Google

```json
{
  "name": "Google",
  "slug": "google",
  "type": "google",
  "client_id": "your-google-client-id",
  "client_secret": "your-google-client-secret",
  "auto_register": true,
  "allowed_domains": "example.com,company.org"
}
```

### Generic OIDC (Keycloak, authentik, etc.)

```json
{
  "name": "Keycloak",
  "slug": "keycloak",
  "type": "oidc",
  "client_id": "posta",
  "client_secret": "your-client-secret",
  "issuer": "https://keycloak.example.com/realms/main",
  "auth_url": "https://keycloak.example.com/realms/main/protocol/openid-connect/auth",
  "token_url": "https://keycloak.example.com/realms/main/protocol/openid-connect/token",
  "userinfo_url": "https://keycloak.example.com/realms/main/protocol/openid-connect/userinfo",
  "scopes": "openid email profile",
  "auto_register": true
}
```

### Fields

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `name` | Yes | — | Display name |
| `slug` | Yes | — | URL-safe identifier (must be unique) |
| `type` | Yes | — | `google` or `oidc` |
| `client_id` | Yes | — | OAuth client ID |
| `client_secret` | Yes | — | OAuth client secret |
| `issuer` | No | — | OIDC issuer URL |
| `auth_url` | No | — | Authorization endpoint (required for `oidc`) |
| `token_url` | No | — | Token endpoint (required for `oidc`) |
| `userinfo_url` | No | — | User info endpoint (required for `oidc`) |
| `scopes` | No | `openid email profile` | OAuth scopes to request |
| `auto_register` | No | `true` | Automatically create accounts for new users |
| `allowed_domains` | No | — | Comma-separated list of allowed email domains |

:::caution
`client_id` and `client_secret` are never returned in API responses.
:::

## Listing Providers

```
GET /api/v1/admin/oauth/providers
```

## Updating a Provider

```
PUT /api/v1/admin/oauth/providers/{id}
```

All fields are optional.

## Deleting a Provider

```
DELETE /api/v1/admin/oauth/providers/{id}
```

## Workspace SSO

Workspace owners can enforce SSO for their workspace members by linking an OAuth provider.

### Set Workspace SSO

```
PUT /api/v1/workspaces/current/sso
```

```json
{
  "provider_id": 1,
  "enforce_sso": true,
  "auto_provision": true,
  "allowed_domains": "company.org"
}
```

| Field | Description |
|-------|-------------|
| `provider_id` | ID of the OAuth provider to use |
| `enforce_sso` | When `true`, workspace members must log in via SSO |
| `auto_provision` | Automatically add authenticated users as workspace members |
| `allowed_domains` | Restrict access to specific email domains |

### Get Workspace SSO

```
GET /api/v1/workspaces/current/sso
```

### Remove Workspace SSO

```
DELETE /api/v1/workspaces/current/sso
```
