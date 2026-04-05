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

package crypto

import (
	"strings"
	"testing"
)

func TestEncryptDecryptWithKey(t *testing.T) {
	Init("test-secret-key")
	defer Init("")

	plaintext := "my-smtp-password-123"

	encrypted, err := Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	if !strings.HasPrefix(encrypted, "enc:") {
		t.Fatalf("encrypted value should start with 'enc:', got: %s", encrypted[:10])
	}

	decrypted, err := Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if decrypted != plaintext {
		t.Fatalf("got %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptWithoutKey(t *testing.T) {
	Init("")

	encoded, err := Encrypt("my-password")
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	if strings.HasPrefix(encoded, "enc:") {
		t.Fatal("without key, should use base64 not enc: prefix")
	}

	decoded, err := Decrypt(encoded)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if decoded != "my-password" {
		t.Fatalf("got %q, want %q", decoded, "my-password")
	}
}

func TestDecryptLegacyBase64(t *testing.T) {
	Init("some-key")
	defer Init("")

	// "my-password" in base64
	legacy := "bXktcGFzc3dvcmQ="
	decrypted, err := Decrypt(legacy)
	if err != nil {
		t.Fatalf("Decrypt legacy failed: %v", err)
	}
	if decrypted != "my-password" {
		t.Fatalf("got %q, want %q", decrypted, "my-password")
	}
}

func TestIsEncrypted(t *testing.T) {
	if IsEncrypted("bXktcGFzc3dvcmQ=") {
		t.Fatal("base64 should not be detected as encrypted")
	}
	if !IsEncrypted("enc:abcdef") {
		t.Fatal("enc: prefixed should be detected as encrypted")
	}
	if IsEncrypted("") {
		t.Fatal("empty string should not be encrypted")
	}
}

func TestEncryptProducesDifferentCiphertexts(t *testing.T) {
	Init("test-key")
	defer Init("")

	enc1, _ := Encrypt("same")
	enc2, _ := Encrypt("same")
	if enc1 == enc2 {
		t.Fatal("same plaintext should produce different ciphertexts (random nonce)")
	}

	dec1, _ := Decrypt(enc1)
	dec2, _ := Decrypt(enc2)
	if dec1 != "same" || dec2 != "same" {
		t.Fatal("both should decrypt to 'same'")
	}
}

func TestEnabled(t *testing.T) {
	Init("")
	if Enabled() {
		t.Fatal("should not be enabled without key")
	}
	Init("key")
	if !Enabled() {
		t.Fatal("should be enabled with key")
	}
	Init("")
}
