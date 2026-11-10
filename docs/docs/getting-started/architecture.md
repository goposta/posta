---
sidebar_position: 6
title: Architecture
description: How Posta is put together, what each process does, and how to deploy it
---

# Architecture

Posta ships as a **single binary that runs in two modes**. `posta` serves the
HTTP API, the dashboard, the website widgets, and the SMTP listeners; `posta worker`
consumes the job queue. Both are stateless — all state lives in PostgreSQL, Redis,
and object storage — so you can run as many servers and workers as your traffic and
send volume need.

```mermaid
flowchart LR
    App["Your app<br/>REST API / SMTP client"]
    MX["Inbound mail<br/>MX delivery"]
    Site["Your website<br/>forms, surveys and signups<br/>live chat widget"]
    Team["Your team<br/>dashboard · chat inbox"]

    subgraph posta["Posta"]
        Server["<b>posta</b><br/>HTTP API :9000 · Dashboard<br/>Form and chat APIs · widget scripts<br/>Inbound SMTP :2525 · Relay :2526"]
        W1["<b>worker 1</b><br/>asynq consumer<br/>probes + metrics :9000"]
        W2["<b>worker 2</b><br/>asynq consumer<br/>probes + metrics :9000"]
    end

    subgraph shared["Shared state"]
        Redis[("Redis<br/>Asynq queues + scheduler<br/>realtime pub/sub")]
        PG[("PostgreSQL<br/>emails · templates · contacts<br/>messages · responses · chats")]
        Blob[("Object storage<br/>S3-compatible or filesystem<br/>attachments · raw inbound · chat files")]
    end

    Providers["SMTP providers<br/>outbound delivery"]
    Hooks["Your webhook<br/>endpoints"]

    App --> Server
    MX --> Server
    Site <-->|"posts · SSE"| Server
    Team <-->|SSE| Server
    Server -- enqueue --> Redis
    Server <-->|pub/sub| Redis
    Redis -- dequeue --> W1
    Redis -- dequeue --> W2
    Server <--> PG
    W1 <--> PG
    W2 <--> PG
    Server -.-> Blob
    W1 & W2 -.-> Blob
    W1 & W2 --> Providers
    W1 & W2 --> Hooks
```

## The two processes

### `posta` — the server

Accepts work and answers questions. It never sends mail itself.

| Listener | Default port | Purpose |
|----------|--------------|---------|
| HTTP | `9000` | REST API, dashboard, web view, tracking pixels, form ingest, chat visitor API, widget scripts |
| Inbound SMTP | `2525` | Receives mail delivered to your MX records |
| SMTP relay | `2526` | Accepts mail from an existing SMTP client |

Everything that takes time — delivery, retries, campaign batches, inbound
parsing, webhook fan-out — is written to PostgreSQL and enqueued in Redis. The
request returns as soon as the job is durable, which is why a send is answered in
milliseconds regardless of how slow the receiving mail server is.

### `posta worker` — the consumer

Pulls jobs off the queue and does the slow work: opening SMTP connections,
retrying failures, walking campaign recipient lists, parsing inbound messages,
delivering webhooks, and running scheduled jobs such as retention cleanup and
daily reports.

Workers are interchangeable. Adding one increases throughput; losing one leaves
its in-flight jobs to be retried by another.

## Website widgets

Forms and live chat run on your own site from scripts Posta serves, with a public
key per form or widget and no API key on the page. Both are off by default, and
both check the page's origin against each form's or widget's allowlist.

### Forms, surveys and signups

`/widget/v1/posta-form.js` renders a form, or a plain HTML form posts to
`/api/v1/f/{key}`. The server scores each submission for spam and stores it:
contact messages land in the shared inbox, while surveys, registrations and
signups are kept as responses and summarised per question. Notifications, replies
and webhooks go out through the worker. Enable with `POSTA_MESSAGES_ENABLED`.

See [Web forms](/docs/messages/overview).

### Live chat

