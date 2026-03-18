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

package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/jkaninda/logger"

	"github.com/jkaninda/posta/internal/models"
	"github.com/jkaninda/posta/internal/storage/repositories"
)

const (
	defaultMaxRetries    = 3
	defaultRetryBaseWait = 2 * time.Second
	defaultTimeout       = 10 * time.Second
)

// Config holds configurable settings for the webhook dispatcher.
type Config struct {
	MaxRetries int
	Timeout    time.Duration
}

type Dispatcher struct {
	repo              *repositories.WebhookRepository
	deliveryRepo      *repositories.WebhookDeliveryRepository
	client            *http.Client
	maxRetries        int
	onDeliverySuccess func()
	onDeliveryFailed  func()
	onDeliveryDone    func(float64) // duration in seconds
}

func NewDispatcher(repo *repositories.WebhookRepository) *Dispatcher {
	return &Dispatcher{
		repo:       repo,
		maxRetries: defaultMaxRetries,
		client: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// SetConfig applies configurable settings to the dispatcher.
func (d *Dispatcher) SetConfig(cfg Config) {
	if cfg.MaxRetries > 0 {
		d.maxRetries = cfg.MaxRetries
	}
	if cfg.Timeout > 0 {
		d.client.Timeout = cfg.Timeout
	}
}

// SetDeliveryRepo sets the repository used to record delivery attempts.
func (d *Dispatcher) SetDeliveryRepo(repo *repositories.WebhookDeliveryRepository) {
	d.deliveryRepo = repo
}

// OnDeliverySuccess sets a callback invoked after each successful webhook delivery.
func (d *Dispatcher) OnDeliverySuccess(fn func()) { d.onDeliverySuccess = fn }

// OnDeliveryFailed sets a callback invoked after each failed webhook delivery.
func (d *Dispatcher) OnDeliveryFailed(fn func()) { d.onDeliveryFailed = fn }

// OnDeliveryDone sets a callback invoked after each delivery attempt with its duration.
func (d *Dispatcher) OnDeliveryDone(fn func(float64)) { d.onDeliveryDone = fn }

type Payload struct {
	Event     string `json:"event"`
	EmailID   string `json:"email_id"`
	Timestamp string `json:"timestamp"`
}

// Dispatch sends webhook notifications for the given event. Runs asynchronously.
func (d *Dispatcher) Dispatch(userID uint, event string, emailID string, from string) {
	go func() {
		webhooks, err := d.repo.FindByUserIDAndEvent(userID, event)
		if err != nil {
			logger.Error("failed to find webhooks", "error", err)
			return
		}

		payload := Payload{
			Event:     event,
			EmailID:   emailID,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}

		body, err := json.Marshal(payload)
		if err != nil {
			logger.Error("failed to marshal webhook payload", "error", err)
			return
		}

		for _, wh := range webhooks {
			if !matchesFilters(wh.Filters, from) {
				continue
			}
			d.deliverWithRetry(userID, wh, body, event)
		}
	}()
}

// matchesFilters checks if the sender address matches the webhook's filters.
func matchesFilters(filters []string, from string) bool {
	if len(filters) == 0 {
		return true
	}
	if from == "" {
		return true
	}

	addr := from
	if parsed, err := mail.ParseAddress(from); err == nil {
		addr = parsed.Address
	}
	addr = strings.ToLower(addr)

	domain := ""
	if parts := strings.SplitN(addr, "@", 2); len(parts) == 2 {
		domain = parts[1]
	}

	for _, f := range filters {
		f = strings.ToLower(strings.TrimSpace(f))
		if f == "" {
			continue
		}
		if f == addr {
			return true
		}
		if domain != "" && f == domain {
			return true
		}
	}
	return false
}

// deliverWithRetry attempts to deliver a webhook with exponential backoff retries.
func (d *Dispatcher) deliverWithRetry(userID uint, wh models.Webhook, body []byte, event string) {
	var lastErr string
	var lastStatus int

	start := time.Now()

	for attempt := 1; attempt <= d.maxRetries; attempt++ {
		statusCode, err := d.doRequest(wh.URL, wh.Secret, body)
		lastStatus = statusCode

		if err == nil && statusCode < 400 {
			d.recordDelivery(userID, wh.ID, event, models.WebhookDeliverySuccess, statusCode, "", attempt)
			if d.onDeliverySuccess != nil {
				d.onDeliverySuccess()
			}
			if d.onDeliveryDone != nil {
				d.onDeliveryDone(time.Since(start).Seconds())
			}
			logger.Info("webhook delivered", "url", wh.URL, "event", event, "attempt", attempt)
			return
		}

		if err != nil {
			lastErr = err.Error()
			logger.Warn("webhook delivery failed", "url", wh.URL, "attempt", attempt, "error", err)
		} else {
			lastErr = fmt.Sprintf("HTTP %d", statusCode)
			logger.Warn("webhook returned error status", "url", wh.URL, "attempt", attempt, "status", statusCode)
		}

		if attempt < d.maxRetries {
			time.Sleep(defaultRetryBaseWait * time.Duration(attempt))
		}
	}

	// All retries exhausted
	d.recordDelivery(userID, wh.ID, event, models.WebhookDeliveryFailed, lastStatus, lastErr, d.maxRetries)
	if d.onDeliveryFailed != nil {
		d.onDeliveryFailed()
	}
	if d.onDeliveryDone != nil {
		d.onDeliveryDone(time.Since(start).Seconds())
	}
}

// signPayload computes an HMAC-SHA256 signature of body using the given secret.
func signPayload(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// doRequest sends a single HTTP POST to the webhook URL with an HMAC signature.
func (d *Dispatcher) doRequest(url string, secret string, body []byte) (int, error) {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Posta-Webhook/1.0")
	if secret != "" {
		req.Header.Set("X-Posta-Signature", "sha256="+signPayload(secret, body))
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return 0, err
	}
	_ = resp.Body.Close()
	return resp.StatusCode, nil
}

// recordDelivery persists a webhook delivery record if the delivery repo is configured.
func (d *Dispatcher) recordDelivery(userID, webhookID uint, event string, status models.WebhookDeliveryStatus, httpStatus int, errMsg string, attempt int) {
	if d.deliveryRepo == nil {
		return
	}
	delivery := &models.WebhookDelivery{
		WebhookID:      webhookID,
		UserID:         userID,
		Event:          event,
		Status:         status,
		HTTPStatusCode: httpStatus,
		ErrorMessage:   errMsg,
		Attempt:        attempt,
	}
	if err := d.deliveryRepo.Create(delivery); err != nil {
		logger.Error("failed to record webhook delivery", "error", err)
	}
}

// Validate ensures the webhook URL is reachable.
func (d *Dispatcher) Validate(url string) error {
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return fmt.Errorf("invalid webhook URL: %w", err)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook URL unreachable: %w", err)
	}
	_ = resp.Body.Close()

	return nil
}
