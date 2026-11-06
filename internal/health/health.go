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

// Package health is shared by server and worker so one probe gets the same shape and meaning of "ready".
package health

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// A hung probe is a failed probe: an orchestrator deciding where to send work needs a quick "no".
const probeTimeout = 2 * time.Second

const (
	StatusOK       = "ok"
	StatusReady    = "ready"
	StatusNotReady = "not ready"
)

// Status field names are an operator contract read by probes and dashboards, so keep them stable.
type Status struct {
	Status   string `json:"status" example:"ready"`
	Database string `json:"database" example:"ok"`
	Redis    string `json:"redis" example:"ok"`
}

func (s Status) Ready() bool { return s.Status == StatusReady }

// Check reports a nil dependency as unconfigured, not skipped: a worker with no Redis cannot take a job.
func Check(ctx context.Context, db *gorm.DB, rdb *redis.Client) Status {
	s := Status{Status: StatusReady, Database: StatusOK, Redis: StatusOK}

	if err := pingDatabase(ctx, db); err != nil {
		s.Status, s.Database = StatusNotReady, err.Error()
	}
	if err := pingRedis(ctx, rdb); err != nil {
		s.Status, s.Redis = StatusNotReady, err.Error()
	}
	return s
}

func pingDatabase(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return errors.New("not configured")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	return sqlDB.PingContext(ctx)
}

func pingRedis(ctx context.Context, rdb *redis.Client) error {
	if rdb == nil {
		return errors.New("not configured")
	}
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	return rdb.Ping(ctx).Err()
}
