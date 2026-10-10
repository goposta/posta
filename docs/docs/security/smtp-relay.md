---
sidebar_position: 3
title: SMTP Relay
description: Relay mail from an existing SMTP sender through Posta's outbound pipeline
---

# SMTP Relay

The SMTP Relay is a migration aid for teams that already send mail through an SMTP client or library and are not yet ready to rewrite that integration against the HTTP API. A workspace issues SMTP username/password credentials from Posta, the existing SMTP client points at Posta's SMTP Relay instead of its current outbound provider, and every message it sends is parsed and relayed through the **same outbound pipeline** used by `POST /api/v1/emails/send` — the same domain-verification, suppression-list, rate-limit, and delivery handling, just reached over SMTP instead of HTTP. Teams can cut over their SMTP client on day one and migrate call sites to the HTTP API gradually, at their own pace.

This is a separate, purpose-built listener from [Inbound Email](../inbound/overview.md): Inbound accepts anonymous mail addressed to a verified domain, while the Relay requires SMTP AUTH and exists to accept mail *from* your applications, independent of whether Inbound is enabled.

## Enabling the Relay

The SMTP Relay is off by default. Enable it with configuration:

| Variable | Default | Description |
|----------|---------|--------------|
| `POSTA_SMTP_RELAY_ENABLED` | `false` | Master switch. Enables the SMTP relay listener and the `/api/v1/workspaces/current/smtp-credentials` routes. |
| `POSTA_SMTP_RELAY_HOST` | `0.0.0.0` | Bind address for the built-in SMTP relay listener. |
| `POSTA_SMTP_RELAY_PORT` | `2526` | Listener port. Separate from `POSTA_INBOUND_SMTP_PORT` — the two listeners never share a port or process state. |
| `POSTA_SMTP_RELAY_HOSTNAME` | `posta.local` | Hostname announced in the SMTP `EHLO` greeting. |
| `POSTA_SMTP_RELAY_TLS_MODE` | `none` | Set to `starttls` to advertise STARTTLS and require TLS before AUTH. |
| `POSTA_SMTP_RELAY_TLS_CERT_FILE` | — | PEM certificate chain; required with `starttls`. |
| `POSTA_SMTP_RELAY_TLS_KEY_FILE` | — | PEM private key; required with `starttls`. |
| `POSTA_SMTP_RELAY_MAX_MESSAGE_SIZE` | `26214400` (25 MiB) | Maximum raw message size in bytes. Larger messages are rejected with `552`. |
| `POSTA_SMTP_RELAY_RATE_LIMIT` | `60` | Per-IP maximum SMTP sessions per window; `0` disables the limit. |
| `POSTA_SMTP_RELAY_RATE_WINDOW` | `60` | Rate-limit window, in seconds. |

:::warning Transport security
The default `none` mode preserves the legacy plaintext-AUTH relay. Set `POSTA_SMTP_RELAY_TLS_MODE=starttls` and provide a certificate/key to advertise STARTTLS and withhold AUTH until TLS is established (minimum TLS 1.2). Clients must require STARTTLS and verify the certificate against the relay hostname; opportunistic TLS can silently fall back to plaintext. Keep either mode on a private network or VPN; STARTTLS does not make this listener a public MTA.
:::

## How It Works

```
your SMTP client                POSTA_SMTP_RELAY_PORT
       │                                  │
       ├── EHLO / STARTTLS (if enabled) ─►│  require TLS before AUTH
       ├── AUTH PLAIN ───────────────────►│  verify SMTPCredential (workspace-scoped)
       │                                  │
       └── MAIL FROM / RCPT TO / DATA ───►│  parse message
                                           │
                                           ▼
                              email.Service.Send()  ──►  same pipeline as POST /api/v1/emails/send
                                           │
                                           ├─►  domain verification
                                           ├─►  suppression list
                                           ├─►  rate limits / plan quota
                                           └─►  queued for delivery (Email record)
```

