---
sidebar_position: 3
title: Embedding a form
description: Put a Posta form on your website with plain HTML or JavaScript
---

# Embedding a Form

The **Embed** tab of a form gives you paste-ready code. Both variants below post to the same endpoint.

## Plain HTML, no JavaScript

```html
<form action="https://posta.example.com/api/v1/f/YOUR_PUBLIC_KEY" method="POST">
  <label for="posta-name">Name</label>
  <input id="posta-name" type="text" name="name" required>

  <label for="posta-email">Email</label>
  <input id="posta-email" type="email" name="email" required>

  <label for="posta-phone">Phone (optional)</label>
  <input id="posta-phone" type="tel" name="phone" autocomplete="tel">

  <label for="posta-message">Message</label>
  <textarea id="posta-message" name="message" required></textarea>

  <div style="position:absolute;left:-9999px" aria-hidden="true">
    <input type="text" name="_gotcha" tabindex="-1" autocomplete="off">
  </div>

  <input type="hidden" name="_redirect" value="https://example.com/thanks">

  <button type="submit">Send</button>
</form>
```

The honeypot div is off-screen and `aria-hidden`, so it is invisible to sighted users and to screen readers, but a naive bot fills it in and gets rejected.

## JavaScript

```js
const res = await fetch('https://posta.example.com/api/v1/f/YOUR_PUBLIC_KEY', {
  method: 'POST',
  headers: { 'Content-Type': 'text/plain;charset=UTF-8' },
  body: JSON.stringify({
    name: form.name.value,
    email: form.email.value,
    phone: form.phone.value,
    message: form.message.value,
  }),
})
const { data } = await res.json()
```

:::important
Note the `text/plain;charset=UTF-8` content type. It is deliberate, and the generated snippet uses it.

`Content-Type: application/json` makes the request *preflighted*: the browser sends an `OPTIONS` request first, and Posta answers preflights from the deployment-wide `POSTA_CORS_ORIGINS` allowlist — which will not contain your customers' websites. `text/plain` is a CORS-safelisted content type, so the browser skips the preflight and posts directly. Posta parses a `text/plain` body as JSON.

The same applies to `application/x-www-form-urlencoded` and `multipart/form-data`: both are safelisted and post without a preflight.

`application/json` still works fine for server-to-server calls, where CORS never applies.
:::

Posta echoes your form's origin back in `Access-Control-Allow-Origin` on the response, so your script can read the result.

## With a nonce

When the form has `require_nonce` enabled, fetch a token first and submit it as `_nonce`:

```js
const { data: nonce } = await (
  await fetch('https://posta.example.com/api/v1/f/YOUR_PUBLIC_KEY/nonce')
).json()

// …later, when the visitor submits
await fetch('https://posta.example.com/api/v1/f/YOUR_PUBLIC_KEY', {
  method: 'POST',
  headers: { 'Content-Type': 'text/plain;charset=UTF-8' },
  body: JSON.stringify({ ...values, _nonce: nonce.nonce }),
})
```

Fetch the nonce when the form is rendered, not when it is submitted — `min_fill_seconds` rejects a token that is younger than the configured age, which is what catches a bot that fills and posts instantly. Each nonce is single-use and expires after 30 minutes.

## Attachments

Set `allow_attachments` on the form and post `multipart/form-data`:

```html
<form action="https://posta.example.com/api/v1/f/YOUR_PUBLIC_KEY"
      method="POST" enctype="multipart/form-data">
  <input type="file" name="resume">
</form>
```

At most 5 files per submission, each within `POSTA_MESSAGES_MAX_ATTACH_SIZE`. Files go to blob storage when it is configured, and are base64-encoded on the record otherwise.

## Reserved field names

These are stripped before storage and never appear on the message:

| Field | Purpose |
|-------|---------|
| `_gotcha` | Honeypot (or whatever the form's `honeypot_field` is set to). |
| `_nonce` | Signed anti-bot token. |
| `_redirect` / `_next` | Post-submit redirect target. |
| `_subject` | Sets the message subject. |
| `_captcha`, `_cc` | Reserved for future use. |

`_redirect` is only honoured when the target host is on the form's origin allowlist or matches the configured `redirect_url` host. Anything else is ignored, because an open redirect on a trusted domain is a phishing primitive.

## Field mapping

Posta extracts well-known fields case-insensitively:

| Target | Field names it looks for |
|--------|--------------------------|
| Sender email | `email`, `e-mail`, `_replyto`, `reply_to`, `replyto`, `from`, `sender_email`, `your-email` |
| Sender name | `name`, `full_name`, `fullname`, `your-name`, `sender_name`, or `first_name` + `last_name` |
| Phone | `phone`, `phone_number`, `phonenumber`, `tel`, `telephone`, `mobile`, `cell`, `your-phone`, `sender_phone` |
| Subject | `_subject`, `subject`, `topic`, `your-subject` |
| Body | `message`, `body`, `comments`, `comment`, `content`, `description`, `your-message` |

If nothing matches, Posta falls back to the first value that parses as an email address and the longest value as the body. Every field is stored regardless, in the order it was submitted.

The phone number is optional and has no fallback: if none of those keys are present, `sender_phone` is empty. Submitted values are normalised down to digits and the separators `+ - ( ) . /` — enough to keep `+33 6 12 34 56 78` and `(555) 010-9999` readable, while dropping anything that could reach an email header. A value with no digits at all is discarded.

A message with no usable address is still stored and notified — but it cannot be replied to, and the dashboard says so.
