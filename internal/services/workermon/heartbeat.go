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
