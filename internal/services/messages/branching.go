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

import "github.com/goposta/posta/internal/models"

// Page is one page of a form: the section that opens it (nil for the first
// page, before any section) and its active questions, in order.
type Page struct {
	Section   *models.FieldDef
	Questions []*models.FieldDef
}

// Pages splits a schema's active entries into pages. A form without sections
// is one page.
func Pages(defs models.FieldDefs) []Page {
	pages := []Page{{}}
	for i := range defs {
		f := &defs[i]
		if f.Archived {
			continue
		}
		if f.IsSection() {
			pages = append(pages, Page{Section: f})
			continue
		}
		last := &pages[len(pages)-1]
		last.Questions = append(last.Questions, f)
	}
	// A form that opens with a section has no questions before it.
	if len(pages) > 1 && len(pages[0].Questions) == 0 {
		pages = pages[1:]
	}
	return pages
}

// Route walks a form the way a respondent moves through it and returns the
// keys of the questions on the pages they reached. answer gives a question's
// first submitted value. Hidden questions are always included: an embed sets
// them, they sit on no page.
//
// After each page the respondent goes where the chosen option of a branching
// question points, else where the section's Next points, else to the next
// page. Targets only ever lie ahead (CheckSchema), so the walk ends.
func Route(defs models.FieldDefs, answer func(key string) string) map[string]bool {
	pages := Pages(defs)
	index := make(map[string]int, len(pages))
	for i, p := range pages {
		if p.Section != nil {
			index[p.Section.Key] = i
		}
	}
	reached := make(map[string]bool)
	for _, f := range defs {
		if f.Type == models.FieldHidden && !f.Archived {
			reached[f.Key] = true
		}
	}
	for i := 0; i < len(pages); {
		p := pages[i]
		for _, q := range p.Questions {
			reached[q.Key] = true
		}
		target := ""
		if p.Section != nil {
			target = p.Section.Next
		}
		for _, q := range p.Questions {
			if goto_ := branchTarget(q, answer(q.Key)); goto_ != "" {
				target = goto_
			}
		}
		next := i + 1
		switch target {
		case "":
		case models.GotoSubmit:
			return reached
		default:
			if j, ok := index[target]; ok && j > i {
				next = j
			}
		}
		i = next
	}
	return reached
}

// branchTarget is where answering q with value leads, or "" when q does not
// branch on that value.
func branchTarget(q *models.FieldDef, value string) string {
	if q.Type != models.FieldSelect || value == "" {
		return ""
	}
	for _, o := range q.Options {
		if o.Value == value {
			return o.Goto
		}
	}
	return ""
}

// branches reports whether any option of q leads somewhere.
func branches(q *models.FieldDef) bool {
	for _, o := range q.Options {
		if o.Goto != "" {
			return true
		}
	}
	return false
}

// BranchTargets lists a schema's branching, keyed by where each target is
// set: "section:<key>" for a section's Next, "option:<question>=<value>" for
// an option's Goto. It is empty when the form does not branch.
func BranchTargets(defs models.FieldDefs) map[string]string {
	out := map[string]string{}
	for _, d := range defs {
		if d.IsSection() && d.Next != "" {
			out["section:"+d.Key] = d.Next
		}
		for _, o := range d.Options {
			if o.Goto != "" {
				out["option:"+d.Key+"="+o.Value] = o.Goto
			}
		}
	}
	return out
}
