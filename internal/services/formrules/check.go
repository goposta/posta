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

package formrules

import (
	"fmt"
	"net/mail"
	"regexp"
	"strconv"
	"strings"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/messages"
)

const (
	maxConditions  = 20
	maxActions     = 20
	maxRuleEmails  = 20
	maxPatternLen  = 256
	maxRuleNameLen = 120
)

// Destinations is what a rule may point at: this workspace's members, teams
// and webhooks. Nothing else, and nothing from submission data.
type Destinations struct {
	Members  map[uint]bool
	Teams    map[uint]bool
	Webhooks map[uint]bool
}

var schemalessKey = regexp.MustCompile(`^[a-z0-9_.\-\[\]]{1,64}$`)

var orderedTypes = map[models.FieldType]bool{
	models.FieldNumber: true, models.FieldScale: true, models.FieldRating: true,
	models.FieldDate: true, models.FieldTime: true,
}

var textTypes = map[models.FieldType]bool{
	models.FieldText: true, models.FieldTextarea: true, models.FieldEmail: true,
	models.FieldPhone: true, models.FieldURL: true, models.FieldHidden: true,
}

// Check validates a rule about to be saved for form, and normalizes it:
// trimmed values, lowercased tags and addresses, actions stripped of
// settings their type does not use.
func Check(r *models.FormRule, form *models.Form, d Destinations) error {
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" || len([]rune(r.Name)) > maxRuleNameLen {
		return fmt.Errorf("a rule needs a name of at most %d characters", maxRuleNameLen)
	}
	switch r.Match {
	case "":
		r.Match = models.RuleMatchAll
	case models.RuleMatchAll, models.RuleMatchAny:
	default:
		return fmt.Errorf("match must be all or any")
	}
	if len(r.Conditions) > maxConditions {
		return fmt.Errorf("a rule can have at most %d conditions", maxConditions)
	}
	for i := range r.Conditions {
		if err := checkCondition(&r.Conditions[i], form); err != nil {
			return fmt.Errorf("condition %d: %w", i+1, err)
		}
	}
	if len(r.Actions) == 0 || len(r.Actions) > maxActions {
		return fmt.Errorf("a rule needs between 1 and %d actions", maxActions)
	}
	for i := range r.Actions {
		clean, err := checkAction(r.Actions[i], d)
		if err != nil {
			return fmt.Errorf("action %d: %w", i+1, err)
		}
		r.Actions[i] = clean
	}
	return nil
}

func checkCondition(c *models.RuleCondition, form *models.Form) error {
	c.Field = strings.TrimSpace(c.Field)
	var def *models.FieldDef
	switch c.Field {
	case FactOrigin, FactReferer, FactSenderEmail, FactSpamScore, FactStatus:
	default:
		if form.HasSchema() {
			for i := range form.Fields {
				if form.Fields[i].Key == c.Field && !form.Fields[i].IsSection() {
					def = &form.Fields[i]
				}
			}
			if def == nil {
				return fmt.Errorf("%q is not a question on this form", c.Field)
			}
			if def.Archived {
				return fmt.Errorf("question %q is archived, so the rule would never match", c.Field)
			}
		} else {
			c.Field = strings.ToLower(c.Field)
			if !schemalessKey.MatchString(c.Field) {
				return fmt.Errorf("%q is not a field name", c.Field)
			}
		}
	}

	switch c.Op {
	case "exists", "not_exists":
		c.Value = nil
		return nil
	case "eq", "neq":
		s, err := single(c.Value)
		if err != nil {
			return err
		}
		c.Value = s
		return checkValues(c, def, []string{s})
	case "in", "not_in":
		list := wanted(c.Value)
		clean := make([]string, 0, len(list))
		for _, v := range list {
			if v = strings.TrimSpace(v); v != "" {
				clean = append(clean, v)
			}
		}
		if len(clean) == 0 {
			return fmt.Errorf("%s needs at least one value", c.Op)
		}
		c.Value = clean
		return checkValues(c, def, clean)
	case "contains", "not_contains":
		s, err := single(c.Value)
		if err != nil {
			return err
		}
		c.Value = s
		if def != nil && !textTypes[def.Type] && def.Type != models.FieldMultiSelect {
			return fmt.Errorf("contains does not apply to a %s question", def.Type)
		}
		if def != nil && def.Type == models.FieldMultiSelect {
			return checkValues(c, def, []string{s})
		}
		return nil
	case "matches":
		s, err := single(c.Value)
		if err != nil {
			return err
		}
		if len(s) > maxPatternLen {
			return fmt.Errorf("the pattern is longer than %d characters", maxPatternLen)
		}
		if _, err := regexp.Compile(s); err != nil {
			return fmt.Errorf("the pattern does not compile: %v", err)
		}
		if def != nil && !textTypes[def.Type] {
			return fmt.Errorf("matches applies to text questions, not %s", def.Type)
		}
		c.Value = s
		return nil
	case "gt", "gte", "lt", "lte":
		s, err := single(c.Value)
		if err != nil {
			return err
		}
		c.Value = s
		_, numErr := strconv.ParseFloat(s, 64)
		switch {
		case c.Field == FactSpamScore:
		case def != nil && orderedTypes[def.Type]:
			if def.Type == models.FieldDate || def.Type == models.FieldTime {
				return checkValues(c, def, []string{s})
			}
		case def == nil && strings.HasPrefix(c.Field, "$"):
			return fmt.Errorf("%s cannot be compared with %s", c.Field, c.Op)
		case def != nil:
			return fmt.Errorf("%s only compares numbers, dates and times, not a %s question", c.Op, def.Type)
		}
		if numErr != nil {
			return fmt.Errorf("%q is not a number", s)
		}
		return nil
	}
	return fmt.Errorf("unknown operator %q", c.Op)
}