1. Your SMTP client opens a connection to `POSTA_SMTP_RELAY_HOST:POSTA_SMTP_RELAY_PORT` and authenticates with `AUTH PLAIN`, using an SMTP Relay username and password.
2. Posta looks up the credential, confirms it is not revoked, and ties the rest of the session to the credential's workspace. `MAIL FROM`, `RCPT TO`, and `DATA` are all rejected until AUTH succeeds.
3. On `DATA`, Posta reads the raw message (bounded by `POSTA_SMTP_RELAY_MAX_MESSAGE_SIZE`), parses subject, HTML/text bodies, and attachments from the MIME body, and builds a send request using the SMTP envelope addresses (`MAIL FROM` / `RCPT TO`) rather than the parsed header addresses — the same behavior a normal MTA would have.
4. That request is handed to the same `email.Service.Send` used by the HTTP API, scoped to the credential's workspace and owning user. It goes through the identical checks: sender domain verification, suppression-list filtering, rate limits, plan quota, and attachment-size validation, and a normal `Email` record is created and queued for delivery.
5. The SMTP response code reflects the outcome of that call — see [Send Outcomes](#send-outcomes) below.

## Issuing a Credential

SMTP credentials are workspace-scoped and always require a workspace — there is no personal/unscoped credential. Create one from the dashboard under **Developers → SMTP Relay** (`/smtp-relay`), or directly via the API:

```
POST /api/v1/workspaces/current/smtp-credentials
```

```bash
curl -X POST http://localhost:9000/api/v1/workspaces/current/smtp-credentials \
  -H "Authorization: Bearer <jwt>" \
  -H "X-Posta-Workspace-Id: 1" \
  -H "Content-Type: application/json" \
  -d '{ "name": "Legacy app relay", "allowed_ips": ["203.0.113.0/24"] }'
```

Response (`201`):

```json
{
  "success": true,
  "data": {
    "id": 7,
    "name": "Legacy app relay",
    "username": "smtp_9f3c2a1b7e4d5601",
    "password": "8b1c...e02f",
    "host": "posta.local",
    "port": 2526,
    "encryption": "starttls",
    "created_at": "2026-07-20T00:00:00Z",
    "message": "Save this password securely. It will not be shown again."
  }
}
```

:::warning
**Save the password immediately.** Like an API key, the plaintext password is only returned once, at creation time. Posta stores only a hash of it.
:::

Unlike API keys, an SMTP credential has no expiry or scopes — it is either usable or revoked. It does support an IP allowlist (`allowed_ips`), same as [API keys](./api-keys.md#security-features): an empty list permits any IP.

### Listing Credentials

```
GET /api/v1/workspaces/current/smtp-credentials
```

Returns a paginated list scoped to the current workspace. Passwords are never returned — only credential metadata (`id`, `name`, `username`, `allowed_ips`, `revoked`, `created_at`, `last_used_at`).

### Revoking a Credential

Instantly disables a credential without deleting it, mirroring [API key revocation](./api-keys.md#revoking-a-key):

```
POST /api/v1/workspaces/current/smtp-credentials/{id}/revoke
```

A revoked credential fails `AUTH` immediately on its next connection attempt; any already-open session is not forcibly disconnected but subsequent commands on a *new* session will be rejected.

### Deleting a Credential

```
DELETE /api/v1/workspaces/current/smtp-credentials/{id}
```

Permanently removes the credential record.

## Connecting Your SMTP Client

Point your existing SMTP client at the Relay host and port with the generated username/password. Match the client encryption setting to the listener configuration:

| Setting | Value |
|---|---|
| Host | `POSTA_SMTP_RELAY_HOST` (or wherever it's reachable from your app) |
| Port | `POSTA_SMTP_RELAY_PORT` (default `2526`) |
| Encryption | `starttls` when enabled (required and certificate-verified); otherwise `none` only on a trusted private path |
| Auth mechanism | `PLAIN` |
| Username / Password | From the credential creation response |

```bash
swaks --server localhost --port 2526 \
  --tls --tls-verify \
  --auth PLAIN --auth-user smtp_9f3c2a1b7e4d5601 --auth-password 8b1c...e02f \
  --from sender@yourdomain.com --to recipient@example.com \
  --header "Subject: Hello from the SMTP Relay" \
  --body "This message was relayed through Posta's outbound pipeline."
```

The example assumes STARTTLS mode and a trusted relay certificate. In `none` mode, omit the `--tls` flags and restrict access to a trusted private path. `AUTH PLAIN` is the only mechanism the Relay advertises; clients that default to `LOGIN` or `CRAM-MD5` should be configured to use `PLAIN` explicitly. The listener accepts up to 50 recipients per message.

## Send Outcomes

Because the Relay relays into the same `email.Service.Send` used by `POST /api/v1/emails/send`, an accepted message ends up in the identical set of [email statuses](../email-sending/email-status.md#status-values) (`pending`, `queued`, `processing`, `sent`, `failed`, `suppressed`) — the Relay does not introduce any new ones. The SMTP response code the client sees just reflects whether Posta accepted the message for that pipeline, not its eventual delivery outcome:

| SMTP response | Meaning |
|---|---|
| `250` | Accepted and handed to the outbound pipeline as an `Email` record. This includes the case where every recipient was suppressed — Posta still accepts the message, logs it with `suppressed` status, and does not attempt delivery, exactly as the HTTP API does. |
| `550 5.7.1` | Sender domain is not [verified](../smtp-domains/domain-verification.md) for this workspace. |
| `452 4.7.0` / `421 4.7.0` | [Rate limit](./rate-limiting.md) or plan quota exceeded; retry later. |
| `552 5.3.4` | Message exceeds `POSTA_SMTP_RELAY_MAX_MESSAGE_SIZE`. |
| `554 5.6.0` | Message could not be parsed (malformed MIME). |
| `451 4.3.0` | Temporary failure (e.g. a transient error while queuing). |
| `502 5.7.0` | A mail command was sent before a successful `AUTH`. |
| `535 5.7.8` | `AUTH` failed — unknown or revoked credential, wrong password, inactive user, or the connecting IP is not on the credential's `allowed_ips`. |

Check the eventual delivery outcome of a message the same way you would for an API-sent email — via `GET /api/v1/emails/{id}/status` using the `id` Posta assigned, or by browsing the workspace's email log in the dashboard.

## Next Steps

- [Domain Verification](../smtp-domains/domain-verification.md) — required before a sender address can relay mail.
- [API Keys](./api-keys.md) — the HTTP-API equivalent of a Relay credential, for when you're ready to migrate fully.
- [Rate Limiting](./rate-limiting.md) — how send limits are enforced across both the HTTP API and the Relay.
- [Email Status](../email-sending/email-status.md) — track a relayed message after it's accepted.
