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

package tracking

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/goposta/posta/internal/storage/repositories"
)

// A web-view link exposes full message content, so unlike unsubscribe tokens it expires.
const defaultWebViewTTL = 90 * 24 * time.Hour

// Service handles link rewriting and pixel injection for campaign emails.
type Service struct {
	repo    *repositories.TrackingRepository
	baseURL string // e.g. "https://posta.example.com"
	hmacKey []byte
}

func NewService(repo *repositories.TrackingRepository, baseURL string, hmacKey []byte) *Service {
	return &Service{repo: repo, baseURL: strings.TrimRight(baseURL, "/"), hmacKey: hmacKey}
}

// (?i) makes the href match case-insensitive so HREF="…" still matches.
var linkRegex = regexp.MustCompile(`(?i)href\s*=\s*["'](https?://[^"']+)["']`)

// ProcessHTML rewrites links for click tracking and injects the open tracking pixel.
func (s *Service) ProcessHTML(html string, campaignID uint, messageID uint) string {
	if html == "" {
		return html
	}

	trackingPrefix := s.baseURL + "/t/"

	html = linkRegex.ReplaceAllStringFunc(html, func(match string) string {
		sub := linkRegex.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		originalURL := sub[1]

		// Match the exact tracking prefix; a bare "/t/" check would skip legitimate links such as blog posts.
		if strings.HasPrefix(originalURL, trackingPrefix) {
			return match
		}
		// Skip non-http schemes that slipped through the regex bound (defense-in-depth).
		lower := strings.ToLower(originalURL)
		if strings.HasPrefix(lower, "mailto:") || strings.HasPrefix(lower, "tel:") {
			return match
		}

		hash := hashLink(campaignID, originalURL)
		_, err := s.repo.FindOrCreateLink(campaignID, originalURL, hash)
		if err != nil {
			return match
		}

		trackedURL := s.ClickURL(messageID, hash)
		return strings.Replace(match, originalURL, trackedURL, 1)
	})

	// The pixel URL is HMAC-signed so a third party can't inflate metrics via the predictable /t/o/<id> path.
	pixel := fmt.Sprintf(`<img src="%s" width="1" height="1" alt="" style="display:none" />`, s.OpenURL(messageID))
	if strings.Contains(html, "</body>") {
		html = strings.Replace(html, "</body>", pixel+"</body>", 1)
	} else {
		html += pixel
	}

	return html
}

// OpenURL builds a signed open-tracking pixel URL.
func (s *Service) OpenURL(messageID uint) string {
	return fmt.Sprintf("%s/t/o/%d.gif?sig=%s", s.baseURL, messageID, s.signOpen(messageID))
}

// ClickURL builds a signed click-tracking redirect URL.
func (s *Service) ClickURL(messageID uint, hash string) string {
	return fmt.Sprintf("%s/t/c/%d/%s?sig=%s", s.baseURL, messageID, hash, s.signClick(messageID, hash))
}

func (s *Service) VerifyOpenSig(messageID uint, sig string) bool {
	return hmac.Equal([]byte(sig), []byte(s.signOpen(messageID)))
}

func (s *Service) VerifyClickSig(messageID uint, hash, sig string) bool {
	return hmac.Equal([]byte(sig), []byte(s.signClick(messageID, hash)))
}

func (s *Service) signOpen(messageID uint) string {
	mac := hmac.New(sha256.New, s.hmacKey)
	_, _ = fmt.Fprintf(mac, "open:%d", messageID)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)[:16])
}

func (s *Service) signClick(messageID uint, hash string) string {
	mac := hmac.New(sha256.New, s.hmacKey)
	_, _ = fmt.Fprintf(mac, "click:%d:%s", messageID, hash)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)[:16])
}

// SignUnsubscribeToken creates an HMAC-signed token encoding the message ID.
func (s *Service) SignUnsubscribeToken(messageID uint) string {
	payload := strconv.FormatUint(uint64(messageID), 10)
	mac := hmac.New(sha256.New, s.hmacKey)
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + sig
}

