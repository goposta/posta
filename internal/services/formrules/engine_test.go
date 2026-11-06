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
	"reflect"
	"strings"
	"testing"

	"github.com/goposta/posta/internal/models"
)

func uintp(v uint) *uint { return &v }

func testForm() *models.Form {
	return &models.Form{Fields: models.FieldDefs{
		{Key: "email", Label: "Email", Type: models.FieldEmail},
		{Key: "dept", Label: "Department", Type: models.FieldSelect, Options: []models.FieldOption{{Value: "sales"}, {Value: "support"}, {Value: "Billing"}}},
		{Key: "topics", Label: "Topics", Type: models.FieldMultiSelect, Options: []models.FieldOption{{Value: "api"}, {Value: "ui"}}},
		{Key: "seats", Label: "Seats", Type: models.FieldNumber},
		{Key: "nps", Label: "NPS", Type: models.FieldScale},
		{Key: "start", Label: "Start", Type: models.FieldDate},
		{Key: "vip", Label: "VIP", Type: models.FieldCheckbox},
		{Key: "notes", Label: "Notes", Type: models.FieldTextarea},
		{Key: "old", Label: "Old", Type: models.FieldText, Archived: true},
		{Key: "page2", Label: "Page 2", Type: models.FieldSection},
	}}
}

func facts(data map[string]any) Facts {
	f := FactsFor(&models.Message{Status: models.MessageStatusReceived, Origin: "https://acme.com", SenderEmail: "ada@bigcorp.com", SpamScore: 2.5, Data: data}, testForm())
	return f
}

func TestConditionOperators(t *testing.T) {
	f := facts(map[string]any{
		"dept": "sales", "topics": []any{"api", "ui"}, "seats": float64(40), "nps": float64(9),
		"start": "2026-12-01", "vip": true, "notes": "Need an INVOICE copy", "email": "ada@bigcorp.com",
	})
	cases := []struct {
		field, op string
		value     any
		want      bool
	}{
		{"dept", "eq", "sales", true},
		{"dept", "eq", "Sales", false}, // option values are exact
		{"dept", "neq", "support", true},
		{"dept", "in", []any{"support", "sales"}, true},
		{"dept", "not_in", []any{"support"}, true},
		{"topics", "in", []any{"ui"}, true}, // any overlap
		{"topics", "contains", "api", true},
		{"topics", "not_contains", "api", false},
		{"seats", "gt", "39", true},
		{"seats", "gt", "100", false},
		{"seats", "eq", "40", true},
		{"nps", "gte", "9", true},
		{"nps", "lt", "9", false},
		{"start", "gt", "2026-11-30", true},
		{"start", "lte", "2026-11-30", false},
		{"vip", "eq", "true", true},
		{"vip", "eq", "false", false},
		{"notes", "contains", "invoice", true}, // text is case-insensitive
		{"notes", "matches", `invoice\s+copy`, true},
		{"notes", "exists", nil, true},
		{"missing", "not_exists", nil, true},
		{"seats", "not_exists", nil, false},
		{FactOrigin, "eq", "HTTPS://ACME.COM", true},
		{FactSenderEmail, "contains", "@bigcorp.com", true},
		{FactSpamScore, "lt", "3", true},
		{FactStatus, "eq", "received", true},
	}
	for _, c := range cases {
		r := models.FormRule{Conditions: models.RuleConditions{{Field: c.field, Op: c.op, Value: c.value}}}
		if got := Matches(&r, f); got != c.want {
			t.Errorf("%s %s %v = %v, want %v", c.field, c.op, c.value, got, c.want)
		}
	}
}

func TestSchemalessFields(t *testing.T) {
	msg := &models.Message{Status: models.MessageStatusReceived, FieldsJSON: `[{"key":"Department","value":"Sales"},{"key":"budget","value":"5000"}]`}
	f := FactsFor(msg, &models.Form{})
	for _, c := range []models.RuleCondition{
		{Field: "department", Op: "eq", Value: "sales"},
		{Field: "budget", Op: "gte", Value: "1000"},
	} {
		r := models.FormRule{Conditions: models.RuleConditions{c}}
		if !Matches(&r, f) {
			t.Errorf("%+v did not match a schemaless submission", c)
		}
	}
}

