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

// Package upgrade applies one-shot versioned data migrations after AutoMigrate at start, and refuses downgrades.
package upgrade

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/goposta/posta/internal/models"
	"github.com/jkaninda/logger"
	"gorm.io/gorm"
)

const (
	KeyAppVersion        = "app.version"
	KeyAppFirstStartedAt = "app.first_started_at"
	KeyAppLastStartedAt  = "app.last_started_at"
)

// ErrDowngrade is returned when the binary is older than the stored version, unless AllowDowngrade is set.
var ErrDowngrade = errors.New("upgrade: binary is older than the database version (downgrade)")

// Options tunes the upgrade run; the zero value is the safe default.
type Options struct {
	// AllowDowngrade lets boot proceed on an older binary; for emergencies only, and it logs a loud warning.
	AllowDowngrade bool

	// PlanEnforcement mirrors config.PlanEnforcement for steps that provision plan-bearing rows.
	PlanEnforcement bool
}

// runOptions exposes Options to registry steps; safe because Run holds the advisory lock for the whole run.
var runOptions Options

func Run(ctx context.Context, db *gorm.DB, binaryVersion string, opts Options) error {
	return withLock(ctx, db, func(ctx context.Context) error {
		return runLocked(ctx, db, binaryVersion, opts)
	})
}

func runLocked(ctx context.Context, db *gorm.DB, binaryVersion string, opts Options) error {
	runOptions = opts
	stored, fresh, err := readVersion(db)
	if err != nil {
		return fmt.Errorf("upgrade: read stored version: %w", err)
	}
	if fresh {
		applied, err := loadAppliedIDs(db)
		if err != nil {
			return fmt.Errorf("upgrade: load applied steps: %w", err)
		}
		fresh = len(applied) == 0
	}

	switch {
	case fresh:
		if err := markAllApplied(db, binaryVersion); err != nil {
			return fmt.Errorf("upgrade: bootstrap fresh install: %w", err)
		}
		logger.Info("upgrade: fresh install, sealed step registry", "version", binaryVersion)

	case IsDev(binaryVersion):
		logger.Warn("upgrade: running dev binary against versioned database — skipping version pin",
			"stored_version", stored)

	case IsDowngrade(binaryVersion, stored):
		if !opts.AllowDowngrade {
			return fmt.Errorf("%w: binary=%s, database=%s", ErrDowngrade, binaryVersion, stored)
		}
		logger.Warn("upgrade: downgrade explicitly allowed",
			"binary_version", binaryVersion, "stored_version", stored)
	}
	if err := applyPending(ctx, db, binaryVersion); err != nil {
		return err
	}

	if !IsDev(binaryVersion) {
		if err := writeVersion(db, binaryVersion); err != nil {
			return fmt.Errorf("upgrade: persist version: %w", err)
		}
	}
	return nil
}

func applyPending(ctx context.Context, db *gorm.DB, binaryVersion string) error {
	applied, err := loadAppliedIDs(db)
	if err != nil {
		return fmt.Errorf("upgrade: load applied steps: %w", err)
	}

	for _, step := range registry {
		if _, ok := applied[step.ID]; ok {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		start := time.Now()
		logger.Info("upgrade: applying step", "id", step.ID)

		err := db.Transaction(func(tx *gorm.DB) error {
			if err := step.Apply(tx); err != nil {
				return err
			}
			return tx.Create(&models.UpgradeStep{
				ID:         step.ID,
				AppVersion: binaryVersion,
				AppliedAt:  time.Now().UTC(),
				DurationMS: time.Since(start).Milliseconds(),
			}).Error
		})
		if err != nil {
			return fmt.Errorf("upgrade: step %q failed: %w", step.ID, err)
		}
		logger.Info("upgrade: step applied", "id", step.ID, "duration_ms", time.Since(start).Milliseconds())
	}
	return nil
}

func loadAppliedIDs(db *gorm.DB) (map[string]struct{}, error) {
	var ids []string
	if err := db.Model(&models.UpgradeStep{}).Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	out := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		out[id] = struct{}{}
	}
	return out, nil
}

func markAllApplied(db *gorm.DB, binaryVersion string) error {
	if len(registry) == 0 {
		return nil
	}
	now := time.Now().UTC()
	rows := make([]models.UpgradeStep, 0, len(registry))
	for _, s := range registry {
		rows = append(rows, models.UpgradeStep{
			ID:         s.ID,
			AppVersion: binaryVersion,
			AppliedAt:  now,
		})
	}
	return db.Create(&rows).Error
}
