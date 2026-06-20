// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package cache

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/providers/google/gcs"
)

const cacheControlHeaderValue = "public, max-age=31536000, immutable"

var bufPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

var imgCache objectCache

var loadImageCache = sync.OnceValue(func() error {
	switch config.GetEnvironment() {
	case config.EnvProduction:
		bucketName := os.Getenv("IMAGEPROXY_BUCKET")
		var err error
		imgCache, err = gcs.Connect(context.Background(), bucketName, "")
		if err != nil {
			return fmt.Errorf("connect to gcs: %w", err)
		}
	default:
		var err error
		imgCache, err = newDirCache("imgproxy")
		if err != nil {
			return fmt.Errorf("create dir cache: %w", err)
		}
	}

	return nil
})

func GetImage(ctx context.Context, key string, buf *bytes.Buffer) error {
	if err := loadImageCache(); err != nil {
		return fmt.Errorf("load cache: %w", err)
	}
	if err := imgCache.Copy(ctx, key, buf); err != nil {
		return fmt.Errorf("copy image from cache: %w", err)
	}
	return nil
}

func SaveImage(ctx context.Context, id string, data []byte) error {
	if err := loadImageCache(); err != nil {
		return fmt.Errorf("load cache: %w", err)
	}

	imgCache.Set(ctx, id, data)
	return nil
}

var avatarCache objectCache

var loadAvatarCache = sync.OnceValue(func() error {
	switch config.GetEnvironment() {
	case config.EnvProduction:
		bucketName := os.Getenv("FORAGD_SERVER_BUCKET")
		var err error
		avatarCache, err = gcs.Connect(context.Background(), bucketName, "avatars")
		if err != nil {
			return fmt.Errorf("connect to gcs: %w", err)
		}
	default:
		var err error
		avatarCache, err = newDirCache("avatars")
		if err != nil {
			return fmt.Errorf("create dir cache: %w", err)
		}
	}

	return nil
})

func SaveAvatar(ctx context.Context, id string, data []byte) error {
	if err := loadAvatarCache(); err != nil {
		return fmt.Errorf("load cache: %w", err)
	}

	avatarCache.Set(ctx, id, data)
	return nil
}

var thumbnailCache objectCache

var loadThumbnailCache = sync.OnceValue(func() error {
	switch config.GetEnvironment() {
	case config.EnvProduction:
		bucketName := os.Getenv("FORAGD_SERVER_BUCKET")
		var err error
		thumbnailCache, err = gcs.Connect(context.Background(), bucketName, "subscription_images")
		if err != nil {
			return fmt.Errorf("connect to gcs: %w", err)
		}
	default:
		var err error
		thumbnailCache, err = newDirCache("subscription_images")
		if err != nil {
			return fmt.Errorf("create dir cache: %w", err)
		}
	}

	return nil
})

func SaveThumbnail(ctx context.Context, id string, data []byte) error {
	if err := loadThumbnailCache(); err != nil {
		return fmt.Errorf("load cache: %w", err)
	}

	thumbnailCache.Set(ctx, id, data)
	return nil
}

var screenshotCache objectCache

var loadScreenshotCache = sync.OnceValue(func() error {
	switch config.GetEnvironment() {
	case config.EnvProduction:
		bucketName := os.Getenv("FORAGD_SERVER_BUCKET")
		var err error
		screenshotCache, err = gcs.Connect(context.Background(), bucketName, "screenshots")
		if err != nil {
			return fmt.Errorf("connect to gcs: %w", err)
		}
	default:
		var err error
		screenshotCache, err = newDirCache("screenshots")
		if err != nil {
			return fmt.Errorf("create dir cache: %w", err)
		}
	}

	return nil
})

func SaveScreenshot(ctx context.Context, id string, data []byte) error {
	if err := loadScreenshotCache(); err != nil {
		return fmt.Errorf("load cache: %w", err)
	}

	screenshotCache.Set(ctx, id, data)
	return nil
}

// HandleImage handles fetching and displaying an image from one of the image caches.
func HandleImage(res http.ResponseWriter, req *http.Request) {
	key := chi.URLParam(req, "*")

	imgBuf, ok := bufPool.Get().(*bytes.Buffer)
	if !ok {
		res.WriteHeader(http.StatusInternalServerError)
		slogctx.FromCtx(req.Context()).Error("Get image buffer failed.")
		return
	}
	imgBuf.Reset()
	defer bufPool.Put(imgBuf)

	var err error
	switch {
	case strings.HasPrefix(req.URL.Path, "/img/avatar"):
		// Load the image cache.
		if err := loadAvatarCache(); err != nil {
			res.WriteHeader(http.StatusInternalServerError)
			slogctx.FromCtx(req.Context()).Error("Load avatar cache failed.",
				slog.Any("error", err),
			)
			return
		}
		err = avatarCache.Copy(req.Context(), key, imgBuf)
	case strings.HasPrefix(req.URL.Path, "/img/subscription"):
		// Load the image cache.
		if err = loadThumbnailCache(); err != nil {
			res.WriteHeader(http.StatusInternalServerError)
			slogctx.FromCtx(req.Context()).Error("Load subscription image cache failed.",
				slog.Any("error", err),
			)
			return
		}
		err = thumbnailCache.Copy(req.Context(), key, imgBuf)
	case strings.HasPrefix(req.URL.Path, "/img/screenshot"):
		// Load the image cache.
		if err = loadScreenshotCache(); err != nil {
			res.WriteHeader(http.StatusInternalServerError)
			slogctx.FromCtx(req.Context()).Error("Load subscription image cache failed.",
				slog.Any("error", err),
			)
			return
		}
		err = screenshotCache.Copy(req.Context(), key, imgBuf)
	default:
		res.WriteHeader(http.StatusUnprocessableEntity)
		slogctx.FromCtx(req.Context()).Error("Invalid image cache.")
		return
	}

	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(res, req)
			return
		}
		res.WriteHeader(http.StatusInternalServerError)
		slogctx.FromCtx(req.Context()).Error("Write image data.",
			slog.Any("error", err),
		)
		return
	}
	_, err = res.Write(imgBuf.Bytes())
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		slogctx.FromCtx(req.Context()).Error("Write image data.",
			slog.Any("error", err),
		)
		return
	}

	// Return success.
	res.Header().Set("Cache-Control", cacheControlHeaderValue)
	res.WriteHeader(http.StatusOK)
}