func TestMatchAllAny(t *testing.T) {
	f := facts(map[string]any{"dept": "sales", "seats": float64(5)})
	conds := models.RuleConditions{{Field: "dept", Op: "eq", Value: "sales"}, {Field: "seats", Op: "gt", Value: "10"}}
	all := models.FormRule{Match: models.RuleMatchAll, Conditions: conds}
	anyRule := models.FormRule{Match: models.RuleMatchAny, Conditions: conds}
	if Matches(&all, f) || !Matches(&anyRule, f) {
		t.Error("all/any mixed up")
	}
	if empty := (models.FormRule{}); !Matches(&empty, f) {
		t.Error("a rule without conditions always matches")
	}
}

func rule(uuid string, pos int, conds models.RuleConditions, actions ...models.RuleAction) models.FormRule {
	return models.FormRule{UUID: uuid, Name: uuid, Position: pos, Enabled: true, Conditions: conds, Actions: actions}
}

func TestEvaluateMerges(t *testing.T) {
	sales := models.RuleConditions{{Field: "dept", Op: "eq", Value: "sales"}}
	rules := []models.FormRule{
		// Out of order on purpose: Position decides.
		rule("c", 3, nil, models.RuleAction{Type: models.ActionAssignMember, UserIDs: []uint{9}}, models.RuleAction{Type: models.ActionSetPriority, Priority: "low"}),
		rule("a", 1, sales,
			models.RuleAction{Type: models.ActionNotifyEmails, Emails: []string{"sales@acme.com"}},
			models.RuleAction{Type: models.ActionAssignTeam, TeamID: uintp(2)},
			models.RuleAction{Type: models.ActionAddTags, Tags: []string{"lead"}},
			models.RuleAction{Type: models.ActionSetPriority, Priority: "high"}),
		rule("b", 2, sales,
			models.RuleAction{Type: models.ActionNotifyEmails, Emails: []string{"SALES@acme.com", "boss@acme.com"}},
			models.RuleAction{Type: models.ActionAssignTeam, TeamID: uintp(3)},
			models.RuleAction{Type: models.ActionAssignMember, UserIDs: []uint{7}},
			models.RuleAction{Type: models.ActionAddTags, Tags: []string{"lead", "hot"}},
			models.RuleAction{Type: models.ActionWebhook, WebhookID: uintp(4)}),
		{UUID: "off", Position: 0, Enabled: false, Actions: models.RuleActions{{Type: models.ActionSuppressDefault}}},
	}
	o := Evaluate(rules, facts(map[string]any{"dept": "sales"}))
	if !reflect.DeepEqual(o.Matched, []string{"a", "b", "c"}) {
		t.Fatalf("matched %v", o.Matched)
	}
	if want := []Recipient{{Email: "sales@acme.com"}, {Email: "boss@acme.com"}}; !reflect.DeepEqual(o.Notify, want) {
		t.Errorf("notify %+v", o.Notify)
	}
	if *o.AssignTeam != 2 || *o.AssignUser != 7 {
		t.Errorf("first assignment should win: team %d user %d", *o.AssignTeam, *o.AssignUser)
	}
	if !reflect.DeepEqual(o.Tags, []string{"lead", "hot"}) || o.Priority != "high" || !reflect.DeepEqual(o.Webhooks, []uint{4}) {
		t.Errorf("tags %v priority %q webhooks %v", o.Tags, o.Priority, o.Webhooks)
	}
	if o.SuppressDefault {
		t.Error("a disabled rule applied")
	}

	// Stop ends evaluation after its rule.
	rules[1].Stop = true
	if o := Evaluate(rules, facts(map[string]any{"dept": "sales"})); !reflect.DeepEqual(o.Matched, []string{"a"}) {
		t.Errorf("with stop, matched %v", o.Matched)
	}

	// Combine rebuilds the same outcome from the stored matches.
	rules[1].Stop = false
	if c := Combine(rules, []string{"b", "a", "c"}); !reflect.DeepEqual(c, o) {
		t.Errorf("Combine = %+v, want %+v", c, o)
	}
}

