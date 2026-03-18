/*
 * Copyright 2026 Jonas Kaninda
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/jkaninda/okapi"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "posta_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "posta_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	emailsSentTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "posta_emails_sent_total",
			Help: "Total number of emails sent successfully",
		},
	)

	emailsFailedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "posta_emails_failed_total",
			Help: "Total number of emails that failed to send",
		},
	)

	emailsQueuedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "posta_emails_queued_total",
			Help: "Total number of emails enqueued for delivery",
		},
	)

	emailRetriesTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "posta_email_retries_total",
			Help: "Total number of email retry attempts",
		},
	)

	webhookDeliveriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "posta_webhook_deliveries_total",
			Help: "Total number of webhook delivery attempts by status",
		},
		[]string{"status"},
	)

	webhookDeliveryDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "posta_webhook_delivery_duration_seconds",
			Help:    "Duration of webhook delivery attempts in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)

	bouncesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "posta_bounces_total",
			Help: "Total number of bounces recorded by type",
		},
		[]string{"type"},
	)

	suppressionsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "posta_suppressions_total",
			Help: "Total number of email suppressions added",
		},
	)
)

func init() {
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpRequestDuration)
	prometheus.MustRegister(emailsSentTotal)
	prometheus.MustRegister(emailsFailedTotal)
	prometheus.MustRegister(emailsQueuedTotal)
	prometheus.MustRegister(emailRetriesTotal)
	prometheus.MustRegister(webhookDeliveriesTotal)
	prometheus.MustRegister(webhookDeliveryDuration)
	prometheus.MustRegister(bouncesTotal)
	prometheus.MustRegister(suppressionsTotal)
}

// IncrementEmailSent increments the emails sent counter.
func IncrementEmailSent() {
	emailsSentTotal.Inc()
}

// IncrementEmailFailed increments the emails failed counter.
func IncrementEmailFailed() {
	emailsFailedTotal.Inc()
}

// IncrementEmailQueued increments the emails queued counter.
func IncrementEmailQueued() {
	emailsQueuedTotal.Inc()
}

// IncrementEmailRetry increments the email retries counter.
func IncrementEmailRetry() {
	emailRetriesTotal.Inc()
}

// IncrementWebhookDelivery increments the webhook delivery counter for the given status.
func IncrementWebhookDelivery(status string) {
	webhookDeliveriesTotal.WithLabelValues(status).Inc()
}

// ObserveWebhookDeliveryDuration records a webhook delivery duration.
func ObserveWebhookDeliveryDuration(seconds float64) {
	webhookDeliveryDuration.Observe(seconds)
}

// IncrementBounce increments the bounce counter for the given type (hard, soft, complaint).
func IncrementBounce(bounceType string) {
	bouncesTotal.WithLabelValues(bounceType).Inc()
}

// IncrementSuppression increments the suppression counter.
func IncrementSuppression() {
	suppressionsTotal.Inc()
}

// PrometheusMiddleware records HTTP request metrics.
func PrometheusMiddleware() okapi.Middleware {
	return func(c *okapi.Context) error {
		start := time.Now()

		err := c.Next()

		duration := time.Since(start).Seconds()
		method := c.Request().Method
		path := c.Path()
		status := strconv.Itoa(c.Response().StatusCode())

		httpRequestsTotal.WithLabelValues(method, path, status).Inc()
		httpRequestDuration.WithLabelValues(method, path).Observe(duration)

		return err
	}

}

// MetricsHandler returns the Prometheus metrics handler.
func MetricsHandler() okapi.HandlerFunc {
	handler := promhttp.Handler()
	return func(c *okapi.Context) error {
		handler.ServeHTTP(c.Response().(http.ResponseWriter), c.Request())
		return nil
	}
}
