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
	"errors"
	"time"

	"github.com/goposta/posta/internal/models"
	"gorm.io/gorm"
)

// Cached because every submission, nonce and preflight resolves the form; editors must invalidate on change.
const (
	formCacheTTL    = 60 * time.Second
	formMissTTL     = 30 * time.Second
	formCachePrefix = "form:pk:"
)

// Unknown keys are cached too, so a scan of random keys does not hit Postgres on every request.
type formCacheEntry struct {
	Missing bool         `json:"missing,omitempty"`
	Form    *models.Form `json:"form,omitempty"`
}

// formKeyCache is satisfied by *cache.Cache.
type formKeyCache interface {
	Get(ctx context.Context, key string, dest any) bool
	Set(ctx context.Context, key string, value any, ttl time.Duration)
	Delete(ctx context.Context, key string)
}

func (s *Service) FindFormByKey(key string) (*models.Form, error) {
	ctx := context.Background()
	if s.formCache != nil && key != "" {
		var entry formCacheEntry
		if s.formCache.Get(ctx, formCachePrefix+key, &entry) {
			if entry.Missing || entry.Form == nil {
				return nil, ErrFormNotFound
			}
			return entry.Form, nil
		}
	}

	f, err := s.formRepo.FindByPublicKey(key)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if s.formCache != nil && key != "" {
				s.formCache.Set(ctx, formCachePrefix+key, formCacheEntry{Missing: true}, formMissTTL)
			}
			return nil, ErrFormNotFound
		}
		return nil, err
	}
	if s.formCache != nil {
		f.LastEditedBy = nil
		s.formCache.Set(ctx, formCachePrefix+key, formCacheEntry{Form: f}, formCacheTTL)
	}
	return f, nil
}

// Call InvalidateFormKey with both the old and new key when a key rotates.
func (s *Service) InvalidateFormKey(keys ...string) {
	if s.formCache == nil {
		return
	}
	ctx := context.Background()
	for _, k := range keys {
		if k != "" {
			s.formCache.Delete(ctx, formCachePrefix+k)
		}
	}
}
