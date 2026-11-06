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
	"net/mail"
	"strings"
)

// Named constants keep one provider from splitting into two analytics buckets through spelling drift.
const (
	ProviderAOL             = "AOL"
	ProviderAppleICloud     = "Apple iCloud"
	ProviderFastmail        = "Fastmail"
	ProviderGMX             = "GMX"
	ProviderGmail           = "Gmail"
	ProviderGoogleWorkspace = "Google Workspace"
	ProviderOutlook         = "Outlook"
	ProviderProton          = "Proton"
	ProviderYahoo           = "Yahoo"
	ProviderYandex          = "Yandex"
	ProviderZoho            = "Zoho"
	ProviderOther           = "Other"
)

var providerByDomain = map[string]string{
	"gmail.com":      ProviderGmail,
	"googlemail.com": ProviderGmail,

	// Google Workspace primary domain
	"google.com": ProviderGoogleWorkspace,

	"outlook.com":   ProviderOutlook,
	"hotmail.com":   ProviderOutlook,
	"live.com":      ProviderOutlook,
	"msn.com":       ProviderOutlook,
	"outlook.co.uk": ProviderOutlook,
	"hotmail.co.uk": ProviderOutlook,
	"live.co.uk":    ProviderOutlook,
	"hotmail.fr":    ProviderOutlook,
	"outlook.fr":    ProviderOutlook,
	"live.fr":       ProviderOutlook,

	"yahoo.com":      ProviderYahoo,
	"yahoo.co.uk":    ProviderYahoo,
	"yahoo.fr":       ProviderYahoo,
	"yahoo.de":       ProviderYahoo,
	"yahoo.co.jp":    ProviderYahoo,
	"yahoo.ca":       ProviderYahoo,
	"ymail.com":      ProviderYahoo,
	"rocketmail.com": ProviderYahoo,

	"icloud.com": ProviderAppleICloud,
	"me.com":     ProviderAppleICloud,
	"mac.com":    ProviderAppleICloud,

	"proton.me":      ProviderProton,
	"protonmail.com": ProviderProton,
	"pm.me":          ProviderProton,

	"aol.com": ProviderAOL,

	"gmx.com":  ProviderGMX,
	"gmx.net":  ProviderGMX,
	"gmx.de":   ProviderGMX,
	"mail.com": ProviderGMX,

	"zoho.com":     ProviderZoho,
	"zohomail.com": ProviderZoho,

	"fastmail.com": ProviderFastmail,
	"fastmail.fm":  ProviderFastmail,

	"yandex.ru":  ProviderYandex,
	"yandex.com": ProviderYandex,
}

// Entries must begin with "." so "googlefoo.com" never matches.
var providerBySuffix = []struct {
	suffix   string
	provider string
}{
	{".google.com", "Google Workspace"},
}

func NormalizeDomain(addr string) string {
	trimmed := strings.TrimSpace(addr)
	if trimmed == "" {
		return ""
	}
	parsed, err := mail.ParseAddress(trimmed)
	if err != nil {
		if at := strings.LastIndex(trimmed, "@"); at >= 0 {
			tail := trimmed[at+1:]
			end := len(tail)
			for i, r := range tail {
				if notDomainRune(r) {
					end = i
					break
				}
			}
			return strings.ToLower(tail[:end])
		}
		return ""
	}
	at := strings.LastIndex(parsed.Address, "@")
	if at < 0 {
		return ""
	}
	return strings.ToLower(parsed.Address[at+1:])
}

func notDomainRune(r rune) bool {
	return (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '.' && r != '-'
}

func ClassifyProvider(addr string) string {
	return ClassifyProviderFromDomain(NormalizeDomain(addr))
}

func ClassifyProviderFromDomain(domain string) string {
	if domain == "" {
		return ProviderOther
	}
	if p, ok := providerByDomain[domain]; ok {
		return p
	}
	for _, s := range providerBySuffix {
		if strings.HasSuffix(domain, s.suffix) {
			return s.provider
		}
	}
	return ProviderOther
}

func ClassifyRecipients(recipients []string) string {
	for _, r := range recipients {
		if p := ClassifyProvider(r); p != ProviderOther {
			return p
		}
		if NormalizeDomain(r) != "" {
			return ProviderOther
		}
	}
	return ProviderOther
}
