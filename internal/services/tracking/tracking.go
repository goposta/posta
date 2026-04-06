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

var linkRegex = regexp.MustCompile(`href\s*=\s*["'](https?://[^"']+)["']`)

// ProcessHTML rewrites links for click tracking and injects the open tracking pixel.
func (s *Service) ProcessHTML(html string, campaignID uint, messageID uint) string {
	if html == "" {
		return html
	}

	// Rewrite links for click tracking
	html = linkRegex.ReplaceAllStringFunc(html, func(match string) string {
		// Extract URL from href="..."
		sub := linkRegex.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		originalURL := sub[1]

		// Skip unsubscribe links and tracking URLs
		if strings.Contains(originalURL, "/t/") {
			return match
		}

		hash := hashLink(campaignID, originalURL)
		_, err := s.repo.FindOrCreateLink(campaignID, originalURL, hash)
		if err != nil {
			return match
		}

		trackedURL := fmt.Sprintf("%s/t/c/%d/%s", s.baseURL, messageID, hash)
		return strings.Replace(match, originalURL, trackedURL, 1)
	})

	// Inject open tracking pixel before </body>
	pixel := fmt.Sprintf(`<img src="%s/t/o/%d.png" width="1" height="1" alt="" style="display:none" />`, s.baseURL, messageID)
	if strings.Contains(html, "</body>") {
		html = strings.Replace(html, "</body>", pixel+"</body>", 1)
	} else {
		html += pixel
	}

	return html
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

// hashLink generates a deterministic hash for a campaign + URL combination.
func hashLink(campaignID uint, url string) string {
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "%d:%s", campaignID, url)
	return hex.EncodeToString(h.Sum(nil))[:16]
}
