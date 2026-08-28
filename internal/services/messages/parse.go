// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package messages

import (
	"net/mail"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/goposta/posta/internal/models"
)

const (
	maxFieldValueBytes  = 8192
	maxHeaderFieldRunes = 200
)

var reservedFields = map[string]bool{
	"_gotcha":   true,
	"_nonce":    true,
	"_redirect": true,
	"_captcha":  true,
	"_cc":       true,
	"_next":     true,
	"_honeypot": true,
}

var (
	emailKeys   = []string{"email", "e-mail", "_replyto", "reply_to", "replyto", "from", "sender_email", "your-email"}
	nameKeys    = []string{"name", "full_name", "fullname", "your-name", "sender_name"}
	subjectKeys = []string{"_subject", "subject", "topic", "your-subject"}
	bodyKeys    = []string{"message", "body", "comments", "comment", "content", "description", "your-message"}
	phoneKeys   = []string{"phone", "phone_number", "phonenumber", "tel", "telephone", "mobile", "cell", "your-phone", "sender_phone"}
)

const (
	maxPhoneRunes   = 40
	phoneEdgeCutset = " -.,/"
)

func IsReserved(key string) bool {
	return reservedFields[strings.ToLower(key)]
}

func Sanitize(value string) string {
	value = strings.Map(func(r rune) rune {
		if r == '\r' || r == 0 {
			return -1
		}
		if r == '\n' || r == '\t' {
			return r
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, value)
	value = norm.NFC.String(value)
	value = strings.TrimSpace(value)
	if len(value) > maxFieldValueBytes {
		value = string([]rune(value)[:maxFieldValueBytes/4])
	}
	return value
}

func SanitizeHeaderValue(value string) string {
	value = strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' || r == 0 || unicode.IsControl(r) {
			return -1
		}
		return r
	}, value)
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) > maxHeaderFieldRunes {
		value = string(runes[:maxHeaderFieldRunes])
	}
	return value
}

type Extracted struct {
	Email   string
	Name    string
	Phone   string
	Subject string
	Body    string
}

func Extract(fields []models.MessageField) Extracted {
	var out Extracted

	out.Email = SanitizeHeaderValue(pick(fields, emailKeys))
	out.Name = SanitizeHeaderValue(pick(fields, nameKeys))
	out.Phone = NormalizePhone(pick(fields, phoneKeys))
	out.Subject = SanitizeHeaderValue(pick(fields, subjectKeys))
	out.Body = pick(fields, bodyKeys)

	if out.Email == "" {
		out.Email = firstAddress(fields)
	}
	if out.Name == "" {
		out.Name = joinNameParts(fields)
	}
	if out.Body == "" {
		out.Body = longestValue(fields)
	}
	return out
}

func pick(fields []models.MessageField, keys []string) string {
	for _, want := range keys {
		for _, f := range fields {
			if strings.EqualFold(strings.TrimSpace(f.Key), want) && strings.TrimSpace(f.Value) != "" {
				return f.Value
			}
		}
	}
	return ""
}

func firstAddress(fields []models.MessageField) string {
	for _, f := range fields {
		v := strings.TrimSpace(f.Value)
		if v == "" || !strings.Contains(v, "@") {
			continue
		}
		if addr, err := mail.ParseAddress(v); err == nil {
			return addr.Address
		}
	}
	return ""
}

func joinNameParts(fields []models.MessageField) string {
	var first, last string
	for _, f := range fields {
		switch strings.ToLower(strings.TrimSpace(f.Key)) {
		case "first_name", "firstname", "given_name":
			first = f.Value
		case "last_name", "lastname", "surname", "family_name":
			last = f.Value
		}
	}
	return SanitizeHeaderValue(strings.TrimSpace(first + " " + last))
}

func longestValue(fields []models.MessageField) string {
	var best string
	for _, f := range fields {
		if IsReserved(f.Key) {
			continue
		}
		if len(f.Value) > len(best) {
			best = f.Value
		}
	}
	return best
}

func NormalizeEmail(email string) string {
	email = strings.TrimSpace(email)
	if addr, err := mail.ParseAddress(email); err == nil {
		return strings.ToLower(addr.Address)
	}
	return strings.ToLower(email)
}

func NormalizePhone(v string) string {
	v = SanitizeHeaderValue(v)
	if v == "" {
		return ""
	}

	var b strings.Builder
	for _, r := range v {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '+' && b.Len() == 0:
			b.WriteRune(r)
		case r == ' ', r == '-', r == '(', r == ')', r == '.', r == '/':
			b.WriteRune(r)
		}
	}

	out := strings.Join(strings.Fields(b.String()), " ")
	out = strings.TrimLeft(strings.TrimRight(out, phoneEdgeCutset), phoneEdgeCutset)
	if !strings.ContainsAny(out, "0123456789") {
		return ""
	}
	if runes := []rune(out); len(runes) > maxPhoneRunes {
		out = strings.TrimSpace(string(runes[:maxPhoneRunes]))
	}
	return out
}

func PhoneAliases() []string { return phoneKeys }

func SummarizedKeys() []string {
	keys := make([]string, 0, len(emailKeys)+len(nameKeys)+len(subjectKeys)+len(phoneKeys)+4)
	keys = append(keys, emailKeys...)
	keys = append(keys, nameKeys...)
	keys = append(keys, subjectKeys...)
	keys = append(keys, phoneKeys...)
	keys = append(keys, "first_name", "firstname", "last_name", "lastname")
	return keys
}
