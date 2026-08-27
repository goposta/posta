// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package messagescan

import (
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode"
)

type signal func(s *Scanner, in Input) (float64, string)

var builtinSignals = []signal{
	signalSuppressed,
	signalInvalidSender,
	signalDisposableDomain,
	signalLinkCount,
	signalShortener,
	signalBodyLength,
	signalShouting,
	signalMarkup,
	signalRepeatIP,
	signalRepeatSender,
	signalBotUserAgent,
}

func signalSuppressed(s *Scanner, in Input) (float64, string) {
	if s.lookup == nil || in.SenderEmail == "" {
		return 0, ""
	}
	if s.lookup.IsSuppressed(in.WorkspaceID, in.SenderEmail) {
		return 5, "sender_suppressed"
	}
	return 0, ""
}

func signalInvalidSender(s *Scanner, in Input) (float64, string) {
	if in.SenderEmail == "" {
		return 0, ""
	}
	if _, err := mail.ParseAddress(in.SenderEmail); err != nil {
		return 3, "sender_unparseable"
	}
	return 0, ""
}

func signalDisposableDomain(s *Scanner, in Input) (float64, string) {
	domain := emailDomain(in.SenderEmail)
	if domain == "" {
		return 0, ""
	}
	if disposableDomains[domain] {
		return 4, "disposable_email"
	}
	return 0, ""
}

func signalLinkCount(s *Scanner, in Input) (float64, string) {
	n := countLinks(in.Body) + countLinks(in.Subject)
	if n <= 5 {
		return 0, ""
	}
	score := float64(n - 5)
	if score > 6 {
		score = 6
	}
	return score, fmt.Sprintf("links:%d", n)
}

func signalShortener(s *Scanner, in Input) (float64, string) {
	lower := strings.ToLower(in.Body + " " + in.Subject)
	for _, d := range shortenerDomains {
		if strings.Contains(lower, d) {
			return 2, "url_shortener"
		}
	}
	return 0, ""
}

func signalBodyLength(s *Scanner, in Input) (float64, string) {
	n := len([]rune(strings.TrimSpace(in.Body)))
	switch {
	case n == 0:
		return 0, ""
	case n < 10:
		return 2, "body_too_short"
	case n > 20000:
		return 2, "body_too_long"
	}
	return 0, ""
}

func signalShouting(s *Scanner, in Input) (float64, string) {
	text := in.Body
	if len([]rune(text)) < 40 {
		return 0, ""
	}
	var letters, upper int
	for _, r := range text {
		if unicode.IsLetter(r) {
			letters++
			if unicode.IsUpper(r) {
				upper++
			}
		}
	}
	if letters == 0 {
		return 0, ""
	}
	if float64(upper)/float64(letters) >= 0.7 {
		return 2, "excessive_caps"
	}
	return 0, ""
}

func signalMarkup(s *Scanner, in Input) (float64, string) {
	lower := strings.ToLower(in.Body)
	if strings.Contains(lower, "<a href") || strings.Contains(lower, "[url=") || strings.Contains(lower, "[link=") {
		return 3, "embedded_markup"
	}
	if strings.Contains(lower, "<script") || strings.Contains(lower, "javascript:") {
		return 5, "embedded_script"
	}
	return 0, ""
}

func signalRepeatIP(s *Scanner, in Input) (float64, string) {
	if s.lookup == nil || in.ClientIP == "" {
		return 0, ""
	}
	n := s.lookup.SubmissionsFromIP(in.ClientIP, time.Now().Add(-time.Hour))
	if n >= 5 {
		return 3, fmt.Sprintf("repeat_ip:%d", n)
	}
	return 0, ""
}

func signalRepeatSender(s *Scanner, in Input) (float64, string) {
	if s.lookup == nil || in.SenderEmail == "" || in.Form == nil {
		return 0, ""
	}
	n := s.lookup.SubmissionsFromSender(in.Form.ID, in.SenderEmail, time.Now().Add(-time.Hour))
	if n >= 3 {
		return 2, fmt.Sprintf("repeat_sender:%d", n)
	}
	return 0, ""
}

func signalBotUserAgent(s *Scanner, in Input) (float64, string) {
	ua := strings.ToLower(in.UserAgent)
	if ua == "" {
		return 0, ""
	}
	for _, needle := range botAgents {
		if strings.Contains(ua, needle) {
			return 2, "bot_user_agent"
		}
	}
	return 0, ""
}

func countLinks(text string) int {
	lower := strings.ToLower(text)
	return strings.Count(lower, "http://") + strings.Count(lower, "https://") + strings.Count(lower, "www.")
}

func emailDomain(email string) string {
	at := strings.LastIndex(email, "@")
	if at < 0 || at == len(email)-1 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(email[at+1:]))
}

func containsText(haystack, needle string, caseSensitive bool) bool {
	if needle == "" {
		return false
	}
	if caseSensitive {
		return strings.Contains(haystack, needle)
	}
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}

func containsWord(haystack, needle string, caseSensitive bool) bool {
	if needle == "" {
		return false
	}
	if !caseSensitive {
		haystack = strings.ToLower(haystack)
		needle = strings.ToLower(needle)
	}
	idx := 0
	for {
		found := strings.Index(haystack[idx:], needle)
		if found < 0 {
			return false
		}
		start := idx + found
		end := start + len(needle)
		beforeOK := start == 0 || !isWordRune(rune(haystack[start-1]))
		afterOK := end == len(haystack) || !isWordRune(rune(haystack[end]))
		if beforeOK && afterOK {
			return true
		}
		idx = start + 1
		if idx >= len(haystack) {
			return false
		}
	}
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

var shortenerDomains = []string{
	"bit.ly", "t.co/", "goo.gl", "tinyurl.com", "ow.ly", "is.gd",
	"buff.ly", "adf.ly", "cutt.ly", "rebrand.ly", "shorturl.at",
}

var botAgents = []string{
	"python-requests", "curl/", "wget/", "go-http-client", "libwww-perl",
	"scrapy", "httpclient", "okhttp", "phantomjs", "headlesschrome",
}

var disposableDomains = map[string]bool{
	"mailinator.com": true, "guerrillamail.com": true, "10minutemail.com": true,
	"tempmail.com": true, "temp-mail.org": true, "throwawaymail.com": true,
	"yopmail.com": true, "trashmail.com": true, "sharklasers.com": true,
	"getnada.com": true, "dispostable.com": true, "maildrop.cc": true,
	"fakeinbox.com": true, "mailnesia.com": true, "tempinbox.com": true,
	"spamgourmet.com": true, "mytemp.email": true, "moakt.com": true,
	"emailondeck.com": true, "mohmal.com": true, "tempr.email": true,
	"discard.email": true, "grr.la": true, "spam4.me": true,
}
