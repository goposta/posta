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
	"strings"

	"github.com/goposta/posta/internal/models"
)

// The worker and the digest pass repositories; tests pass a map.
type Directory interface {
	Members(workspaceID uint) (map[uint]models.WorkspaceRole, error)
	Team(id uint) (*models.Team, error)
}

type Resolved struct {
	// Default is the form's own route: its notification addresses, or the owners and admins when it has none.
	Default bool
	Emails  []string
	UserIDs []uint
}

func (r Resolved) Empty() bool { return !r.Default && len(r.Emails) == 0 && len(r.UserIDs) == 0 }

// Teams expand to members plus their address, ex-members are dropped, and the default route is not duplicated.
func Resolve(form *models.Form, out Outcome, dir Directory) (Resolved, error) {
	res := Resolved{Default: !out.SuppressDefault}
	if len(out.Notify) == 0 || form.WorkspaceID == nil {
		return res, nil
	}
	members, err := dir.Members(*form.WorkspaceID)
	if err != nil {
		return res, err
	}
	defaultEmails := map[string]bool{}
	for _, e := range form.NotifyEmails {
		defaultEmails[strings.ToLower(strings.TrimSpace(e))] = true
	}
	defaultIsAdmins := len(form.NotifyEmails) == 0

	seenEmail := map[string]bool{}
	seenUser := map[uint]bool{}
	addEmail := func(e string) {
		e = strings.ToLower(strings.TrimSpace(e))
		if e == "" || seenEmail[e] || res.Default && defaultEmails[e] {
			return
		}
		seenEmail[e] = true
		res.Emails = append(res.Emails, e)
	}
	addUser := func(id uint) {
		role, member := members[id]
		if !member || seenUser[id] || res.Default && defaultIsAdmins && role.CanManageMembers() {
			return
		}
		seenUser[id] = true
		res.UserIDs = append(res.UserIDs, id)
	}
	for _, r := range out.Notify {
		switch {
		case r.Email != "":
			addEmail(r.Email)
		case r.UserID != 0:
			addUser(r.UserID)
		case r.TeamID != 0:
			team, err := dir.Team(r.TeamID)
			if err != nil || team.WorkspaceID == nil || *team.WorkspaceID != *form.WorkspaceID {
				continue
			}
			for _, m := range team.Members {
				addUser(m.UserID)
			}
			addEmail(team.Email)
		}
	}
	return res, nil
}

// CachedDirectory remembers lookups for the life of one job run.
type CachedDirectory struct {
	Next    Directory
	members map[uint]map[uint]models.WorkspaceRole
	teams   map[uint]*models.Team
}

func (c *CachedDirectory) Members(workspaceID uint) (map[uint]models.WorkspaceRole, error) {
	if m, ok := c.members[workspaceID]; ok {
		return m, nil
	}
	m, err := c.Next.Members(workspaceID)
	if err != nil {
		return nil, err
	}
	if c.members == nil {
		c.members = map[uint]map[uint]models.WorkspaceRole{}
	}
	c.members[workspaceID] = m
	return m, nil
}

func (c *CachedDirectory) Team(id uint) (*models.Team, error) {
	if t, ok := c.teams[id]; ok {
		return t, nil
	}
	t, err := c.Next.Team(id)
	if err != nil {
		return nil, err
	}
	if c.teams == nil {
		c.teams = map[uint]*models.Team{}
	}
	c.teams[id] = t
	return t, nil
}
