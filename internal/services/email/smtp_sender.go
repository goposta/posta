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
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/netguard"
)

// SendError separates a permanent RCPT TO rejection from a transient error; Error() keeps the original string.
type SendError struct {
	Stage     string // "RCPT TO", "MAIL FROM", "DATA", ...
	Recipient string // bare recipient address, set for RCPT TO failures
	Code      int    // SMTP reply code (e.g. 550); 0 when not an SMTP reply
	Msg       string // server reply text, incl. enhanced status code (5.1.1 ...)
	Err       error
}

func (e *SendError) Permanent() bool { return e.Code >= 500 && e.Code < 600 }

func (e *SendError) Error() string { return e.Err.Error() }
func (e *SendError) Unwrap() error { return e.Err }

// smtpReply returns (0, "") when err does not wrap a *textproto.Error, e.g. a connection failure.
func smtpReply(err error) (int, string) {
	var t *textproto.Error
	if errors.As(err, &t) {
		return t.Code, t.Msg
	}
	return 0, ""
}

type SMTPSender struct{}

// Private addresses are allowed by default since a relay on the same host or network is common.
var workspaceNetwork = netguard.Policy{AllowPrivate: true}

// SetWorkspaceSMTPNetwork must be called once at startup, before any mail is sent.
func SetWorkspaceSMTPNetwork(p netguard.Policy) {
	workspaceNetwork = p
}

// Only workspace-configured servers get the guarded dialer; system and platform servers are admin-configured.
func dialerFor(server *models.SMTPServer) *net.Dialer {
	if server.WorkspaceID != nil && !server.IsSystem {
		return workspaceNetwork.Dialer(0)
	}
	return &net.Dialer{}
}

func dialSMTP(d *net.Dialer, addr, host string) (*smtp.Client, error) {
	conn, err := d.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return client, nil
}

// DescribeSMTPError keeps SMTP replies but strips addresses and ports so a network error cannot map the network.
func DescribeSMTPError(err error) string {
	if err == nil {
		return ""
	}
	stage := err.Error()
	if i := strings.Index(stage, ":"); i > 0 {
		stage = stage[:i]
	} else {
		stage = "SMTP connection failed"
	}
	var reply *textproto.Error
	if errors.As(err, &reply) {
		return fmt.Sprintf("%s: %d %s", stage, reply.Code, reply.Msg)
	}
	return stage + ": " + netguard.Describe(err)
}

func NewSMTPSender() *SMTPSender {
	return &SMTPSender{}
}

func (s *SMTPSender) TestConnection(server *models.SMTPServer) error {
	addr := fmt.Sprintf("%s:%d", server.Host, server.Port)
	tlsConfig := &tls.Config{ServerName: server.Host}
	dialer := dialerFor(server)

	var client *smtp.Client
	var err error

	switch server.Encryption {
	case models.EncryptionSSL:
		conn, dialErr := tls.DialWithDialer(dialer, "tcp", addr, tlsConfig)
		if dialErr != nil {
			return fmt.Errorf("SSL dial failed: %w", dialErr)
		}
		defer func() { _ = conn.Close() }()
		client, err = smtp.NewClient(conn, server.Host)
		if err != nil {
			return fmt.Errorf("SMTP client creation failed: %w", err)
		}

	case models.EncryptionSTARTTLS:
		client, err = dialSMTP(dialer, addr, server.Host)
		if err != nil {
			return fmt.Errorf("SMTP dial failed: %w", err)
		}
		if err = client.StartTLS(tlsConfig); err != nil {
			_ = client.Close()
			return fmt.Errorf("STARTTLS failed: %w", err)
		}

	default: // none
		client, err = dialSMTP(dialer, addr, server.Host)
		if err != nil {
			return fmt.Errorf("SMTP dial failed: %w", err)
		}
	}
	defer func() { _ = client.Close() }()

	if server.Username != "" {
		if err := client.Auth(newAuth(server.Host, server.Username, server.Password)); err != nil {
			return fmt.Errorf("SMTP auth failed: %w", err)
		}
	}
	return client.Quit()
}

// autoAuth uses PLAIN when the server offers it, otherwise LOGIN (Exchange Online).
type autoAuth struct {
	plain            smtp.Auth
	host, user, pass string
	login            bool
}

func newAuth(host, username, password string) smtp.Auth {
	return &autoAuth{plain: smtp.PlainAuth("", username, password, host), host: host, user: username, pass: password}
}

