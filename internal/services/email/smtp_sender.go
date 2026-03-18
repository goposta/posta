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
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/mail"
	"net/smtp"
	"strings"

	"github.com/jkaninda/posta/internal/models"
)

type SMTPSender struct{}

func NewSMTPSender() *SMTPSender {
	return &SMTPSender{}
}

// TestConnection verifies that a connection to the SMTP server can be
// established and, when credentials are provided, that authentication succeeds.
func (s *SMTPSender) TestConnection(server *models.SMTPServer) error {
	addr := fmt.Sprintf("%s:%d", server.Host, server.Port)
	tlsConfig := &tls.Config{ServerName: server.Host}

	var client *smtp.Client
	var err error

	switch server.Encryption {
	case models.EncryptionSSL:
		conn, dialErr := tls.Dial("tcp", addr, tlsConfig)
		if dialErr != nil {
			return fmt.Errorf("SSL dial failed: %w", dialErr)
		}
		defer func() { _ = conn.Close() }()
		client, err = smtp.NewClient(conn, server.Host)
		if err != nil {
			return fmt.Errorf("SMTP client creation failed: %w", err)
		}

	case models.EncryptionSTARTTLS:
		client, err = smtp.Dial(addr)
		if err != nil {
			return fmt.Errorf("SMTP dial failed: %w", err)
		}
		if err = client.StartTLS(tlsConfig); err != nil {
			_ = client.Close()
			return fmt.Errorf("STARTTLS failed: %w", err)
		}

	default: // none
		client, err = smtp.Dial(addr)
		if err != nil {
			return fmt.Errorf("SMTP dial failed: %w", err)
		}
	}
	defer func() { _ = client.Close() }()

	if server.Username != "" {
		if err := client.Auth(smtp.PlainAuth("", server.Username, server.Password, server.Host)); err != nil {
			return fmt.Errorf("SMTP auth failed: %w", err)
		}
	}
	return client.Quit()
}

func (s *SMTPSender) Send(server *models.SMTPServer, from string, to []string, subject, htmlBody, textBody string, attachments []models.Attachment, headers map[string]string, listUnsubscribeURL string, listUnsubscribePost bool) error {
	addr := fmt.Sprintf("%s:%d", server.Host, server.Port)

	var auth smtp.Auth
	if server.Username != "" {
		auth = smtp.PlainAuth("", server.Username, server.Password, server.Host)
	}

	msg := buildMessage(from, to, subject, htmlBody, textBody, attachments, headers, listUnsubscribeURL, listUnsubscribePost)

	// Extract bare email for SMTP envelope (MAIL FROM), keep full format for headers
	envelopeFrom := envelopeAddress(from)

	switch server.Encryption {
	case models.EncryptionSSL:
		return sendWithImplicitTLS(addr, auth, server.Host, envelopeFrom, to, msg)
	case models.EncryptionSTARTTLS:
		return sendWithSTARTTLS(addr, auth, server.Host, envelopeFrom, to, msg)
	default:
		return smtp.SendMail(addr, auth, envelopeFrom, to, msg)
	}
}

// envelopeAddress extracts the bare email from an RFC 5322 address like
// "Display Name <user@example.com>", returning just "user@example.com".
// If parsing fails, it returns the input unchanged.
func envelopeAddress(from string) string {
	addr, err := mail.ParseAddress(from)
	if err != nil {
		return from
	}
	return addr.Address
}

func sendWithImplicitTLS(addr string, auth smtp.Auth, host string, from string, to []string, msg []byte) error {
	tlsConfig := &tls.Config{ServerName: host}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
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

func sendWithSTARTTLS(addr string, auth smtp.Auth, host string, from string, to []string, msg []byte) error {
	client, err := smtp.Dial(addr)
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

func sendViaClient(client *smtp.Client, auth smtp.Auth, from string, to []string, msg []byte) error {
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth failed: %w", err)
		}
	}

	if err := client.Mail(from); err != nil {
		return fmt.Errorf("SMTP MAIL FROM failed: %w", err)
	}
	for _, addr := range to {
		if err := client.Rcpt(envelopeAddress(addr)); err != nil {
			return fmt.Errorf("SMTP RCPT TO failed: %w", err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA failed: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("SMTP write failed: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("SMTP close failed: %w", err)
	}

	return client.Quit()
}

func buildMessage(from string, to []string, subject, htmlBody, textBody string, attachments []models.Attachment, headers map[string]string, listUnsubscribeURL string, listUnsubscribePost bool) []byte {
	var b strings.Builder

	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(to, ", "))
	fmt.Fprintf(&b, "Subject: %s\r\n", subject)
	b.WriteString("MIME-Version: 1.0\r\n")

	// RFC 8058: List-Unsubscribe headers
	if listUnsubscribeURL != "" {
		fmt.Fprintf(&b, "List-Unsubscribe: <%s>\r\n", listUnsubscribeURL)
		if listUnsubscribePost {
			b.WriteString("List-Unsubscribe-Post: List-Unsubscribe=One-Click\r\n")
		}
	}

	// Write custom headers (already filtered for safety)
	for key, value := range headers {
		fmt.Fprintf(&b, "%s: %s\r\n", key, value)
	}

	hasAttachments := len(attachments) > 0

	if hasAttachments {
		mixedBoundary := "posta-mixed-boundary"
		altBoundary := "posta-alt-boundary"

		fmt.Fprintf(&b, "Content-Type: multipart/mixed; boundary=\"%s\"\r\n\r\n", mixedBoundary)

		// Body part
		fmt.Fprintf(&b, "--%s\r\n", mixedBoundary)

		if htmlBody != "" && textBody != "" {
			fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n", altBoundary)
			fmt.Fprintf(&b, "--%s\r\n", altBoundary)
			b.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n\r\n")
			b.WriteString(textBody)
			fmt.Fprintf(&b, "\r\n--%s\r\n", altBoundary)
			b.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n\r\n")
			b.WriteString(htmlBody)
			fmt.Fprintf(&b, "\r\n--%s--\r\n", altBoundary)
		} else if htmlBody != "" {
			b.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n\r\n")
			b.WriteString(htmlBody)
		} else {
			b.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n\r\n")
			b.WriteString(textBody)
		}

		// Attachment parts
		for _, att := range attachments {
			fmt.Fprintf(&b, "\r\n--%s\r\n", mixedBoundary)
			contentType := att.ContentType
			if contentType == "" {
				contentType = "application/octet-stream"
			}
			fmt.Fprintf(&b, "Content-Type: %s; name=\"%s\"\r\n", contentType, att.Filename)
			b.WriteString("Content-Transfer-Encoding: base64\r\n")
			fmt.Fprintf(&b, "Content-Disposition: attachment; filename=\"%s\"\r\n\r\n", att.Filename)

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
		altBoundary := "posta-boundary-abc123"
		if htmlBody != "" && textBody != "" {
			fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n", altBoundary)
			fmt.Fprintf(&b, "--%s\r\n", altBoundary)
			b.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n\r\n")
			b.WriteString(textBody)
			fmt.Fprintf(&b, "\r\n--%s\r\n", altBoundary)
			b.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n\r\n")
			b.WriteString(htmlBody)
			fmt.Fprintf(&b, "\r\n--%s--\r\n", altBoundary)
		} else if htmlBody != "" {
			b.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n\r\n")
			b.WriteString(htmlBody)
		} else {
			b.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n\r\n")
			b.WriteString(textBody)
		}
	}

	return []byte(b.String())
}

// ValidateAttachments checks that attachments are within size limits.
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
