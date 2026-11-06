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

package upgrade

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

const advisoryLockKey int64 = 0x506F73746155706C // "PostaUpl"

// withLock serializes concurrent replica boots through a Postgres advisory lock so only one runs the registry.
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
