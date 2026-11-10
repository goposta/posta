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

package chat

import (
	"testing"
	"time"
)

func TestReplyBucket(t *testing.T) {
	for _, tc := range []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, ReplyMinutes},
		{59 * time.Minute, ReplyMinutes},
		{time.Hour, ReplyHours},
		{23 * time.Hour, ReplyHours},
		{24 * time.Hour, ReplyDay},
		{72 * time.Hour, ReplyDay},
	} {
		if got := ReplyBucket(tc.d.Seconds()); got != tc.want {
			t.Errorf("%v: got %s, want %s", tc.d, got, tc.want)
		}
	}
}

func TestFirstName(t *testing.T) {
	for in, want := range map[string]string{"Jane Doe": "Jane", "  Bob ": "Bob", "": "", "Ana\u0000Maria": "AnaMaria"} {
		if got := FirstName(in); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}
