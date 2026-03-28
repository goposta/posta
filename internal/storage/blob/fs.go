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
	"os"
	"path/filepath"
)

type fsStore struct {
	basePath string
}

func newFSStore(cfg Config) (*fsStore, error) {
	base := cfg.FSBasePath
	if base == "" {
		base = "data/attachments"
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create blob directory %q: %w", base, err)
	}
	return &fsStore{basePath: base}, nil
}

func (f *fsStore) Put(_ context.Context, key string, r io.Reader, _ string) error {
	path := filepath.Join(f.basePath, key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("fs mkdir %q: %w", filepath.Dir(path), err)
	}
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("fs create %q: %w", path, err)
	}
	defer file.Close()
	if _, err := io.Copy(file, r); err != nil {
		return fmt.Errorf("fs write %q: %w", path, err)
	}
	return nil
}

func (f *fsStore) Get(_ context.Context, key string) (io.ReadCloser, error) {
	path := filepath.Join(f.basePath, key)
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("fs open %q: %w", path, err)
	}
	return file, nil
}

func (f *fsStore) Delete(_ context.Context, key string) error {
	path := filepath.Join(f.basePath, key)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("fs delete %q: %w", path, err)
	}
	return nil
}
