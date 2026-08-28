---
sidebar_position: 4
title: Spam filtering
description: How Posta scores web form submissions and how to add your own rules
---

# Spam Filtering

Every submission runs through a scanner that produces a **score**, a list of **reasons**, and an **action**. The form's thresholds map the score onto the action; a rule with an explicit action sets a floor that the score cannot undo.

| Action | Stored | Notified | Webhook | Visible in inbox |
|--------|--------|----------|---------|------------------|
| `allow` | yes | yes | `message.received` | yes |
| `flag` | yes | if `notify_on_flagged` | `message.received` | yes, marked |
| `quarantine` | yes | no | `message.spam` | yes, under the spam filter |
| `reject` | yes | no | no | no |

## Immediate rejections

Three checks bypass scoring entirely and reject outright:

- the honeypot field was filled in
- a required nonce was missing, expired, replayed, or submitted faster than `min_fill_seconds`

The submitter still receives the normal `202` acknowledgement.

## Built-in signals

| Signal | Score |
|--------|-------|
| Sender address is on the workspace suppression list | +5 |
| Sender address does not parse as an email address | +3 |
| Sender domain is a known disposable-mail provider | +4 |
| More than 5 URLs in the subject and body | +1 per extra URL, capped at +6 |
| A known URL shortener appears in the text | +2 |
| Body shorter than 10 characters, or longer than 20,000 | +2 |
| 70% or more uppercase letters over 40+ characters | +2 |
| Raw `<a href>` or BBCode in a plain-text field | +3 |
| `<script>` or `javascript:` in the body | +5 |
| 5 or more submissions from this IP in the last hour | +3 |
| 3 or more submissions from this address on this form in the last hour | +2 |
| A scripted user agent (`curl`, `python-requests`, headless browsers) | +2 |

Each signal that fires adds a named reason to the message, which the dashboard renders as a chip so you can see exactly why something was flagged.

## Your own filters

Add filters under **Messages → Spam Filters**, or via `POST /workspaces/current/message-filters`.

| Kind | Matches |
|------|---------|
| `keyword` | A whole word. `cat` does not match `concatenate`. |
| `phrase` | A substring anywhere. |
| `regex` | A regular expression. Go's `regexp` is RE2 — linear time, no catastrophic backtracking. Patterns are capped at 512 characters and validated when you save. |
| `email` | The sender address, exactly. |
| `domain` | The sender domain, including subdomains. `spam.test` matches `mail.spam.test` but not `notspam.test`. |
| `ip` | The client IP, exactly. |

| Action | Effect |
|--------|--------|
| `score` | Adds `score` to the running total. The default. |
| `flag` / `quarantine` / `reject` | Sets a floor: the message lands at least there, whatever the score. |
| `allowlist` | Short-circuits the entire scan. The message is allowed with score 0. |

Scope a filter to one form with `form_id`, or leave it null to apply it workspace-wide. Restrict it to specific fields with `fields: ["subject"]`; blank checks every field, plus the sender name and address.

`allowlist` is the escape hatch for the customer whose emails keep tripping the link rule. It is evaluated before everything else.

## Testing a rule before you save it

`POST /workspaces/current/message-filters/test` runs a candidate pattern over your recent messages and reports how many it would have matched, with up to 10 samples. The dashboard exposes this as **Test against recent messages** in the new-filter dialog. Use it before adding anything with a `reject` action.

## Feedback loop

Marking a message as spam in the dashboard quarantines it and offers to create a filter from the sender's domain in the same step. Marking a quarantined message as **Not spam** clears the score and reasons and reopens it.

Each filter tracks its own `hit_count` and `last_hit_at`. A rule that has not fired in months is a rule to delete.
