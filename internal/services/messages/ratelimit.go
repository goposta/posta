// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package messages

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrRateLimited = errors.New("too many submissions")

type quota struct {
	key    string
	limit  int
	window time.Duration
}

func (s *Service) checkQuotas(ctx context.Context, formID, workspaceID uint, ip, email string) error {
	if s.redis == nil {
		return nil
	}

	quotas := []quota{
		{fmt.Sprintf("form:q:ip:%d:%s", formID, ip), s.cfg.PerIPHourly, time.Hour},
		{fmt.Sprintf("form:q:form:%d", formID), s.cfg.PerFormHourly, time.Hour},
		{fmt.Sprintf("form:q:ws:%d", workspaceID), s.cfg.PerWorkspaceDaily, 24 * time.Hour},
	}
	if email != "" {
		quotas = append(quotas, quota{
			fmt.Sprintf("form:q:email:%d:%s", formID, hashKey(email)), s.cfg.PerEmailHourly, time.Hour,
		})
	}

	for _, q := range quotas {
		if q.limit <= 0 || strings.HasSuffix(q.key, ":") {
			continue
		}
		count, err := s.redis.Incr(ctx, q.key).Result()
		if err != nil {
			return ErrRateLimited
		}
		if count == 1 {
			s.redis.Expire(ctx, q.key, q.window)
		}
		if count > int64(q.limit) {
			return ErrRateLimited
		}
	}
	return nil
}

func hashKey(v string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(v)))
	return hex.EncodeToString(sum[:8])
}
