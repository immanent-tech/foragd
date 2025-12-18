// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	slogctx "github.com/veqryn/slog-context"
)

type objectCache interface {
	Get(ctx context.Context, key string) ([]byte, bool)
	Set(ctx context.Context, key string, value []byte)
	Delete(ctx context.Context, key string)
}

// dirCache is a really simple object cache using a directory on the local filesystem. It is used in development
// environment to simulate an image cache.
type dirCache struct {
	*os.Root
}

func newDirCache(prefix string) (*dirCache, error) {
	root, err := os.OpenRoot(filepath.Join("deployments", prefix))
	if err != nil {
		return nil, fmt.Errorf("unable to open dircache: %w", err)
	}
	return &dirCache{Root: root}, nil
}

func (d *dirCache) Get(ctx context.Context, key string) ([]byte, bool) {
	data, err := d.ReadFile(key)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			slogctx.FromCtx(ctx).Error("Unable to get file.",
				slog.Any("error", err),
			)
		}
		return nil, false
	}
	return data, true
}
func (d *dirCache) Set(ctx context.Context, key string, value []byte) {
	const defaultFilePerms = 0666
	if err := d.WriteFile(key, value, defaultFilePerms); err != nil {
		slogctx.FromCtx(ctx).Error("Unable to save file: %w",
			slog.Any("error", err),
		)
	}
}
func (d *dirCache) Delete(ctx context.Context, key string) {
	if err := d.Remove(key); err != nil {
		slogctx.FromCtx(ctx).Error("Unable to remove file: %w",
			slog.Any("error", err),
		)
	}
}