func TestSpamIsNotRouted(t *testing.T) {
	rules := []models.FormRule{rule("a", 0, nil, models.RuleAction{Type: models.ActionSuppressDefault})}
	for _, status := range []models.MessageStatus{models.MessageStatusQuarantined, models.MessageStatusRejected} {
		f := facts(nil)
		f.Status = status
		if o := Evaluate(rules, f); len(o.Matched) > 0 || o.SuppressDefault {
			t.Errorf("%s was routed", status)
		}
	}
	f := facts(nil)
	f.Status = models.MessageStatusFlagged
	if o := Evaluate(rules, f); !o.SuppressDefault {
		t.Error("a flagged submission is routed")
	}
}

func TestCheck(t *testing.T) {
	dest := Destinations{Members: map[uint]bool{7: true}, Teams: map[uint]bool{2: true}, Webhooks: map[uint]bool{4: true}}
	good := models.FormRule{
		Name:       " Sales ",
		Conditions: models.RuleConditions{{Field: "dept", Op: "in", Value: []any{"sales", " Billing "}}, {Field: "seats", Op: "gte", Value: float64(10)}},
		Actions: models.RuleActions{
			{Type: models.ActionNotifyEmails, Emails: []string{" Sales@Acme.com "}, TeamID: uintp(99)},
			{Type: models.ActionAssignTeam, TeamID: uintp(2)},
			{Type: models.ActionAddTags, Tags: []string{"Lead"}},
		},
	}
	if err := Check(&good, testForm(), dest); err != nil {
		t.Fatalf("valid rule rejected: %v", err)
	}
	if good.Name != "Sales" || good.Match != models.RuleMatchAll || good.Actions[0].Emails[0] != "sales@acme.com" || good.Actions[0].TeamID != nil || good.Actions[2].Tags[0] != "lead" {
		t.Errorf("not normalized: %+v", good)
	}
	if v := good.Conditions[0].Value.([]string); v[1] != "Billing" {
		t.Errorf("in values not trimmed: %v", v)
	}

	bad := []struct {
		name string
		rule models.FormRule
		want string
	}{
		{"no name", models.FormRule{Actions: models.RuleActions{{Type: models.ActionSuppressDefault}}}, "name"},
		{"no actions", models.FormRule{Name: "x"}, "actions"},
		{"unknown field", ruleWith(models.RuleCondition{Field: "nope", Op: "eq", Value: "x"}), "not a question"},
		{"section as field", ruleWith(models.RuleCondition{Field: "page2", Op: "exists"}), "not a question"},
		{"archived field", ruleWith(models.RuleCondition{Field: "old", Op: "eq", Value: "x"}), "archived"},
		{"not an option", ruleWith(models.RuleCondition{Field: "dept", Op: "eq", Value: "Sales"}), "not an option"},
		{"gt on choice", ruleWith(models.RuleCondition{Field: "dept", Op: "gt", Value: "1"}), "only compares"},
		{"gt not a number", ruleWith(models.RuleCondition{Field: "seats", Op: "gt", Value: "many"}), "not a number"},
		{"bad date", ruleWith(models.RuleCondition{Field: "start", Op: "gt", Value: "12/01/2026"}), "YYYY-MM-DD"},
		{"bad regex", ruleWith(models.RuleCondition{Field: "notes", Op: "matches", Value: "("}), "compile"},
		{"in without values", ruleWith(models.RuleCondition{Field: "dept", Op: "in", Value: []any{}}), "at least one"},
		{"unknown op", ruleWith(models.RuleCondition{Field: "dept", Op: "like", Value: "x"}), "unknown operator"},
		{"checkbox value", ruleWith(models.RuleCondition{Field: "vip", Op: "eq", Value: "maybe"}), "true or false"},
		// No action can take its destination from the submission.
		{"field as address", ruleDoing(models.RuleAction{Type: models.ActionNotifyEmails, Emails: []string{"email"}}), "not an email address"},
		{"template as address", ruleDoing(models.RuleAction{Type: models.ActionNotifyEmails, Emails: []string{"{{email}}"}}), "not an email address"},
		{"display name address", ruleDoing(models.RuleAction{Type: models.ActionNotifyEmails, Emails: []string{"Eve <eve@evil.com>"}}), "not an email address"},
		{"other workspace member", ruleDoing(models.RuleAction{Type: models.ActionNotifyMembers, UserIDs: []uint{8}}), "not a member"},
		{"other workspace team", ruleDoing(models.RuleAction{Type: models.ActionAssignTeam, TeamID: uintp(3)}), "teams"},
		{"other workspace webhook", ruleDoing(models.RuleAction{Type: models.ActionWebhook, WebhookID: uintp(5)}), "webhooks"},
		{"assign two members", ruleDoing(models.RuleAction{Type: models.ActionAssignMember, UserIDs: []uint{7, 7}}), "one member"},
		{"bad priority", ruleDoing(models.RuleAction{Type: models.ActionSetPriority, Priority: "asap"}), "priority"},
		{"bad tag", ruleDoing(models.RuleAction{Type: models.ActionAddTags, Tags: []string{"a,b"}}), "tag"},
		{"unknown action", ruleDoing(models.RuleAction{Type: "forward_to_field"}), "unknown action"},
	}
	for _, c := range bad {
		r := c.rule
		err := Check(&r, testForm(), dest)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want it to mention %q", c.name, err, c.want)
		}
	}

	// A form without questions accepts any field name, lowercased.
	free := ruleWith(models.RuleCondition{Field: "Department", Op: "eq", Value: "sales"})
	if err := Check(&free, &models.Form{}, dest); err != nil || free.Conditions[0].Field != "department" {
		t.Errorf("schemaless field: %v %q", err, free.Conditions[0].Field)
	}
}

