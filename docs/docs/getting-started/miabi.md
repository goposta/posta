---
sidebar_position: 2.5
title: Deploy on Miabi
description: Run Posta on Miabi from a manifest, or set it up by hand in the Miabi console
---

# Deploy on Miabi

[Miabi](https://miabi.io) runs Posta with a managed PostgreSQL, a dedicated Redis, generated secrets and an HTTPS route with automatic TLS. There are three ways to get there:

| Way | Effort | Choose it when |
|---|---|---|
| [Marketplace template](./managed-posta) | One click | You want Posta running with the least setup |
| [Manifest](#with-a-manifest) | One command | You want the deployment in a file you can review, commit and re-apply |
| [Console, by hand](#by-hand-in-the-console) | About fifteen minutes | You want to see and choose every piece |

All three give you the same self-hosted Posta; everything else in these docs applies unchanged.

## With a manifest

The [`deploy/miabi/`](https://github.com/goposta/posta/tree/main/deploy/miabi) folder holds two `miabi.io/v1` manifests. Each declares the whole deployment as one project, so applying it again converges to the file instead of duplicating anything.

| Manifest | Topology | Use it for |
|---|---|---|
| [`posta.yaml`](https://github.com/goposta/posta/blob/main/deploy/miabi/posta.yaml) | Server and a dedicated worker in one stack, attachments on a volume they share | Small and medium installs, on one node |
| [`posta-replicated.yaml`](https://github.com/goposta/posta/blob/main/deploy/miabi/posta-replicated.yaml) | Server and dedicated workers as replicated services, attachments on S3 | Production on a Miabi cluster |

1. Replace `posta.example.com` with your hostname (it appears three times) and `admin@example.com` with your address.
2. For `posta-replicated.yaml` only, set the S3 bucket settings and seal the secret key:

   ```bash
   echo -n "$S3_SECRET" | miabi secrets seal posta-s3-secret-key
   ```

   Paste the output over the `SealedSecret` value. It is sealed to your workspace's key, so the file is safe to commit.
3. Preview, then apply:

   ```bash
   miabi apply -f posta.yaml --dry-run
   miabi apply -f posta.yaml
   ```

4. Point your hostname's DNS at Miabi. Once the route has its certificate, open `https://posta.example.com`.
5. Reveal the generated `posta-admin-password` in the workspace's **Secrets** and sign in.

The manifests generate the JWT secret, the encryption key and the first admin password into the workspace vault, so no secret is ever written in the file.

:::caution Keep the server at one replica
The worker scales freely, but the API server does not yet: each server replica runs the scheduled jobs and the failed-email retry loop on its own. `posta-replicated.yaml` ships with one server replica; scale `posta-worker` when delivery falls behind.
:::

## By hand in the console

These steps build the same deployment as `posta.yaml`: a `posta` stack holding the server and a dedicated worker. Settings both need go into the stack's shared environment once. Everything happens in your workspace in the Miabi console.

### 1. Create the secrets

Open **Secrets** in the workspace sidebar and create three secrets. Use the **Generator** button on the vault's toolbar, which names and saves the value in one step:

| Name | Generator | Used for |
|---|---|---|
| `posta-jwt-secret` | Token, 32 bytes, hex | Signing sessions. Anyone who has it can forge a login |
| `posta-encryption-key` | Token, 32 bytes, hex | Encrypting stored SMTP passwords |
| `posta-admin-password` | Password, 24 characters | The first admin account |

:::caution
Never rotate `posta-encryption-key` once Posta has stored SMTP servers: their passwords would no longer decrypt.
:::

### 2. Create the database and Redis

From **Data → Databases → New database**, create:

- `posta-db`, engine **PostgreSQL**, version `17-alpine`;
- `posta-redis`, engine **Redis**, version `8-alpine`.

Miabi generates their credentials and keeps them in the vault.

### 3. Create a volume for attachments

From **Data → Volumes → Create volume**, create `posta-data` with a size limit of about 5 GB. The server stores email attachments and inbound message bodies there, and the worker reads them when sending.

### 4. Create the stack and its shared environment

From **Stacks → New stack**, create a stack named `posta`. On its page, under **Shared environment**, add:

| Variable | Value |
|---|---|
| `POSTA_ENV` | `production` |
| `POSTA_WEB_URL` | `https://posta.example.com` (your hostname) |
| `POSTA_JWT_SECRET` | `${{ secrets.posta-jwt-secret }}` |
| `POSTA_ENCRYPTION_KEY` | `${{ secrets.posta-encryption-key }}` |
| `POSTA_DB_SSL_MODE` | `disable` |
| `POSTA_BLOB_PROVIDER` | `filesystem` |
| `POSTA_BLOB_FS_PATH` | `/data/attachments` |

**Import .env** takes the same variables as `KEY=VALUE` lines. The secret references resolve from the vault when each app deploys, so the values themselves never sit in the stack.

The worker needs the same secrets as the server: it signs tracking links with the JWT secret and decrypts SMTP passwords with the encryption key. `POSTA_WEB_URL` is the base of its tracking and unsubscribe links. Posta connects to PostgreSQL without TLS, which is right on the workspace's private network.

To send invitations and password resets, also add the `POSTA_SYSTEM_SMTP_*` variables here (see [Configuration](./configuration)), referencing the password from a secret.

### 5. Create the server

Click **New application**, choose **Docker image**, and set:

- **Image**: `jkaninda/posta`
- **Tag**: a release tag such as the latest on [Docker Hub](https://hub.docker.com/r/jkaninda/posta), rather than `latest`, so a redeploy never upgrades Posta by surprise
- **Container port**: `9000`

Name it `posta`. Then, in the application's **Settings**:

- **Stack**: `posta`.
- **Run as user**: `100:101`, the image's `posta` user. Miabi then makes the volume writable by it; without this, attachments cannot be saved.
- **Healthcheck**: HTTP, path `/healthz`, port `9000`.

In the **Databases** tab, link `posta-db`. Open **Environment variables** in the link dialog and map the fields to Posta's settings, unticking the rest:

| Field | Variable |
|---|---|
| `host` | `POSTA_DB_HOST` |
| `port` | `POSTA_DB_PORT` |
| `name` | `POSTA_DB_NAME` |
| `user` | `POSTA_DB_USER` |
| `password` | `POSTA_DB_PASSWORD` |
| `url`, `database_url` | unticked |

Then link `posta-redis` and map its `url` field to `POSTA_REDIS_URL`, unticking the others.

In the **Environment** tab, add what only the server needs:

| Variable | Value |
|---|---|
| `POSTA_ADMIN_EMAIL` | your email address |
| `POSTA_ADMIN_PASSWORD` | `${{ secrets.posta-admin-password }}` |
| `POSTA_EMBEDDED_WORKER` | `false` |
| `POSTA_TRUSTED_PROXIES` | `10.63.0.0/16` |

`POSTA_EMBEDDED_WORKER=false` leaves delivery to the worker. `POSTA_TRUSTED_PROXIES` lets Posta see visitors' real addresses behind the Miabi gateway, for rate limits and audit logs. `10.63.0.0/16` is the gateway network's default subnet; use your `MIABI_SUBNET` if your Miabi install sets one.

In the **Volumes** tab, attach `posta-data` at mount path `/data`.

### 6. Create the worker

Create a second **Docker image** application named `posta-worker`, with the same image and the same tag as the server. Keep the two tags in step when you upgrade. In its **Settings**:

- **Stack**: `posta`.
- **Command**: `worker`.
- **Run as user**: `100:101`.
- **Healthcheck**: HTTP, path `/healthz`, port `9000`.

Link `posta-db` and `posta-redis` with the same variable mapping as the server. In the **Environment** tab, add:

| Variable | Value |
|---|---|
| `POSTA_WORKER_HEALTH_ENABLED` | `true` |
| `POSTA_WORKER_HEALTH_PORT` | `9000` |
| `POSTA_WORKER_CONCURRENCY` | `10` |

Attach `posta-data` at `/data` here too. Both apps then run on the volume's node. The worker needs no route.

### 7. Deploy and add the route

On the stack's page, click **Deploy all**. When both applications are healthy:

1. Add your domain under **Networking → Domains** and complete its DNS verification.
2. In the `posta` application's **Routes** tab, create a route for your hostname on port `9000`. Miabi issues the TLS certificate automatically.

### 8. Sign in

Reveal `posta-admin-password` in **Secrets**, open your hostname, and sign in with `POSTA_ADMIN_EMAIL` and that password. The admin account is created only on the first start against an empty database.

### Going further

- **Changing a shared setting.** Edit it under the stack's **Shared environment**. Both apps are then marked **Redeploy required**, and the stack page redeploys just those.
- **Replicas across a cluster.** A volume belongs to one node, so replicas on several nodes need attachments on S3. [`posta-replicated.yaml`](https://github.com/goposta/posta/blob/main/deploy/miabi/posta-replicated.yaml) has that setup ready to apply.
- **Receiving email.** On the server, set `POSTA_INBOUND_ENABLED=true` and `POSTA_INBOUND_HOSTNAME` to the host your MX record names, and publish container port `2525` on the node's port `25`. See [Inbound email](/docs/inbound/overview).
- **Upgrading.** Change the image tag on both applications and click **Deploy all**. Posta migrates its database when the server starts.
