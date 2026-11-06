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

package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

type FieldType string

const (
	FieldText        FieldType = "text"
	FieldTextarea    FieldType = "textarea"
	FieldEmail       FieldType = "email"
	FieldPhone       FieldType = "phone"
	FieldNumber      FieldType = "number"
	FieldURL         FieldType = "url"
	FieldDate        FieldType = "date"
	FieldTime        FieldType = "time"
	FieldSelect      FieldType = "select"
	FieldMultiSelect FieldType = "multiselect"
	FieldCheckbox    FieldType = "checkbox"
	FieldScale       FieldType = "scale"
	FieldRating      FieldType = "rating"
	FieldHidden      FieldType = "hidden"
	// FieldSection is not a question: it starts a new page holding the questions up to the next section.
	FieldSection FieldType = "section"
)

// GotoSubmit is a branch target that ends the form, submitting from the current page.
const GotoSubmit = "submit"

// FieldRole marks the field carrying one of a message's summary values, instead of guessing from key names.
type FieldRole string

const (
	RoleNone        FieldRole = ""
	RoleSenderEmail FieldRole = "sender_email"
	RoleSenderName  FieldRole = "sender_name"
	RoleSenderPhone FieldRole = "sender_phone"
	RoleSubject     FieldRole = "subject"
	RoleBody        FieldRole = "body"
)

// FieldDisplay chooses how a choice field is drawn; it never changes what is accepted.
type FieldDisplay string

const (
	DisplayDefault    FieldDisplay = ""
	DisplayRadio      FieldDisplay = "radio"      // select
	DisplayDropdown   FieldDisplay = "dropdown"   // select
	DisplayCheckboxes FieldDisplay = "checkboxes" // multiselect
	DisplayList       FieldDisplay = "list"       // multiselect
)

// UnknownFieldsPolicy decides what happens to submitted keys the schema does not declare.
type UnknownFieldsPolicy string

const (
	UnknownFieldsKeep   UnknownFieldsPolicy = "keep"
	UnknownFieldsDrop   UnknownFieldsPolicy = "drop"
	UnknownFieldsReject UnknownFieldsPolicy = "reject"
)

type FieldOption struct {
	Value string `json:"value"`
	Label string `json:"label,omitempty"`
	// Goto (single-choice only) leads to a later section key or GotoSubmit after this page; empty follows Next.
	Goto string `json:"goto,omitempty"`
}

// A saved key can be archived but never renamed or removed, so old submissions keep their meaning.
type FieldDef struct {
	Key      string    `json:"key"`
	Label    string    `json:"label"`
	Type     FieldType `json:"type"`
	Required bool      `json:"required,omitempty"`
	Role     FieldRole `json:"role,omitempty"`
	PII      bool      `json:"pii,omitempty"`

	MinLen  *int     `json:"min_len,omitempty"`
	MaxLen  *int     `json:"max_len,omitempty"`
	Min     *float64 `json:"min,omitempty"` // number; scale low end
	Max     *float64 `json:"max,omitempty"` // number; scale high end; rating stars
	Pattern string   `json:"pattern,omitempty"`

	Options  []FieldOption `json:"options,omitempty"`
	Display  FieldDisplay  `json:"display,omitempty"`
	MinLabel string        `json:"min_label,omitempty"` // scale
	MaxLabel string        `json:"max_label,omitempty"` // scale

	Placeholder string `json:"placeholder,omitempty"`
	Help        string `json:"help,omitempty"`
	Archived    bool   `json:"archived,omitempty"`

	// Next is where a section's page leads absent a branch: a later section key or GotoSubmit; empty continues.
	Next string `json:"next,omitempty"`
}

func (f *FieldDef) IsSection() bool { return f.Type == FieldSection }

// FieldDefs is a form's jsonb schema; nil means no schema, accepting any fields.
type FieldDefs []FieldDef

func (d FieldDefs) Value() (driver.Value, error) {
	if d == nil {
		return nil, nil
	}
	return json.Marshal(d)
}

func (d *FieldDefs) Scan(value any) error {
	raw, err := jsonBytes(value)
	if err != nil || raw == nil {
		*d = nil
		return err
	}
	return json.Unmarshal(raw, d)
}

func (d FieldDefs) Active() FieldDefs {
	out := make(FieldDefs, 0, len(d))
	for _, f := range d {
		if !f.Archived {
			out = append(out, f)
		}
	}
	return out
}

// Questions returns the entries that ask something, archived ones included, leaving out sections.
func (d FieldDefs) Questions() FieldDefs {
	out := make(FieldDefs, 0, len(d))
	for _, f := range d {
		if !f.IsSection() {
			out = append(out, f)
		}
	}
	return out
}

// SubmissionData keeps numbers, booleans and multi-select arrays as their JSON types.
type SubmissionData map[string]any

func (d SubmissionData) Value() (driver.Value, error) {
	if d == nil {
		return nil, nil
	}
	return json.Marshal(d)
}

func (d *SubmissionData) Scan(value any) error {
	raw, err := jsonBytes(value)
	if err != nil || raw == nil {
		*d = nil
		return err
	}
	return json.Unmarshal(raw, d)
}

// FormAppearance is a form's saved look for its hosted page and widget default; embed attributes win.
type FormAppearance struct {
	Accent         string `json:"accent,omitempty"`           // #RRGGBB
	Theme          string `json:"theme,omitempty"`            // light, dark or auto
	HeaderImageURL string `json:"header_image_url,omitempty"` // https only
}

func (a FormAppearance) Value() (driver.Value, error) {
	if a == (FormAppearance{}) {
		return nil, nil
	}
	return json.Marshal(a)
}

func (a *FormAppearance) Scan(value any) error {
	raw, err := jsonBytes(value)
	if err != nil || raw == nil {
		*a = FormAppearance{}
		return err
	}
	return json.Unmarshal(raw, a)
}

func jsonBytes(value any) ([]byte, error) {
	switch v := value.(type) {
	case nil:
		return nil, nil
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	default:
		return nil, fmt.Errorf("unsupported JSON column value %T", value)
	}
}
