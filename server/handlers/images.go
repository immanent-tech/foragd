// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	slogctx "github.com/veqryn/slog-context"
	"github.com/zeebo/xxh3"
	"golang.org/x/sync/errgroup"

	"github.com/immanent-tech/foragd/client"
	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/google/gcs"
	"github.com/immanent-tech/foragd/reverseproxy"
	"github.com/immanent-tech/foragd/web"
)

func ImageProxy(proxyURLBase string) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Load the image cache.
		if err := loadImageCache(); err != nil {
			res.WriteHeader(http.StatusInternalServerError)
			slogctx.FromCtx(req.Context()).Error("Load image cache failed.",
				slog.Any("error", err),
			)
			return
		}

		// Get the URL parameters for the request.
		paramStr := chi.URLParam(req, "*")
		params := strings.Split(paramStr, "/")

		// Generate a unique hash for the image and processing options.
		imgHash := strconv.FormatUint(
			xxh3.Hash([]byte(strings.Join(params[1:len(params)-1], "|")+params[len(params)-1])),
			10,
		)

		imgBuf, ok := bufPool.Get().(*bytes.Buffer)
		if !ok {
			res.WriteHeader(http.StatusInternalServerError)
			slogctx.FromCtx(req.Context()).Error("Get image buffer failed.")
			return
		}
		imgBuf.Reset()
		defer bufPool.Put(imgBuf)

		var found bool
		// Try to fetch the image from the cache. If found, return the cached image.
		if err := imgCache.Copy(req.Context(), imgHash, imgBuf); err == nil {
			found = true
			// Write the image to the response.
			if _, err := res.Write(imgBuf.Bytes()); err != nil {
				res.WriteHeader(http.StatusInternalServerError)
				slogctx.FromCtx(req.Context()).Error("Write image data.",
					slog.Any("error", err),
				)
				return
			}
			// Return success.
			res.Header().Set("Cache-Control", "public, max-age=31536000, s-maxage=31536000, immutable")
			res.WriteHeader(http.StatusOK)
			return
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
				slogctx.FromCtx(req.Context()).Error("Generate encoded URL for image proxy failed.",
					slog.Any("error", err),
				)
				return
			}
			proxiedURL = string(originalURL)
		}

		// Fetch the remote image.
		if err := getRemoteImage(req.Context(), proxiedURL, imgBuf); err != nil {
			if apiErr, ok := errors.AsType[*models.APIError](err); ok {
				res.WriteHeader(apiErr.StatusCode)
			} else {
				res.WriteHeader(http.StatusInternalServerError)
			}
			slogctx.FromCtx(req.Context()).Error("Fetch remote image failed.",
				slog.Any("error", err),
			)
			sendImagePlaceholder(req.Context(), res, imgBuf)
			return
		}

		wg, jobCtx := errgroup.WithContext(req.Context())
		defer jobCtx.Done()
		// Write the image to the response.
		wg.Go(func() error {
			if _, err := res.Write(imgBuf.Bytes()); err != nil {
				res.WriteHeader(http.StatusInternalServerError)
				sendImagePlaceholder(req.Context(), res, imgBuf)
				return fmt.Errorf("write image: %w", err)
			}
			res.WriteHeader(http.StatusOK)
			return nil
		})
		// Save to the cache if not saved already.
		wg.Go(func() error {
			if !found {
				imgCache.Set(jobCtx, imgHash, imgBuf.Bytes())
			}
			return nil
		})

		if err := wg.Wait(); err != nil {
			slogctx.FromCtx(req.Context()).Error("Run background image jobs failed.",
				slog.Any("error", err),
			)
			return
		}

		res.Header().Set("Cache-Control", "public, max-age=31536000, s-maxage=31536000, immutable")
		res.WriteHeader(http.StatusOK)
	}
}

