// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/justinas/alice"
	"github.com/spaolacci/murmur3"
	slogctx "github.com/veqryn/slog-context"
	"golang.org/x/sync/errgroup"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/google/gcs"
)

type objectCache interface {
	Get(ctx context.Context, key string) ([]byte, bool)
	Set(ctx context.Context, key string, value []byte)
	Delete(ctx context.Context, key string)
}

var imgCache objectCache

func ImageProxy(proxyURLBase string) http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Load the http client used for making requests to the image proxy.
		err := loadHTTPClient()
		if err != nil {
			res.WriteHeader(http.StatusInternalServerError)
			return models.NewAPIError(fmt.Errorf("image proxy: decode image url: %w", err),
				http.StatusInternalServerError,
			)
		}
		// Load the image cache.
		err = loadImageCache()
		if err != nil {
			res.WriteHeader(http.StatusInternalServerError)
			return models.NewAPIError(fmt.Errorf("image proxy: decode image url: %w", err),
				http.StatusInternalServerError,
			)
		}

		// Get the URL parameters for the request.
		paramStr := chi.URLParam(req, "*")
		params := strings.Split(paramStr, "/")

		// Generate a unique hash for the image and processing options.
		imgHash := strconv.FormatUint(
			murmur3.Sum64([]byte(strings.Join(params[1:len(params)-1], "|")+params[len(params)-1])),
			10,
		)

		// Try to fetch the image from the cache. If found, return the cached image.
		imageData, found := imgCache.Get(req.Context(), imgHash)
		if found {
			// Write the image to the response.
			_, err = res.Write(imageData)
			if err != nil {
				res.WriteHeader(http.StatusInternalServerError)
				return models.NewAPIError(
					fmt.Errorf("image proxy: write image: %w", err),
					http.StatusInternalServerError,
				)
			}
			res.WriteHeader(http.StatusOK)
			return nil
		}

		// Generate a URL to either send the image to the image proxy for processing or fetch it directly.
		var proxiedURL string
		if proxyURLBase != "" { // Generate image URL through proxy.
			// Generate signed URL to pass to proxy.
			proxiedURL = proxyURLBase + "/" + paramStr
		} else { // No proxy supplied, use direct image URL.
			originalURL, err := base64.RawURLEncoding.DecodeString(params[len(params)-1])
			if err != nil {
				res.WriteHeader(http.StatusInternalServerError)
				return models.NewAPIError(fmt.Errorf("image proxy: decode image url: %w", err),
					http.StatusInternalServerError,
				)
			}
			proxiedURL = string(originalURL)
		}

		// Fetch the image (either from proxy or direct).
		resp, err := httpClient.R().
			SetDoNotParseResponse(true).
			Get(proxiedURL)
		if err != nil {
			res.WriteHeader(http.StatusInternalServerError)
			return models.NewAPIError(
				fmt.Errorf("image proxy: send image request: %w", err),
				http.StatusInternalServerError,
			)
		}
		if resp.IsError() {
			res.WriteHeader(resp.StatusCode())
			return models.NewAPIError(
				fmt.Errorf("image proxy: send image request: %w: %s", ErrBackendAPIError, resp.Status()),
				resp.StatusCode(),
			)
		}
		imageData, err = io.ReadAll(resp.RawBody())
		if err != nil {
			res.WriteHeader(http.StatusInternalServerError)
			return models.NewAPIError(
				fmt.Errorf("image proxy: read image response: %w", err),
				http.StatusInternalServerError,
			)
		}

		var wg errgroup.Group
		// Write the image to the response.
		wg.Go(func() error {
			_, err = res.Write(imageData)
			if err != nil {
				res.WriteHeader(http.StatusInternalServerError)
				return models.NewAPIError(
					fmt.Errorf("image proxy: write image: %w", err),
					http.StatusInternalServerError,
				)
			}
			res.WriteHeader(http.StatusOK)
			return nil
		})
		// Save to the cache if not saved already.
		wg.Go(func() error {
			if !found {
				imgCache.Set(req.Context(), imgHash, imageData)
			}
			return nil
		})

		if err := wg.Wait(); err != nil {
			return err
		}

		return nil
	})).ServeHTTP
}

var loadImageCache = sync.OnceValue(func() error {
	switch config.Environment {
	case "production":
		bucketName := os.Getenv("FORAGD_IMAGEPROXY_BUCKET")
		var err error
		imgCache, err = gcs.Connect(context.Background(), bucketName, "")
		if err != nil {
			return fmt.Errorf("unable to load image cache: %w", err)
		}
	default:
		var err error
		imgCache, err = newDirCache()
		if err != nil {
			return fmt.Errorf("unable to load image cache: %w", err)
		}
	}

	return nil
})

// dirCache is a really simple object cache using a directory on the local filesystem. It is used in development
// environment to simulate an image cache.
type dirCache struct {
	*os.Root
}

func newDirCache() (*dirCache, error) {
	root, err := os.OpenRoot("deployments/imgproxy")
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
	err := d.WriteFile(key, value, 0666)
	if err != nil {
		slogctx.FromCtx(ctx).Error("Unable to save file: %w",
			slog.Any("error", err),
		)
	}
}
func (d *dirCache) Delete(_ context.Context, key string) {
	d.Remove(key)
}
