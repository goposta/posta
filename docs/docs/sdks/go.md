---
sidebar_position: 2
title: Go
description: Posta Go SDK
---

# Go SDK

Official Go client for Posta, built on [okapi/client](https://github.com/jkaninda/okapi).

## Installation

```bash
go get github.com/goposta/posta-go
```

**Requires:** Go 1.25+

## Quick Start

```go
package main

import (
    "fmt"
    "log"

    posta "github.com/goposta/posta-go"
)

func main() {
    client := posta.New("https://posta.example.com", "your-api-key")

    resp, err := client.Emails.Send(&posta.SendEmailRequest{
        From:    "sender@example.com",
        To:      []string{"recipient@example.com"},
        Subject: "Hello from Posta",
        HTML:    "<h1>Hello!</h1><p>This is a test email.</p>",
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Email sent: id=%s status=%s\n", resp.ID, resp.Status)
}
```

## Send Template Email

```go
resp, err := client.Emails.SendTemplate(&posta.SendTemplateEmailRequest{
    Template: "welcome",
    To:       []string{"user@example.com"},
    From:     "noreply@example.com",
    Language: "en",
    TemplateData: map[string]any{
        "name": "Alice",
    },
})
```

## Batch Send

```go
resp, err := client.Emails.SendBatch(&posta.BatchRequest{
    Template: "newsletter",
    From:     "news@example.com",
    Recipients: []posta.BatchRecipient{
        {Email: "user1@example.com", TemplateData: map[string]any{"name": "Bob"}},
        {Email: "user2@example.com", Language: "fr", TemplateData: map[string]any{"name": "Carol"}},
    },
})
fmt.Printf("Sent: %d, Failed: %d\n", resp.Sent, resp.Failed)
```

## Check Delivery Status

```go
status, err := client.Emails.Status("email-uuid")
fmt.Printf("Status: %s\n", status.Status)
```

## Error Handling

```go
_, err := client.Emails.Status("invalid-uuid")
if err != nil {
    var apiErr *posta.APIError
    if errors.As(err, &apiErr) {
        fmt.Printf("Status: %d\n", apiErr.StatusCode)
        if apiErr.Info != nil {
            fmt.Printf("Message: %s\n", apiErr.Info.Message)
        }
    }
}
```

## Full API coverage

The client covers the whole Posta API through resource services on the client:

| Service | Covers |
|---------|--------|
| `Emails` | send, templated and batch sends, preview, verify, status, retry, list, get |
| `Templates` | templates, versions, localizations, preview, send-test, import/export |
| `Campaigns` | CRUD, send, pause, resume, cancel, duplicate, messages, analytics |
| `Subscribers`, `SubscriberLists` | CRUD, members, segments, opt-ins and opt-outs |
| `Domains`, `SMTPServers`, `SMTPCredentials` | sending infrastructure and DNS verification |
| `Webhooks` | endpoints, delivery logs, and `posta.VerifySignature` |
| `Forms`, `Messages`, `MessageFilters` | web forms and the submissions they collect |
| `Inbound` | received email, raw `.eml`, attachments |
| `Workspaces`, `APIKeys`, `Users`, `Admin` | tenant and platform administration |

See the [client README](https://github.com/goposta/posta-go) for the full list.

## Workspaces

Workspace-scoped endpoints resolve the active workspace from the
`X-Posta-Workspace-Id` header. A workspace-bound API key carries its workspace
already; an account-wide key or a user session must name one:

```go
client := posta.New(baseURL, apiKey, posta.WithWorkspace(42))
```

## Verifying webhooks

Posta signs each delivery with HMAC-SHA256 over the raw body. Verify it against
the exact bytes received:

```go
if !posta.VerifySignature(body, r.Header.Get(posta.SignatureHeader), secret) {
    http.Error(w, "bad signature", http.StatusUnauthorized)
    return
}
```
