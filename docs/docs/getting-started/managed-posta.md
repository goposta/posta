---
sidebar_position: 2
title: Managed Posta
description: Deploy your own Posta from the Miabi Marketplace, without writing Docker Compose
---

# Managed Posta on Miabi

Posta is available in the **[Miabi Marketplace](https://marketplace.miabi.io/templates/posta)** as a ready-to-run template, deployed with a **dedicated worker**.

This is still self-hosted Posta — your own instance, your data, the same AGPL-3.0 build. What changes is the setup: instead of writing a Docker Compose file, wiring PostgreSQL and Redis, and running a second process for the worker, you deploy from the Marketplace and Miabi provisions and manages it for you.

<div style={{textAlign: 'center', margin: '2rem 0'}}>
  <a href="https://marketplace.miabi.io/templates/posta" className="button button--primary button--lg">
    Deploy Posta on Miabi →
  </a>
</div>

{/* Screenshot: add the image at docs/static/img/screenshots/miabi-posta.png, then uncomment.
<p align="center">
  <img src="/img/screenshots/miabi-posta.png" alt="Posta in the Miabi Marketplace" width="900" />
</p>
*/}

## What the template does for you

- **No Docker Compose to write.** The template carries the deployment: the API server, its database and cache, and the environment wiring you would otherwise assemble by hand.
- **A dedicated worker, included.** Outbound sending is queue-based in Posta, so a separate worker process is what makes queued sending, automatic retries, campaigns, and scheduled jobs work. The template ships it alongside the API server — the same [production topology](./installation#docker-compose--dedicated-worker-production) described below.
- **Miabi runs the deployment.** Provisioning, running, and managing the infrastructure are handled for you.

## It is still your Posta

Same REST API, same dashboard, same [SDKs](/docs/sdks/overview), same [configuration](./configuration) — everything in these docs applies unchanged. Posta stays open source under AGPL-3.0, so nothing about deploying this way locks you in.

## Prefer to wire it up yourself?

Deploy it anywhere Docker or a Go binary runs — see [Installation](./installation) for the Docker Compose examples and build-from-source instructions.
