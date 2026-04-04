---
sidebar_position: 1
title: SMTP Servers
description: Configure SMTP servers for email delivery
---

# SMTP Servers

Configure one or more SMTP servers for email delivery. Posta routes emails through your configured SMTP servers.

:::tip
For the full request/response schema, see the interactive [API Reference](https://app.goposta.dev/docs).
:::

## Adding an SMTP Server

Add a server from the dashboard or via `POST /api/v1/users/me/smtp-servers`. Example:

```json
{
  "host": "smtp.gmail.com",
  "port": 587,
  "username": "your-email@gmail.com",
  "password": "your-app-password",
  "encryption": "starttls",
  "max_retries": 3,
  "allowed_emails": ["noreply@yourdomain.com", "alerts@yourdomain.com"]
}
```

### Encryption Options

| Value | Port | Description |
|-------|------|-------------|
| `none` | 25 | No encryption (not recommended) |
| `starttls` | 587 | Upgrade to TLS after connecting |
| `ssl` | 465 | TLS from the start |

## Testing Connections

Verify SMTP credentials and connectivity before sending via `POST /api/v1/users/me/smtp-servers/{id}/test`. This validates the hostname, port, credentials, and encryption.

## Sender Restrictions

Use `allowed_emails` to restrict which sender addresses can use a specific SMTP server. This is useful when different servers are configured for different brands or departments.

:::note
Passwords are never returned in API responses.
:::

## Common SMTP Providers

| Provider | Host | Port | Encryption |
|----------|------|------|------------|
| Gmail | `smtp.gmail.com` | 587 | `starttls` |
| Outlook | `smtp.office365.com` | 587 | `starttls` |
| Amazon SES | `email-smtp.us-east-1.amazonaws.com` | 587 | `starttls` |
| Mailgun | `smtp.mailgun.org` | 587 | `starttls` |
| Postfix (local) | `localhost` | 25 | `none` |
