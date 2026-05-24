/*
 * Copyright 2026 Jonas Kaninda
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package email

import (
	"sort"
	"strings"
)

// Canonical reserved variable names.
const (
	VarWebView      = "posta_web_view_url"
	VarWebViewAlias = "posta_mail_web_link"
	VarUnsubscribe  = "posta_unsubscribe_url"
)

// Posta reserves the {{ posta_* }} template-variable namespace for system values
// it injects itself (the hosted "view in browser" link, the unsubscribe link, …).
//
// These values are per-message, but a template is often rendered once and reused
// across many recipients (e.g. campaigns render once per language). So rendering
// does not substitute the real URL — it substitutes a stable *sentinel*, which is
// then replaced per message once the email identity (UUID / id) is known
// (SubstituteSystemLinks).
//
// The sentinels are written as root-relative URLs so that:
//   - html/template's URL filter passes them through unchanged (no #ZgotmplZ),
//   - premailer leaves them alone during CSS inlining,
//   - the campaign click-rewriter (which only matches http(s):// hrefs) ignores them.
const (
	sentinelWebView     = "/__posta_web_view__"
	sentinelUnsubscribe = "/__posta_unsubscribe__"
)

// Canonical template variable names and their documented aliases. All map to a
// sentinel; unknown {{ posta_* }} vars are not injected (and will render empty
// under missingkey=zero, or error under missingkey=error — by design, reserved).
var systemVarSentinels = map[string]any{
	VarWebView:      sentinelWebView,
	VarWebViewAlias: sentinelWebView, // alias for posta_web_view_url
	VarUnsubscribe:  sentinelUnsubscribe,
}

// SystemVarNames returns the reserved {{ posta_* }} variable names (canonical and
// aliases), sorted. Useful for editor/template-view surfaces that advertise the
// available variables.
func SystemVarNames() []string {
	names := make([]string, 0, len(systemVarSentinels))
	for k := range systemVarSentinels {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// WithSystemVarNames returns a copy of data with each reserved {{ posta_* }}
// variable present but set to its own name. Used by previews / template views
// where no real message exists: the template renders without missing-key errors
// and the author sees which system variable sits where, without any generated
// URL/data.
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

// WithSystemVars returns a copy of data with the reserved posta_* variables added.
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

// HasSystemSentinels reports whether s still contains any unresolved sentinel,
// so callers can skip the substitution+update when a template uses no system vars.
func HasSystemSentinels(s string) bool {
	return strings.Contains(s, sentinelWebView) || strings.Contains(s, sentinelUnsubscribe)
}

// SubstituteSystemLinks replaces the reserved sentinels with the real per-message
// URLs. An empty URL (e.g. a disabled feature) removes the sentinel rather than
// leaving a broken root-relative link in the message.
func SubstituteSystemLinks(s, webViewURL, unsubscribeURL string) string {
	if s == "" {
		return s
	}
	s = strings.ReplaceAll(s, sentinelWebView, webViewURL)
	s = strings.ReplaceAll(s, sentinelUnsubscribe, unsubscribeURL)
	return s
}
