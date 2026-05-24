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
