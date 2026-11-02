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

// Package formrules decides where a form submission goes. It is pure: no I/O
// and no clock, so the worker, the digest and the dry run all agree.
package formrules

import (
	"encoding/json"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/goposta/posta/internal/models"
)

// Built-in facts a condition can test besides the answers.
const (
	FactOrigin      = "$origin"
	FactReferer     = "$referer"
	FactSenderEmail = "$sender_email"
	FactSpamScore   = "$spam_score"
	FactStatus      = "$status"
)

// Facts is what rules can see of a submission.
type Facts struct {
	Status      models.MessageStatus
	SpamScore   float64
	Origin      string
	Referer     string
	SenderEmail string
	// Data holds typed answers (forms with questions); Fields the submitted
	// values by key, for forms without.
	Data   map[string]any
	Fields map[string]string
	// Schema types the answers, so "10" > "9" compares as numbers.
	Schema map[string]*models.FieldDef
}

// FactsFor reads the facts of a stored submission.
func FactsFor(msg *models.Message, form *models.Form) Facts {
	f := Facts{
		Status:      msg.Status,
		SpamScore:   msg.SpamScore,
		Origin:      msg.Origin,
		Referer:     msg.Referer,
		SenderEmail: msg.SenderEmail,
		Data:        msg.Data,
		Fields:      map[string]string{},
		Schema:      map[string]*models.FieldDef{},
	}
	if msg.FieldsJSON != "" {
		var fields []models.MessageField
		if json.Unmarshal([]byte(msg.FieldsJSON), &fields) == nil {
			for _, fl := range fields {
				key := strings.ToLower(strings.TrimSpace(fl.Key))
				if _, seen := f.Fields[key]; !seen {
					f.Fields[key] = fl.Value
				}
			}
		}
	}
	if form != nil {
		for i := range form.Fields {
			if !form.Fields[i].IsSection() {
				f.Schema[form.Fields[i].Key] = &form.Fields[i]
			}
		}
	}
	return f
}

// Recipient is one destination of a notification, chosen by an admin.
type Recipient struct {
	Email  string `json:"email,omitempty"`
	UserID uint   `json:"user_id,omitempty"`
	TeamID uint   `json:"team_id,omitempty"`
}

// Outcome is what the matching rules decided.
type Outcome struct {
	Notify          []Recipient `json:"notify,omitempty"`
	AssignUser      *uint       `json:"assign_user,omitempty"`
	AssignTeam      *uint       `json:"assign_team,omitempty"`
	Tags            []string    `json:"tags,omitempty"`
	Priority        string      `json:"priority,omitempty"`
	Webhooks        []uint      `json:"webhooks,omitempty"`
	SuppressDefault bool        `json:"suppress_default,omitempty"`
	Matched         []string    `json:"matched,omitempty"`
}

// Routable reports whether a submission is routed at all. Spam keeps the
// old behaviour: no notification, no routing.
func Routable(status models.MessageStatus) bool {
	return status == models.MessageStatusReceived || status == models.MessageStatusFlagged
}