func single(v any) (string, error) {
	switch v.(type) {
	case nil:
		return "", fmt.Errorf("a value is required")
	case []any, []string:
		return "", fmt.Errorf("expected one value, not a list")
	}
	s := strings.TrimSpace(toString(v))
	if s == "" {
		return "", fmt.Errorf("a value is required")
	}
	return s, nil
}

// checkValues type-checks condition values against the question.
func checkValues(c *models.RuleCondition, def *models.FieldDef, values []string) error {
	if c.Field == FactStatus {
		for _, v := range values {
			if v != string(models.MessageStatusReceived) && v != string(models.MessageStatusFlagged) {
				return fmt.Errorf("$status is received or flagged for any routed submission, not %q", v)
			}
		}
		return nil
	}
	if def == nil {
		return nil
	}
	switch def.Type {
	case models.FieldSelect, models.FieldMultiSelect:
		for _, v := range values {
			found := false
			for _, o := range def.Options {
				found = found || o.Value == v
			}
			if !found {
				return fmt.Errorf("%q is not an option of %q", v, def.Key)
			}
		}
	case models.FieldCheckbox:
		for _, v := range values {
			if _, err := strconv.ParseBool(v); err != nil {
				return fmt.Errorf("a checkbox is true or false, not %q", v)
			}
		}
	case models.FieldNumber, models.FieldScale, models.FieldRating:
		for _, v := range values {
			if _, err := strconv.ParseFloat(v, 64); err != nil {
				return fmt.Errorf("%q is not a number", v)
			}
		}
	case models.FieldDate:
		for _, v := range values {
			if len(v) != 10 || v[4] != '-' || v[7] != '-' {
				return fmt.Errorf("a date is written YYYY-MM-DD, not %q", v)
			}
		}
	case models.FieldTime:
		for _, v := range values {
			if len(v) != 5 || v[2] != ':' {
				return fmt.Errorf("a time is written HH:MM, not %q", v)
			}
		}
	}
	return nil
}

// checkAction validates one action and returns it with only the settings its
// type uses.
func checkAction(a models.RuleAction, d Destinations) (models.RuleAction, error) {
	clean := models.RuleAction{Type: a.Type}
	switch a.Type {
	case models.ActionNotifyEmails:
		if len(a.Emails) == 0 || len(a.Emails) > maxRuleEmails {
			return clean, fmt.Errorf("give between 1 and %d addresses", maxRuleEmails)
		}
		for _, e := range a.Emails {
			e = strings.ToLower(strings.TrimSpace(e))
			addr, err := mail.ParseAddress(e)
			if err != nil || addr.Address != e || !strings.Contains(e[strings.LastIndex(e, "@")+1:], ".") {
				return clean, fmt.Errorf("%q is not an email address", e)
			}
			clean.Emails = append(clean.Emails, e)
		}
	case models.ActionNotifyMembers, models.ActionAssignMember:
		if len(a.UserIDs) == 0 {
			return clean, fmt.Errorf("choose a member")
		}
		if a.Type == models.ActionAssignMember && len(a.UserIDs) != 1 {
			return clean, fmt.Errorf("a submission is assigned to one member")
		}
		for _, id := range a.UserIDs {
			if !d.Members[id] {
				return clean, fmt.Errorf("user %d is not a member of this workspace", id)
			}
		}
		clean.UserIDs = a.UserIDs
	case models.ActionNotifyTeam, models.ActionAssignTeam:
		if a.TeamID == nil || !d.Teams[*a.TeamID] {
			return clean, fmt.Errorf("choose one of this workspace's teams")
		}
		clean.TeamID = a.TeamID
	case models.ActionAddTags:
		tags, err := messages.ApplyTags(nil, a.Tags, nil)
		if err != nil {
			return clean, err
		}
		if len(tags) == 0 {
			return clean, fmt.Errorf("give at least one tag")
		}
		clean.Tags = tags
	case models.ActionSetPriority:
		if !models.ValidPriority(models.MessagePriority(a.Priority)) {
			return clean, fmt.Errorf("priority must be low, normal, high or urgent")
		}
		clean.Priority = a.Priority
	case models.ActionWebhook:
		if a.WebhookID == nil || !d.Webhooks[*a.WebhookID] {
			return clean, fmt.Errorf("choose one of this workspace's webhooks")
		}
		clean.WebhookID = a.WebhookID
	case models.ActionSuppressDefault:
	default:
		return clean, fmt.Errorf("unknown action %q", a.Type)
	}
	return clean, nil
}
