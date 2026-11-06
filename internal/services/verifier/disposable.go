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

package verifier

import "strings"

// disposableDomains is a representative, non-exhaustive set of throwaway providers.
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

// roleLocalParts name a function or team rather than a person: deliverable, but risky for cold sends.
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

func isDisposable(domain string) bool {
	return disposableDomains[strings.ToLower(domain)]
}

func isRoleAccount(local string) bool {
	return roleLocalParts[strings.ToLower(local)]
}
