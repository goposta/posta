---
sidebar_position: 3.5
title: Fields and validation
description: Declare a form's questions so Posta validates submissions and stores typed answers
---

# Fields and Validation

By default a form accepts any fields and stores them as text. Give it a **schema** and Posta will:

- reject a submission that does not match it, with an error for each field;
- store each answer with its type (numbers as numbers, checkboxes as `true`/`false`, multi-selects as lists) in the message's `data`;
- take the sender's email, name, phone, subject and message from the fields you mark, instead of guessing from field names;
- render exactly these questions in the [website widget](./embedding.md#website-widget).

A form without a schema keeps working exactly as before.

## Setting a schema

Send `fields` when you create or update a form:

```bash
curl -X PUT https://posta.example.com/api/v1/workspaces/current/forms/42 \
  -H "Authorization: Bearer $POSTA_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "fields": [
      { "key": "email", "label": "Email", "type": "email", "required": true, "role": "sender_email" },
      { "key": "department", "label": "Department", "type": "select", "required": true,
        "options": [ { "value": "sales", "label": "Sales" }, { "value": "support", "label": "Support" } ] },
      { "key": "message", "label": "Message", "type": "textarea", "role": "body" },
      { "key": "terms", "label": "I accept the terms", "type": "checkbox", "required": true }
    ]
  }'
```

Each change to `fields` increases the form's `schema_version`, and every message records the version that validated it.

### Field types

| Type | Accepts | Stored as |
|---|---|---|
| `text` | Up to 200 characters by default | string |
| `textarea` | Up to 8000 characters by default | string |
| `email` | An email address | string |
| `phone` | A phone number with at least 6 digits | string |
| `number` | A number, optionally between `min` and `max` | number |
| `url` | An `http` or `https` address | string |
| `date` | `YYYY-MM-DD` | string |
| `time` | `HH:MM` | string |
| `select` | One of `options` | the option's `value` |
| `multiselect` | Any of `options`, sent as a JSON array or repeated keys | list of values |
| `checkbox` | `true`/`on`/`yes`/`1` or `false`/`off`/`no`/`0`; absent means false | boolean |
| `scale` | A whole number from `min` (0 or 1, default 1) to `max` (2 to 10, default 5) | number |
| `rating` | A whole number from 1 to `max` stars (3 to 10, default 5) | number |
| `hidden` | Text not shown to the visitor | string |

### Field settings

| Setting | Applies to | Meaning |
|---|---|---|
| `key` | all | Lowercase letters, digits and `_`, up to 64 characters. Cannot be renamed or removed once saved. |
| `label` | all | Shown to the visitor and used in error messages |
| `required` | all | A required `checkbox` must be ticked |
| `role` | see below | Which message summary value the field fills |
| `min_len`, `max_len` | `text`, `textarea`, `hidden` | Length limits in characters |
| `min`, `max` | `number`, `scale`, `rating` | Range |
| `pattern` | `text`, `textarea`, `hidden` | A regular expression ([RE2 syntax](https://github.com/google/re2/wiki/Syntax)) the whole value must match |
| `options` | `select`, `multiselect` | `{ "value": "...", "label": "..." }`, 1 to 200 |
| `display` | `select`: `radio` or `dropdown`; `multiselect`: `checkboxes` or `list` | How the widget draws it. By default a select with up to 6 options shows as radio buttons. |
| `min_label`, `max_label` | `scale` | Words at each end, such as "Not likely" and "Very likely" |
| `placeholder`, `help` | all | Hint text in the widget |
| `pii` | all | Marks personal data, for the privacy features that build on schemas |
| `archived` | all | Hides the field and stops validating it, while keeping it on old messages |

**Roles:** `sender_email` (an `email` field), `sender_name` (`text`), `sender_phone` (`phone`), `subject` (`text` or `select`) and `body` (`textarea` or `text`). Each role can be used once. A role you leave unset is still filled by guessing from field names, as for forms without a schema.

### Changing a schema

You can add fields, edit labels and settings, and reorder fields at any time. To retire a field, set `"archived": true`; removing it is refused, because older messages refer to it. Submissions from old embeds that still send an archived field are never rejected for it.

### Fields the schema does not declare

`unknown_fields` decides what happens to them:

- `keep` (default): stored with the message, but not in `data`;
- `drop`: discarded;
- `reject`: the submission fails with an `unknown_field` error.

## Validation errors

A submission that does not match the schema is not stored.

**JavaScript and the widget** get `422 Unprocessable Entity`:

```json
{
  "success": false,
  "error": {
    "code": "validation_failed",
    "message": "Some fields need attention.",
    "fields": [
      { "key": "email", "label": "Email", "code": "invalid_email", "message": "Enter a valid email address." }
    ]
  }
}
```

The widget shows each message under its field. To show your own wording, map the `code`: `required`, `invalid_email`, `invalid_phone`, `invalid_number`, `invalid_url`, `invalid_date`, `invalid_time`, `invalid_option`, `invalid_value`, `too_short`, `too_long`, `out_of_range`, `pattern` or `unknown_field`.

**Plain HTML forms** are sent to the form's `error_redirect_url` with `?posta_error=validation_failed&field=<key>` for the first problem. Without one, Posta shows a short page listing the errors.

Spam protection stays silent: a submission caught by the honeypot or the nonce check is still answered with `202`, whatever its fields contain, so bots learn nothing about your schema.
