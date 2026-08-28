// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package handlers

import (
	"reflect"
	"testing"
)

func assertEnumTagsAreStrings(t *testing.T, v any) {
	t.Helper()
	walkEnumTags(t, reflect.TypeOf(v), "")
}

func walkEnumTags(t *testing.T, typ reflect.Type, path string) {
	t.Helper()
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return
	}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		name := field.Name
		if path != "" {
			name = path + "." + name
		}

		if enum, ok := field.Tag.Lookup("enum"); ok {
			kind := field.Type.Kind()
			if kind == reflect.Slice {
				kind = field.Type.Elem().Kind()
			}
			if kind != reflect.String {
				t.Errorf("%s has enum:%q but kind %s; okapi only validates enum on string fields, "+
					"so every request to this endpoint fails to bind", name, enum, field.Type)
			}
		}

		if field.Type.Kind() == reflect.Struct {
			walkEnumTags(t, field.Type, name)
		}
	}
}

func TestRequestEnumTagsOnlyOnStrings(t *testing.T) {
	requests := []any{
		CreateFormRequest{},
		UpdateFormRequest{},
		FormIDRequest{},
		MessageListRequest{},
		UpdateMessageStateRequest{},
		AssignMessageRequest{},
		MarkSpamRequest{},
		ReplyMessageRequest{},
		MessageAnalyticsRequest{},
		CreateMessageFilterRequest{},
		UpdateMessageFilterRequest{},
		TestMessageFilterRequest{},
	}
	for _, req := range requests {
		assertEnumTagsAreStrings(t, req)
	}
}

func TestParseNotifyMode(t *testing.T) {
	valid := map[string]string{
		"immediate": "immediate",
		"HOURLY":    "hourly",
		" daily ":   "daily",
		"off":       "off",
	}
	for in, want := range valid {
		got, ok := parseNotifyMode(in)
		if !ok || string(got) != want {
			t.Fatalf("parseNotifyMode(%q) = %q,%v; want %q,true", in, got, ok, want)
		}
	}

	for _, in := range []string{"", "weekly", "nonsense"} {
		if _, ok := parseNotifyMode(in); ok {
			t.Fatalf("parseNotifyMode(%q) reported valid", in)
		}
	}
}
