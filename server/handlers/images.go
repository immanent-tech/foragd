// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/spaolacci/murmur3"
	slogctx "github.com/veqryn/slog-context"
	"golang.org/x/sync/errgroup"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/providers/google/gcs"
)

func ImageProxy(proxyURLBase string) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Cache-Control", "public, max-age=31536000, s-maxage=31536000, immutable")

		// Load the http client used for making requests to the image proxy.
		if err := loadHTTPClient(); err != nil {
			res.WriteHeader(http.StatusInternalServerError)
			slogctx.FromCtx(req.Context()).Error("Load http client failed.",
				slog.Any("error", err),
			)
			return
		}
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
			murmur3.Sum64([]byte(strings.Join(params[1:len(params)-1], "|")+params[len(params)-1])),
			10,
		)

		imgBufPtr, ok := imgBufPool.Get().(*[]byte)
		if !ok {
			res.WriteHeader(http.StatusInternalServerError)
			slogctx.FromCtx(req.Context()).Error("Get image buffer failed.")
			return
		}
		imgBuf := *imgBufPtr
		defer imgBufPool.Put(imgBufPtr)
		var found bool

		// Try to fetch the image from the cache. If found, return the cached image.
		imgBuf, found = imgCache.Get(req.Context(), imgHash)
		if found {
			// Write the image to the response.
			if _, err := res.Write(imgBuf); err != nil {
				res.WriteHeader(http.StatusInternalServerError)
				slogctx.FromCtx(req.Context()).Error("Write image data.",
					slog.Any("error", err),
				)
				return
			}
			// Return success.
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

		// Fetch the image (either from proxy or direct).
		resp, err := httpClient.R().
			SetDoNotParseResponse(true).
			Get(proxiedURL)
		if err != nil {
			res.WriteHeader(http.StatusInternalServerError)
			slogctx.FromCtx(req.Context()).Error("Get image from proxy failed.",
				slog.Any("error", err),
			)
			return
		}
		if resp.IsError() {
			res.WriteHeader(resp.StatusCode())
			slogctx.FromCtx(req.Context()).Error("Image proxy returned error status code.",
				slog.Any("error", resp.Status()),
			)
			return
		}

		imgBuf, err = io.ReadAll(resp.RawBody())
		if err != nil {
			res.WriteHeader(http.StatusInternalServerError)
			slogctx.FromCtx(req.Context()).Error("Read image proxy response failed.",
				slog.Any("error", err),
			)
			return
		}

		wg, jobCtx := errgroup.WithContext(req.Context())
		defer jobCtx.Done()
		// Write the image to the response.
		wg.Go(func() error {
			_, err = res.Write(imgBuf)
			if err != nil {
				res.WriteHeader(http.StatusInternalServerError)
				return fmt.Errorf("write image: %w", err)
			}
			res.WriteHeader(http.StatusOK)
			return nil
		})
		// Save to the cache if not saved already.
		wg.Go(func() error {
			if !found {
				imgCache.Set(jobCtx, imgHash, imgBuf)
			}
			return nil
		})

		if err := wg.Wait(); err != nil {
			slogctx.FromCtx(req.Context()).Error("Run background image jobs failed.",
				slog.Any("error", err),
			)
			return
		}

		res.WriteHeader(http.StatusOK)
	}
}

// Avatar handles fetching and displaying custom avatars for users.
func Avatar() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Load the image cache.
		if err := loadAvatarCache(); err != nil {
			res.WriteHeader(http.StatusInternalServerError)
			slogctx.FromCtx(req.Context()).Error("Load avatar cache failed.",
				slog.Any("error", err),
			)
			return
		}

		key := chi.URLParam(req, "*")

		imgBufPtr, ok := imgBufPool.Get().(*[]byte)
		if !ok {
			res.WriteHeader(http.StatusInternalServerError)
			slogctx.FromCtx(req.Context()).Error("Get image buffer failed.")
			return
		}
		imgBuf := *imgBufPtr
		defer imgBufPool.Put(imgBufPtr)
		var found bool

		// Try to fetch the image from the cache. If found, return the cached image.
		imgBuf, found = avatarCache.Get(req.Context(), key)
		if found {
			// Write the image to the response.
			if _, err := res.Write(imgBuf); err != nil {
				res.WriteHeader(http.StatusInternalServerError)
				slogctx.FromCtx(req.Context()).Error("Write image data.",
					slog.Any("error", err),
				)
				return
			}
			// Return success.
			res.WriteHeader(http.StatusOK)
			return
		} else {
			http.NotFound(res, req)
		}
	}
}

// SubscriptionImage handles fetching and displaying a custom subscription image thumbnail for users.
func SubscriptionImage() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Load the image cache.
		if err := loadSubscriptionImgCache(); err != nil {
			res.WriteHeader(http.StatusInternalServerError)
			slogctx.FromCtx(req.Context()).Error("Load subscription image cache failed.",
				slog.Any("error", err),
			)
			return
		}

		key := chi.URLParam(req, "*")

		imgBufPtr, ok := imgBufPool.Get().(*[]byte)
		if !ok {
			res.WriteHeader(http.StatusInternalServerError)
			slogctx.FromCtx(req.Context()).Error("Get image buffer failed.")
			return
		}
		imgBuf := *imgBufPtr
		defer imgBufPool.Put(imgBufPtr)
		var found bool

		// Try to fetch the image from the cache. If found, return the cached image.
		imgBuf, found = avatarCache.Get(req.Context(), key)
		if found {
			// Write the image to the response.
			if _, err := res.Write(imgBuf); err != nil {
				res.WriteHeader(http.StatusInternalServerError)
				slogctx.FromCtx(req.Context()).Error("Write image data.",
					slog.Any("error", err),
				)
				return
			}
			// Return success.
			res.WriteHeader(http.StatusOK)
			return
		} else {
			http.NotFound(res, req)
		}
	}
}

var imgBufPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 8388608) // TODO: fetch from IMGPROXY_MAX_SRC_FILE_SIZE env var.
		return &buf
	},
}

var imgCache objectCache

var loadImageCache = sync.OnceValue(func() error {
	switch config.Environment {
	case "production":
		bucketName := os.Getenv("FORAGD_IMAGEPROXY_BUCKET")
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
	switch config.Environment {
	case "production":
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

var subscriptionImgCache objectCache

var loadSubscriptionImgCache = sync.OnceValue(func() error {
	switch config.Environment {
	case "production":
		bucketName := os.Getenv("FORAGD_SERVER_BUCKET")
		var err error
		subscriptionImgCache, err = gcs.Connect(context.Background(), bucketName, "subscription_images")
		if err != nil {
			return fmt.Errorf("connect to gcs: %w", err)
		}
	default:
		var err error
		subscriptionImgCache, err = newDirCache("subscription_images")
		if err != nil {
			return fmt.Errorf("create dir cache: %w", err)
		}
	}

	return nil
})
