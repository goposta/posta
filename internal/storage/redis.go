// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"context"
	"fmt"

	"github.com/jkaninda/logger"
	"github.com/redis/go-redis/v9"
)

func NewRedis(opts *redis.Options) (*redis.Client, error) {
	client := redis.NewClient(opts)

	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	logger.Info("redis connected")
	return client, nil
}
