---
sidebar_position: 2
title: Installation
description: Deploy Posta with Docker Compose or from source
---

# Installation

Posta can be deployed using Docker Compose (recommended) or built from source.

:::tip
Full Docker Compose examples are available in the [`examples/`](https://github.com/goposta/posta/tree/main/examples) folder of the repository.
:::

## Docker Compose — Embedded Worker (Simple)

This setup runs Posta with the embedded worker enabled. The email processing worker runs inside the main server process, so no separate worker container is required. Ideal for development and small deployments.

See [`examples/compose.yml`](https://github.com/goposta/posta/blob/main/examples/compose.yml)

```yaml
services:
  posta:
    image: jkaninda/posta:latest
    ports:
      - "9000:9000"
    environment:
      POSTA_DB_HOST: posta-db
      POSTA_DB_NAME: posta
      POSTA_DB_USER: posta
      POSTA_DB_PASSWORD: posta
      POSTA_DB_PORT: 5432
      POSTA_DB_SSL_MODE: disable
      POSTA_REDIS_ADDR: "posta-redis:6379"
      POSTA_JWT_SECRET: "change-me-in-production"
      POSTA_ADMIN_EMAIL: "admin@example.com"
      POSTA_ADMIN_PASSWORD: "admin1234"
      POSTA_EMBEDDED_WORKER: "true"
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:9000/healthz"]
      interval: 30s
      timeout: 5s
      start_period: 10s
      retries: 3
    depends_on:
      posta-db:
        condition: service_healthy
      posta-redis:
        condition: service_healthy
    restart: unless-stopped

  posta-db:
    image: postgres:17-alpine
    environment:
      POSTGRES_USER: posta
      POSTGRES_PASSWORD: posta
      POSTGRES_DB: posta
    volumes:
      - db_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U posta"]
      interval: 5s
      timeout: 5s
      retries: 5

  posta-redis:
    image: redis:7-alpine
    volumes:
      - redis_data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 5s
      retries: 5

volumes:
  db_data:
  redis_data:
```

Start the services:

```bash
docker compose up -d
```

Posta will be available at `http://localhost:9000`.

## Docker Compose — Dedicated Worker (Production)

For production environments, run the worker as a separate container. This allows the API server and background processing to scale independently.

See [`examples/docker-compose-full.yml`](https://github.com/goposta/posta/blob/main/examples/docker-compose-full.yml)

```yaml
services:
  posta:
    image: jkaninda/posta:latest
    ports:
      - "9000:9000"
    environment:
      POSTA_DB_HOST: posta-db
      POSTA_DB_NAME: posta
      POSTA_DB_USER: posta
      POSTA_DB_PASSWORD: posta
      POSTA_DB_PORT: 5432
      POSTA_DB_SSL_MODE: disable
      POSTA_REDIS_ADDR: "posta-redis:6379"
      POSTA_JWT_SECRET: "change-me-in-production"
      POSTA_ADMIN_EMAIL: "admin@example.com"
      POSTA_ADMIN_PASSWORD: "admin1234"
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:9000/healthz"]
      interval: 30s
      timeout: 5s
      start_period: 10s
      retries: 3
    depends_on:
      posta-db:
        condition: service_healthy
      posta-redis:
        condition: service_healthy
    restart: unless-stopped

  worker:
    image: jkaninda/posta:latest
    command: ["worker"]
    environment:
      POSTA_DB_HOST: posta-db
      POSTA_DB_NAME: posta
      POSTA_DB_USER: posta
      POSTA_DB_PASSWORD: posta
      POSTA_DB_PORT: 5432
      POSTA_DB_SSL_MODE: disable
      POSTA_REDIS_ADDR: "posta-redis:6379"
      POSTA_WORKER_CONCURRENCY: "10"
      POSTA_WORKER_MAX_RETRIES: "5"
    depends_on:
      posta-db:
        condition: service_healthy
      posta-redis:
        condition: service_healthy
    restart: unless-stopped

  posta-db:
    image: postgres:17-alpine
    environment:
      POSTGRES_USER: posta
      POSTGRES_PASSWORD: posta
      POSTGRES_DB: posta
    volumes:
      - db_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U posta"]
      interval: 5s
      timeout: 5s
      retries: 5

  posta-redis:
    image: redis:7-alpine
    volumes:
      - redis_data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 5s
      retries: 5

volumes:
  db_data:
  redis_data:
```

```bash
docker compose -f docker-compose-full.yml up -d
```

You can run multiple worker instances for horizontal scaling. All workers share the same Redis queue.

## Build from Source

### Prerequisites

- Go 1.25+
- PostgreSQL 14+
- Redis 7+
- Node.js 18+ (for building the dashboard)

### Steps

```bash
# Clone the repository
git clone https://github.com/goposta/posta.git
cd posta

# Build the binary
make build

# Run the server
./bin/posta server
```

For standalone worker mode:

```bash
# API server
./bin/posta server

# Worker (separate process)
./bin/posta worker
```

## Health Checks

Once running, verify the deployment:

```bash
# Liveness probe
curl http://localhost:9000/api/v1/healthz

# Readiness probe (checks DB + Redis)
curl http://localhost:9000/api/v1/readyz
```
