// Copyright (c) 2026 Jonas Kaninda.
//
// This file is part of Posta.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/jkaninda/okapi"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const labelStatus = "status"

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "posta_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", labelStatus},
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

	emailsQueueSize = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "posta_emails_queue_size",
			Help: "Number of emails currently enqueued for delivery",
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
		[]string{labelStatus},
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

	inboundReceivedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "posta_inbound_messages_received_total",
			Help: "Total number of inbound messages accepted for processing, by source",
		},
		[]string{"source"},
	)

	messagesReceivedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "posta_form_messages_received_total",
			Help: "Total number of web form messages stored, by scan status",
		},
		[]string{labelStatus},
	)

	messageNotificationsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "posta_form_message_notifications_total",
			Help: "Total number of web form message notifications delivered",
		},
	)

	inboundForwardedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "posta_inbound_messages_forwarded_total",
			Help: "Total number of inbound messages successfully dispatched to subscribers",
		},
	)

	inboundFailedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "posta_inbound_messages_failed_total",
			Help: "Total number of inbound messages that permanently failed to forward",
		},
	)

	inboundRejectedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "posta_inbound_messages_rejected_total",
			Help: "Total number of inbound messages rejected at ingestion, by reason",
		},
		[]string{"reason"},
	)

	inboundBytesTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "posta_inbound_bytes_total",
			Help: "Total bytes of raw inbound messages accepted",
		},
	)

	inboundIngestDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "posta_inbound_ingest_duration_seconds",
			Help:    "Time spent ingesting an inbound message (parse + blob upload + DB write)",
			Buckets: prometheus.DefBuckets,
		},
	)

	activeWorkers = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "posta_active_workers",
			Help: "Number of currently-connected Asynq workers (embedded + standalone)",
		},
	)
)

func init() {
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpRequestDuration)
	prometheus.MustRegister(emailsSentTotal)
	prometheus.MustRegister(emailsFailedTotal)
	prometheus.MustRegister(emailsQueueSize)
	prometheus.MustRegister(emailRetriesTotal)
	prometheus.MustRegister(webhookDeliveriesTotal)
	prometheus.MustRegister(webhookDeliveryDuration)
	prometheus.MustRegister(bouncesTotal)
	prometheus.MustRegister(suppressionsTotal)
	prometheus.MustRegister(inboundReceivedTotal)
	prometheus.MustRegister(inboundForwardedTotal)
	prometheus.MustRegister(inboundFailedTotal)
	prometheus.MustRegister(inboundRejectedTotal)
	prometheus.MustRegister(inboundBytesTotal)
	prometheus.MustRegister(inboundIngestDuration)
	prometheus.MustRegister(messagesReceivedTotal)
	prometheus.MustRegister(messageNotificationsTotal)
	prometheus.MustRegister(activeWorkers)
}

func IncrementEmailSent() {
	emailsSentTotal.Inc()
}

func IncrementEmailFailed() {
	emailsFailedTotal.Inc()
}

func IncrementEmailQueued() {
	emailsQueueSize.Inc()
}

// DecrementEmailQueued is called when a queued email finishes delivery (sent or permanently failed).
func DecrementEmailQueued() {
	emailsQueueSize.Dec()
}

func IncrementEmailRetry() {
	emailRetriesTotal.Inc()
}

func IncrementWebhookDelivery(status string) {
	webhookDeliveriesTotal.WithLabelValues(status).Inc()
}

func ObserveWebhookDeliveryDuration(seconds float64) {
	webhookDeliveryDuration.Observe(seconds)
}

func IncrementBounce(bounceType string) {
	bouncesTotal.WithLabelValues(bounceType).Inc()
}

func IncrementSuppression() {
	suppressionsTotal.Inc()
}

func IncrementMessageReceived(status string) {
	messagesReceivedTotal.WithLabelValues(status).Inc()
}

func IncrementMessageNotification() {
	messageNotificationsTotal.Inc()
}

func IncrementInboundReceived(source string) {
	inboundReceivedTotal.WithLabelValues(source).Inc()
}

func IncrementInboundForwarded() {
	inboundForwardedTotal.Inc()
}

func IncrementInboundFailed() {
	inboundFailedTotal.Inc()
}

func IncrementInboundRejected(reason string) {
	inboundRejectedTotal.WithLabelValues(reason).Inc()
}

func AddInboundBytes(n int64) {
	if n > 0 {
		inboundBytesTotal.Add(float64(n))
	}
}

func ObserveInboundIngestDuration(seconds float64) {
	inboundIngestDuration.Observe(seconds)
}

func SetActiveWorkers(n int) {
	activeWorkers.Set(float64(n))
}

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

// HTTPHandler serves metrics on a plain net/http mux, for the dedicated worker which has no okapi router.
func HTTPHandler() http.Handler {
	return promhttp.Handler()
}

func MetricsHandler() okapi.HandlerFunc {
	handler := promhttp.Handler()
	return func(c *okapi.Context) error {
		handler.ServeHTTP(c.Response().(http.ResponseWriter), c.Request())
		return nil
	}
}
