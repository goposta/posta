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

package repositories

import (
	"strings"
	"time"
)

const (
	opFrom     = "from"
	opTo       = "to"
	opSubject  = "subject"
	opTemplate = "template"
	opProvider = "provider"
	opServer   = "server"
	opKey      = "key"
	opStatus   = "status"
	opHas      = "has"
	opAfter    = "after"
	opBefore   = "before"

	colCreatedAt    = "created_at"
	colSentAt       = "sent_at"
	colSender       = "sender"
	colTemplateName = "template_name"
)

type EmailFilter struct {
	Sender        string
	Recipient     string
	Subject       string
	Text          string
	Template      string
	Providers     []string
	Servers       []string
	APIKeys       []string
	Statuses      []string
	HasAttachment bool
	After         *time.Time
	Before        *time.Time
}

// maxSearchQueryLen stops one request building pathologically large predicates against unindexed email columns.
const maxSearchQueryLen = 512

// ParseSearchQuery turns a Twitter-style query (from: to: subject: status: has:attachment after: ...) into an EmailFilter.
func ParseSearchQuery(q string) EmailFilter {
	if len(q) > maxSearchQueryLen {
		q = q[:maxSearchQueryLen]
	}

	var f EmailFilter
	var free []string

	for _, tok := range tokenizeSearch(q) {
		key, val, ok := strings.Cut(tok, ":")
		if !ok {
			free = append(free, tok)
			continue
		}
		switch strings.ToLower(key) {
		case opFrom:
			f.Sender = val
		case opTo:
			f.Recipient = val
		case opSubject:
			f.appendSubject(val)
		case opTemplate:
			f.Template = val
		case opProvider:
			f.Providers = splitList(val)
		case opServer:
			f.Servers = splitList(val)
		case opKey:
			f.APIKeys = splitList(val)
		case opStatus:
			for _, s := range strings.Split(val, ",") {
				if s = strings.ToLower(strings.TrimSpace(s)); s != "" {
					f.Statuses = append(f.Statuses, s)
				}
			}
		case opHas:
			if v := strings.ToLower(val); v == "attachment" || v == "attachments" {
				f.HasAttachment = true
			}
		case opAfter:
			if t, _, ok := parseSearchDate(val); ok {
				f.After = &t
			}
		case opBefore:
			if t, dateOnly, ok := parseSearchDate(val); ok {
				// A date-only bound covers the whole day; an explicit RFC3339 instant is used as given.
				if dateOnly {
					t = t.Add(24 * time.Hour)
				}
				f.Before = &t
			}
		default:
			// Unknown prefix: treat the whole token as free text.
			free = append(free, tok)
		}
	}

	if len(free) > 0 {
		if f.Subject != "" {
			f.appendSubject(strings.Join(free, " "))
		} else {
			f.Text = strings.Join(free, " ")
		}
	}
	return f
}

func splitList(val string) []string {
	var out []string
	for _, s := range strings.Split(val, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func (f *EmailFilter) appendSubject(s string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return
	}
	if f.Subject == "" {
		f.Subject = s
		return
	}
	f.Subject += " " + s
}

// likePattern maps `*` to `%` (else substring match) and escapes real LIKE metacharacters with `\`.
func likePattern(term string) string {
	s := strings.ToLower(strings.TrimSpace(term))
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	if strings.Contains(s, "*") {
		return strings.ReplaceAll(s, "*", "%")
	}
	return "%" + s + "%"
}

func likeAny(col string, terms []string) (string, []any) {
	conds := make([]string, len(terms))
	args := make([]any, len(terms))
	for i, t := range terms {
		conds[i] = col + " LIKE ?"
		args[i] = likePattern(t)
	}
	return "(" + strings.Join(conds, " OR ") + ")", args
}

// sortColumns whitelists sort keys so user input never reaches ORDER BY.
var sortColumns = map[string]string{
	colCreatedAt: colCreatedAt,
	colSentAt:    colSentAt,
	opSubject:    opSubject,
	colSender:    colSender,
	opTemplate:   colTemplateName,
	opStatus:     opStatus,
}

// orderClause treats a leading "-" as descending; empty or unknown input falls back to newest first.
func orderClause(sort string) string {
	const def = "created_at DESC"
	sort = strings.TrimSpace(sort)
	if sort == "" {
		return def
	}
	desc := false
	if strings.HasPrefix(sort, "-") {
		desc = true
		sort = sort[1:]
	}
	col, ok := sortColumns[strings.ToLower(strings.TrimSpace(sort))]
	if !ok {
		return def
	}
	// NULLS LAST keeps null sort values (e.g. unsent emails by sent_at) at the bottom in either direction.
	if desc {
		return col + " DESC NULLS LAST"
	}
	return col + " ASC NULLS LAST"
}

// tokenizeSearch keeps double-quoted values together, even after an operator prefix (subject:"a b").
func tokenizeSearch(q string) []string {
	var tokens []string
	var b strings.Builder
	inQuote := false

	flush := func() {
		if b.Len() > 0 {
			tokens = append(tokens, b.String())
			b.Reset()
		}
	}

	for _, r := range q {
		switch {
		case r == '"':
			inQuote = !inQuote
		case (r == ' ' || r == '\t' || r == '\n') && !inQuote:
			flush()
		default:
			b.WriteRune(r)
		}
	}
	flush()
	return tokens
}

// parseSearchDate reports dateOnly so whole-day rounding never applies to an explicit instant.
func parseSearchDate(s string) (t time.Time, dateOnly bool, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false, false
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, true, true
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, false, true
	}
	return time.Time{}, false, false
}
