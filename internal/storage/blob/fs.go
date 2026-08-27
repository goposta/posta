// SPDX-FileCopyrightText: 2026 Jonas Kaninda
// SPDX-License-Identifier: AGPL-3.0-or-later

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
	defer func() { _ = file.Close() }()
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
