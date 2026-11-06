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

package email

import (
	"sort"
	"strings"
)

const (
	VarWebView      = "posta_web_view_url"
	VarWebViewAlias = "posta_mail_web_link"
	VarUnsubscribe  = "posta_unsubscribe_url"
)

// Sentinels are root-relative URLs so html/template, premailer and the http(s)-only click rewriter pass them through.
const (
	sentinelWebView     = "/__posta_web_view__"
	sentinelUnsubscribe = "/__posta_unsubscribe__"
)

// Unknown {{ posta_* }} vars are not injected, so they render empty or error under missingkey: reserved by design.
var systemVarSentinels = map[string]any{
	VarWebView:      sentinelWebView,
	VarWebViewAlias: sentinelWebView, // alias for posta_web_view_url
	VarUnsubscribe:  sentinelUnsubscribe,
}

func SystemVarNames() []string {
	names := make([]string, 0, len(systemVarSentinels))
	for k := range systemVarSentinels {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// WithSystemVarNames sets each reserved variable to its own name, for previews where no real message exists.
func WithSystemVarNames(data map[string]any) map[string]any {
	out := make(map[string]any, len(data)+len(systemVarSentinels))
	for k, v := range data {
		out[k] = v
	}
	for k := range systemVarSentinels {
		out[k] = k
	}
	return out
}

// System values are written last so user-supplied data cannot shadow them.
func WithSystemVars(data map[string]any) map[string]any {
	out := make(map[string]any, len(data)+len(systemVarSentinels))
	for k, v := range data {
		out[k] = v
	}
	for k, v := range systemVarSentinels {
		out[k] = v
	}
	return out
}

func HasSystemSentinels(s string) bool {
	return strings.Contains(s, sentinelWebView) || strings.Contains(s, sentinelUnsubscribe)
}

// An empty URL removes the sentinel rather than leaving a broken root-relative link.
func SubstituteSystemLinks(s, webViewURL, unsubscribeURL string) string {
	if s == "" {
		return s
	}
	s = strings.ReplaceAll(s, sentinelWebView, webViewURL)
	s = strings.ReplaceAll(s, sentinelUnsubscribe, unsubscribeURL)
	return s
}