func (a *autoAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if !offers(server.Auth, "LOGIN") || offers(server.Auth, "PLAIN") {
		return a.plain.Start(server)
	}
	if !server.TLS && server.Name != "localhost" && server.Name != "127.0.0.1" && server.Name != "::1" {
		return "", nil, errors.New("unencrypted connection")
	}
	if server.Name != a.host {
		return "", nil, errors.New("wrong host name")
	}
	a.login = true
	return "LOGIN", nil, nil
}

func (a *autoAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !a.login {
		return a.plain.Next(fromServer, more)
	}
	if !more {
		return nil, nil
	}
	switch strings.ToLower(strings.TrimSpace(string(fromServer))) {
	case "username:":
		return []byte(a.user), nil
	case "password:":
		return []byte(a.pass), nil
	}
	return nil, fmt.Errorf("unexpected LOGIN challenge %q", fromServer)
}

func offers(mechanisms []string, name string) bool {
	return slices.ContainsFunc(mechanisms, func(m string) bool { return strings.EqualFold(m, name) })
}

func (s *SMTPSender) Send(server *models.SMTPServer, from string, to []string, subject, htmlBody, textBody string, attachments []models.Attachment, headers map[string]string, listUnsubscribeURL, listUnsubscribeMailto string, listUnsubscribePost bool) error {
	addr := fmt.Sprintf("%s:%d", server.Host, server.Port)

	var auth smtp.Auth
	if server.Username != "" {
		auth = newAuth(server.Host, server.Username, server.Password)
	}

	msg := buildMessage(from, to, subject, htmlBody, textBody, attachments, headers, listUnsubscribeURL, listUnsubscribeMailto, listUnsubscribePost)

	// Extract bare email for SMTP envelope (MAIL FROM), keep full format for headers
	envelopeFrom := envelopeAddress(from)

	switch server.Encryption {
	case models.EncryptionSSL:
		return sendWithImplicitTLS(dialerFor(server), addr, auth, server.Host, envelopeFrom, to, msg)
	case models.EncryptionSTARTTLS:
		return sendWithSTARTTLS(dialerFor(server), addr, auth, server.Host, envelopeFrom, to, msg)
	default:
		return sendPlain(dialerFor(server), addr, auth, server.Host, envelopeFrom, to, msg)
	}
}

func envelopeAddress(from string) string {
	addr, err := mail.ParseAddress(from)
	if err != nil {
		return from
	}
	return addr.Address
}

func sendWithImplicitTLS(dialer *net.Dialer, addr string, auth smtp.Auth, host string, from string, to []string, msg []byte) error {
	tlsConfig := &tls.Config{ServerName: host}

	conn, err := tls.DialWithDialer(dialer, "tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("SSL dial failed: %w", err)
	}

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("SMTP client creation failed: %w", err)
	}
	defer func() { _ = client.Close() }()

	return sendViaClient(client, auth, from, to, msg)
}

func sendWithSTARTTLS(dialer *net.Dialer, addr string, auth smtp.Auth, host string, from string, to []string, msg []byte) error {
	client, err := dialSMTP(dialer, addr, host)
	if err != nil {
		return fmt.Errorf("SMTP dial failed: %w", err)
	}
	defer func() { _ = client.Close() }()

	tlsConfig := &tls.Config{ServerName: host}
	if err := client.StartTLS(tlsConfig); err != nil {
		return fmt.Errorf("STARTTLS failed: %w", err)
	}

	return sendViaClient(client, auth, from, to, msg)
}

func sendPlain(dialer *net.Dialer, addr string, auth smtp.Auth, host string, from string, to []string, msg []byte) error {
	client, err := dialSMTP(dialer, addr, host)
	if err != nil {
		return fmt.Errorf("SMTP dial failed, host: %s, error: %w", host, err)
	}
	defer func() { _ = client.Close() }()

	return sendViaClient(client, auth, from, to, msg)
}

func sendViaClient(client *smtp.Client, auth smtp.Auth, from string, to []string, msg []byte) error {
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth failed: %w", err)
		}
	}

	if err := client.Mail(from); err != nil {
		return wrapSendError("MAIL FROM", from, fmt.Errorf("SMTP MAIL FROM failed: %w", err))
	}
	for _, addr := range to {
		rcpt := envelopeAddress(addr)
		if err := client.Rcpt(rcpt); err != nil {
			return wrapSendError("RCPT TO", rcpt, fmt.Errorf("SMTP RCPT TO failed: %w", err))
		}
	}

	w, err := client.Data()
	if err != nil {
		return wrapSendError("DATA", "", fmt.Errorf("SMTP DATA failed: %w", err))
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("SMTP write failed: %w", err)
	}
	if err := w.Close(); err != nil {
		return wrapSendError("DATA", "", fmt.Errorf("SMTP close failed: %w", err))
	}

	return client.Quit()
}

