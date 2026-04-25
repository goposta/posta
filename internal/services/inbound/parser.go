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

package inbound

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"time"
)

const contentTypeTextPlain = "text/plain"

// ParsedAttachment represents a decoded attachment extracted from the MIME tree.
type ParsedAttachment struct {
	Filename    string
	ContentType string
	Content     []byte
	Size        int64
}

// ParsedEmail is the normalized shape used throughout the inbound pipeline.
// Both the SMTP path (via ParseRawEmail) and the webhook path (via NewParsedFromFields)
// produce the same struct.
type ParsedEmail struct {
	MessageID   string
	From        string
	To          []string
	Cc          []string
	Subject     string
	Date        time.Time
	TextBody    string
	HTMLBody    string
	Headers     map[string]string
	Attachments []ParsedAttachment
	Raw         []byte
}

// ParseRawEmail parses a raw RFC 5322 message into a ParsedEmail.
func ParseRawEmail(raw []byte) (*ParsedEmail, error) {
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("read message: %w", err)
	}

	p := &ParsedEmail{
		Headers: make(map[string]string),
		Raw:     raw,
	}

	dec := &mime.WordDecoder{}
	for k, vs := range msg.Header {
		if len(vs) == 0 {
			continue
		}
		v := vs[0]
		if decoded, derr := dec.DecodeHeader(v); derr == nil {
			v = decoded
		}
		p.Headers[k] = v
	}

	p.MessageID = strings.Trim(p.Headers["Message-Id"], "<>")
	if p.MessageID == "" {
		p.MessageID = strings.Trim(p.Headers["Message-ID"], "<>")
	}
	p.Subject = p.Headers["Subject"]
	p.From = headerAddress(msg.Header.Get("From"))
	p.To = headerAddressList(msg.Header.Get("To"))
	p.Cc = headerAddressList(msg.Header.Get("Cc"))
	if d, err := mail.ParseDate(msg.Header.Get("Date")); err == nil {
		p.Date = d
	} else {
		p.Date = time.Now().UTC()
	}

	contentType := msg.Header.Get("Content-Type")
	if contentType == "" {
		contentType = contentTypeTextPlain
	}
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType = contentTypeTextPlain
		params = map[string]string{}
	}

	if strings.HasPrefix(mediaType, "multipart/") {
		boundary := params["boundary"]
		if boundary == "" {
			return nil, fmt.Errorf("multipart with no boundary")
		}
		if err := walkMultipart(msg.Body, boundary, msg.Header.Get("Content-Transfer-Encoding"), p); err != nil {
			return nil, err
		}
	} else {
		body, err := readBody(msg.Body, msg.Header.Get("Content-Transfer-Encoding"), params["charset"])
		if err != nil {
			return nil, err
		}
		assignBody(p, mediaType, body)
	}

	return p, nil
}

// walkMultipart recursively descends the MIME tree, collecting text/html bodies
// and attachments.
func walkMultipart(r io.Reader, boundary string, _ string, p *ParsedEmail) error {
	mr := multipart.NewReader(r, boundary)
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("multipart read: %w", err)
		}

		partCT := part.Header.Get("Content-Type")
		if partCT == "" {
			partCT = contentTypeTextPlain
		}
		partMedia, partParams, perr := mime.ParseMediaType(partCT)
		if perr != nil {
			partMedia = "application/octet-stream"
			partParams = map[string]string{}
		}

		disposition, dispParams, _ := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
		isAttachment := disposition == "attachment" || dispParams["filename"] != "" || partParams["name"] != ""

		if strings.HasPrefix(partMedia, "multipart/") {
			nestedBoundary := partParams["boundary"]
			if nestedBoundary != "" {
				if err := walkMultipart(part, nestedBoundary, part.Header.Get("Content-Transfer-Encoding"), p); err != nil {
					_ = part.Close()
					return err
				}
			}
			_ = part.Close()
			continue
		}

		enc := part.Header.Get("Content-Transfer-Encoding")

		if isAttachment {
			raw, rerr := io.ReadAll(part)
			_ = part.Close()
			if rerr != nil {
				return fmt.Errorf("read attachment: %w", rerr)
			}
			decoded, derr := decodeTransfer(raw, enc)
			if derr != nil {
				return fmt.Errorf("decode attachment: %w", derr)
			}
			filename := decodeHeaderValue(dispParams["filename"])
			if filename == "" {
				filename = decodeHeaderValue(partParams["name"])
			}
			p.Attachments = append(p.Attachments, ParsedAttachment{
				Filename:    filename,
				ContentType: partMedia,
				Content:     decoded,
				Size:        int64(len(decoded)),
			})
			continue
		}

		body, berr := readBody(part, enc, partParams["charset"])
		_ = part.Close()
		if berr != nil {
			return berr
		}
		assignBody(p, partMedia, body)
	}
}

func assignBody(p *ParsedEmail, mediaType, body string) {
	switch {
	case mediaType == contentTypeTextPlain && p.TextBody == "":
		p.TextBody = body
	case mediaType == "text/html" && p.HTMLBody == "":
		p.HTMLBody = body
	case mediaType == contentTypeTextPlain:
		p.TextBody += "\n" + body
	case mediaType == "text/html":
		p.HTMLBody += "\n" + body
	}
}

func readBody(r io.Reader, encoding, _ string) (string, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}
	decoded, err := decodeTransfer(raw, encoding)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

func decodeTransfer(raw []byte, encoding string) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "", "7bit", "8bit", "binary":
		return raw, nil
	case "base64":
		clean := bytes.Map(func(r rune) rune {
			if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
				return -1
			}
			return r
		}, raw)
		return base64.StdEncoding.DecodeString(string(clean))
	case "quoted-printable":
		return io.ReadAll(quotedprintable.NewReader(bytes.NewReader(raw)))
	default:
		return raw, nil
	}
}

func decodeHeaderValue(v string) string {
	if v == "" {
		return ""
	}
	dec := &mime.WordDecoder{}
	if out, err := dec.DecodeHeader(v); err == nil {
		return out
	}
	return v
}

func headerAddress(v string) string {
	if v == "" {
		return ""
	}
	if addr, err := mail.ParseAddress(v); err == nil {
		return addr.Address
	}
	return v
}

func headerAddressList(v string) []string {
	if v == "" {
		return nil
	}
	addrs, err := mail.ParseAddressList(v)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(addrs))
	for _, a := range addrs {
		out = append(out, a.Address)
	}
	return out
}
