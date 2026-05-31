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
