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
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/goposta/posta/internal/services/ratelimit"
)

var ErrRateLimited = errors.New("too many submissions")

// quotaChecker is satisfied by *ratelimit.Quotas.
type quotaChecker interface {
	Check(ctx context.Context, quotas []ratelimit.Quota) error
}

// submissionQuotas lists the counters one submission is charged to. A
// honeypot hit is charged to its IP only: bot traffic must not use up the
// shared per-form, per-workspace, or per-address budget that real visitors
// draw on, or a botnet spread across IPs could lock a form.
func (s *Service) submissionQuotas(formID, workspaceID uint, ip, email string, honeypot bool) []ratelimit.Quota {
	var quotas []ratelimit.Quota
	if ip != "" {
		quotas = append(quotas, ratelimit.Quota{
			Key: fmt.Sprintf("form:q:ip:%d:%s", formID, ip), Limit: s.cfg.PerIPHourly, Window: time.Hour,
		})
	}
	if honeypot {
		return quotas
	}
	quotas = append(quotas,
		ratelimit.Quota{Key: fmt.Sprintf("form:q:form:%d", formID), Limit: s.cfg.PerFormHourly, Window: time.Hour},
		ratelimit.Quota{Key: fmt.Sprintf("form:q:ws:%d", workspaceID), Limit: s.cfg.PerWorkspaceDaily, Window: 24 * time.Hour},
	)
	if email != "" {
		quotas = append(quotas, ratelimit.Quota{
			Key: fmt.Sprintf("form:q:email:%d:%s", formID, hashKey(email)), Limit: s.cfg.PerEmailHourly, Window: time.Hour,
		})
	}
	return quotas
}

func (s *Service) checkQuotas(ctx context.Context, formID, workspaceID uint, ip, email string, honeypot bool) error {
	if s.quotas == nil {
		return nil
	}
	if err := s.quotas.Check(ctx, s.submissionQuotas(formID, workspaceID, ip, email, honeypot)); err != nil {
		return ErrRateLimited
	}
	return nil
}

func hashKey(v string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(v)))
	return hex.EncodeToString(sum[:8])
}
