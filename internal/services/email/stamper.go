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

package email

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/goposta/posta/internal/models"
)

// Stamper only mutates a headers map, so campaign and transactional mail share the same logic.
type Stamper struct {
	AppName    string
	AppVersion string
	SignKey    []byte
}

// NewStamper disables X-Posta-Signature when signKey is empty.
func NewStamper(appName, appVersion string, signKey []byte) *Stamper {
	return &Stamper{AppName: appName, AppVersion: appVersion, SignKey: signKey}
}

func (s *Stamper) StampCampaign(headers map[string]string, em *models.Email, campaignID, campaignMessageID uint, trackOpens, trackClicks bool) {
	s.base(headers)
	headers["X-Posta-Campaign-ID"] = fmt.Sprintf("%d", campaignID)
	headers["X-Posta-Message-ID"] = fmt.Sprintf("%d", campaignMessageID)
	headers["X-Posta-Track-Opens"] = boolFlag(trackOpens)
	headers["X-Posta-Track-Clicks"] = boolFlag(trackClicks)

	headers["Precedence"] = "bulk"
	headers["Auto-Submitted"] = "auto-generated"
}

func (s *Stamper) StampTransactional(headers map[string]string, em *models.Email) {
	s.base(headers)
	if em.UUID != "" {
		headers["X-Posta-ID"] = em.UUID
	}
	if em.APIKeyID != nil && *em.APIKeyID > 0 {
		headers["X-Posta-API-Key-ID"] = fmt.Sprintf("%d", *em.APIKeyID)
	}
}

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