// LoadCachedImage handles fetching and displaying an image from one of the image caches.
func LoadCachedImage(res http.ResponseWriter, req *http.Request) {
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
		var thumbnailCache objectCache
		// Load the image cache.
		thumbnailCache, err = loadThumbnailCache()
		if err != nil {
			res.WriteHeader(http.StatusInternalServerError)
			slogctx.FromCtx(req.Context()).Error("Load subscription image cache failed.",
				slog.Any("error", err),
			)
			return
		}
		err = thumbnailCache.Copy(req.Context(), key, imgBuf)
	case strings.HasPrefix(req.URL.Path, "/img/screenshot"):
		var screenshotCache objectCache
		// Load the image cache.
		screenshotCache, err = loadScreenshotCache()
		if err != nil {
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
	res.Header().Set("Cache-Control", "public, max-age=31536000, s-maxage=31536000, immutable")
	res.WriteHeader(http.StatusOK)
}

// getRemoteImage fetches the image at the given url writes it into the image buffer.
func getRemoteImage(ctx context.Context, remoteURL string, buf *bytes.Buffer) error {
	// Load the http client used for making requests to the image proxy.
	httpClient := client.LoadHTTPClient()

	// Fetch the image (either from proxy or direct).
	resp, err := httpClient.R().
		SetContext(ctx).
		SetDoNotParseResponse(true).
		Get(remoteURL)
	if err != nil {

		return &models.APIError{
			InternalError: err,
			StatusCode:    http.StatusInternalServerError,
		}
	}
	if resp.IsError() {
		// When the error response is 403: Forbidden try again with our Cloudflare reverse proxy.
		if resp.StatusCode() == http.StatusForbidden || resp.StatusCode() == http.StatusTooManyRequests {
			proxiedURL, err := reverseproxy.GenerateProxyURL(remoteURL)
			if err != nil {
				return &models.APIError{
					InternalError: fmt.Errorf("generate proxy url: %w", err),
					StatusCode:    http.StatusInternalServerError,
				}
			}
			return getRemoteImage(ctx, proxiedURL, buf)
		}
		return &models.APIError{
			InternalError: errors.New(resp.Status()),
			StatusCode:    resp.StatusCode(),
		}
	}

	// Copy the image data to the buffer.
	defer resp.RawBody().Close()
	_, err = io.Copy(buf, resp.RawBody())
	if err != nil {
		return &models.APIError{
			InternalError: errors.New(resp.Status()),
			StatusCode:    resp.StatusCode(),
		}
	}
	return nil
}

// getPlaceholderImage fetches the placeholder image and writes it into the image buffer.
func getPlaceholderImage(buf *bytes.Buffer) error {
	placeholder, err := web.StaticContentFS.ReadFile("content/images/placeholder.webp")
	if err != nil {
		return fmt.Errorf("read placeholder: %w", err)
	}
	if _, err := buf.Write(placeholder); err != nil {
		return fmt.Errorf("write placeholder: %w", err)
	}
	return nil
}

// sendPlaceholder writes a placeholder image to the http response. If the placeholder image cannot be retrieved, no
// response body is written.
func sendImagePlaceholder(ctx context.Context, res http.ResponseWriter, buf *bytes.Buffer) {
	if err := getPlaceholderImage(buf); err != nil {
		slogctx.FromCtx(ctx).Warn("Could not write placeholder image to buffer.",
			slog.Any("error", err))
		return
	}
	res.Header().Set("Content-Type", "image/webp")
	res.Write(buf.Bytes())
}

var imgCache objectCache

var loadImageCache = sync.OnceValue(func() error {
	switch config.CurrentEnvironment {
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

var avatarCache objectCache

var loadAvatarCache = sync.OnceValue(func() error {
	switch config.CurrentEnvironment {
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

var loadThumbnailCache = sync.OnceValues(func() (objectCache, error) {
	var thumbnailCache objectCache
	switch config.CurrentEnvironment {
	case config.EnvProduction:
		bucketName := os.Getenv("FORAGD_SERVER_BUCKET")
		var err error
		thumbnailCache, err = gcs.Connect(context.Background(), bucketName, "subscription_images")
		if err != nil {
			return nil, fmt.Errorf("connect to gcs: %w", err)
		}
	default:
		var err error
		thumbnailCache, err = newDirCache("subscription_images")
		if err != nil {
			return nil, fmt.Errorf("create dir cache: %w", err)
		}
	}

	return thumbnailCache, nil
})

var loadScreenshotCache = sync.OnceValues(func() (objectCache, error) {
	var screenshotCache objectCache
	switch config.CurrentEnvironment {
	case config.EnvProduction:
		bucketName := os.Getenv("FORAGD_SERVER_BUCKET")
		var err error
		screenshotCache, err = gcs.Connect(context.Background(), bucketName, "screenshots")
		if err != nil {
			return nil, fmt.Errorf("connect to gcs: %w", err)
		}
	default:
		var err error
		screenshotCache, err = newDirCache("screenshots")
		if err != nil {
			return nil, fmt.Errorf("create dir cache: %w", err)
		}
	}

	return screenshotCache, nil
})
