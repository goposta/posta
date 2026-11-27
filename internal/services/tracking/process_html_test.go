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

package tracking

import (
	"strings"
	"testing"
)

const sampleHTML = `<html><body><a href="https://example.com/offer">Offer</a></body></html>`

func TestProcessHTML_TrackingOffLeavesHTMLUntouched(t *testing.T) {
	if got := newTestService().ProcessHTML(sampleHTML, 1, 42, false, false); got != sampleHTML {
		t.Fatalf("ProcessHTML changed the body with tracking off:\n%s", got)
	}
}

// The service has no link repository here, so reaching the click rewriter would panic.
func TestProcessHTML_OpensOnlyKeepsOriginalLinks(t *testing.T) {
	s := newTestService()
	got := s.ProcessHTML(sampleHTML, 1, 42, true, false)
	if !strings.Contains(got, `href="https://example.com/offer"`) {
		t.Errorf("link was rewritten with click tracking off:\n%s", got)
	}
	if !strings.Contains(got, s.OpenURL(42)) {
		t.Errorf("open pixel missing with open tracking on:\n%s", got)
	}
	if !strings.Contains(got, "</a><img") || !strings.HasSuffix(got, "</body></html>") {
		t.Errorf("pixel not placed before </body>:\n%s", got)
	}
}
