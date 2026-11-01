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

package messages

import (
	"fmt"
	"math"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/goposta/posta/internal/models"
)

// FieldError is one reason a submission does not match its form's schema.
// Message is an English default; clients can localise by Code.
type FieldError struct {
	Key     string `json:"key"`
	Label   string `json:"label,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ValidationError is returned by Submit when a submission does not match the
// form's schema. Nothing is stored.
type ValidationError struct {
	Fields []FieldError
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("submission has %d invalid field(s)", len(e.Fields))
}

// Validated is a submission that matches its form's schema.
type Validated struct {
	// Data holds the typed answers, keyed by field key.
	Data models.SubmissionData
	// Fields is the display copy: declared fields in schema order, then kept
	// unknown fields.
	Fields []models.MessageField
	// Roles maps each role to its field's display value.
	Roles map[models.FieldRole]string
}

const (
	maxEmailLength = 254
	maxURLLength   = 2000
	minPhoneDigits = 6
)

var errorMessages = map[string]string{
	"required":       "This field is required.",
	"invalid_email":  "Enter a valid email address.",
	"invalid_phone":  "Enter a valid phone number.",
	"invalid_number": "Enter a number.",
	"invalid_url":    "Enter a full web address starting with http:// or https://.",
	"invalid_date":   "Enter a date as YYYY-MM-DD.",
	"invalid_time":   "Enter a time as HH:MM.",
	"invalid_option": "Choose one of the listed options.",
	"invalid_value":  "This value is not accepted.",
	"pattern":        "This value is not in the expected format.",
	"unknown_field":  "This form does not accept this field.",
}

func fieldError(f *models.FieldDef, key, code, message string) FieldError {
	if message == "" {
		message = errorMessages[code]
	}
	label := ""
	if f != nil {
		label = f.Label
	}
	return FieldError{Key: key, Label: label, Code: code, Message: message}
}

// Validate checks fields against form's schema. A key may repeat, which is
// how a multi-select arrives; other fields use their first value.
func Validate(form *models.Form, fields []models.MessageField) (*Validated, []FieldError) {
	values := make(map[string][]string, len(fields))
	var order []string
	for _, f := range fields {
		key := strings.ToLower(strings.TrimSpace(f.Key))
		if _, seen := values[key]; !seen {
			order = append(order, key)
		}
		if v := strings.TrimSpace(f.Value); v != "" {
			values[key] = append(values[key], v)
		} else if _, seen := values[key]; !seen {
			values[key] = nil
		}
	}

	out := &Validated{
		Data:  models.SubmissionData{},
		Roles: map[models.FieldRole]string{},
	}
	var errs []FieldError
	declared := make(map[string]bool, len(form.Fields))
	for i := range form.Fields {
		def := &form.Fields[i]
		declared[def.Key] = true
		if def.Archived {
			continue
		}
		value, display, err := coerce(def, values[def.Key])
		if err != nil {
			errs = append(errs, *err)
			continue
		}
		if value == nil {
			continue
		}
		out.Data[def.Key] = value
		if display != "" {
			out.Fields = append(out.Fields, models.MessageField{Key: def.Key, Value: display})
			if def.Role != models.RoleNone {
				out.Roles[def.Role] = display
			}
		}
	}

	policy := form.UnknownFieldsPolicy()
	for _, key := range order {
		if declared[key] && !isArchived(form.Fields, key) {
			continue
		}
		vals := values[key]
		// Archived keys come from old embeds; never reject them.
		if policy == models.UnknownFieldsReject && !declared[key] {
			errs = append(errs, fieldError(nil, key, "unknown_field", ""))
			continue
		}
		if policy == models.UnknownFieldsDrop || len(vals) == 0 {
			continue
		}
		out.Fields = append(out.Fields, models.MessageField{Key: key, Value: strings.Join(vals, ", ")})
	}
	if len(errs) > 0 {
		return nil, errs
	}
	return out, nil
}

func isArchived(defs models.FieldDefs, key string) bool {
	for _, d := range defs {
		if d.Key == key {
			return d.Archived
		}
	}
	return false
}

// coerce turns a field's submitted values into its typed value and display
// string. A nil value with no error means the optional field was left empty.
func coerce(f *models.FieldDef, vals []string) (any, string, *FieldError) {
	fail := func(code string) (any, string, *FieldError) {
		e := fieldError(f, f.Key, code, "")
		return nil, "", &e
	}
	failMsg := func(code, msg string) (any, string, *FieldError) {
		e := fieldError(f, f.Key, code, msg)
		return nil, "", &e
	}

	if f.Type == models.FieldCheckbox {
		checked := false
		if len(vals) > 0 {
			switch strings.ToLower(vals[0]) {
			case "on", "true", "1", "yes":
				checked = true
			case "off", "false", "0", "no":
			default:
				return fail("invalid_value")
			}
		}
		if f.Required && !checked {
			return fail("required")
		}
		if checked {
			return true, "Yes", nil
		}
		return false, "No", nil
	}

	if len(vals) == 0 {
		if f.Required {
			return fail("required")
		}
		return nil, "", nil
	}
	v := vals[0]

	switch f.Type {
	case models.FieldText, models.FieldTextarea, models.FieldHidden:
		n := utf8.RuneCountInString(v)
		if f.MinLen != nil && n < *f.MinLen {
			return failMsg("too_short", fmt.Sprintf("Enter at least %d characters.", *f.MinLen))
		}
		if max := MaxLen(f); n > max {
			return failMsg("too_long", fmt.Sprintf("Enter at most %d characters.", max))
		}
		if f.Pattern != "" && !matchesPattern(f.Pattern, v) {
			return fail("pattern")
		}
		return v, v, nil

	case models.FieldEmail:
		addr, err := mail.ParseAddress(v)
		if err != nil || len(addr.Address) > maxEmailLength || !strings.Contains(addr.Address[strings.LastIndex(addr.Address, "@")+1:], ".") {
			return fail("invalid_email")
		}
		return addr.Address, addr.Address, nil

	case models.FieldPhone:
		phone := NormalizePhone(v)
		digits := 0
		for _, r := range phone {
			if r >= '0' && r <= '9' {
				digits++
			}
		}
		if digits < minPhoneDigits {
			return fail("invalid_phone")
		}
		return phone, phone, nil

	case models.FieldNumber:
		n, err := strconv.ParseFloat(v, 64)
		if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
			return fail("invalid_number")
		}
		if (f.Min != nil && n < *f.Min) || (f.Max != nil && n > *f.Max) {
			return failMsg("out_of_range", rangeMessage(f.Min, f.Max))
		}
		return n, v, nil

	case models.FieldURL:
		u, err := url.Parse(v)
		if err != nil || len(v) > maxURLLength || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fail("invalid_url")
		}
		return v, v, nil

	case models.FieldDate:
		d, err := time.Parse("2006-01-02", v)
		if err != nil || d.Year() < 1900 || d.Year() > 2200 {
			return fail("invalid_date")
		}
		return v, v, nil

	case models.FieldTime:
		for _, layout := range []string{"15:04", "15:04:05"} {
			if t, err := time.Parse(layout, v); err == nil {
				s := t.Format("15:04")
				return s, s, nil
			}
		}
		return fail("invalid_time")

	case models.FieldSelect:
		for _, o := range f.Options {
			if o.Value == v {
				return o.Value, optionLabel(o), nil
			}
		}
		return fail("invalid_option")

	case models.FieldMultiSelect:
		chosen := make([]string, 0, len(vals))
		labels := make([]string, 0, len(vals))
		seen := make(map[string]bool, len(vals))
		for _, v := range vals {
			if seen[v] {
				continue
			}
			found := false
			for _, o := range f.Options {
				if o.Value == v {
					chosen = append(chosen, o.Value)
					labels = append(labels, optionLabel(o))
					found = true
					break
				}
			}
			if !found {
				return fail("invalid_option")
			}
			seen[v] = true
		}
		return chosen, strings.Join(labels, ", "), nil

	case models.FieldScale, models.FieldRating:
		lo, hi := ScaleRange(f)
		if f.Type == models.FieldRating {
			lo, hi = 1, RatingStars(f)
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			return fail("invalid_number")
		}
		if n < lo || n > hi {
			low, high := float64(lo), float64(hi)
			return failMsg("out_of_range", rangeMessage(&low, &high))
		}
		return n, strconv.Itoa(n), nil
	}
	return fail("invalid_value")
}

func optionLabel(o models.FieldOption) string {
	if o.Label != "" {
		return o.Label
	}
	return o.Value
}

func rangeMessage(lo, hi *float64) string {
	format := func(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }
	switch {
	case lo != nil && hi != nil:
		return fmt.Sprintf("Enter a value from %s to %s.", format(*lo), format(*hi))
	case lo != nil:
		return fmt.Sprintf("Enter %s or more.", format(*lo))
	default:
		return fmt.Sprintf("Enter %s or less.", format(*hi))
	}
}

var patternCache sync.Map // pattern → *regexp.Regexp

// matchesPattern reports whether the whole value matches pattern. Patterns
// were compiled when the schema was saved, so a failure here is a mismatch.
func matchesPattern(pattern, value string) bool {
	cached, ok := patternCache.Load(pattern)
	if !ok {
		re, err := regexp.Compile(`^(?:` + pattern + `)$`)
		if err != nil {
			return false
		}
		cached, _ = patternCache.LoadOrStore(pattern, re)
	}
	return cached.(*regexp.Regexp).MatchString(value)
}

// ExtractWithRoles fills the summary values from role fields, falling back to
// key-name guessing for roles the schema leaves unset.
func ExtractWithRoles(roles map[models.FieldRole]string, schema models.FieldDefs, fields []models.MessageField) Extracted {
	guessed := Extract(fields)
	has := func(role models.FieldRole) bool {
		for _, f := range schema.Active() {
			if f.Role == role {
				return true
			}
		}
		return false
	}
	pickRole := func(role models.FieldRole, fallback string) string {
		if has(role) {
			return roles[role]
		}
		return fallback
	}
	return Extracted{
		Email:   SanitizeHeaderValue(pickRole(models.RoleSenderEmail, guessed.Email)),
		Name:    SanitizeHeaderValue(pickRole(models.RoleSenderName, guessed.Name)),
		Phone:   NormalizePhone(pickRole(models.RoleSenderPhone, guessed.Phone)),
		Subject: SanitizeHeaderValue(pickRole(models.RoleSubject, guessed.Subject)),
		Body:    pickRole(models.RoleBody, guessed.Body),
	}
}

// JoinRepeated merges repeated keys into one field with comma-separated
// values, in order of first appearance. It is the display copy for forms
// without a schema, matching how multi-value inputs were always stored.
func JoinRepeated(fields []models.MessageField) []models.MessageField {
	index := make(map[string]int, len(fields))
	out := make([]models.MessageField, 0, len(fields))
	for _, f := range fields {
		if i, seen := index[f.Key]; seen {
			if f.Value != "" {
				if out[i].Value != "" {
					out[i].Value += ", "
				}
				out[i].Value += f.Value
			}
			continue
		}
		index[f.Key] = len(out)
		out = append(out, f)
	}
	return out
}

// DistinctKeys counts the different keys among fields.
func DistinctKeys(fields []models.MessageField) int {
	seen := make(map[string]struct{}, len(fields))
	for _, f := range fields {
		seen[f.Key] = struct{}{}
	}
	return len(seen)
}
