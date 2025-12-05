// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Code originally from https://github.com/willnorris/imageproxy/blob/main/internal/gcscache/gcscache.go

package gcs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path"

	"cloud.google.com/go/storage"
	slogctx "github.com/veqryn/slog-context"
)

type Bucket struct {
	storage *storage.BucketHandle
	prefix  string
}

func (b *Bucket) Get(ctx context.Context, key string) ([]byte, bool) {
	r, err := b.object(key).NewReader(ctx)
	if err != nil {
		if !errors.Is(err, storage.ErrObjectNotExist) {
			slogctx.FromCtx(ctx).Warn("Could not get object.",
				slog.String("bucket", b.storage.BucketName()),
				slog.Any("error", err),
			)
		}
		return nil, false
	}
	defer r.Close()

	value, err := io.ReadAll(r)
	if err != nil {
		slogctx.FromCtx(ctx).Warn("Could not read object data.",
			slog.String("bucket", b.storage.BucketName()),
			slog.Any("error", err),
		)
		return nil, false
	}

	return value, true
}

func (b *Bucket) Set(ctx context.Context, key string, value []byte) {
	objWriter := b.object(key).NewWriter(ctx)
	if _, err := objWriter.Write(value); err != nil {
		slogctx.FromCtx(ctx).Warn("Could not write object to bucket.",
			slog.String("bucket", b.storage.BucketName()),
			slog.Any("error", err),
		)
	}
	if err := objWriter.Close(); err != nil {
		slogctx.FromCtx(ctx).Warn("Could not close object writer.",
			slog.String("bucket", b.storage.BucketName()),
			slog.Any("error", err),
		)
	}
}

func (b *Bucket) Delete(ctx context.Context, key string) {
	if err := b.object(key).Delete(ctx); err != nil {
		slogctx.FromCtx(ctx).Warn("Could not delete object.",
			slog.String("bucket", b.storage.BucketName()),
			slog.Any("error", err),
		)
	}
}

func (b *Bucket) object(key string) *storage.ObjectHandle {
	name := path.Join(b.prefix, key)
	return b.storage.Object(name)
}

func Connect(ctx context.Context, name, prefix string) (*Bucket, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create storage client: %w", err)
	}

	return &Bucket{
		prefix:  prefix,
		storage: client.Bucket(name),
	}, nil
}
