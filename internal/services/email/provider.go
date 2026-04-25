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
	"net/mail"
	"strings"
)

const ProviderOther = "Other"

var providerByDomain = map[string]string{
	// Gmail (consumer)
	"gmail.com":      "Gmail",
	"googlemail.com": "Gmail",

	// Google Workspace primary domain
	"google.com": "Google Workspace",

	// Outlook / Microsoft consumer
	"outlook.com":   "Outlook",
	"hotmail.com":   "Outlook",
	"live.com":      "Outlook",
	"msn.com":       "Outlook",
	"outlook.co.uk": "Outlook",
	"hotmail.co.uk": "Outlook",
	"live.co.uk":    "Outlook",
	"hotmail.fr":    "Outlook",
	"outlook.fr":    "Outlook",
	"live.fr":       "Outlook",

	// Yahoo
	"yahoo.com":      "Yahoo",
	"yahoo.co.uk":    "Yahoo",
	"yahoo.fr":       "Yahoo",
	"yahoo.de":       "Yahoo",
	"yahoo.co.jp":    "Yahoo",
	"yahoo.ca":       "Yahoo",
	"ymail.com":      "Yahoo",
	"rocketmail.com": "Yahoo",

	// Apple iCloud
	"icloud.com": "Apple iCloud",
	"me.com":     "Apple iCloud",
	"mac.com":    "Apple iCloud",

	// Proton
	"proton.me":      "Proton",
	"protonmail.com": "Proton",
	"pm.me":          "Proton",

	// AOL
	"aol.com": "AOL",

	// GMX / mail.com
	"gmx.com":  "GMX",
	"gmx.net":  "GMX",
	"gmx.de":   "GMX",
	"mail.com": "GMX",

	// Zoho
	"zoho.com":     "Zoho",
	"zohomail.com": "Zoho",

	// Fastmail
	"fastmail.com": "Fastmail",
	"fastmail.fm":  "Fastmail",

	// Yandex
	"yandex.ru":  "Yandex",
	"yandex.com": "Yandex",
}

// providerBySuffix catches subdomain-style matches (e.g. "mail.google.com").
// Entries must begin with "." so we don't accidentally match "googlefoo.com".
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
	// mail.ParseAddress handles "Name" <a@b>, "<a@b>", and "a@b".
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
