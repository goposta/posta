---
sidebar_position: 1
title: Authentication
description: JWT and API key authentication
---

# Authentication

Posta uses two authentication methods: **JWT tokens** for the dashboard and **API keys** for programmatic access.

## JWT Authentication (Dashboard)

### Login

```
POST /api/v1/auth/login
```

```json
{
  "email": "admin@example.com",
  "password": "your-password",
  "two_factor_code": "123456"
}
```

The `two_factor_code` field is only required if 2FA is enabled on the account.

Response:

```json
{
  "success": true,
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIs...",
    "user": {
      "id": "user-uuid",
      "name": "Admin",
      "email": "admin@example.com",
      "role": "admin"
    }
  }
}
```

Use the token in the `Authorization` header:

```
Authorization: Bearer eyJhbGciOiJIUzI1NiIs...
```

JWT tokens expire after 24 hours.

### Register

```
POST /api/v1/auth/register
```

```json
{
  "name": "New User",
  "email": "user@example.com",
  "password": "secure-password"
}
```

Registration can be disabled via `POSTA_REGISTRATION_ENABLED=false`.

### Check Registration Status

```
GET /api/v1/auth/registration-status
```

## OAuth / SSO Authentication

Posta supports OAuth 2.0 / OpenID Connect for single sign-on. Administrators can configure OAuth providers (Google, Keycloak, authentik, and others) from the Admin Panel under **OAuth**.

Users can then log in via their identity provider without needing a Posta-specific password.

### Supported Providers

Any OAuth 2.0 / OIDC-compliant provider can be configured, including:

- **Google**
- **Keycloak**
- **authentik**
- Custom OIDC providers

### OAuth Login Flow

```
GET /api/v1/auth/oauth/{provider}/authorize
```

Redirects the user to the provider's authorization page. After authentication, the provider redirects back to:

```
GET /api/v1/auth/oauth/callback?code=...&state=...
```

Posta exchanges the authorization code for user info and issues a JWT token.

## API Key Authentication (Programmatic)

API keys are used for sending emails and checking status via the API. See [API Keys](/docs/security/api-keys) for details on creating and managing keys.

```
Authorization: Bearer posta_abc123...
```

## Roles

| Role | Permissions |
|------|------------|
| `user` | Send emails, manage own templates/domains/SMTP/webhooks |
| `admin` | All user permissions + user management, platform settings, shared servers |

## Public Endpoints

These endpoints do not require authentication:

- `GET /api/v1/healthz` — Liveness probe
- `GET /api/v1/readyz` — Readiness probe
- `GET /api/v1/info` — Application info
- `POST /api/v1/auth/login` — Login
- `POST /api/v1/auth/register` — Register
- `GET /api/v1/auth/registration-status` — Check registration
