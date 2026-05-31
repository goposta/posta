---
sidebar_position: 5
title: Stylesheets
description: Manage CSS stylesheets for email templates
---

# Stylesheets

Manage CSS stylesheets that are automatically inlined into email templates for maximum email client compatibility.

## Why Inline CSS?

Most email clients strip `<style>` tags and external stylesheets. Posta automatically converts your CSS into inline styles, ensuring consistent rendering across Gmail, Outlook, Apple Mail, and others.

## Create a Stylesheet

```
POST /api/v1/workspaces/current/stylesheets
```

```json
{
  "name": "brand-styles",
  "css": "h1 { color: #333; font-family: Arial, sans-serif; } .btn { background-color: #7c3aed; color: white; padding: 12px 24px; border-radius: 6px; text-decoration: none; }"
}
```

## List Stylesheets

```
GET /api/v1/workspaces/current/stylesheets?page=1&size=20
```

## Update a Stylesheet

```
PUT /api/v1/workspaces/current/stylesheets/{id}
```

```json
{
  "name": "brand-styles-v2",
  "css": "h1 { color: #1a1a1a; } .btn { background-color: #6d28d9; }"
}
```

## Delete a Stylesheet

```
DELETE /api/v1/workspaces/current/stylesheets/{id}
```

Returns `204 No Content`.
