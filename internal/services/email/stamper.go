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

package email

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/goposta/posta/internal/models"
)

// Stamper adds Posta-specific tracing headers and an HMAC origin signature to
// outbound messages. It is intentionally sender-agnostic: it mutates a headers
// map the caller then hands to the SMTP layer, so the same logic applies to
// campaign mail and transactional/API mail without duplicating code.
type Stamper struct {
	AppName    string
	AppVersion string
	SignKey    []byte
}

// NewStamper builds a Stamper. A zero-length SignKey disables X-Posta-Signature
func NewStamper(appName, appVersion string, signKey []byte) *Stamper {
	return &Stamper{AppName: appName, AppVersion: appVersion, SignKey: signKey}
}

// StampCampaign adds campaign-specific headers. trackOpens
func (s *Stamper) StampCampaign(headers map[string]string, em *models.Email, campaignID, campaignMessageID uint, trackOpens, trackClicks bool) {
	s.base(headers)
	headers["X-Posta-Campaign-ID"] = fmt.Sprintf("%d", campaignID)
	headers["X-Posta-Message-ID"] = fmt.Sprintf("%d", campaignMessageID)
	headers["X-Posta-Track-Opens"] = boolFlag(trackOpens)
	headers["X-Posta-Track-Clicks"] = boolFlag(trackClicks)

	headers["Precedence"] = "bulk"
	headers["Auto-Submitted"] = "auto-generated"
}

// StampTransactional adds the minimum set of correlation headers for API
func (s *Stamper) StampTransactional(headers map[string]string, em *models.Email) {
	s.base(headers)
	if em.UUID != "" {
		headers["X-Posta-ID"] = em.UUID
	}
	if em.APIKeyID != nil && *em.APIKeyID > 0 {
		headers["X-Posta-API-Key-ID"] = fmt.Sprintf("%d", *em.APIKeyID)
	}
}

// Sign writes X-Posta-Signature over a canonical string of stable message
func (s *Stamper) Sign(headers map[string]string, em *models.Email, to []string, subject string) {
	if len(s.SignKey) == 0 || em == nil {
		return
	}
	canonical := fmt.Sprintf("v1:%s:%s:%s:%s",
		em.UUID,
		em.Sender,
		strings.Join(to, ","),
		subject,
	)
	mac := hmac.New(sha256.New, s.SignKey)
	mac.Write([]byte(canonical))
	headers["X-Posta-Signature"] = "v1=" + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *Stamper) base(headers map[string]string) {
	name := s.AppName
	if name == "" {
		name = "Posta"
	}
	version := s.AppVersion
	if version == "" {
		version = "unknown"
	}
	headers["X-Mailer"] = fmt.Sprintf("%s/%s", name, version)
}

func boolFlag(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
