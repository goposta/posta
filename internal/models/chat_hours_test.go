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
	"testing"
	"time"
)

func TestBusinessHoursNextOpen(t *testing.T) {
	paris, _ := time.LoadLocation("Europe/Paris")
	ny, _ := time.LoadLocation("America/New_York")
	weekdays := func(tz string) BusinessHours {
		b := BusinessHours{Timezone: tz}
		for d := 1; d <= 5; d++ {
			b.Days = append(b.Days, HoursRange{Day: d, From: "09:00", To: "17:00"})
		}
		return b
	}
	for _, tc := range []struct {
		name  string
		hours BusinessHours
		now   time.Time
		want  time.Time
	}{
		{"before opening today", weekdays("Europe/Paris"), time.Date(2026, 11, 10, 7, 30, 0, 0, paris), time.Date(2026, 11, 10, 9, 0, 0, 0, paris)},
		{"after closing, next morning", weekdays("Europe/Paris"), time.Date(2026, 11, 10, 18, 0, 0, 0, paris), time.Date(2026, 11, 11, 9, 0, 0, 0, paris)},
		{"friday evening to monday", weekdays("Europe/Paris"), time.Date(2026, 11, 13, 18, 0, 0, 0, paris), time.Date(2026, 11, 16, 9, 0, 0, 0, paris)},
		{"just before midnight", weekdays("Europe/Paris"), time.Date(2026, 11, 10, 23, 59, 0, 0, paris), time.Date(2026, 11, 11, 9, 0, 0, 0, paris)},
		{"spring forward keeps 09:00 local", weekdays("Europe/Paris"), time.Date(2026, 3, 27, 18, 0, 0, 0, paris), time.Date(2026, 3, 30, 9, 0, 0, 0, paris)},
		{"fall back keeps 09:00 local", weekdays("America/New_York"), time.Date(2026, 10, 30, 20, 0, 0, 0, ny), time.Date(2026, 11, 2, 9, 0, 0, 0, ny)},
		{"other zone than the caller", weekdays("America/New_York"), time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC), time.Date(2026, 11, 10, 9, 0, 0, 0, ny)},
		{"one day a week, a week ahead", BusinessHours{Days: []HoursRange{{Day: 2, From: "10:00", To: "11:00"}}}, time.Date(2026, 11, 10, 10, 30, 0, 0, time.UTC), time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC)},
	} {
		got, ok := tc.hours.NextOpen(tc.now)
		if !ok || !got.Equal(tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
	if _, ok := (BusinessHours{}).NextOpen(time.Now()); ok {
		t.Error("hours with no ranges reported a next opening")
	}
}