func wrapSendError(stage, recipient string, err error) error {
	code, msg := smtpReply(err)
	return &SendError{Stage: stage, Recipient: recipient, Code: code, Msg: msg, Err: err}
}

const (
	contentTypeTextPlain = "text/plain"
	contentTypeTextHTML  = "text/html"
)

// Non-ASCII bodies need quoted-printable: without a Content-Transfer-Encoding the part defaults to 7bit.
func writeTextPart(b *strings.Builder, mediaType, body string) {
	fmt.Fprintf(b, "Content-Type: %s; charset=\"UTF-8\"\r\n", mediaType)
	if isASCII(body) {
		b.WriteString("\r\n")
		b.WriteString(body)
		return
	}
	b.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	w := quotedprintable.NewWriter(b)
	_, _ = w.Write([]byte(body))
	_ = w.Close()
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			return false
		}
	}
	return true
}

// Plain text containing "=?" is encoded too, or a reader would decode what merely looks like an encoded-word.
func encodeHeader(s string) string {
	if mime.BEncoding.Encode("UTF-8", s) == s && !strings.Contains(s, "=?") {
		return foldPlain(s)
	}
	return strings.Join(encodeWords(s), "\r\n ")
}

// Encoded-words break only at spaces because some relays keep the whitespace between them.
func encodeWords(s string) []string {
	// 39 bytes encode to 52 base64 chars, keeping even the first line after "Subject: " within RFC 2047's 76.
	const limit = 39
	chunks := []string{""}
	for i, w := range strings.Split(s, " ") {
		if i > 0 {
			w = " " + w
		}
		if n := len(chunks) - 1; chunks[n] != "" && len(chunks[n])+len(w) > limit {
			chunks = append(chunks, "")
		}
		for _, r := range w {
			if n := len(chunks) - 1; len(chunks[n])+utf8.RuneLen(r) > limit {
				chunks = append(chunks, "")
			}
			chunks[len(chunks)-1] += string(r)
		}
	}
	for i, c := range chunks {
		chunks[i] = "=?UTF-8?b?" + base64.StdEncoding.EncodeToString([]byte(c)) + "?="
	}
	return chunks
}

func encodeFrom(from string) string {
	addr, err := mail.ParseAddress(from)
	if err != nil || isASCII(addr.Name) {
		return from
	}
	return strings.Join(encodeWords(addr.Name), "\r\n ") + " <" + addr.Address + ">"
}

// foldPlain keeps each line after "Subject: " within the 78 characters RFC 5322 recommends.
func foldPlain(s string) string {
	var b strings.Builder
	n := len("Subject: ")
	for i, w := range strings.Split(s, " ") {
		if i > 0 {
			if w != "" && n+1+len(w) > 78 {
				b.WriteString("\r\n")
				n = 0
			}
			b.WriteByte(' ')
			n++
		}
		b.WriteString(w)
		n += len(w)
	}
	return b.String()
}

func hasHeader(headers map[string]string, key string) bool {
	for k := range headers {
		if strings.EqualFold(k, key) {
			return true
		}
	}
	return false
}

// A per-message boundary avoids collisions with nested content and fingerprinting, and fits a 76-char line.
func newBoundary() string {
	var token [12]byte
	if _, err := rand.Read(token[:]); err != nil {
		// crypto/rand failing is extraordinary; a UUID keeps the boundary unique.
		return "posta-" + uuid.NewString()
	}
	return "posta-" + hex.EncodeToString(token[:])
}

