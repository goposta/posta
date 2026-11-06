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

package repositories

import (
	"testing"
	"time"
)

func dates(points []DeliveryRatePoint) []string {
	out := make([]string, 0, len(points))
	for _, p := range points {
		out = append(out, p.Date)
	}
	return out
}

// The series is built in UTC to match the queries; time.Truncate would shift it a day on a server west of UTC.
func TestSeriesIsUTCRegardlessOfServerZone(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("tzdata unavailable")
	}
	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Skip("tzdata unavailable")
	}

	want := []string{"2026-03-08", "2026-03-09", "2026-03-10"}
	for _, loc := range []*time.Location{time.UTC, ny, tokyo} {
		from := time.Date(2026, 3, 8, 0, 0, 0, 0, time.UTC).In(loc)
		to := time.Date(2026, 3, 10, 23, 59, 59, 0, time.UTC).In(loc)

		got := dates(buildDeliveryRatePoints(nil, from, to))
		if len(got) != len(want) {
			t.Fatalf("%s: got %v, want %v", loc, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%s: got %v, want %v", loc, got, want)
			}
		}
	}
}

// A row bucketed outside the range (Postgres in a non-UTC zone) must not be dropped without trace.
func TestOutOfRangeRowsAreNotLost(t *testing.T) {
	from := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 3, 23, 59, 59, 0, time.UTC)

	rows := []deliveryRow{
		{Date: "2026-02-28", Status: "sent", Count: 7},   // before the range
		{Date: "2026-03-02", Status: "sent", Count: 100}, // inside
		{Date: "2026-03-04", Status: "failed", Count: 3}, // after the range
	}

	points := buildDeliveryRatePoints(rows, from, to)

	var sent, failed int64
	for _, p := range points {
		sent += p.Sent
		failed += p.Failed
	}
	if sent != 107 || failed != 3 {
		t.Fatalf("accounted for %d sent and %d failed; want 107 and 3", sent, failed)
	}
	if points[0].Sent != 7 {
		t.Errorf("an earlier row should fold onto the first day, got %d", points[0].Sent)
	}
	if points[len(points)-1].Failed != 3 {
		t.Errorf("a later row should fold onto the last day, got %d", points[len(points)-1].Failed)
	}
}

// Quiet days must appear as zeroes; a chart that omits them draws a misleading trend.
func TestSeriesIsGapless(t *testing.T) {
	from := time.Date(2026, 1, 30, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 2, 2, 23, 59, 59, 0, time.UTC)

	points := buildDeliveryRatePoints([]deliveryRow{{Date: "2026-02-01", Status: "sent", Count: 5}}, from, to)

	want := []string{"2026-01-30", "2026-01-31", "2026-02-01", "2026-02-02"}
	got := dates(points)
	for i := range want {
		if i >= len(got) || got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	if points[2].DeliveryRate != 100 {
		t.Errorf("expected 100%% on the day with only sends, got %v", points[2].DeliveryRate)
	}
	if points[0].DeliveryRate != 0 || points[0].Total != 0 {
		t.Errorf("a quiet day should be a zero, got %+v", points[0])
	}
}

func TestInvertedRangeYieldsNoSeries(t *testing.T) {
	from := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	if got := buildDeliveryRatePoints(nil, from, to); len(got) != 0 {
		t.Fatalf("expected an empty series, got %v", dates(got))
	}
	if got := buildBounceRatePoints(nil, from, to); len(got) != 0 {
		t.Fatalf("expected an empty bounce series, got %d points", len(got))
	}
}

func TestBounceSeriesTotalsAndFolding(t *testing.T) {
	from := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 2, 23, 59, 59, 0, time.UTC)

	rows := []bounceRow{
		{Date: "2026-03-01", Type: "hard", Count: 2},
		{Date: "2026-03-01", Type: "soft", Count: 3},
		{Date: "2026-03-01", Type: "complaint", Count: 1},
		{Date: "2026-03-09", Type: "hard", Count: 4}, // out of range
	}

	points := buildBounceRatePoints(rows, from, to)
	if points[0].Total != 6 {
		t.Errorf("expected 6 bounces on the first day, got %d", points[0].Total)
	}
	if points[len(points)-1].Hard != 4 {
		t.Errorf("a later row should fold onto the last day, got %d", points[len(points)-1].Hard)
	}
}

// Suppressed sends never reached the provider, so they count neither for nor against its delivery rate.
func TestProviderRateExcludesSuppressed(t *testing.T) {
	out := buildProviderBreakdown([]providerRow{
		{Provider: "Gmail", Status: "sent", Count: 90},
		{Provider: "Gmail", Status: "failed", Count: 10},
		{Provider: "Gmail", Status: "suppressed", Count: 50},
	})

	if len(out) != 1 {
		t.Fatalf("expected one provider, got %d", len(out))
	}
	g := out[0]
	if g.Suppressed != 50 {
		t.Errorf("expected 50 suppressed, got %d", g.Suppressed)
	}
	if g.Total != 150 {
		t.Errorf("total should count everything addressed to the provider, got %d", g.Total)
	}
	if g.DeliveryRate != 90 {
		t.Errorf("delivery rate should be 90%% of what was actually attempted, got %v", g.DeliveryRate)
	}
}

// Map iteration is random, so equal-volume providers must break ties by name or the table reshuffles.
func TestProviderOrderIsStable(t *testing.T) {
	rows := []providerRow{
		{Provider: "Yahoo", Status: "sent", Count: 10},
		{Provider: "Outlook", Status: "sent", Count: 10},
		{Provider: "Gmail", Status: "sent", Count: 99},
	}
	want := []string{"Gmail", "Outlook", "Yahoo"}

	for i := 0; i < 20; i++ {
		out := buildProviderBreakdown(rows)
		for j := range want {
			if out[j].Provider != want[j] {
				t.Fatalf("run %d: got %s at %d, want %s", i, out[j].Provider, j, want[j])
			}
		}
	}
}
