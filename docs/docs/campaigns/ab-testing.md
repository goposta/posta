---
sidebar_position: 3
title: A/B Testing
description: Test email variants with split audience campaigns
---

# A/B Testing

Run A/B tests by splitting your subscriber list across multiple email variants. Compare performance to find the most effective subject line, content, or design.

:::tip
For the full request/response schema, see the interactive [API Reference](https://app.goposta.dev/docs).
:::

## Creating an A/B Test Campaign

Enable A/B testing when creating a campaign:

```json
{
  "name": "Subject Line Test",
  "subject": "Default subject",
  "from_email": "news@example.com",
  "template_id": 5,
  "list_id": 3,
  "ab_test_enabled": true,
  "ab_test_variants": [
    {
      "name": "Variant A",
      "split_percentage": 50
    },
    {
      "name": "Variant B",
      "split_percentage": 50
    }
  ]
}
```

### Requirements

- Minimum **2 variants** required
- Split percentages must sum to exactly **100**
- Campaign must be in `draft` status to configure variants

### Variant Fields

| Field | Description |
|-------|-------------|
| `name` | Variant identifier (e.g., "Variant A", "Short Subject") |
| `split_percentage` | Percentage of the subscriber list that receives this variant |

## How It Works

When the campaign is sent, Posta randomly splits the subscriber list according to the configured percentages. Each subscriber receives exactly one variant.

## Tracking Results

Use the campaign analytics endpoint to compare variant performance:

```
GET /api/v1/users/me/campaigns/{id}/analytics
```

The response includes delivery stats broken down by variant, allowing you to compare open rates, click rates, and other metrics across variants.

## Campaign Messages

View individual messages sent for a campaign:

```
GET /api/v1/users/me/campaigns/{id}/messages?page=0&size=20
```

Each message includes which variant was sent to that subscriber.
