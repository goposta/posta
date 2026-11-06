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

package chat

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/crypto"
)

func identityWidget(t *testing.T) (*models.ChatWidget, string) {
	t.Helper()
	crypto.Init("test-encryption-key")
	t.Cleanup(func() { crypto.Init("") })
	secret, stored, err := NewIdentitySecret()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored, secret) {
		t.Fatal("the identity secret is stored in the clear")
	}
	return &models.ChatWidget{IdentitySecretEnc: stored}, secret
}

func TestVerifyIdentity(t *testing.T) {
	w, secret := identityWidget(t)
	good := &Identity{UserID: "u_123", Email: "Jane@Acme.com", Signature: SignIdentity(secret, "u_123", "jane@acme.com")}
	if !VerifyIdentity(w, good) {
		t.Fatal("a valid signature did not verify (email case must not matter)")
	}
	for name, id := range map[string]*Identity{
		"tampered email":  {UserID: "u_123", Email: "ceo@acme.com", Signature: good.Signature},
		"tampered user":   {UserID: "u_124", Email: "jane@acme.com", Signature: good.Signature},
		"other secret":    {UserID: "u_123", Email: "jane@acme.com", Signature: SignIdentity("pcs_other", "u_123", "jane@acme.com")},
		"empty signature": {UserID: "u_123", Email: "jane@acme.com"},
		"no user":         {Email: "jane@acme.com", Signature: SignIdentity(secret, "", "jane@acme.com")},
		"nil":             nil,
	} {
		if VerifyIdentity(w, id) {
			t.Errorf("%s: verified", name)
		}
	}
	// Rotating the secret makes old signatures fail.
	_, rotated, _ := NewIdentitySecret()
	w.IdentitySecretEnc = rotated
	if VerifyIdentity(w, good) {
		t.Fatal("an identity signed with the old secret still verifies")
	}
	// No secret: nothing verifies.
	if VerifyIdentity(&models.ChatWidget{}, good) {
		t.Fatal("a widget without a secret verified an identity")
	}
}

func TestCleanAttributes(t *testing.T) {
	got, err := CleanAttributes(map[string]any{"plan": " pro ", "seats": 12.0, "trial": true, "gone": nil})
	if err != nil || got["plan"] != "pro" || got["seats"] != 12.0 || got["trial"] != true || len(got) != 3 {
		t.Fatalf("CleanAttributes = %v, %v", got, err)
	}
	for name, bad := range map[string]map[string]any{
		"nested":    {"a": map[string]any{"b": 1}},
		"array":     {"a": []any{1}},
		"too many":  manyAttrs(31),
		"too big":   {"a": strings.Repeat("x", 199), "b": strings.Repeat("y", 199), "c": strings.Repeat("z", 199), "d": strings.Repeat("w", 199), "e": strings.Repeat("v", 199), "f": strings.Repeat("u", 199), "g": strings.Repeat("t", 199), "h": strings.Repeat("s", 199), "i": strings.Repeat("r", 199), "j": strings.Repeat("q", 199), "k": strings.Repeat("p", 199)},
		"blank key": {" ": "x"},
	} {
		if _, err := CleanAttributes(bad); err != ErrInvalidAttributes {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func manyAttrs(n int) map[string]any {
	out := map[string]any{}
	for i := 0; i < n; i++ {
		out[strings.Repeat("k", i+1)] = true
	}
	return out
}

type memBlobs map[string][]byte

func (m memBlobs) Put(_ context.Context, key string, r io.Reader, _ string) error {
	b, err := io.ReadAll(r)
	m[key] = b
	return err
}
func (m memBlobs) Get(_ context.Context, key string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(m[key])), nil
}
func (m memBlobs) Delete(_ context.Context, key string) error { delete(m, key); return nil }

func TestStoreAttachmentSniffsTheType(t *testing.T) {
	ws := uint(1)
	blobs := memBlobs{}
	s := &Service{blobs: blobs}
	w := &models.ChatWidget{AllowAttachments: true}
	conv := &models.ChatConversation{UUID: "c1", WorkspaceID: &ws}
	png := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 64)...)

	att, err := s.storeAttachment(context.Background(), w, conv, Upload{Filename: "../../etc/shot.png", Data: png})
	if err != nil || att.ContentType != "image/png" || att.Filename != "shot.png" || !strings.HasSuffix(att.Key, ".png") {
		t.Fatalf("png: %+v, %v", att, err)
	}
	if _, ok := blobs[att.Key]; !ok {
		t.Fatal("file not stored")
	}
	// The claimed name does not decide the type: HTML named .png is refused.
	if _, err := s.storeAttachment(context.Background(), w, conv, Upload{Filename: "x.png", Data: []byte("<html><script>alert(1)</script>")}); err != ErrAttachmentType {
		t.Fatalf("html as png: err = %v", err)
	}
	if _, err := s.storeAttachment(context.Background(), w, conv, Upload{Filename: "big.pdf", Data: make([]byte, MaxAttachmentBytes+1)}); err != ErrAttachmentTooLarge {
		t.Fatalf("oversize: err = %v", err)
	}
	if _, err := s.storeAttachment(context.Background(), &models.ChatWidget{}, conv, Upload{Data: png}); err != ErrAttachmentsOff {
		t.Fatalf("disabled widget: err = %v", err)
	}
}
