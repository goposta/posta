---
sidebar_position: 2.5
title: Sharing a form
description: Share a form with a link, control when it accepts responses, and set how it looks
---

# Sharing a Form

Every form has a **share link**: a page Posta hosts at `https://posta.example.com/f/YOUR_PUBLIC_KEY`. Send it by email, post it in a chat, or put it behind a QR code. Nobody needs a website. You find it at the top of the form's **Embed** tab.

The page shows the form's name and description, then the form itself, through the same [widget](./embedding.md#website-widget) you can embed on a website. Submissions arrive in the inbox like any other, with the same spam protection.

## Prefilled links

Add `?<field key>=<value>` to the link to answer a question in advance:

```
https://posta.example.com/f/YOUR_PUBLIC_KEY?plan=pro&source=newsletter
```

For a multi-select, repeat the key: `?topics=api&topics=smtp`. A value that is not one of the question's options is ignored. Questions marked as personal data (`pii`) are never prefilled, so a link cannot plant someone's details in another person's response. Without a [schema](./fields-and-validation.md), the link can prefill `name`, `email`, `phone`, `subject` and `message`.

## Responses

The form's **Responses** tab, or the same settings in the API:

| Setting | API field | Effect |
|---|---|---|
| Confirmation message | `confirmation_message` | Shown after someone submits. Up to 500 characters. |
| Submit another response | `allow_another_response` | Shows a link that brings the form back after submitting. |
| Stop accepting responses after | `close_at` | An RFC 3339 time. From then on the form is closed. |
| Response limit | `max_responses` | The form closes after this many responses. Spam does not count. `0` means no limit. |
| Closed message | `closed_message` | Shown instead of the form while it is closed. |

A form is **closed** while its status is **Paused**, after `close_at`, or once it reaches `max_responses`. The share page and every embed then show the closed message instead of the form, and a submission sent anyway is answered `403` with that message, so nobody is told "thanks" for a response that was not kept.

## Appearance

`appearance` sets the look of the share page and the default look of embeds:

```json
{ "accent": "#0f766e", "theme": "dark", "header_image_url": "https://example.com/banner.png" }
```

- `accent`: a colour like `#9333ea`;
- `theme`: `light`, `dark`, or `auto` to follow the visitor's system;
- `header_image_url`: an `https` image shown at the top of the share page.

On a website, `data-accent` and `data-theme` on the embed take precedence, so the same form can match different pages.

## For administrators

Public pages on your domain can be abused for phishing, so share pages:

- warn visitors to never submit passwords, and offer a **Report abuse** link;
- offer no password question type;
- are served with a strict Content Security Policy and cannot be framed by other sites;
- are hidden from search engines.

Under **Admin → Settings → Security**:

- **Hosted Form Pages** turns share pages off for the whole platform. Links then answer `404`; embeds on websites keep working.
- **Abuse Report Email** is where the **Report abuse** link sends reports. Empty uses the admin email.

Settings take effect within 30 seconds.
