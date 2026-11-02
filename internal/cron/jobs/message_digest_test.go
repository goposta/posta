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

package jobs

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/goposta/posta/internal/models"
)

type digestDir struct{ ws uint }

func (d digestDir) Members(uint) (map[uint]models.WorkspaceRole, error) {
	return map[uint]models.WorkspaceRole{1: models.WorkspaceRoleOwner, 7: models.WorkspaceRoleEditor, 8: models.WorkspaceRoleEditor}, nil
}

func (d digestDir) Team(id uint) (*models.Team, error) {
	switch id {
	case 3:
		return &models.Team{ID: 3, WorkspaceID: &d.ws, Members: []models.TeamMember{{UserID: 7}}}, nil
	case 4:
		return &models.Team{ID: 4, WorkspaceID: &d.ws, Email: "billing@acme.com", Members: []models.TeamMember{{UserID: 8}}}, nil
	}
	return nil, errors.New("not found")
}

// Two rules sending to two teams give each team its own digest, and the
// default route keeps everything unless a rule suppresses it.
func TestDigestTargetsFollowRules(t *testing.T) {
	ws := uint(5)
	form := &models.Form{ID: 1, WorkspaceID: &ws, Fields: models.FieldDefs{
		{Key: "dept", Label: "Dept", Type: models.FieldSelect, Options: []models.FieldOption{{Value: "sales"}, {Value: "billing"}}},
	}}
	sales, billing := uint(3), uint(4)
	rules := []models.FormRule{
		{UUID: "s", Enabled: true, Conditions: models.RuleConditions{{Field: "dept", Op: "eq", Value: "sales"}},
			Actions: models.RuleActions{{Type: models.ActionNotifyTeam, TeamID: &sales}, {Type: models.ActionSuppressDefault}}},
		{UUID: "b", Enabled: true, Conditions: models.RuleConditions{{Field: "dept", Op: "eq", Value: "billing"}},
			Actions: models.RuleActions{{Type: models.ActionNotifyTeam, TeamID: &billing}}},
	}
	j := &MessageDigestJob{}
	dir := digestDir{ws: ws}
	msg := func(dept string) *models.Message {
		return &models.Message{Status: models.MessageStatusReceived, Data: models.SubmissionData{"dept": dept}}
	}

	if got := j.targets(form, rules, dir, msg("sales")); !reflect.DeepEqual(got, []digestTarget{{UserID: 7}}) {
		t.Errorf("sales: %+v", got)
	}
	if got := j.targets(form, rules, dir, msg("billing")); !reflect.DeepEqual(got, []digestTarget{{Default: true}, {Email: "billing@acme.com"}, {UserID: 8}}) {
		t.Errorf("billing: %+v", got)
	}
	if got := j.targets(form, rules, dir, msg("other")); !reflect.DeepEqual(got, []digestTarget{{Default: true}}) {
		t.Errorf("unmatched: %+v", got)
	}

	// A routed submission replays its stored matches, not today's rules.
	routed := msg("billing")
	now := time.Now()
	routed.RoutedAt, routed.MatchedRules = &now, []string{"s"}
	if got := j.targets(form, rules, dir, routed); !reflect.DeepEqual(got, []digestTarget{{UserID: 7}}) {
		t.Errorf("routed: %+v", got)
	}
	if got := j.targets(form, nil, dir, msg("sales")); !reflect.DeepEqual(got, []digestTarget{{Default: true}}) {
		t.Errorf("no rules: %+v", got)
	}
}
