// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package email

import (
	"net/mail"
	"strings"
)

// Mailbox provider names, as reported by deliverability analytics. Named so the
// domain table below cannot drift into "Outlook" in one row and "outlook" in
// another, which would split one provider into two buckets in the breakdown.
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
	// Gmail (consumer)
	"gmail.com":      ProviderGmail,
	"googlemail.com": ProviderGmail,

	// Google Workspace primary domain
	"google.com": ProviderGoogleWorkspace,

	// Outlook / Microsoft consumer
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

	// Yahoo
	"yahoo.com":      ProviderYahoo,
	"yahoo.co.uk":    ProviderYahoo,
	"yahoo.fr":       ProviderYahoo,
	"yahoo.de":       ProviderYahoo,
	"yahoo.co.jp":    ProviderYahoo,
	"yahoo.ca":       ProviderYahoo,
	"ymail.com":      ProviderYahoo,
	"rocketmail.com": ProviderYahoo,

	// Apple iCloud
	"icloud.com": ProviderAppleICloud,
	"me.com":     ProviderAppleICloud,
	"mac.com":    ProviderAppleICloud,

	// Proton
	"proton.me":      ProviderProton,
	"protonmail.com": ProviderProton,
	"pm.me":          ProviderProton,

	// AOL
	"aol.com": ProviderAOL,

	// GMX / mail.com
	"gmx.com":  ProviderGMX,
	"gmx.net":  ProviderGMX,
	"gmx.de":   ProviderGMX,
	"mail.com": ProviderGMX,

	// Zoho
	"zoho.com":     ProviderZoho,
	"zohomail.com": ProviderZoho,

	// Fastmail
	"fastmail.com": ProviderFastmail,
	"fastmail.fm":  ProviderFastmail,

	// Yandex
	"yandex.ru":  ProviderYandex,
	"yandex.com": ProviderYandex,
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
