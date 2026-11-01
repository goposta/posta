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
	"regexp"
	"strings"

	"github.com/goposta/posta/internal/models"
)

const (
	MaxSchemaFields    = 100
	maxFieldOptions    = 200
	maxPatternLength   = 256
	defaultTextMaxLen  = 200
	defaultLongMaxLen  = 8000
	defaultScaleMin    = 1
	defaultScaleMax    = 5
	defaultRatingStars = 5
)

var fieldKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_]{0,63}$`)

var fieldTypes = map[models.FieldType]bool{
	models.FieldText: true, models.FieldTextarea: true, models.FieldEmail: true,
	models.FieldPhone: true, models.FieldNumber: true, models.FieldURL: true,
	models.FieldDate: true, models.FieldTime: true, models.FieldSelect: true,
	models.FieldMultiSelect: true, models.FieldCheckbox: true, models.FieldScale: true,
	models.FieldRating: true, models.FieldHidden: true,
}

// roleTypes lists the field types each role may sit on.
var roleTypes = map[models.FieldRole][]models.FieldType{
	models.RoleSenderEmail: {models.FieldEmail},
	models.RoleSenderName:  {models.FieldText},
	models.RoleSenderPhone: {models.FieldPhone},
	models.RoleSubject:     {models.FieldText, models.FieldSelect},
	models.RoleBody:        {models.FieldTextarea, models.FieldText},
}

// CheckSchema reports the first problem with a schema about to be saved.
// previous is the schema currently stored; its keys must all survive, since
// stored submissions refer to them. A key can be archived instead.
func CheckSchema(defs models.FieldDefs, previous models.FieldDefs, honeypot string) error {
	if len(defs) > MaxSchemaFields {
		return fmt.Errorf("a form can have at most %d fields", MaxSchemaFields)
	}
	seen := make(map[string]bool, len(defs))
	roles := make(map[models.FieldRole]string)
	for i := range defs {
		f := &defs[i]
		f.Key = strings.TrimSpace(f.Key)
		f.Label = strings.TrimSpace(f.Label)
		if !fieldKeyPattern.MatchString(f.Key) {
			return fmt.Errorf("field key %q must be 1-64 lowercase letters, digits or underscores, starting with a letter or digit", f.Key)
		}
		if seen[f.Key] {
			return fmt.Errorf("field key %q is used twice", f.Key)
		}
		seen[f.Key] = true
		if strings.EqualFold(f.Key, honeypot) {
			return fmt.Errorf("field key %q is the form's honeypot field", f.Key)
		}
		if !fieldTypes[f.Type] {
			return fmt.Errorf("field %q has unknown type %q", f.Key, f.Type)
		}
		if f.Label == "" && f.Type != models.FieldHidden {
			return fmt.Errorf("field %q needs a label", f.Key)
		}
		if err := checkRole(f, roles); err != nil {
			return err
		}
		if err := checkFieldSettings(f); err != nil {
			return err
		}
	}
	for _, old := range previous {
		if !seen[old.Key] {
			return fmt.Errorf("field %q cannot be removed because submissions may use it; archive it instead", old.Key)
		}
	}
	return nil
}

func checkRole(f *models.FieldDef, roles map[models.FieldRole]string) error {
	if f.Role == models.RoleNone {
		return nil
	}
	allowed, known := roleTypes[f.Role]
	if !known {
		return fmt.Errorf("field %q has unknown role %q", f.Key, f.Role)
	}
	ok := false
	for _, t := range allowed {
		ok = ok || t == f.Type
	}
	if !ok {
		return fmt.Errorf("field %q: role %s does not fit a %s field", f.Key, f.Role, f.Type)
	}
	if f.Archived {
		return nil
	}
	if other, dup := roles[f.Role]; dup {
		return fmt.Errorf("fields %q and %q both have role %s", other, f.Key, f.Role)
	}
	roles[f.Role] = f.Key
	return nil
}

func checkFieldSettings(f *models.FieldDef) error {
	if f.MinLen != nil && *f.MinLen < 0 {
		return fmt.Errorf("field %q: min_len cannot be negative", f.Key)
	}
	if f.MaxLen != nil && (*f.MaxLen < 1 || *f.MaxLen > defaultLongMaxLen) {
		return fmt.Errorf("field %q: max_len must be between 1 and %d", f.Key, defaultLongMaxLen)
	}
	if f.MinLen != nil && f.MaxLen != nil && *f.MinLen > *f.MaxLen {
		return fmt.Errorf("field %q: min_len is above max_len", f.Key)
	}
	if f.Pattern != "" {
		if len(f.Pattern) > maxPatternLength {
			return fmt.Errorf("field %q: pattern is longer than %d characters", f.Key, maxPatternLength)
		}
		if _, err := regexp.Compile(f.Pattern); err != nil {
			return fmt.Errorf("field %q: pattern does not compile: %v", f.Key, err)
		}
	}

	switch f.Type {
	case models.FieldSelect, models.FieldMultiSelect:
		if len(f.Options) == 0 || len(f.Options) > maxFieldOptions {
			return fmt.Errorf("field %q needs between 1 and %d options", f.Key, maxFieldOptions)
		}
		values := make(map[string]bool, len(f.Options))
		for i := range f.Options {
			o := &f.Options[i]
			o.Value = strings.TrimSpace(o.Value)
			o.Label = strings.TrimSpace(o.Label)
			if o.Value == "" {
				return fmt.Errorf("field %q has an option without a value", f.Key)
			}
			if values[o.Value] {
				return fmt.Errorf("field %q has option %q twice", f.Key, o.Value)
			}
			values[o.Value] = true
		}
		valid := f.Display == models.DisplayDefault
		if f.Type == models.FieldSelect {
			valid = valid || f.Display == models.DisplayRadio || f.Display == models.DisplayDropdown
		} else {
			valid = valid || f.Display == models.DisplayCheckboxes || f.Display == models.DisplayList
		}
		if !valid {
			return fmt.Errorf("field %q: display %q does not fit a %s field", f.Key, f.Display, f.Type)
		}
	case models.FieldNumber:
		if f.Min != nil && f.Max != nil && *f.Min > *f.Max {
			return fmt.Errorf("field %q: min is above max", f.Key)
		}
	case models.FieldScale:
		lo, hi := ScaleRange(f)
		if (lo != 0 && lo != 1) || hi < 2 || hi > 10 {
			return fmt.Errorf("field %q: a scale runs from 0 or 1 to between 2 and 10", f.Key)
		}
	case models.FieldRating:
		if stars := RatingStars(f); stars < 3 || stars > 10 {
			return fmt.Errorf("field %q: a rating has between 3 and 10 stars", f.Key)
		}
	}
	if f.Display != models.DisplayDefault && f.Type != models.FieldSelect && f.Type != models.FieldMultiSelect {
		return fmt.Errorf("field %q: display only applies to select and multiselect fields", f.Key)
	}
	return nil
}

// ScaleRange returns a scale field's low and high ends.
func ScaleRange(f *models.FieldDef) (int, int) {
	lo, hi := defaultScaleMin, defaultScaleMax
	if f.Min != nil {
		lo = int(*f.Min)
	}
	if f.Max != nil {
		hi = int(*f.Max)
	}
	return lo, hi
}

// RatingStars returns a rating field's number of stars.
func RatingStars(f *models.FieldDef) int {
	if f.Max != nil {
		return int(*f.Max)
	}
	return defaultRatingStars
}

// MaxLen returns the longest accepted value of a text-like field.
func MaxLen(f *models.FieldDef) int {
	if f.MaxLen != nil {
		return *f.MaxLen
	}
	if f.Type == models.FieldTextarea {
		return defaultLongMaxLen
	}
	return defaultTextMaxLen
}
