// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package verifier

import "strings"

// disposableDomains is a representative (non-exhaustive) set of throwaway email
// providers.
var disposableDomains = map[string]bool{
	"mailinator.com":    true,
	"guerrillamail.com": true,
	"guerrillamail.net": true,
	"sharklasers.com":   true,
	"grr.la":            true,
	"10minutemail.com":  true,
	"10minutemail.net":  true,
	"tempmail.com":      true,
	"temp-mail.org":     true,
	"throwawaymail.com": true,
	"yopmail.com":       true,
	"yopmail.net":       true,
	"getnada.com":       true,
	"nada.email":        true,
	"trashmail.com":     true,
	"trashmail.de":      true,
	"dispostable.com":   true,
	"maildrop.cc":       true,
	"mailnesia.com":     true,
	"fakeinbox.com":     true,
	"spamgourmet.com":   true,
	"mintemail.com":     true,
	"mohmal.com":        true,
	"emailondeck.com":   true,
	"tempinbox.com":     true,
	"discard.email":     true,
	"mailcatch.com":     true,
	"inboxbear.com":     true,
	"33mail.com":        true,
	"burnermail.io":     true,
}

// roleLocalParts are mailbox names that usually point at a function/team rather
// than a person. Deliverable, but risky for cold/marketing sends.
var roleLocalParts = map[string]bool{
	"admin":         true,
	"administrator": true,
	"abuse":         true,
	"billing":       true,
	"contact":       true,
	"help":          true,
	"hello":         true,
	"hostmaster":    true,
	"info":          true,
	"mail":          true,
	"marketing":     true,
	"no-reply":      true,
	"noreply":       true,
	"office":        true,
	"postmaster":    true,
	"sales":         true,
	"security":      true,
	"support":       true,
	"team":          true,
	"webmaster":     true,
	"nepasrepondre": true,
}

// isDisposable reports whether the domain belongs to a known throwaway provider.
func isDisposable(domain string) bool {
	return disposableDomains[strings.ToLower(domain)]
}

// isRoleAccount reports whether the local part is a role/function mailbox.
func isRoleAccount(local string) bool {
	return roleLocalParts[strings.ToLower(local)]
}