`/widget/v1/posta-chat.js` talks to the visitor API under `/api/v1/chat/{key}`.
Visitors and agents follow a conversation over server-sent events. Any server can
hold the stream: messages, typing and presence travel between servers through
Redis pub/sub, so a visitor connected to one replica sees an agent typing on
another. Replies to visitors who have left, and alerts about visitors waiting for
an answer, are emailed by the worker. Enable with `POSTA_CHAT_ENABLED`.

Each server holds at most `POSTA_CHAT_MAX_STREAMS` open streams; over it, widgets
fall back to polling. A reverse proxy in front of Posta must not buffer the stream
responses. See [Live chat](/docs/chat/overview).

## Running the worker

An embedded worker (`POSTA_EMBEDDED_WORKER=true`) is fine for evaluation and
small installs — one container, nothing else to run. Production deployments
should split them, so sending, retries, campaigns, and scheduled jobs scale
independently of request traffic and a burst of API calls cannot starve delivery.

See [`deploy/compose/docker-compose-full.yml`](https://github.com/goposta/posta/blob/main/deploy/compose/docker-compose-full.yml)
for a two-process deployment.

:::warning
Run **at least one** worker. With no worker and no embedded worker, the API
accepts sends and returns `queued` — and nothing ever delivers them. Everything
looks healthy while the queue grows.
:::

### Health and metrics

A dedicated worker serves the same probes as the server, on the same port
(`POSTA_PORT`, default `9000`). Server and worker run as separate containers, so
one port and one health check command cover both.

| Endpoint | Meaning |
|----------|---------|
| `/healthz` | The process is alive. Checks nothing else on purpose — a liveness probe that fails during a database blip gets the worker restarted, which loses in-flight work and does not fix the database. |
| `/readyz` | Dependencies are reachable **and** the worker is consuming. Returns `503` otherwise, so an orchestrator stops routing to a worker that has stopped processing. |
| `/metrics` | Prometheus exposition for this process. |

The image ships a health check that covers both roles:

```yaml
healthcheck:
  test: ["CMD", "posta-healthcheck"]
  interval: 30s
  timeout: 5s
  start_period: 10s
  retries: 3
```

Set `POSTA_WORKER_HEALTH_ENABLED=false` to disable the listener, or
`POSTA_WORKER_HEALTH_PORT` to move it.

## Shared state

| Store | Holds | Required |
|-------|-------|----------|
| **PostgreSQL** | Emails, templates, contacts, subscribers, campaigns, domains, form messages and responses, chat conversations, logs, audit trail | Yes |
| **Redis** | Asynq queues, the scheduler, caches, rate-limit counters, agent presence, realtime pub/sub between servers | Yes |
| **Object storage** | Attachments, raw inbound messages, chat files and widget images | No |

Object storage is optional. With `POSTA_BLOB_PROVIDER` unset, attachments and raw
inbound messages stay in the database.

:::caution
Any multi-node deployment should use `s3`. A local `fs` path is not shared
between the server and its workers, so a worker on another host cannot read an
attachment the server wrote.
:::

## Queues

Jobs are split across three Asynq queues so that a large campaign cannot delay a
password reset:

| Queue | Weight | Carries |
|-------|--------|---------|
| `transactional` | 6 | API and relayed sends, inbound parsing and forwarding, form messages |
| `bulk` | 3 | Campaign starts, campaign batches, and the individual campaign sends they produce |
| `low` | 1 | Per-workspace daily reports |

The weights are relative, not exclusive: a worker with nothing transactional to
do will happily process bulk. Scheduled maintenance such as retention cleanup
runs on the cron scheduler in-process rather than through a queue.

## Stack

- **Backend** — Go, using the [Okapi](https://github.com/jkaninda/okapi) web framework
- **Frontend** — Vue 3 + Vite, embedded into the binary
- **Database** — PostgreSQL
- **Queue** — Redis with [Asynq](https://github.com/hibiken/asynq)
- **Metrics** — Prometheus

## Next steps

- [Installation](/docs/getting-started/installation) — deploy with Docker or from source
- [Configuration](/docs/getting-started/configuration) — every environment variable
- [Quick Start](/docs/getting-started/quickstart) — send your first email
- [Screenshots](/docs/getting-started/screenshots) — a tour of the dashboard and the website widgets
