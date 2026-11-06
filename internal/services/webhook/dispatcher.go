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

package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/jkaninda/logger"
	"golang.org/x/net/http/httpproxy"
	"golang.org/x/net/proxy"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/netguard"
	"github.com/goposta/posta/internal/storage/repositories"
)

const (
	defaultMaxRetries    = 3
	defaultRetryBaseWait = 2 * time.Second
	defaultTimeout       = 10 * time.Second
)

type Config struct {
	MaxRetries int
	Timeout    time.Duration
	// ProxyURL hides the server's IP from webhook endpoints; supports http, https and socks5.
	ProxyURL string
	// Network's zero value blocks private and reserved addresses.
	Network netguard.Policy
}

type Dispatcher struct {
	repo              *repositories.WebhookRepository
	deliveryRepo      *repositories.WebhookDeliveryRepository
	client            *http.Client
	network           netguard.Policy
	proxied           bool // a proxy dials the target, so the dialer cannot check it
	maxRetries        int
	onDeliverySuccess func()
	onDeliveryFailed  func()
	onDeliveryDone    func(float64) // duration in seconds
}

func NewDispatcher(repo *repositories.WebhookRepository) *Dispatcher {
	d := &Dispatcher{
		repo:       repo,
		maxRetries: defaultMaxRetries,
		client: &http.Client{
			Timeout: defaultTimeout,
			// A redirect could send the request somewhere the URL check never saw, so it is not followed.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
	d.client.Transport = d.guardedTransport()
	return d
}

// guardedTransport checks each dialed address, so DNS can't sneak a private target past; env proxies stay honoured.
func (d *Dispatcher) guardedTransport() *http.Transport {
	env := httpproxy.FromEnvironment()
	proxyFor := env.ProxyFunc()
	proxies := map[string]bool{}
	for _, raw := range []string{env.HTTPProxy, env.HTTPSProxy} {
		if u, err := url.Parse(raw); err == nil && u.Host != "" {
			port := u.Port()
			if port == "" {
				port = map[string]string{"https": "443"}[u.Scheme]
				if port == "" {
					port = "80"
				}
			}
			proxies[net.JoinHostPort(u.Hostname(), port)] = true
		}
	}

	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Proxy = func(req *http.Request) (*url.URL, error) {
		u, err := proxyFor(req.URL)
		if u != nil && err == nil {
			if cerr := d.network.CheckURL(req.Context(), req.URL.String()); cerr != nil {
				return nil, fmt.Errorf("%w: %v", netguard.ErrBlocked, cerr)
			}
		}
		return u, err
	}
	t.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		if proxies[addr] {
			return (&net.Dialer{Timeout: 30 * time.Second}).DialContext(ctx, network, addr)
		}
		return d.network.DialContext(ctx, network, addr)
	}
	return t
}

func (d *Dispatcher) Network() netguard.Policy {
	return d.network
}

func (d *Dispatcher) SetConfig(cfg Config) {
	if cfg.MaxRetries > 0 {
		d.maxRetries = cfg.MaxRetries
	}
	if cfg.Timeout > 0 {
		d.client.Timeout = cfg.Timeout
	}
	d.network = cfg.Network
	d.client.Transport = d.guardedTransport()
	if cfg.ProxyURL != "" {
		transport, err := proxyTransport(cfg.ProxyURL)
		if err != nil {
			logger.Error("invalid webhook proxy URL, requests will be sent directly", "error", err)
			return
		}
		d.client.Transport = transport
		d.proxied = true
		logger.Info("webhook proxy configured", "proxy", cfg.ProxyURL)
	}
}

func proxyTransport(rawURL string) (*http.Transport, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse proxy URL: %w", err)
	}

	switch parsed.Scheme {
	case "http", "https":
		return &http.Transport{
			Proxy: http.ProxyURL(parsed),
		}, nil

	case "socks5":
		auth := &proxy.Auth{}
		if parsed.User != nil {
			auth.User = parsed.User.Username()
			auth.Password, _ = parsed.User.Password()
		} else {
			auth = nil
		}
		dialer, err := proxy.SOCKS5("tcp", parsed.Host, auth, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
		}
		contextDialer, ok := dialer.(proxy.ContextDialer)
		if !ok {
			return nil, fmt.Errorf("SOCKS5 dialer does not support DialContext")
		}
		return &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return contextDialer.DialContext(ctx, network, addr)
			},
		}, nil

	default:
		return nil, fmt.Errorf("unsupported proxy scheme %q (use http, https, or socks5)", parsed.Scheme)
	}
}

func (d *Dispatcher) SetDeliveryRepo(repo *repositories.WebhookDeliveryRepository) {
	d.deliveryRepo = repo
}

func (d *Dispatcher) OnDeliverySuccess(fn func()) { d.onDeliverySuccess = fn }

func (d *Dispatcher) OnDeliveryFailed(fn func()) { d.onDeliveryFailed = fn }

