// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

package workermon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	heartbeatInterval = 15 * time.Second
	heartbeatTTL      = 45 * time.Second
	heartbeatPrefix   = "posta:worker:"
)

type WorkerHeartbeat struct {
	Version  string `json:"version"`
	CommitID string `json:"commit_id"`
}

func heartbeatKey(host string, pid int) string {
	return fmt.Sprintf("%s%s:%d", heartbeatPrefix, host, pid)
}

func StartHeartbeat(ctx context.Context, rdb *redis.Client, version, commitID string) {
	if rdb == nil {
		return
	}
	host, _ := os.Hostname()
	key := heartbeatKey(host, os.Getpid())
	payload, _ := json.Marshal(WorkerHeartbeat{Version: version, CommitID: commitID})

	set := func() {
		c, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = rdb.Set(c, key, payload, heartbeatTTL).Err()
	}

	set()
	go func() {
		t := time.NewTicker(heartbeatInterval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				c, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				_ = rdb.Del(c, key).Err()
				cancel()
				return
			case <-t.C:
				set()
			}
		}
	}()
}

func ReadHeartbeats(ctx context.Context, rdb *redis.Client) map[string]WorkerHeartbeat {
	out := map[string]WorkerHeartbeat{}
	if rdb == nil {
		return out
	}
	var cursor uint64
	for {
		keys, next, err := rdb.Scan(ctx, cursor, heartbeatPrefix+"*", 100).Result()
		if err != nil {
			return out
		}
		for _, k := range keys {
			val, err := rdb.Get(ctx, k).Result()
			if err != nil {
				continue
			}
			var hb WorkerHeartbeat
			if json.Unmarshal([]byte(val), &hb) == nil {
				out[strings.TrimPrefix(k, heartbeatPrefix)] = hb
			}
		}
		if next == 0 {
			break
		}
		cursor = next
	}
	return out
}
