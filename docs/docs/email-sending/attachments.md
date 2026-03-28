---
sidebar_position: 5
title: Attachments
description: Send emails with file attachments
---

# Attachments

Attach files to emails by providing base64-encoded content in the `attachments` array.

:::tip
For the full request/response schema, see the interactive [Swagger](/swagger/index.html) or [ReDoc](/redoc) API documentation.
:::

## Example

```bash
# First, base64-encode your file
BASE64_CONTENT=$(base64 -i report.pdf)

curl -X POST http://localhost:9000/api/v1/emails/send \
  -H "Authorization: Bearer your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "from": "sender@example.com",
    "to": ["recipient@example.com"],
    "subject": "Your Report",
    "html": "<p>Please find the report attached.</p>",
    "attachments": [
      {
        "filename": "report.pdf",
        "content": "'$BASE64_CONTENT'",
        "content_type": "application/pdf"
      }
    ]
  }'
```

## Multiple Attachments

You can attach multiple files in a single email:

```json
{
  "attachments": [
    {
      "filename": "report.pdf",
      "content": "<base64-encoded>",
      "content_type": "application/pdf"
    },
    {
      "filename": "data.csv",
      "content": "<base64-encoded>",
      "content_type": "text/csv"
    }
  ]
}
```

## Common MIME Types

| Extension | MIME Type |
|-----------|----------|
| `.pdf` | `application/pdf` |
| `.csv` | `text/csv` |
| `.xlsx` | `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet` |
| `.png` | `image/png` |
| `.jpg` | `image/jpeg` |
| `.zip` | `application/zip` |

## With SDKs

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

<Tabs>
<TabItem value="go" label="Go">

```go
resp, err := client.SendEmail(&posta.SendEmailRequest{
    From:    "sender@example.com",
    To:      []string{"recipient@example.com"},
    Subject: "Your Report",
    HTML:    "<p>Report attached.</p>",
    Attachments: []posta.Attachment{{
        Filename:    "report.pdf",
        Content:     base64EncodedContent,
        ContentType: "application/pdf",
    }},
})
```

</TabItem>
</Tabs>
