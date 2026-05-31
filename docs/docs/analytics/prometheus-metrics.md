---
sidebar_position: 3
title: Prometheus Metrics
description: Production monitoring with Prometheus
---

# Prometheus Metrics

Posta exposes Prometheus-compatible metrics for production monitoring when enabled. The metrics endpoint is served at the root path — not under `/api/v1`.

```
GET /metrics
```

## Enabling Metrics

```bash
POSTA_METRICS_ENABLED=true
```

## Available Metrics

| Metric | Type | Description |
|--------|------|-------------|
| HTTP request count | Counter | Total HTTP requests by method, path, status |
| HTTP request duration | Histogram | Request latency by endpoint |
| Emails sent | Counter | Total emails successfully sent |
| Emails failed | Counter | Total email delivery failures |
| Emails queued | Counter | Total emails added to queue |
| Emails retried | Counter | Total email retry attempts |
| Webhook deliveries | Counter | Webhook delivery attempts (success/failed) |
| Webhook duration | Histogram | Webhook delivery latency |

## Scrape Configuration

Add Posta to your Prometheus configuration:

```yaml
scrape_configs:
  - job_name: 'posta'
    static_configs:
      - targets: ['posta:9000']
    metrics_path: '/metrics'
```

## Grafana Dashboard

Use the exposed metrics to build Grafana dashboards for:

- Email delivery rate over time
- Failure rate and error trends
- Queue depth and processing latency
- Webhook delivery reliability
- API request volume and latency