func (d *Dispatcher) OnDeliveryDone(fn func(float64)) { d.onDeliveryDone = fn }

type Payload struct {
	Event     string `json:"event"`
	EmailID   string `json:"email_id"`
	Timestamp string `json:"timestamp"`
}

// CampaignPayload is the shape for campaign.started and campaign.completed events.
type CampaignPayload struct {
	Event      string `json:"event"`
	CampaignID uint   `json:"campaign_id"`
	Name       string `json:"name"`
	Timestamp  string `json:"timestamp"`
}

// Dispatch runs asynchronously; a nil workspaceID targets the user's personal webhooks.
func (d *Dispatcher) Dispatch(userID uint, workspaceID *uint, event string, emailID string, from string) {
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

	d.DispatchJSON(userID, workspaceID, event, body, from)
}

// DispatchCampaign runs asynchronously and scopes webhooks like Dispatch.
func (d *Dispatcher) DispatchCampaign(userID uint, workspaceID *uint, event string, campaignID uint, name string, from string) {
	payload := CampaignPayload{
		Event:      event,
		CampaignID: campaignID,
		Name:       name,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		logger.Error("failed to marshal campaign webhook payload", "error", err)
		return
	}

	d.DispatchJSON(userID, workspaceID, event, body, from)
}

// DispatchJSON lets callers own the payload shape (inbound pipeline); scoped like Dispatch, runs asynchronously.
func (d *Dispatcher) DispatchJSON(userID uint, workspaceID *uint, event string, body []byte, fromAddr string) {
	go func() {
		scope := repositories.ResourceScope{UserID: userID, WorkspaceID: workspaceID}
		webhooks, err := d.repo.FindByScopeAndEvent(scope, event)
		if err != nil {
			logger.Error("failed to find webhooks", "error", err)
			return
		}

		for _, wh := range webhooks {
			if !matchesFilters(wh.Filters, fromAddr) {
				continue
			}
			d.deliverWithRetry(userID, wh, body, event)
		}
	}()
}

// DispatchJSONTo skips webhooks subscribed to event, which DispatchJSON already reaches, to avoid duplicates.
func (d *Dispatcher) DispatchJSONTo(workspaceID uint, webhookIDs []uint, event string, body []byte) {
	if len(webhookIDs) == 0 {
		return
	}
	go func() {
		for _, id := range webhookIDs {
			wh, err := d.repo.FindByID(id)
			if err != nil || wh.WorkspaceID == nil || *wh.WorkspaceID != workspaceID {
				continue
			}
			subscribed := false
			for _, e := range wh.Events {
				subscribed = subscribed || e == event
			}
			if subscribed {
				continue
			}
			d.deliverWithRetry(wh.UserID, *wh, body, event)
		}
	}()
}

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

		if err == nil && statusCode < 300 {
			d.recordDelivery(userID, wh.ID, wh.WorkspaceID, event, models.WebhookDeliverySuccess, statusCode, "", attempt)
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
			// Members read the delivery log, so it names the failure, not the address; the server log has the full error.
			lastErr = netguard.Describe(err)
			logger.Warn("webhook delivery failed", "url", wh.URL, "attempt", attempt, "error", err)
		} else if statusCode < 400 {
			lastErr = fmt.Sprintf("HTTP %d: redirects are not followed", statusCode)
			logger.Warn("webhook redirected", "url", wh.URL, "attempt", attempt, "status", statusCode)
		} else {
			lastErr = fmt.Sprintf("HTTP %d", statusCode)
			logger.Warn("webhook returned error status", "url", wh.URL, "attempt", attempt, "status", statusCode)
		}

		if attempt < d.maxRetries {
			time.Sleep(defaultRetryBaseWait * time.Duration(attempt))
		}
	}

	d.recordDelivery(userID, wh.ID, wh.WorkspaceID, event, models.WebhookDeliveryFailed, lastStatus, lastErr, d.maxRetries)
	if d.onDeliveryFailed != nil {
		d.onDeliveryFailed()
	}
	if d.onDeliveryDone != nil {
		d.onDeliveryDone(time.Since(start).Seconds())
	}
}

func signPayload(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func (d *Dispatcher) doRequest(url string, secret string, body []byte) (int, error) {
	// Through a proxy the dialer only reaches the proxy, so the target's host is checked here.
	if d.proxied {
		if err := d.network.CheckURL(context.Background(), url); err != nil {
			return 0, fmt.Errorf("%w: %v", netguard.ErrBlocked, err)
		}
	}
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

func (d *Dispatcher) recordDelivery(userID, webhookID uint, workspaceID *uint, event string, status models.WebhookDeliveryStatus, httpStatus int, errMsg string, attempt int) {
	if d.deliveryRepo == nil {
		return
	}
	delivery := &models.WebhookDelivery{
		WebhookID:      webhookID,
		UserID:         userID,
		WorkspaceID:    workspaceID,
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
