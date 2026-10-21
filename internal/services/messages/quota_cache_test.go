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

package messages

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/goposta/posta/internal/services/ratelimit"
)

func quotaKeys(qs []ratelimit.Quota) []string {
	keys := make([]string, len(qs))
	for i, q := range qs {
		keys[i] = q.Key
	}
	return keys
}

func TestSubmissionQuotasChargeEverySharedCounter(t *testing.T) {
	s := &Service{cfg: Config{PerIPHourly: 20, PerFormHourly: 200, PerEmailHourly: 5, PerWorkspaceDaily: 1000}}
	keys := quotaKeys(s.submissionQuotas(1, 2, "203.0.113.7", "ada@example.com", false))
	want := []string{"form:q:ip:1:203.0.113.7", "form:q:form:1", "form:q:ws:2", "form:q:email:1:"}
	if len(keys) != len(want) {
		t.Fatalf("keys = %v", keys)
	}
	for i := range want {
		if !strings.HasPrefix(keys[i], want[i]) {
			t.Fatalf("keys = %v, want prefixes %v", keys, want)
		}
	}
}

func TestSubmissionQuotasChargeHoneypotToIPOnly(t *testing.T) {
	s := &Service{cfg: Config{PerIPHourly: 20, PerFormHourly: 200, PerEmailHourly: 5, PerWorkspaceDaily: 1000}}
	keys := quotaKeys(s.submissionQuotas(1, 2, "203.0.113.7", "ada@example.com", true))
	if len(keys) != 1 || keys[0] != "form:q:ip:1:203.0.113.7" {
		t.Fatalf("a honeypot hit must only count against its IP, got %v", keys)
	}
}

func TestSubmissionQuotasSkipMissingIPAndEmail(t *testing.T) {
	s := &Service{cfg: Config{PerIPHourly: 20, PerFormHourly: 200, PerEmailHourly: 5, PerWorkspaceDaily: 1000}}
	for _, k := range quotaKeys(s.submissionQuotas(1, 2, "", "", false)) {
		if strings.Contains(k, ":ip:") || strings.Contains(k, ":email:") {
			t.Fatalf("unexpected quota %q for a request with no IP or email", k)
		}
	}
}

type failingQuotas struct{}

func (failingQuotas) Check(context.Context, []ratelimit.Quota) error {
	return ratelimit.ErrQuotaUnavailable
}

func TestCheckQuotasFailsClosed(t *testing.T) {
	s := &Service{quotas: failingQuotas{}}
	if err := s.checkQuotas(context.Background(), 1, 2, "ip", "", false); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("err = %v, want ErrRateLimited when the quota store is down", err)
	}
}

// memCache stands in for *cache.Cache, round-tripping through JSON like Redis.
type memCache struct {
	data    map[string][]byte
	deleted []string
}

func (m *memCache) Get(_ context.Context, key string, dest any) bool {
	raw, ok := m.data[key]
	return ok && json.Unmarshal(raw, dest) == nil
}

func (m *memCache) Set(_ context.Context, key string, value any, _ time.Duration) {
	raw, _ := json.Marshal(value)
	if m.data == nil {
		m.data = map[string][]byte{}
	}
	m.data[key] = raw
}

func (m *memCache) Delete(_ context.Context, key string) {
	delete(m.data, key)
	m.deleted = append(m.deleted, key)
}

// The service under test has no formRepo: a cache hit must never reach it.
func TestFindFormByKeyServesCachedForm(t *testing.T) {
	ws := uint(4)
	c := &memCache{}
	c.Set(context.Background(), formCachePrefix+"pk1", formCacheEntry{Form: &models.Form{
		ID: 9, WorkspaceID: &ws, PublicKey: "pk1", Status: models.FormStatusActive,
		AllowedOrigins: []string{"https://example.com"}, HoneypotField: "_hp",
	}}, time.Minute)
	s := &Service{formCache: c}

	f, err := s.FindFormByKey("pk1")
	if err != nil {
		t.Fatal(err)
	}
	if f.ID != 9 || *f.WorkspaceID != 4 || f.Honeypot() != "_hp" || len(f.AllowedOrigins) != 1 {
		t.Fatalf("cached form lost fields: %+v", f)
	}
}

func TestFindFormByKeyServesCachedMiss(t *testing.T) {
	c := &memCache{}
	c.Set(context.Background(), formCachePrefix+"nope", formCacheEntry{Missing: true}, time.Minute)
	s := &Service{formCache: c}
	if _, err := s.FindFormByKey("nope"); !errors.Is(err, ErrFormNotFound) {
		t.Fatalf("err = %v, want ErrFormNotFound", err)
	}
}

func TestInvalidateFormKeyDropsEveryKey(t *testing.T) {
	c := &memCache{}
	s := &Service{formCache: c}
	s.InvalidateFormKey("old", "new", "")
	if len(c.deleted) != 2 || c.deleted[0] != formCachePrefix+"old" || c.deleted[1] != formCachePrefix+"new" {
		t.Fatalf("deleted = %v", c.deleted)
	}
}
