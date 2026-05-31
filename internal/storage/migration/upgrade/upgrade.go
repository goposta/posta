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

// Package upgrade tracks the app version stored in the database and applies
// one-shot data migrations recorded in the in-tree step registry. It runs at
// server start, after the GORM schema AutoMigrate.
//
// Schema migrations live in internal/storage/migration. This package only
// handles versioned data migrations and the version pin used to refuse
// downgrades. See step.go for the registry contract.
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

// Settings keys persisted in the platform `settings` table.
const (
	KeyAppVersion        = "app.version"
	KeyAppFirstStartedAt = "app.first_started_at"
	KeyAppLastStartedAt  = "app.last_started_at"
)

// ErrDowngrade is returned when the binary version is older than the version
// recorded in the database. Callers may choose to allow it via AllowDowngrade.
var ErrDowngrade = errors.New("upgrade: binary is older than the database version (downgrade)")

// Options tunes the upgrade run. Zero value is the safe default.
type Options struct {
	// AllowDowngrade lets the boot proceed even when the binary is older than
	// the stored version. Intended for emergencies; logs a loud warning.
	AllowDowngrade bool

	// PlanEnforcement mirrors config.PlanEnforcement. It is consumed by data
	// steps that provision plan-bearing rows (the personal-workspace backfill).
	PlanEnforcement bool
}

// runOptions holds the Options for the in-progress run so registry steps — whose
// Apply signature takes only a *gorm.DB — can read runtime configuration. Access
// is safe: Run holds the advisory lock for the entire run, serializing it.
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
