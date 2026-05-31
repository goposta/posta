---
sidebar_position: 2
title: Contact Lists
description: Why contacts are not manually grouped, and where to manage mailing lists
---

# Contact Lists

Posta does **not** provide a "contact list" resource. [Contacts](/docs/contacts/contact-management) are auto-tracked recipients — they are created and updated by the system as you send mail, and are read-only. You cannot manually create lists of contacts or add/remove members.

If you are looking to group recipients, you almost certainly want one of the dedicated resources below.

## Reusable mailing lists → Subscriber Lists

To build audiences you can target with campaigns and let people subscribe to or unsubscribe from, use **Subscriber Lists**. Subscribers are first-class records (with custom attributes and subscription state), and lists can be static or dynamic (segment-based).

See [Subscriber Lists](/docs/subscribers/subscriber-lists) and [Subscriber Management](/docs/subscribers/subscriber-management).

## Opt-out groups → Unsubscribe Lists

To give recipients a granular way to opt out of one category of mail (for example "Product updates") without blocking everything, use **Unsubscribe Lists**. A send references an unsubscribe list so Posta can mint a one-click link that suppresses the recipient on that list only.

See [Unsubscribe Lists](/docs/contacts/unsubscribe-lists).

## Blocking individual addresses → Suppression List

To stop sending to a specific address (globally or for one unsubscribe list), use the [Suppression List](/docs/contacts/suppression-list). Hard bounces and complaints add addresses here automatically.

:::note
There is no `contact-lists` API. Contacts are tracked automatically; grouping and opt-out are handled by Subscriber Lists and Unsubscribe Lists.
:::
