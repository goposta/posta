---
sidebar_position: 6
title: OAuth Providers
description: Configure OAuth / SSO providers for user authentication
---

# OAuth Providers

Administrators can configure OAuth 2.0 / OpenID Connect providers to allow users to log in via single sign-on. Any OIDC-compliant identity provider is supported.

:::tip
For the full request/response schema, see the interactive API Reference at `/docs` on your Posta instance.
:::

:::info Posta Enterprise
Posta Community signs in through **one** OAuth provider. Adding a second needs a
[Posta Enterprise](https://goposta.dev/enterprise) license; installations that already
had several keep them. For LDAP or Active Directory, see
[LDAP Directories](./ldap-directories.md).
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
  "name": "google",
  "display_name": "Google",
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
  "name": "keycloak",
  "display_name": "Keycloak",
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
| `name` | No | from `display_name` | Identifier in the sign-in URLs (`/auth/oauth/{name}/authorize`): lowercase letters, digits and hyphens, unique. Cannot change later. |
| `display_name` | No | `name` | Shown on the sign-in button. One of `name` and `display_name` is required. |
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
