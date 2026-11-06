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
	"encoding/json"
	"strings"

	"github.com/goposta/posta/internal/services/crypto"
	"gorm.io/gorm"
)

// PII answers are encrypted at rest; SenderEmail stays plain since replies, threading and erasure match on it.

// piiLicensed is set from the license at boot by server and worker; until then it says no.
var piiLicensed = func() bool { return false }

func SetPIIEncryptionGate(licensed func() bool) {
	if licensed != nil {
		piiLicensed = licensed
	}
}

// EncryptsPII governs new answers only; answers already encrypted stay encrypted and readable regardless.
func EncryptsPII() bool { return crypto.Enabled() && piiLicensed() }

// piiSeal records which values of a message are kept encrypted.
type piiSeal struct {
	keys    map[string]bool // Data and FieldsJSON keys
	columns map[string]bool // sender_name, sender_phone, subject, body
	plain   *Message        // the plaintext while BeforeSave has it encrypted
}

// roleColumn maps the roles whose column copies the answer.
var roleColumn = map[FieldRole]string{
	RoleSenderName:  "sender_name",
	RoleSenderPhone: "sender_phone",
	RoleSubject:     "subject",
	RoleBody:        "body",
}

// SealPII is a no-op without a key or Enterprise license; Posta never calls a reversible encoding encryption.
func (m *Message) SealPII(defs FieldDefs) {
	if !EncryptsPII() {
		return
	}
	seal := &piiSeal{keys: map[string]bool{}, columns: map[string]bool{}}
	for _, f := range defs {
		if !f.PII || f.IsSection() {
			continue
		}
		seal.keys[f.Key] = true
		if col, ok := roleColumn[f.Role]; ok {
			seal.columns[col] = true
		}
	}
	if len(seal.keys) == 0 {
		return
	}
	m.seal = seal
}

// PIIEncrypted lists the keys whose answers this message keeps encrypted.
func (m *Message) PIIEncrypted() []string {
	if m.seal == nil {
		return nil
	}
	out := make([]string, 0, len(m.seal.keys))
	for k := range m.seal.keys {
		out = append(out, k)
	}
	return out
}

func (m *Message) column(name string) *string {
	switch name {
	case "sender_name":
		return &m.SenderName
	case "sender_phone":
		return &m.SenderPhone
	case "subject":
		return &m.Subject
	case "body":
		return &m.Body
	}
	return nil
}

// BeforeSave encrypts the sealed values; AfterSave restores the plaintext so the caller keeps readable values.
func (m *Message) BeforeSave(*gorm.DB) error {
	if m.seal == nil || !crypto.Enabled() {
		return nil
	}
	plain := *m
	plain.Data = cloneData(m.Data)
	m.seal.plain = &plain

	if len(m.Data) > 0 {
		data := cloneData(m.Data)
		for k := range m.seal.keys {
			v, ok := data[k]
			if !ok || v == nil {
				continue
			}
			if s, isString := v.(string); isString && crypto.IsEncrypted(s) {
				continue
			}
			sealed, err := sealValue(v)
			if err != nil {
				return err
			}
			data[k] = sealed
		}
		m.Data = data
	}
	if m.FieldsJSON != "" {
		var fields []MessageField
		if err := json.Unmarshal([]byte(m.FieldsJSON), &fields); err == nil {
			for i := range fields {
				if !m.seal.keys[fields[i].Key] || fields[i].Value == "" || crypto.IsEncrypted(fields[i].Value) {
					continue
				}
				enc, err := crypto.Encrypt(fields[i].Value)
				if err != nil {
					return err
				}
				fields[i].Value = enc
			}
			raw, _ := json.Marshal(fields)
			m.FieldsJSON = string(raw)
		}
	}
	for col := range m.seal.columns {
		p := m.column(col)
		if *p == "" || crypto.IsEncrypted(*p) {
			continue
		}
		enc, err := crypto.Encrypt(*p)
		if err != nil {
			return err
		}
		*p = enc
	}
	return nil
}

func (m *Message) AfterSave(*gorm.DB) error {
	if m.seal == nil || m.seal.plain == nil {
		return nil
	}
	plain := m.seal.plain
	m.Data, m.FieldsJSON = plain.Data, plain.FieldsJSON
	m.SenderName, m.SenderPhone, m.Subject, m.Body = plain.SenderName, plain.SenderPhone, plain.Subject, plain.Body
	m.seal.plain = nil
	return nil
}

// AfterFind decrypts and remembers sealed values for re-encryption; undecryptable values are left as stored.
func (m *Message) AfterFind(*gorm.DB) error {
	seal := &piiSeal{keys: map[string]bool{}, columns: map[string]bool{}}
	for k, v := range m.Data {
		s, ok := v.(string)
		if !ok || !crypto.IsEncrypted(s) {
			continue
		}
		if plain, err := openValue(s); err == nil {
			m.Data[k] = plain
			seal.keys[k] = true
		}
	}
	if strings.Contains(m.FieldsJSON, `"enc:`) {
		var fields []MessageField
		if err := json.Unmarshal([]byte(m.FieldsJSON), &fields); err == nil {
			for i := range fields {
				if !crypto.IsEncrypted(fields[i].Value) {
					continue
				}
				if plain, err := crypto.Decrypt(fields[i].Value); err == nil {
					fields[i].Value = plain
					seal.keys[fields[i].Key] = true
				}
			}
			raw, _ := json.Marshal(fields)
			m.FieldsJSON = string(raw)
		}
	}
	for col := range roleColumn {
		name := roleColumn[col]
		p := m.column(name)
		if !crypto.IsEncrypted(*p) {
			continue
		}
		if plain, err := crypto.Decrypt(*p); err == nil {
			*p = plain
			seal.columns[name] = true
		}
	}
	if len(seal.keys) > 0 || len(seal.columns) > 0 {
		m.seal = seal
	}
	return nil
}

// sealValue encrypts a typed answer as JSON so a number or list comes back as the same type.
func sealValue(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return crypto.Encrypt(string(raw))
}

func openValue(s string) (any, error) {
	plain, err := crypto.Decrypt(s)
	if err != nil {
		return nil, err
	}
	var v any
	if err := json.Unmarshal([]byte(plain), &v); err != nil {
		return plain, nil
	}
	return v, nil
}

func cloneData(d SubmissionData) SubmissionData {
	if d == nil {
		return nil
	}
	out := make(SubmissionData, len(d))
	for k, v := range d {
		out[k] = v
	}
	return out
}
