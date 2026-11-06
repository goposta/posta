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
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
)

const VisitorTokenPrefix = "pcv_"

// The token itself is handed to the browser once and never kept; only its hash is stored.
func NewVisitorToken() (token, hash string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	token = VisitorTokenPrefix + base64.RawURLEncoding.EncodeToString(raw)
	return token, HashVisitorToken(token), nil
}

func HashVisitorToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func BearerVisitorToken(header string) string {
	const bearer = "Bearer "
	if len(header) <= len(bearer) || !strings.EqualFold(header[:len(bearer)], bearer) {
		return ""
	}
	token := strings.TrimSpace(header[len(bearer):])
	if !strings.HasPrefix(token, VisitorTokenPrefix) || len(token) > 128 {
		return ""
	}
	return token
}
