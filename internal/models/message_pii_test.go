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

package models

import (
	"reflect"
	"strings"
	"testing"

	"github.com/goposta/posta/internal/services/crypto"
)

func piiForm() FieldDefs {
	return FieldDefs{
		{Key: "email", Type: FieldEmail, Role: RoleSenderEmail, PII: true},
		{Key: "name", Type: FieldText, Role: RoleSenderName, PII: true},
		{Key: "salary", Type: FieldNumber, PII: true},
		{Key: "langs", Type: FieldMultiSelect, PII: true},
		{Key: "dept", Type: FieldSelect, Role: RoleSubject},
		{Key: "page", Type: FieldSection, PII: true},
	}
}

func piiMessage() *Message {
	return &Message{
		SenderEmail: "ada@example.com", SenderName: "Ada", Subject: "sales", Body: "hello",
		FieldsJSON: `[{"key":"email","value":"ada@example.com"},{"key":"name","value":"Ada"},{"key":"salary","value":"90000"},{"key":"dept","value":"Sales"}]`,
		Data:       SubmissionData{"email": "ada@example.com", "name": "Ada", "salary": float64(90000), "langs": []any{"go", "sql"}, "dept": "sales"},
	}
}

// licensed switches on encryption at rest, an Enterprise feature, for one test.
func licensed(t *testing.T) {
	t.Helper()
	prev := piiLicensed
	SetPIIEncryptionGate(func() bool { return true })
	t.Cleanup(func() { piiLicensed = prev })
}

func TestPIIEncryptedAtRest(t *testing.T) {
	licensed(t)
	crypto.Init("test-key-for-message-pii")
	defer crypto.Init("")

	m := piiMessage()
	m.SealPII(piiForm())
	if err := m.BeforeSave(nil); err != nil {
		t.Fatal(err)
	}
	stored := *m
	stored.Data = cloneData(m.Data)

	for _, k := range []string{"email", "name", "salary", "langs"} {
		if s, ok := stored.Data[k].(string); !ok || !crypto.IsEncrypted(s) {
			t.Errorf("data.%s stored as %v", k, stored.Data[k])
		}
	}
	if stored.Data["dept"] != "sales" {
		t.Error("an answer that is not personal data was encrypted")
	}
	if strings.Contains(stored.FieldsJSON, "90000") || strings.Contains(stored.FieldsJSON, `"Ada"`) || !strings.Contains(stored.FieldsJSON, `"Sales"`) {
		t.Errorf("fields_json stored as %s", stored.FieldsJSON)
	}
	if !crypto.IsEncrypted(stored.SenderName) || crypto.IsEncrypted(stored.Subject) || crypto.IsEncrypted(stored.Body) {
		t.Errorf("columns: name %q subject %q body %q", stored.SenderName, stored.Subject, stored.Body)
	}
	if stored.SenderEmail != "ada@example.com" {
		t.Error("sender_email must stay readable: replies and suppression match on it")
	}

	// The caller keeps plaintext after saving.
	_ = m.AfterSave(nil)
	if !reflect.DeepEqual(m.Data, piiMessage().Data) || m.SenderName != "Ada" || m.FieldsJSON != piiMessage().FieldsJSON {
		t.Errorf("after save: %+v", m)
	}

	// Loading decrypts back to the same types, and a later save re-encrypts.
	loaded := stored
	loaded.seal = nil
	if err := loaded.AfterFind(nil); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded.Data, piiMessage().Data) || loaded.SenderName != "Ada" || loaded.FieldsJSON != piiMessage().FieldsJSON {
		t.Errorf("loaded: data %v name %q fields %s", loaded.Data, loaded.SenderName, loaded.FieldsJSON)
	}
	_ = loaded.BeforeSave(nil)
	if s, _ := loaded.Data["salary"].(string); !crypto.IsEncrypted(s) || !crypto.IsEncrypted(loaded.SenderName) {
		t.Error("saving a loaded message wrote personal data in plaintext")
	}
}

func TestPIINotEncryptedWithoutKey(t *testing.T) {
	licensed(t)
	crypto.Init("")
	m := piiMessage()
	m.SealPII(piiForm())
	_ = m.BeforeSave(nil)
	if m.Data["salary"] != float64(90000) || m.SenderName != "Ada" {
		t.Error("without a key, values must be stored as given, not encoded")
	}
}

func TestPIINotEncryptedWithoutLicense(t *testing.T) {
	crypto.Init("test-key-for-message-pii")
	defer crypto.Init("")
	m := piiMessage()
	m.SealPII(piiForm())
	_ = m.BeforeSave(nil)
	if m.Data["salary"] != float64(90000) || m.SenderName != "Ada" || EncryptsPII() {
		t.Error("without an Enterprise license, new answers must be stored as given")
	}

	// Answers encrypted under a license stay encrypted, and readable, after it
	// is gone: saving a loaded message encrypts it again.
	licensed(t)
	sealed := piiMessage()
	sealed.SealPII(piiForm())
	_ = sealed.BeforeSave(nil)
	stored := *sealed
	stored.Data, stored.seal = cloneData(sealed.Data), nil
	SetPIIEncryptionGate(func() bool { return false })
	_ = stored.AfterFind(nil)
	if stored.Data["salary"] != float64(90000) {
		t.Fatalf("not readable without a license: %v", stored.Data["salary"])
	}
	_ = stored.BeforeSave(nil)
	if s, _ := stored.Data["salary"].(string); !crypto.IsEncrypted(s) {
		t.Error("an encrypted answer was written back in plaintext once the license was gone")
	}
}

func TestPIIUndecryptableLeftAsStored(t *testing.T) {
	licensed(t)
	crypto.Init("key-one")
	m := piiMessage()
	m.SealPII(piiForm())
	_ = m.BeforeSave(nil)
	stored := *m
	stored.Data = cloneData(m.Data)
	stored.seal = nil

	crypto.Init("key-two")
	defer crypto.Init("")
	_ = stored.AfterFind(nil)
	if s, _ := stored.Data["salary"].(string); !crypto.IsEncrypted(s) {
		t.Errorf("a value under another key should stay as stored, got %v", stored.Data["salary"])
	}
}
