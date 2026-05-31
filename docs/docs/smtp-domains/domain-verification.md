---
sidebar_position: 2
title: Domain Verification
description: Verify sending domains with SPF, DKIM, and DMARC
---

# Domain Verification

Verify your sending domains to improve deliverability and prevent spoofing. Posta checks SPF, DKIM, and DMARC DNS records.

## Register a Domain

```
POST /api/v1/workspaces/current/domains
```

```bash
curl -X POST http://localhost:9000/api/v1/workspaces/current/domains \
  -H "Authorization: Bearer <jwt>" \
  -H "X-Posta-Workspace-Id: 1" \
  -H "Content-Type: application/json" \
  -d '{"domain": "yourdomain.com"}'
```

Response:

```json
{
  "success": true,
  "data": {
    "id": 1,
    "domain": "yourdomain.com",
    "ownership_verified": false,
    "spf_verified": false,
    "dkim_verified": false,
    "dmarc_verified": false,
    "verification_token": "abc123def456",
    "dns_records": {
      "verification": {
        "type": "TXT",
        "host": "yourdomain.com",
        "value": "posta-verification=abc123def456"
      },
      "spf": {
        "type": "TXT",
        "host": "yourdomain.com",
        "value": "v=spf1 include:_spf.posta ~all"
      },
      "dkim": {
        "type": "CNAME",
        "host": "posta._domainkey.yourdomain.com",
        "value": "posta._domainkey.posta"
      },
      "dmarc": {
        "type": "TXT",
        "host": "_dmarc.yourdomain.com",
        "value": "v=DMARC1; p=none; rua=mailto:dmarc@yourdomain.com"
      }
    }
  }
}
```

## Add DNS Records

Add the following DNS records to your domain:

### 1. Ownership Verification (TXT)

| Type | Host | Value |
|------|------|-------|
| TXT | `yourdomain.com` | `posta-verification=<verification_token>` |

### 2. SPF Record (TXT)

| Type | Host | Value |
|------|------|-------|
| TXT | `yourdomain.com` | `v=spf1 include:_spf.posta ~all` |

### 3. DKIM Record (CNAME)

| Type | Host | Value |
|------|------|-------|
| CNAME | `posta._domainkey.yourdomain.com` | `posta._domainkey.posta` |

### 4. DMARC Record (TXT)

| Type | Host | Value |
|------|------|-------|
| TXT | `_dmarc.yourdomain.com` | `v=DMARC1; p=none; rua=mailto:dmarc@yourdomain.com` |

## Verify DNS Records

After adding DNS records, trigger verification:

```
POST /api/v1/workspaces/current/domains/{id}/verify
```

```bash
curl -X POST http://localhost:9000/api/v1/workspaces/current/domains/1/verify \
  -H "Authorization: Bearer <jwt>" \
  -H "X-Posta-Workspace-Id: 1"
```

Response:

```json
{
  "success": true,
  "data": {
    "domain": {
      "id": 1,
      "domain": "yourdomain.com",
      "ownership_verified": true,
      "spf_verified": true,
      "dkim_verified": true,
      "dmarc_verified": false
    },
    "verification": {
      "ownership_verified": true,
      "spf_verified": true,
      "dkim_verified": true,
      "dmarc_verified": false
    },
    "fully_verified": false
  }
}
```

:::tip
DNS propagation can take up to 48 hours. If verification fails, wait and try again.
:::

## Domain Enforcement

When `require_verified_domain` is enabled in user settings, Posta will reject emails from unverified domains. This adds an extra layer of security to prevent unauthorized sending.

## List Domains

```
GET /api/v1/workspaces/current/domains?page=1&size=20
```

## Get Domain Details

```
GET /api/v1/workspaces/current/domains/{id}
```

The response includes the current `dns_records` values alongside the domain's verification status.

## Delete a Domain

```
DELETE /api/v1/workspaces/current/domains/{id}
```
