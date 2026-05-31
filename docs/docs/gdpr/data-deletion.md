---
sidebar_position: 2
title: Data Deletion
description: Delete contact and email data for GDPR compliance
---

# Data Deletion

Posta provides endpoints for GDPR-compliant data deletion scoped to the active workspace.

## Delete Contact Data

Remove all data associated with a specific email address, or all contacts in the workspace:

```
POST /api/v1/workspaces/current/gdpr/delete-contacts
```

### Delete a Specific Contact

```json
{
  "email": "user@example.com"
}
```

Response:

```json
{
  "success": true,
  "data": {
    "deleted": 1,
    "message": "Contact user@example.com and associated data deleted"
  }
}
```

This removes the contact from:
- The contacts list
- All contact list memberships
- The suppression list

### Delete All Contacts

Omit the `email` field (or pass an empty string) to delete all contacts in the workspace:

```json
{}
```

Response:

```json
{
  "success": true,
  "data": {
    "deleted": 500,
    "message": "All contacts deleted"
  }
}
```

```bash
curl -X POST http://localhost:9000/api/v1/workspaces/current/gdpr/delete-contacts \
  -H "Authorization: Bearer <jwt>" \
  -H "X-Posta-Workspace-Id: 1" \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com"}'
```

## Delete Email Logs

Remove email logs scoped to the active workspace, optionally filtered by age:

```
POST /api/v1/workspaces/current/gdpr/delete-email-logs
```

### Delete Logs Older Than 30 Days

```json
{
  "older_than_days": 30
}
```

Response:

```json
{
  "success": true,
  "data": {
    "deleted": 1500,
    "message": "Email logs older than 30 days deleted"
  }
}
```

### Delete All Email Logs

Pass `0` to delete all email logs regardless of age:

```json
{
  "older_than_days": 0
}
```

Response:

```json
{
  "success": true,
  "data": {
    "deleted": 8200,
    "message": "All email logs deleted"
  }
}
```

:::note
Deleting email logs also removes associated bounce records.
:::

```bash
curl -X POST http://localhost:9000/api/v1/workspaces/current/gdpr/delete-email-logs \
  -H "Authorization: Bearer <jwt>" \
  -H "X-Posta-Workspace-Id: 1" \
  -H "Content-Type: application/json" \
  -d '{"older_than_days": 30}'
```

## Automatic Retention

Administrators can configure automatic data retention via [Platform Settings](/docs/admin/platform-settings):

- `email_retention_days` — Auto-delete email logs after N days
- `webhook_retention_days` — Auto-delete webhook delivery history after N days
- `audit_log_retention_days` — Auto-delete audit log entries after N days

The retention cleanup job runs daily at 2:00 AM.