func (s *Service) VerifyUnsubscribeToken(token string) (uint, error) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return 0, errors.New("invalid token format")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return 0, errors.New("invalid token encoding")
	}
	payload := string(payloadBytes)

	mac := hmac.New(sha256.New, s.hmacKey)
	mac.Write(payloadBytes)
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(parts[1]), []byte(expectedSig)) {
		return 0, errors.New("invalid token signature")
	}

	id, err := strconv.ParseUint(payload, 10, 64)
	if err != nil {
		return 0, errors.New("invalid message ID in token")
	}
	return uint(id), nil
}

func (s *Service) UnsubscribeURL(messageID uint) string {
	return fmt.Sprintf("%s/t/u/%s", s.baseURL, s.SignUnsubscribeToken(messageID))
}

// SignTxUnsubscribeToken's "tx:" prefix stops it being replayed against the campaign-token handler.
func (s *Service) SignTxUnsubscribeToken(emailID uint) string {
	payload := "tx:" + strconv.FormatUint(uint64(emailID), 10)
	mac := hmac.New(sha256.New, s.hmacKey)
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + sig
}

func (s *Service) VerifyTxUnsubscribeToken(token string) (uint, error) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return 0, errors.New("invalid token format")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return 0, errors.New("invalid token encoding")
	}
	mac := hmac.New(sha256.New, s.hmacKey)
	mac.Write(payloadBytes)
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(parts[1]), []byte(expectedSig)) {
		return 0, errors.New("invalid token signature")
	}
	payload := string(payloadBytes)
	if !strings.HasPrefix(payload, "tx:") {
		return 0, errors.New("wrong token kind")
	}
	id, err := strconv.ParseUint(payload[3:], 10, 64)
	if err != nil {
		return 0, errors.New("invalid email ID in token")
	}
	return uint(id), nil
}

// TxUnsubscribeURL is the one-click unsubscribe URL, suitable for RFC 8058 List-Unsubscribe-Post.
func (s *Service) TxUnsubscribeURL(emailID uint) string {
	return fmt.Sprintf("%s/t/u/tx/%s", s.baseURL, s.SignTxUnsubscribeToken(emailID))
}

// SignWebViewToken encodes the non-enumerable Email UUID and an expiry; its "view:" prefix blocks unsubscribe replay.
func (s *Service) SignWebViewToken(emailUUID string, ttl time.Duration) string {
	exp := time.Now().Add(ttl).Unix()
	payload := fmt.Sprintf("view:%s:%d", emailUUID, exp)
	mac := hmac.New(sha256.New, s.hmacKey)
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + sig
}

func (s *Service) VerifyWebViewToken(token string) (string, error) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return "", errors.New("invalid token format")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", errors.New("invalid token encoding")
	}
	mac := hmac.New(sha256.New, s.hmacKey)
	mac.Write(payloadBytes)
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(parts[1]), []byte(expectedSig)) {
		return "", errors.New("invalid token signature")
	}
	payload := string(payloadBytes)
	if !strings.HasPrefix(payload, "view:") {
		return "", errors.New("wrong token kind")
	}
	rest := payload[len("view:"):]
	sep := strings.LastIndex(rest, ":")
	if sep <= 0 {
		return "", errors.New("invalid token payload")
	}
	emailUUID := rest[:sep]
	exp, err := strconv.ParseInt(rest[sep+1:], 10, 64)
	if err != nil {
		return "", errors.New("invalid token expiry")
	}
	if time.Now().Unix() > exp {
		return "", errors.New("token expired")
	}
	return emailUUID, nil
}

// WebViewURL returns "" for an empty UUID.
func (s *Service) WebViewURL(emailUUID string) string {
	if emailUUID == "" {
		return ""
	}
	return fmt.Sprintf("%s/t/v/%s", s.baseURL, s.SignWebViewToken(emailUUID, defaultWebViewTTL))
}

// hashLink generates a deterministic hash for a campaign + URL combination.
func hashLink(campaignID uint, url string) string {
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "%d:%s", campaignID, url)
	return hex.EncodeToString(h.Sum(nil))[:16]
}
