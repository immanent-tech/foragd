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

	"github.com/immanent-tech/go-base/config"

	"github.com/immanent-tech/foragd/providers/google/gcs"
)

const cacheControlHeaderValue = "public, max-age=31536000, immutable"

var bufPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

type ImageCache struct {
	caches map[string]ObjectCache
}

var NewCache = sync.OnceValues(func() (*ImageCache, error) {
	appCfg, err := config.LoadAppConfig()
	if err != nil {
		return nil, fmt.Errorf("load app config: %w", err)
	}
	c := &ImageCache{
		caches: make(map[string]ObjectCache),
	}
	switch appCfg.GetAppEnvironment() {
	case config.EnvProduction:
		bucketName := os.Getenv("IMAGEPROXY_BUCKET")
		imgCache, err := gcs.Connect(context.Background(), bucketName, "")
		if err != nil {
			return nil, fmt.Errorf("connect to gcs:%s %w", bucketName, err)
		}
		c.caches["imgproxy"] = imgCache
		avatarCache, err := gcs.Connect(context.Background(), bucketName, "avatars")
		if err != nil {
			return nil, fmt.Errorf("connect to gcs:%s/%s %w", bucketName, "avatars", err)
		}
		c.caches["avatars"] = avatarCache
		thumbnailCache, err := gcs.Connect(context.Background(), bucketName, "subscription_images")
		if err != nil {
			return nil, fmt.Errorf("connect to gcs:%s/%s %w", bucketName, "subscription_images", err)
		}
		c.caches["subscription_thumbnails"] = thumbnailCache
		screenshotCache, err := gcs.Connect(context.Background(), bucketName, "screenshots")
		if err != nil {
			return nil, fmt.Errorf("connect to gcs:%s/%s %w", bucketName, "screenshots", err)
		}
		c.caches["screenshots"] = screenshotCache
	default:
		var err error
		imgCache, err := newDirCache("imgproxy")
		if err != nil {
			return nil, fmt.Errorf("create dir cache: %s:  %w", "imgproxy", err)
		}
		c.caches["imgproxy"] = imgCache
		avatarCache, err := newDirCache("avatars")
		if err != nil {
			return nil, fmt.Errorf("create dir cache: %s:  %w", "avatars", err)
		}
		c.caches["avatars"] = avatarCache
		thumbnailCache, err := newDirCache("subscription_images")
		if err != nil {
			return nil, fmt.Errorf("create dir cache: %s: %w", "subscription_images", err)
		}
		c.caches["subscription_thumbnails"] = thumbnailCache
		screenshotsCache, err := newDirCache("screenshots")
		if err != nil {
			return nil, fmt.Errorf("create dir cache: %s: %w", "screenshots", err)
		}
		c.caches["screenshots"] = screenshotsCache
	}

	return c, nil
})

func (c *ImageCache) GetImage(ctx context.Context, key string, buf *bytes.Buffer) error {
	if err := c.caches["imgproxy"].Copy(ctx, key, buf); err != nil {
		return fmt.Errorf("copy image from cache: %w", err)
	}
	return nil
}

func (c *ImageCache) SaveImage(ctx context.Context, id string, data []byte) error {
	c.caches["imgproxy"].Set(ctx, id, data)
	return nil
}

func (c *ImageCache) GetAvatar(ctx context.Context, key string, buf *bytes.Buffer) error {
	if err := c.caches["avatars"].Copy(ctx, key, buf); err != nil {
		return fmt.Errorf("copy avatar from cache: %w", err)
	}
	return nil
}

func (c *ImageCache) SaveAvatar(ctx context.Context, id string, data []byte) error {
	c.caches["avatars"].Set(ctx, id, data)
	return nil
}

func (c *ImageCache) GetThumbnail(ctx context.Context, key string, buf *bytes.Buffer) error {
	if err := c.caches["subscription_thumbnails"].Copy(ctx, key, buf); err != nil {
		return fmt.Errorf("copy thumbnail from cache: %w", err)
	}
	return nil
}

func (c *ImageCache) SaveThumbnail(ctx context.Context, id string, data []byte) error {
	c.caches["subscription_thumbnails"].Set(ctx, id, data)
	return nil
}

func (c *ImageCache) GetScreenshot(ctx context.Context, key string, buf *bytes.Buffer) error {
	if err := c.caches["screenshots"].Copy(ctx, key, buf); err != nil {
		return fmt.Errorf("copy screenshot from cache: %w", err)
	}
	return nil
}

func (c *ImageCache) SaveScreenshot(ctx context.Context, id string, data []byte) error {
	c.caches["screenshots"].Set(ctx, id, data)
	return nil
}

// HandleImage handles fetching and displaying an image from one of the image caches.
func HandleImage(cache *ImageCache) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
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
			err = cache.GetAvatar(req.Context(), key, imgBuf)
		case strings.HasPrefix(req.URL.Path, "/img/subscription"):
			err = cache.GetThumbnail(req.Context(), key, imgBuf)
		case strings.HasPrefix(req.URL.Path, "/img/screenshot"):
			err = cache.GetScreenshot(req.Context(), key, imgBuf)
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
}
