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

package blob

import (
	"context"
	"fmt"
	"io"
)

// Store abstracts object/blob storage for email attachments.
// Implementations may target S3-compatible services, local filesystem, etc.
type Store interface {
	// Put writes data to the given key and returns the key on success.
	Put(ctx context.Context, key string, r io.Reader, contentType string) error
	// Get returns a reader for the given key.
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	// Delete removes the object at the given key.
	Delete(ctx context.Context, key string) error
}

// Config holds the configuration for the blob store.
type Config struct {
	// Provider is the storage backend: "s3" or "fs" (filesystem).
	Provider string
	// S3 settings
	S3Endpoint        string
	S3Region          string
	S3Bucket          string
	S3AccessKeyID     string
	S3SecretAccessKey string
	S3UseSSL          bool
	S3ForcePathStyle  bool
	// Filesystem settings
	FSBasePath string
}

// New creates a new Store based on the configuration.
func New(cfg Config) (Store, error) {
	switch cfg.Provider {
	case "s3":
		return newS3Store(cfg)
	case "fs", "filesystem":
		return newFSStore(cfg)
	case "":
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported blob storage provider: %q", cfg.Provider)
	}
}
