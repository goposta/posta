---
sidebar_position: 4
title: Quick Start
description: Send your first email with Posta in minutes
---

# Quick Start

This guide walks you through sending your first email with Posta.

## 1. Start Posta

Follow the [Installation](/docs/getting-started/installation) guide to start Posta with Docker Compose.

## 2. Log In and Create an API Key

Log in to the dashboard at `http://localhost:9000` with the admin credentials configured during setup.

Navigate to **API Keys** and create a new key:

```bash
curl -X POST http://localhost:9000/api/v1/users/me/api-keys \
  -H "Authorization: Bearer <your-jwt-token>" \
  -H "Content-Type: application/json" \
  -d '{"name": "my-app"}'
```

**Save the returned key** — it will not be shown again.

## 3. Configure an SMTP Server

Add your SMTP server:

```bash
curl -X POST http://localhost:9000/api/v1/users/me/smtp-servers \
  -H "Authorization: Bearer <your-jwt-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "host": "smtp.gmail.com",
    "port": 587,
    "username": "your-email@gmail.com",
    "password": "your-app-password",
    "encryption": "starttls"
  }'
```

## 4. Send Your First Email

Use your API key to send an email:

```bash
curl -X POST http://localhost:9000/api/v1/emails/send \
  -H "Authorization: Bearer <your-api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "from": "sender@yourdomain.com",
    "to": ["recipient@example.com"],
    "subject": "Hello from Posta!",
    "html": "<h1>Welcome!</h1><p>Your Posta instance is working.</p>"
  }'
```

Response:

```json
{
  "success": true,
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "status": "queued"
  }
}
```

## 5. Check Delivery Status

Poll the status of your email:

```bash
curl http://localhost:9000/api/v1/emails/550e8400-e29b-41d4-a716-446655440000/status \
  -H "Authorization: Bearer <your-api-key>"
```

Response:

```json
{
  "success": true,
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "status": "sent",
    "retry_count": 0,
    "created_at": "2026-01-01T00:00:00Z",
    "sent_at": "2026-01-01T00:00:01Z"
  }
}
```

## Using an SDK

Instead of raw HTTP calls, use one of the official SDKs:

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

<Tabs>
<TabItem value="go" label="Go">

```go
client := posta.New("http://localhost:9000", "your-api-key")

resp, err := client.SendEmail(&posta.SendEmailRequest{
    From:    "sender@yourdomain.com",
    To:      []string{"recipient@example.com"},
    Subject: "Hello from Posta!",
    HTML:    "<h1>Welcome!</h1>",
})
```

</TabItem>
<TabItem value="php" label="PHP">

```php
$client = new PostaClient('http://localhost:9000', 'your-api-key');

$response = $client->sendEmail([
    'from' => 'sender@yourdomain.com',
    'to' => ['recipient@example.com'],
    'subject' => 'Hello from Posta!',
    'html' => '<h1>Welcome!</h1>',
]);
```

</TabItem>
<TabItem value="java" label="Java">

```java
PostaClient client = new PostaClient("http://localhost:9000", "your-api-key");

SendResponse response = client.sendEmail(new SendEmailRequest()
    .from("sender@yourdomain.com")
    .to(List.of("recipient@example.com"))
    .subject("Hello from Posta!")
    .html("<h1>Welcome!</h1>"));
```

</TabItem>
</Tabs>

## Next Steps

- [Template Email](/docs/email-sending/template-email) — Send emails using templates
- [Batch Email](/docs/email-sending/batch-email) — Send to multiple recipients
- [Domain Verification](/docs/smtp-domains/domain-verification) — Verify your sending domain
