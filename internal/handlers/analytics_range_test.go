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

package handlers

import (
	"strings"
	"testing"
	"time"
)

func TestParseTimeRangeIsUTCAndInclusive(t *testing.T) {
	from, to, err := parseTimeRange("2026-03-01", "2026-03-31")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if from.Location() != time.UTC || to.Location() != time.UTC {
		t.Fatalf("boundaries must be UTC to match how rows are bucketed, got %s and %s", from.Location(), to.Location())
	}
	if from.Format(time.RFC3339) != "2026-03-01T00:00:00Z" {
		t.Errorf("from should be midnight UTC, got %s", from.Format(time.RFC3339))
	}
	// Anything accepted in the final second of the day must be included.
	lastSecond := time.Date(2026, 3, 31, 23, 59, 59, 0, time.UTC)
	if to.Before(lastSecond) {
		t.Errorf("to (%s) excludes the last second of the day", to.Format(time.RFC3339Nano))
	}
	if !to.Before(time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("to (%s) leaks into the next day", to.Format(time.RFC3339Nano))
	}
}

func TestParseTimeRangeDefaultsToThirtyDays(t *testing.T) {
	from, to, err := parseTimeRange("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	days := int(to.Sub(from).Hours()/24) + 1
	if days != defaultAnalyticsRangeDays {
		t.Errorf("default range is %d days, want %d", days, defaultAnalyticsRangeDays)
	}
}

// Without a cap the server would build a point per day, per chart, in memory and in the response.
func TestParseTimeRangeRejectsAbsurdSpans(t *testing.T) {
	_, _, err := parseTimeRange("1970-01-01", "2099-01-01")
	if err == nil {
		t.Fatal("expected a range that large to be refused")
	}
	if !strings.Contains(err.Error(), "maximum") {
		t.Errorf("the error should say what the limit is, got %q", err)
	}
}

func TestParseTimeRangeRejectsBadInput(t *testing.T) {
	if _, _, err := parseTimeRange("2026-03-10", "2026-03-01"); err == nil {
		t.Error("expected an inverted range to be refused")
	}
	if _, _, err := parseTimeRange("last-tuesday", ""); err == nil {
		t.Error("expected an unparseable date to be refused")
	}
	if _, _, err := parseTimeRange("", "not-a-date"); err == nil {
		t.Error("expected an unparseable date to be refused")
	}
}

// Duration.Hours() rounds 9599h59m59.999999999s to 9600; a range exactly on the cap must still pass.
func TestParseTimeRangeCapBoundary(t *testing.T) {
	today := time.Now().UTC().Format("2006-01-02")

	at := time.Now().UTC().AddDate(0, 0, -(maxAnalyticsRangeDays - 1)).Format("2006-01-02")
	if _, _, err := parseTimeRange(at, today); err != nil {
		t.Errorf("a range exactly at the cap should be allowed: %v", err)
	}

	over := time.Now().UTC().AddDate(0, 0, -maxAnalyticsRangeDays).Format("2006-01-02")
	if _, _, err := parseTimeRange(over, today); err == nil {
		t.Error("a range one day past the cap should be refused")
	}
}
