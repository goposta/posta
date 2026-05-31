---
sidebar_position: 4
title: Health Checks
description: Liveness and readiness probes
---

# Health Checks

Posta provides health check endpoints for container orchestration and load balancers.

## Liveness Probe

```
GET /healthz
```

Returns `200 OK` if the server is running:

```json
{
  "status": "ok",
  "timestamp": "2026-01-01T00:00:00Z"
}
```

## Readiness Probe

```
GET /readyz
```

Checks database and Redis connectivity:

```json
{
  "status": "ok",
  "checks": {
    "database": "ok",
    "redis": "ok"
  }
}
```

Returns `503 Service Unavailable` if any dependency is down.

## Application Info

```
GET /api/v1/info
```

```json
{
  "name": "posta",
  "version": "1.0.0",
  "commit": "abc123"
}
```

## Kubernetes Configuration

```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: 9000
  initialDelaySeconds: 5
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /readyz
    port: 9000
  initialDelaySeconds: 10
  periodSeconds: 15
```

## Docker Compose

```yaml
healthcheck:
  test: ["CMD", "curl", "-f", "http://localhost:9000/healthz"]
  interval: 30s
  timeout: 10s
  retries: 3
```
