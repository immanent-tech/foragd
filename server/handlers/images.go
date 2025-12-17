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
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/google/gcs"
)

var imgCache objectCache

func ImageProxy(proxyURLBase string) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
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

		imgBufPtr := imgBufPool.Get().(*[]byte)
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
			slogctx.FromCtx(req.Context()).Error("Image proxy returned error response.",
				slog.Any("error", err),
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

		res.Header().Set("Cache-Control", "public, max-age=604800, s-maxage=43200")

		var wg errgroup.Group
		// Write the image to the response.
		wg.Go(func() error {
			_, err = res.Write(imgBuf)
			if err != nil {
				res.WriteHeader(http.StatusInternalServerError)
				return models.NewAPIError(
					fmt.Errorf("write image: %w", err),
					http.StatusInternalServerError,
				)
			}
			res.WriteHeader(http.StatusOK)
			return nil
		})
		// Save to the cache if not saved already.
		wg.Go(func() error {
			if !found {
				imgCache.Set(req.Context(), imgHash, imgBuf)
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

var imgBufPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 8388608) // TODO: fetch from IMGPROXY_MAX_SRC_FILE_SIZE env var.
		return &buf
	},
}

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
		imgCache, err = newDirCache()
		if err != nil {
			return fmt.Errorf("create dir cache: %w", err)
		}
	}

	return nil
})