// Ordered returns the enabled rules in evaluation order.
func Ordered(rules []models.FormRule) []models.FormRule {
	out := make([]models.FormRule, 0, len(rules))
	for _, r := range rules {
		if r.Enabled {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Position != out[j].Position {
			return out[i].Position < out[j].Position
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// Evaluate runs the rules over a submission. Every matching rule applies, in
// Position order, until one with Stop matches.
func Evaluate(rules []models.FormRule, facts Facts) Outcome {
	if !Routable(facts.Status) {
		return Outcome{}
	}
	var matched []models.FormRule
	for _, r := range Ordered(rules) {
		if !Matches(&r, facts) {
			continue
		}
		matched = append(matched, r)
		if r.Stop {
			break
		}
	}
	return merge(matched)
}

// Combine rebuilds the outcome of rules that already matched, in their
// order, from their current actions. The worker uses it on a retry and the
// digest at send time, so neither re-evaluates against changed rules.
func Combine(rules []models.FormRule, matched []string) Outcome {
	want := make(map[string]bool, len(matched))
	for _, id := range matched {
		want[id] = true
	}
	var picked []models.FormRule
	for _, r := range Ordered(rules) {
		if want[r.UUID] {
			picked = append(picked, r)
		}
	}
	return merge(picked)
}

var priorityRank = map[string]int{"low": 1, "normal": 2, "high": 3, "urgent": 4}

// merge folds the actions of matched rules: recipients, tags and webhooks are
// a union, the first assignment wins, and the highest priority wins.
func merge(matched []models.FormRule) Outcome {
	var o Outcome
	seenRecipient := map[Recipient]bool{}
	seenTag := map[string]bool{}
	seenHook := map[uint]bool{}
	notify := func(r Recipient) {
		if !seenRecipient[r] {
			seenRecipient[r] = true
			o.Notify = append(o.Notify, r)
		}
	}
	for _, rule := range matched {
		o.Matched = append(o.Matched, rule.UUID)
		for _, a := range rule.Actions {
			switch a.Type {
			case models.ActionNotifyEmails:
				for _, e := range a.Emails {
					notify(Recipient{Email: strings.ToLower(strings.TrimSpace(e))})
				}
			case models.ActionNotifyMembers:
				for _, id := range a.UserIDs {
					notify(Recipient{UserID: id})
				}
			case models.ActionNotifyTeam:
				if a.TeamID != nil {
					notify(Recipient{TeamID: *a.TeamID})
				}
			case models.ActionAssignMember:
				if o.AssignUser == nil && len(a.UserIDs) > 0 {
					id := a.UserIDs[0]
					o.AssignUser = &id
				}
			case models.ActionAssignTeam:
				if o.AssignTeam == nil && a.TeamID != nil {
					id := *a.TeamID
					o.AssignTeam = &id
				}
			case models.ActionAddTags:
				for _, t := range a.Tags {
					if !seenTag[t] {
						seenTag[t] = true
						o.Tags = append(o.Tags, t)
					}
				}
			case models.ActionSetPriority:
				if priorityRank[a.Priority] > priorityRank[o.Priority] {
					o.Priority = a.Priority
				}
			case models.ActionWebhook:
				if a.WebhookID != nil && !seenHook[*a.WebhookID] {
					seenHook[*a.WebhookID] = true
					o.Webhooks = append(o.Webhooks, *a.WebhookID)
				}
			case models.ActionSuppressDefault:
				o.SuppressDefault = true
			}
		}
	}
	return o
}

// Matches reports whether a rule's conditions hold. A rule without
// conditions always matches.
func Matches(r *models.FormRule, facts Facts) bool {
	if len(r.Conditions) == 0 {
		return true
	}
	any := r.Match == models.RuleMatchAny
	for _, c := range r.Conditions {
		ok := test(c, facts)
		if any && ok {
			return true
		}
		if !any && !ok {
			return false
		}
	}
	return !any
}

// value is a fact as rules see it: text, a number, a boolean, or a list.
type value struct {
	present bool
	text    string
	num     *float64
	boolean *bool
	list    []string
	exact   bool // a choice value: compared as written, not case-folded
}

func lookup(field string, f Facts) value {
	switch field {
	case FactOrigin:
		return textValue(f.Origin)
	case FactReferer:
		return textValue(f.Referer)
	case FactSenderEmail:
		return textValue(f.SenderEmail)
	case FactStatus:
		v := textValue(string(f.Status))
		v.exact = true
		return v
	case FactSpamScore:
		n := f.SpamScore
		return value{present: true, num: &n, text: strconv.FormatFloat(n, 'f', -1, 64)}
	}
	def := f.Schema[field]
	if raw, ok := f.Data[field]; ok && raw != nil {
		v := typedValue(raw)
		v.exact = def != nil && (def.Type == models.FieldSelect || def.Type == models.FieldMultiSelect)
		return v
	}
	if def != nil {
		return value{}
	}
	// A form without questions: compare the submitted text.
	if s, ok := f.Fields[strings.ToLower(field)]; ok {
		return textValue(s)
	}
	return value{}
}

func textValue(s string) value {
	s = strings.TrimSpace(s)
	v := value{present: s != "", text: s}
	if n, err := strconv.ParseFloat(s, 64); err == nil {
		v.num = &n
	}
	return v
}

func typedValue(raw any) value {
	switch t := raw.(type) {
	case float64:
		return value{present: true, num: &t, text: strconv.FormatFloat(t, 'f', -1, 64)}
	case int:
		n := float64(t)
		return value{present: true, num: &n, text: strconv.Itoa(t)}
	case bool:
		text := "false"
		if t {
			text = "true"
		}
		return value{present: true, boolean: &t, text: text}
	case []any:
		list := make([]string, 0, len(t))
		for _, x := range t {
			list = append(list, toString(x))
		}
		return value{present: len(list) > 0, list: list, text: strings.Join(list, ", ")}
	case []string:
		return value{present: len(t) > 0, list: t, text: strings.Join(t, ", ")}
	case string:
		return textValue(t)
	}
	return textValue(toString(raw))
}

func toString(x any) string {
	switch t := x.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	}
	raw, _ := json.Marshal(x)
	return string(raw)
}

// wanted reads a condition's value as a list of strings.
func wanted(v any) []string {
	switch t := v.(type) {
	case nil:
		return nil
	case []any:
		out := make([]string, 0, len(t))
		for _, x := range t {
			out = append(out, toString(x))
		}
		return out
	case []string:
		return t
	}
	return []string{toString(v)}
}

func test(c models.RuleCondition, f Facts) bool {
	v := lookup(c.Field, f)
	switch c.Op {
	case "exists":
		return v.present
	case "not_exists":
		return !v.present
	case "eq":
		return v.present && equalAny(v, wanted(c.Value))
	case "neq":
		return !v.present || !equalAny(v, wanted(c.Value))
	case "in":
		return v.present && equalAny(v, wanted(c.Value))
	case "not_in":
		return !v.present || !equalAny(v, wanted(c.Value))
	case "contains":
		return v.present && containsAny(v, wanted(c.Value))
	case "not_contains":
		return !v.present || !containsAny(v, wanted(c.Value))
	case "matches":
		re := compiled(toString(c.Value))
		return v.present && re != nil && re.MatchString(v.text)
	case opGT, opGTE, opLT, opLTE:
		return v.present && compare(v, toString(c.Value), c.Op)
	}
	return false
}

func same(a, b string, exact bool) bool {
	if exact {
		return a == b
	}
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

// equalAny: for a list answer (checkboxes), any overlap; for a boolean, true
// or false; for a number, numeric equality; otherwise text.
func equalAny(v value, want []string) bool {
	for _, w := range want {
		switch {
		case v.list != nil:
			for _, item := range v.list {
				if same(item, w, v.exact) {
					return true
				}
			}
		case v.boolean != nil:
			if b, err := strconv.ParseBool(w); err == nil && b == *v.boolean {
				return true
			}
		case v.num != nil:
			if n, err := strconv.ParseFloat(strings.TrimSpace(w), 64); err == nil && n == *v.num {
				return true
			}
			if same(v.text, w, v.exact) {
				return true
			}
		default:
			if same(v.text, w, v.exact) {
				return true
			}
		}
	}
	return false
}

func containsAny(v value, want []string) bool {
	for _, w := range want {
		if v.list != nil {
			for _, item := range v.list {
				if same(item, w, v.exact) {
					return true
				}
			}
			continue
		}
		if w != "" && strings.Contains(strings.ToLower(v.text), strings.ToLower(w)) {
			return true
		}
	}
	return false
}

// compare orders numbers numerically, and dates and times as text, which
// sorts YYYY-MM-DD and HH:MM correctly.
func compare(v value, w string, op string) bool {
	var cmp int
	if n, err := strconv.ParseFloat(strings.TrimSpace(w), 64); err == nil && v.num != nil {
		switch {
		case *v.num < n:
			cmp = -1
		case *v.num > n:
			cmp = 1
		}
	} else {
		cmp = strings.Compare(v.text, strings.TrimSpace(w))
	}
	switch op {
	case opGT:
		return cmp > 0
	case opGTE:
		return cmp >= 0
	case opLT:
		return cmp < 0
	}
	return cmp <= 0
}

// The comparison operators: numbers, dates and times compare as such, other
// values as text.
const (
	opGT  = "gt"
	opGTE = "gte"
	opLT  = "lt"
	opLTE = "lte"
)

var regexCache sync.Map // pattern → *regexp.Regexp (nil when it does not compile)

// compiled returns the case-insensitive regexp for pattern. Patterns are
// checked when a rule is saved; one that no longer compiles never matches.
func compiled(pattern string) *regexp.Regexp {
	if cached, ok := regexCache.Load(pattern); ok {
		re, _ := cached.(*regexp.Regexp)
		return re
	}
	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		regexCache.Store(pattern, (*regexp.Regexp)(nil))
		return nil
	}
	regexCache.Store(pattern, re)
	return re
}
