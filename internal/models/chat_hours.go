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
	"time"
)

// BusinessHours are weekly staffed ranges in one time zone; no days means always.
type BusinessHours struct {
	Timezone string       `json:"timezone,omitempty" doc:"IANA time zone, e.g. Europe/Paris; UTC when empty"`
	Days     []HoursRange `json:"days,omitempty"`
}

// HoursRange Day 0 is Sunday; From and To are HH:MM in the hours' zone, To after From.
type HoursRange struct {
	Day  int    `json:"day" minimum:"0" maximum:"6"`
	From string `json:"from" example:"09:00"`
	To   string `json:"to" example:"17:30"`
}

func (b BusinessHours) Value() (driver.Value, error) {
	if b.Timezone == "" && len(b.Days) == 0 {
		return nil, nil
	}
	return json.Marshal(b)
}

func (b *BusinessHours) Scan(value any) error {
	*b = BusinessHours{}
	switch v := value.(type) {
	case nil:
		return nil
	case []byte:
		return json.Unmarshal(v, b)
	case string:
		return json.Unmarshal([]byte(v), b)
	}
	return fmt.Errorf("business hours: unsupported type %T", value)
}

func (b BusinessHours) Validate() error {
	if _, err := b.location(); err != nil {
		return fmt.Errorf("business_hours.timezone %q is not a known time zone", b.Timezone)
	}
	for _, r := range b.Days {
		from, okFrom := clockMinutes(r.From)
		to, okTo := clockMinutes(r.To)
		if r.Day < 0 || r.Day > 6 || !okFrom || !okTo || to <= from {
			return fmt.Errorf("business_hours: each range needs a day 0-6 and HH:MM from before to")
		}
	}
	return nil
}

// Open treats no ranges as always open.
func (b BusinessHours) Open(t time.Time) bool {
	if len(b.Days) == 0 {
		return true
	}
	loc, err := b.location()
	if err != nil {
		return true
	}
	local := t.In(loc)
	now := local.Hour()*60 + local.Minute()
	for _, r := range b.Days {
		if r.Day != int(local.Weekday()) {
			continue
		}
		from, _ := clockMinutes(r.From)
		to, _ := clockMinutes(r.To)
		if now >= from && now < to {
			return true
		}
	}
	return false
}

func (b BusinessHours) location() (*time.Location, error) {
	if b.Timezone == "" {
		return time.UTC, nil
	}
	return time.LoadLocation(b.Timezone)
}

// clockMinutes reads HH:MM as minutes after midnight; 24:00 is the end of day.
func clockMinutes(s string) (int, bool) {
	var h, m int
	if len(s) != 5 || s[2] != ':' {
		return 0, false
	}
	if _, err := fmt.Sscanf(s, "%02d:%02d", &h, &m); err != nil {
		return 0, false
	}
	if h < 0 || m < 0 || m > 59 || h > 24 || (h == 24 && m != 0) {
		return 0, false
	}
	return h*60 + m, true
}
