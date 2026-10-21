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
	"reflect"
	"strings"
	"testing"

	"github.com/goposta/posta/internal/dto"
)

// TestWebhookEnumMatchesRegistry pins the subscribable events (the enum okapi
// validates against) to the documented registry. Adding an event to one list
// but not the other is how message.replied went missing.
func TestWebhookEnumMatchesRegistry(t *testing.T) {
	body, ok := reflect.TypeOf(CreateWebhookRequest{}).FieldByName("Body")
	if !ok {
		t.Fatal("CreateWebhookRequest has no Body")
	}
	events, ok := body.Type.FieldByName("Events")
	if !ok {
		t.Fatal("CreateWebhookRequest.Body has no Events")
	}
	enum := strings.Split(events.Tag.Get("enum"), ",")
	if want := dto.WebhookEventNames(); !reflect.DeepEqual(enum, want) {
		t.Fatalf("enum tag = %v\nregistry = %v", enum, want)
	}
}

func TestWebhookRegistryNamesAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, e := range dto.WebhookEvents {
		if seen[e.Name] {
			t.Fatalf("duplicate webhook event %q", e.Name)
		}
		seen[e.Name] = true
		if e.Description == "" || e.Payload == nil {
			t.Fatalf("event %q needs a description and a payload type", e.Name)
		}
	}
}