func buildMessage(from string, to []string, subject, htmlBody, textBody string, attachments []models.Attachment, headers map[string]string, listUnsubscribeURL, listUnsubscribeMailto string, listUnsubscribePost bool) []byte {
	var b strings.Builder

	fmt.Fprintf(&b, "From: %s\r\n", encodeFrom(from))
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(to, ", "))
	fmt.Fprintf(&b, "Subject: %s\r\n", encodeHeader(subject))

	// Relays may forward a message without Date or Message-ID, so emit both unless the caller supplied them.
	if !hasHeader(headers, "Date") {
		fmt.Fprintf(&b, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	}
	if !hasHeader(headers, "Message-ID") {
		domain := "posta.local"
		if addr, err := mail.ParseAddress(from); err == nil {
			if i := strings.LastIndex(addr.Address, "@"); i >= 0 && i+1 < len(addr.Address) {
				domain = addr.Address[i+1:]
			}
		}
		fmt.Fprintf(&b, "Message-ID: <%s@%s>\r\n", uuid.NewString(), domain)
	}

	b.WriteString("MIME-Version: 1.0\r\n")

	// RFC 2369 / 8058 List-Unsubscribe. Emit mailto first, then the https URL.
	var luParts []string
	if listUnsubscribeMailto != "" {
		luParts = append(luParts, "<"+listUnsubscribeMailto+">")
	}
	if listUnsubscribeURL != "" {
		luParts = append(luParts, "<"+listUnsubscribeURL+">")
	}
	if len(luParts) > 0 {
		fmt.Fprintf(&b, "List-Unsubscribe: %s\r\n", strings.Join(luParts, ", "))
		// RFC 8058 one-click applies to the https POST target only.
		if listUnsubscribePost && listUnsubscribeURL != "" {
			b.WriteString("List-Unsubscribe-Post: List-Unsubscribe=One-Click\r\n")
		}
	}

	for key, value := range headers {
		fmt.Fprintf(&b, "%s: %s\r\n", key, value)
	}

	hasAttachments := len(attachments) > 0

	if hasAttachments {
		mixedBoundary := newBoundary()
		altBoundary := newBoundary()

		fmt.Fprintf(&b, "Content-Type: multipart/mixed; boundary=\"%s\"\r\n\r\n", mixedBoundary)

		fmt.Fprintf(&b, "--%s\r\n", mixedBoundary)

		if htmlBody != "" && textBody != "" {
			fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n", altBoundary)
			fmt.Fprintf(&b, "--%s\r\n", altBoundary)
			writeTextPart(&b, contentTypeTextPlain, textBody)
			fmt.Fprintf(&b, "\r\n--%s\r\n", altBoundary)
			writeTextPart(&b, contentTypeTextHTML, htmlBody)
			fmt.Fprintf(&b, "\r\n--%s--\r\n", altBoundary)
		} else if htmlBody != "" {
			writeTextPart(&b, contentTypeTextHTML, htmlBody)
		} else {
			writeTextPart(&b, contentTypeTextPlain, textBody)
		}

		for _, att := range attachments {
			fmt.Fprintf(&b, "\r\n--%s\r\n", mixedBoundary)
			contentType := att.ContentType
			if contentType == "" {
				contentType = "application/octet-stream"
			}
			fmt.Fprintf(&b, "Content-Type: %s\r\n", mime.FormatMediaType(contentType, map[string]string{"name": att.Filename}))
			b.WriteString("Content-Transfer-Encoding: base64\r\n")
			fmt.Fprintf(&b, "Content-Disposition: %s\r\n\r\n", mime.FormatMediaType("attachment", map[string]string{"filename": att.Filename}))

			// Content is already base64-encoded from the API request
			content := att.Content
			for len(content) > 76 {
				b.WriteString(content[:76])
				b.WriteString("\r\n")
				content = content[76:]
			}
			if len(content) > 0 {
				b.WriteString(content)
			}
		}
		fmt.Fprintf(&b, "\r\n--%s--\r\n", mixedBoundary)
	} else {
		altBoundary := newBoundary()
		if htmlBody != "" && textBody != "" {
			fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n", altBoundary)
			fmt.Fprintf(&b, "--%s\r\n", altBoundary)
			writeTextPart(&b, contentTypeTextPlain, textBody)
			fmt.Fprintf(&b, "\r\n--%s\r\n", altBoundary)
			writeTextPart(&b, contentTypeTextHTML, htmlBody)
			fmt.Fprintf(&b, "\r\n--%s--\r\n", altBoundary)
		} else if htmlBody != "" {
			writeTextPart(&b, contentTypeTextHTML, htmlBody)
		} else {
			writeTextPart(&b, contentTypeTextPlain, textBody)
		}
	}

	return []byte(b.String())
}

func ValidateAttachments(attachments []models.Attachment, maxAttachmentSize, maxTotalSize int64) error {
	var totalSize int64
	for _, att := range attachments {
		if att.Filename == "" {
			return fmt.Errorf("attachment filename is required")
		}
		decoded, err := base64.StdEncoding.DecodeString(att.Content)
		if err != nil {
			return fmt.Errorf("attachment %q has invalid base64 content", att.Filename)
		}
		size := int64(len(decoded))
		if size > maxAttachmentSize {
			return fmt.Errorf("attachment %q exceeds maximum size of %d bytes", att.Filename, maxAttachmentSize)
		}
		totalSize += size
	}
	if totalSize > maxTotalSize {
		return fmt.Errorf("total attachment size exceeds maximum of %d bytes", maxTotalSize)
	}
	return nil
}
