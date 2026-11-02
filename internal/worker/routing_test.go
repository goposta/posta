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

package worker

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/goposta/posta/internal/models"
)

type fakeRules struct {
	rules []models.FormRule
	hits  [][]string
}

func (f *fakeRules) ListByForm(uint) ([]models.FormRule, error) { return f.rules, nil }
func (f *fakeRules) RecordHits(uuids []string, _ time.Time) error {
	f.hits = append(f.hits, uuids)
	return nil
}

type fakeDirectory struct{ ws uint }

func (f fakeDirectory) Members(uint) (map[uint]models.WorkspaceRole, error) {
	return map[uint]models.WorkspaceRole{1: models.WorkspaceRoleOwner, 7: models.WorkspaceRoleEditor, 8: models.WorkspaceRoleEditor}, nil
}

func (f fakeDirectory) Team(id uint) (*models.Team, error) {
	if id != 3 {
		return nil, errors.New("not found")
	}
	return &models.Team{ID: 3, WorkspaceID: &f.ws, Email: "sales@acme.com", Members: []models.TeamMember{{UserID: 8}}}, nil
}

func salesRule(suppress bool) models.FormRule {
	team, hook := uint(3), uint(9)
	actions := models.RuleActions{
		{Type: models.ActionNotifyTeam, TeamID: &team},
		{Type: models.ActionAssignTeam, TeamID: &team},
		{Type: models.ActionAssignMember, UserIDs: []uint{7}},
		{Type: models.ActionAddTags, Tags: []string{"lead"}},
		{Type: models.ActionSetPriority, Priority: "high"},
		{Type: models.ActionWebhook, WebhookID: &hook},
	}
	if suppress {
		actions = append(actions, models.RuleAction{Type: models.ActionSuppressDefault})
	}
	return models.FormRule{
		UUID: "r-sales", Name: "Sales", Enabled: true,
		Conditions: models.RuleConditions{{Field: "dept", Op: "eq", Value: "sales"}},
		Actions:    actions,
	}
}

func routedFixture(suppress bool) (*MessageProcessHandler, *fakeMessageStore, *fakeDispatcher, *fakeNotifier, *fakeRules) {
	ws := uint(5)
	form := immediateForm()
	form.WorkspaceID = &ws
	form.Fields = models.FieldDefs{{Key: "dept", Label: "Dept", Type: models.FieldSelect, Options: []models.FieldOption{{Value: "sales"}}}}
	store := &fakeMessageStore{msg: &models.Message{
		ID: 1, WorkspaceID: &ws, Status: models.MessageStatusReceived,
		Data: models.SubmissionData{"dept": "sales"}, Tags: []string{"existing"},
	}}
	h, d, n := newTestHandler(store, form)
	rules := &fakeRules{rules: []models.FormRule{salesRule(suppress)}}
	h.rules, h.directory = rules, fakeDirectory{ws: ws}
	return h, store, d, n, rules
}

func TestProcessTaskAppliesRulesOnce(t *testing.T) {
	h, store, d, n, rules := routedFixture(false)
	n.err = errors.New("smtp down")
	if err := h.ProcessTask(context.Background(), processTask(t)); err == nil {
		t.Fatal("a failed notification must fail the task")
	}

	m := store.msg
	if m.RoutedAt == nil || !reflect.DeepEqual([]string(m.MatchedRules), []string{"r-sales"}) {
		t.Fatalf("not routed: %+v", m)
	}
	routing := store.updates[0]
	if routing["team_id"] != uint(3) || routing["assigned_to_id"] != uint(7) || routing["priority"] != "high" {
		t.Errorf("routing update %+v", routing)
	}
	if tags := fmt.Sprint(routing["tags"]); tags != "[existing lead]" {
		t.Errorf("tags %s", tags)
	}
	if !reflect.DeepEqual(d.direct, []uint{9}) {
		t.Errorf("rule webhook dispatched to %v", d.direct)
	}
	// Default route (admins) plus the team: its member and its address.
	// The owner is already among the admins.
	if !reflect.DeepEqual(n.sent, []string{"admins", "to:sales@acme.com", "user"}) || !reflect.DeepEqual(n.users, []uint{8}) {
		t.Errorf("sent %v users %v", n.sent, n.users)
	}

	// The retry notifies the same people, without routing or counting again.
	n.err, n.sent, n.users = nil, nil, nil
	updates := len(store.updates)
	if err := h.ProcessTask(context.Background(), processTask(t)); err != nil {
		t.Fatal(err)
	}
	if len(rules.hits) != 1 {
		t.Errorf("rule hits recorded %d times", len(rules.hits))
	}
	if len(store.updates) != updates || len(d.direct) != 1 {
		t.Errorf("the retry routed or dispatched again: %d updates, direct %v", len(store.updates)-updates, d.direct)
	}
	if !reflect.DeepEqual(n.sent, []string{"admins", "to:sales@acme.com", "user"}) {
		t.Errorf("retry sent %v", n.sent)
	}
}

func TestProcessTaskSuppressedDefault(t *testing.T) {
	h, _, _, n, _ := routedFixture(true)
	if err := h.ProcessTask(context.Background(), processTask(t)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(n.sent, []string{"to:sales@acme.com", "user"}) {
		t.Errorf("sent %v", n.sent)
	}
}

func TestProcessTaskNoMatchKeepsDefault(t *testing.T) {
	h, store, d, n, _ := routedFixture(true)
	store.msg.Data = models.SubmissionData{"dept": "support"}
	if err := h.ProcessTask(context.Background(), processTask(t)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(n.sent, []string{"admins"}) || len(d.direct) != 0 || store.msg.TeamID != nil {
		t.Errorf("unmatched submission: sent %v direct %v", n.sent, d.direct)
	}
	if store.msg.RoutedAt == nil {
		t.Error("an unmatched submission is still marked routed")
	}
}

func TestRulesNeverLowerPriority(t *testing.T) {
	if higherPriority("high", "urgent") || !higherPriority("high", "") || higherPriority("", "low") {
		t.Error("higherPriority")
	}
	if got := mergeTags([]string{"a"}, []string{"a", "b"}); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("mergeTags = %v", got)
	}
}
