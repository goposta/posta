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

package repositories

import (
	"strings"
	"time"
)

// Search operator keys and the DB columns they map to. Kept as constants so the
// same literals aren't scattered across the parser and the sort whitelist.
const (
	opFrom     = "from"
	opTo       = "to"
	opSubject  = "subject"
	opTemplate = "template"
	opStatus   = "status"
	opHas      = "has"
	opAfter    = "after"
	opBefore   = "before"

	colCreatedAt    = "created_at"
	colSentAt       = "sent_at"
	colSender       = "sender"
	colTemplateName = "template_name"
)

// EmailFilter captures optional list filters for outbound email queries.
type EmailFilter struct {
	Sender        string
	Recipient     string
	Subject       string
	Template      string
	Statuses      []string
	HasAttachment bool
	After         *time.Time
	Before        *time.Time
}

// maxSearchQueryLen bounds the raw search string before parsing so a single
// request cannot build pathologically large predicates (huge status lists, many
// wildcards, or very long LIKE terms) against the unindexed email columns.
const maxSearchQueryLen = 512

// ParseSearchQuery turns a Twitter-style search string into an EmailFilter.
// Supported operators: from: to: subject: template: status: has:attachment after: before:
// Bare words (no known prefix) accumulate into Subject as a free-text search.
//
// Example:
//
//	from:alice@x.com to:bob subject:"weekly report" has:attachment after:2026-01-01 status:sent invoice
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
		case opStatus:
			// Accept a comma-separated list, e.g. status:sent,failed.
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
				// A date-only bound covers the whole day (created_at < next
				// midnight). An explicit RFC3339 instant is used as given.
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
		f.appendSubject(strings.Join(free, " "))
	}
	return f
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

// likePattern builds a case-insensitive SQL LIKE pattern from a user term.
// A `*` acts as a wildcard (mapped to `%`); without one the term matches as a
// substring. Any real LIKE metacharacters in the input are escaped so they stay
// literal (Postgres LIKE uses `\` as the default escape).
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

// sortColumns whitelists the user-facing sort keys to real, safe column names.
// ORDER BY is built only from these constants — user input never reaches SQL.
var sortColumns = map[string]string{
	colCreatedAt: colCreatedAt,
	colSentAt:    colSentAt,
	opSubject:    opSubject,
	colSender:    colSender,
	opTemplate:   colTemplateName,
	opStatus:     opStatus,
}

// orderClause maps an optional sort key to a safe "column DIR" ORDER BY clause.
// A leading "-" means descending. Empty or unknown input falls back to the
// default (newest first), so the current behaviour is preserved.
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
	// NULLS LAST keeps rows with a null sort value (e.g. unsent emails when
	// sorting by sent_at) at the bottom regardless of direction.
	if desc {
		return col + " DESC NULLS LAST"
	}
	return col + " ASC NULLS LAST"
}

// tokenizeSearch splits a query on whitespace while keeping double-quoted
// values together (quotes may follow an operator prefix, e.g. subject:"a b").
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

// parseSearchDate accepts a date-only (YYYY-MM-DD) or RFC3339 timestamp. It
// reports dateOnly=true when the value carried no time component, so callers can
// apply whole-day rounding only in that case (never to an explicit instant).
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
