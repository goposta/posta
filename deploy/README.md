# Deploy

| Example | Topology | Use it for |
|---|---|---|
| [`compose/compose.yml`](compose/compose.yml) | One container, embedded worker | Development, small installs |
| [`compose/docker-compose-full.yml`](compose/docker-compose-full.yml) | Server + dedicated worker | Production on a single Docker host |
| [`miabi/posta.yaml`](miabi/posta.yaml) | One app, embedded worker, managed Postgres/Redis, TLS route | Small and medium installs on [Miabi](https://miabi.io) |
| [`miabi/posta-replicated.yaml`](miabi/posta-replicated.yaml) | Replicated server and workers as Swarm services, S3 attachments | Production on a Miabi cluster |

## Docker Compose

```sh
cd compose
export POSTA_JWT_SECRET=$(openssl rand -hex 32)
export POSTA_ADMIN_PASSWORD='a-strong-password'
docker compose up -d                                 # embedded worker
docker compose -f docker-compose-full.yml up -d      # dedicated worker
```

[`compose/.env.example`](compose/.env.example) lists every setting. Copy it to `compose/.env` to keep
values out of your shell history.

## Miabi

```sh
cd miabi
miabi apply -f posta.yaml --dry-run    # preview the plan
miabi apply -f posta.yaml              # converge the workspace
```

Both manifests generate the JWT secret, encryption key and initial admin password into the workspace
vault. Reveal `posta-admin-password` there after the first apply.

To set the same deployment up by hand in the Miabi console instead, follow
[Deploy on Miabi](https://docs.goposta.dev/docs/getting-started/miabi) in the docs.

## Scaling

The worker is stateless and scales freely: every instance consumes the same Redis queue.

The API server does not scale freely yet. Each server process runs the scheduled jobs and the
failed-email retry loop without a lock, so extra server replicas repeat them. Keep the server at one
replica and add workers. `miabi/posta-replicated.yaml` ships that way.