func ruleWith(c models.RuleCondition) models.FormRule {
	return models.FormRule{Name: "r", Conditions: models.RuleConditions{c}, Actions: models.RuleActions{{Type: models.ActionSuppressDefault}}}
}

func ruleDoing(a models.RuleAction) models.FormRule {
	return models.FormRule{Name: "r", Actions: models.RuleActions{a}}
}

type fakeDir struct {
	members map[uint]models.WorkspaceRole
	teams   map[uint]*models.Team
}

func (f fakeDir) Members(uint) (map[uint]models.WorkspaceRole, error) { return f.members, nil }
func (f fakeDir) Team(id uint) (*models.Team, error) {
	if t, ok := f.teams[id]; ok {
		return t, nil
	}
	return nil, errNotFound
}

type notFound struct{}

func (notFound) Error() string { return "not found" }

var errNotFound = notFound{}

func TestResolve(t *testing.T) {
	ws, other := uint(1), uint(2)
	dir := fakeDir{
		members: map[uint]models.WorkspaceRole{1: models.WorkspaceRoleOwner, 2: models.WorkspaceRoleEditor, 3: models.WorkspaceRoleViewer},
		teams: map[uint]*models.Team{
			10: {ID: 10, WorkspaceID: &ws, Email: "Sales@acme.com", Members: []models.TeamMember{{UserID: 1}, {UserID: 2}, {UserID: 99}}},
			11: {ID: 11, WorkspaceID: &other, Email: "x@other.com", Members: []models.TeamMember{{UserID: 3}}},
		},
	}
	out := Outcome{Notify: []Recipient{{TeamID: 10}, {UserID: 3}, {Email: "sales@acme.com"}, {Email: "ops@acme.com"}, {TeamID: 11}}}

	// The owner is not added twice, and user 99 left the workspace.
	form := &models.Form{WorkspaceID: &ws}
	res, err := Resolve(form, out, dir)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Default || !reflect.DeepEqual(res.UserIDs, []uint{2, 3}) || !reflect.DeepEqual(res.Emails, []string{"sales@acme.com", "ops@acme.com"}) {
		t.Errorf("got %+v", res)
	}

	// Suppressed default: the owner is notified through the team.
	out.SuppressDefault = true
	res, _ = Resolve(form, out, dir)
	if res.Default || !reflect.DeepEqual(res.UserIDs, []uint{1, 2, 3}) {
		t.Errorf("suppressed: %+v", res)
	}

	// A default address list already covering an address is not repeated.
	out.SuppressDefault = false
	form.NotifyEmails = []string{"OPS@acme.com"}
	res, _ = Resolve(form, out, dir)
	if !reflect.DeepEqual(res.Emails, []string{"sales@acme.com"}) || !reflect.DeepEqual(res.UserIDs, []uint{1, 2, 3}) {
		t.Errorf("with notify_emails: %+v", res)
	}

	if res, _ := Resolve(form, Outcome{SuppressDefault: true}, dir); !res.Empty() {
		t.Errorf("suppressed with no rule recipients should notify nobody: %+v", res)
	}
}
