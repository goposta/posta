// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package upgrade

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

const advisoryLockKey int64 = 0x506F73746155706C // "PostaUpl"

// withLock acquires a Postgres session-level advisory lock for the upgrade
// flow, runs fn, then releases the lock. Concurrent boots of multiple
// replicas serialize through this lock so only one runs the registry.
func withLock(ctx context.Context, db *gorm.DB, fn func(context.Context) error) error {
	conn, err := db.DB()
	if err != nil {
		return fmt.Errorf("upgrade: acquire raw conn: %w", err)
	}
	// Use a single connection so the lock is held for the duration of fn.
	c, err := conn.Conn(ctx)
	if err != nil {
		return fmt.Errorf("upgrade: pin conn: %w", err)
	}
	defer func() { _ = c.Close() }()

	if _, err := c.ExecContext(ctx, "SELECT pg_advisory_lock($1)", advisoryLockKey); err != nil {
		return fmt.Errorf("upgrade: acquire lock: %w", err)
	}
	defer func() {
		_, _ = c.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", advisoryLockKey)
	}()

	return fn(ctx)
}
