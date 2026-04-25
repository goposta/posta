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

	"github.com/goposta/posta/internal/storage/repositories"
)

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

	// Rewrite links for click tracking
	html = linkRegex.ReplaceAllStringFunc(html, func(match string) string {
		sub := linkRegex.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		originalURL := sub[1]

		// Skip URLs that are already our own tracking endpoints. The previous
		// `Contains("/t/")` check accidentally skipped any legitimate link
		// whose path segment contained "/t/" (e.g. blog posts).
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

	// Inject open tracking pixel before </body>. The URL is HMAC-signed so
	// metrics can't be inflated by a third party hitting the predictable
	// /t/o/<msg_id> path.
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

// VerifyOpenSig checks an open-tracking signature.
func (s *Service) VerifyOpenSig(messageID uint, sig string) bool {
	return hmac.Equal([]byte(sig), []byte(s.signOpen(messageID)))
}

// VerifyClickSig checks a click-tracking signature.
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

// VerifyUnsubscribeToken verifies the HMAC token and returns the message ID.
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

// UnsubscribeURL generates the unsubscribe URL for a campaign message.
func (s *Service) UnsubscribeURL(messageID uint) string {
	return fmt.Sprintf("%s/t/u/%s", s.baseURL, s.SignUnsubscribeToken(messageID))
}

// SignTxUnsubscribeToken creates an HMAC-signed token encoding a transactional
// email ID. The "tx:" prefix distinguishes it from campaign-message tokens so
// the two token kinds can't be confused or replayed against the wrong handler.
func (s *Service) SignTxUnsubscribeToken(emailID uint) string {
	payload := "tx:" + strconv.FormatUint(uint64(emailID), 10)
	mac := hmac.New(sha256.New, s.hmacKey)
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + sig
}

// VerifyTxUnsubscribeToken verifies a transactional unsubscribe
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

// TxUnsubscribeURL returns the public one-click unsubscribe URL for a
// transactional email. Suitable for RFC 8058 List-Unsubscribe-Post headers.
func (s *Service) TxUnsubscribeURL(emailID uint) string {
	return fmt.Sprintf("%s/t/u/tx/%s", s.baseURL, s.SignTxUnsubscribeToken(emailID))
}

// hashLink generates a deterministic hash for a campaign + URL combination.
func hashLink(campaignID uint, url string) string {
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "%d:%s", campaignID, url)
	return hex.EncodeToString(h.Sum(nil))[:16]
}
