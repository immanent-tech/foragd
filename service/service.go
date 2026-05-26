// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package service

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"

	"github.com/immanent-tech/go-syndication/opengraph"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/client"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/pkg/formats/html"
)

// ExtractMainImage will attempt to extract a URL to what is likely the "main" image of a page (i.e., typically used on
// article/post pages).
func ExtractMainImage(ctx context.Context, rawURL string) (string, error) {
	sourceURL, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse URL %s: %w", rawURL, err)
	}

	// Create a buffer for the feed data.
	pageBuf, ok := bufPool.Get().(*bytes.Buffer)
	if !ok {
		return "", errors.New("get buffer failed")
	}
	pageBuf.Reset()
	defer bufPool.Put(pageBuf)

	resp, err := client.Load().R().
		SetContext(ctx).
		SetDoNotParseResponse(true).
		// SetDebug(true).
		Get(sourceURL.String())
	defer resp.RawBody().Close()
	if err != nil || resp.IsError() {
		return "", models.NewAPIError(resp.StatusCode(), err)
	}
	if resp.Header().Get("Content-Encoding") == "gzip" {
		// For gzipped response, uncompress first.
		reader, err := gzip.NewReader(resp.RawBody())
		if err != nil {
			return "", fmt.Errorf("read gzip response: %w", err)
		}
		defer reader.Close()
		const maxBodySize = 10 * 1024 * 1024 // 10 MB limit
		limitReader := io.LimitReader(reader, maxBodySize)
		if _, err := io.Copy(pageBuf, limitReader); err != nil {
			return "", models.NewAPIError(http.StatusUnprocessableEntity, fmt.Errorf("read response: %w", err))
		}
	} else {
		// Read response directly.
		if _, err := io.Copy(pageBuf, resp.RawBody()); err != nil {
			return "", models.NewAPIError(http.StatusUnprocessableEntity, fmt.Errorf("read response: %w", err))
		}
	}

	var foundURL string

	// Try to parse opengraph data out of the page content.
	if og, err := opengraph.ParseBytes(pageBuf.Bytes()); err != nil {
		slogctx.FromCtx(ctx).Debug("Could not parse opengraph data for URL.",
			slog.String("url", rawURL),
			slog.Any("error", err))
	} else {
		foundURL = og.Image
	}

	// Try to find the "main" image in the page content.
	if foundURL == "" {
		foundURL, _ = html.FindMainImage(pageBuf.Bytes(), rawURL)
	}

	// Parse the found URL.
	imgURL, err := url.Parse(foundURL)
	if err != nil {
		return foundURL, models.NewAPIError(
			http.StatusUnprocessableEntity,
			fmt.Errorf("parse image URL %q: %w", foundURL, err),
		)
	}

	// Check it points to an actual image.
	if !slices.ContainsFunc(models.ImageExtensions, func(ext string) bool {
		return strings.HasSuffix(imgURL.Path, ext)
	}) {
		return "", models.NewAPIError(http.StatusNotAcceptable, errors.New("invalid image extension"))
	}

	// If it is not an absolute URL, resolve it relative to the page URL.
	if !imgURL.IsAbs() {
		return sourceURL.ResolveReference(imgURL).String(), nil
	}

	return imgURL.String(), nil
}

// ExtractFavicon will attempt to extract a URL to what is likely the favicon of a page.
func ExtractFavicon(ctx context.Context, rawURL string) (string, error) {
	sourceURL, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse URL %s: %w", rawURL, err)
	}

	// Create a buffer for the feed data.
	pageBuf, ok := bufPool.Get().(*bytes.Buffer)
	if !ok {
		return "", errors.New("get buffer failed")
	}
	pageBuf.Reset()
	defer bufPool.Put(pageBuf)

	resp, err := client.Load().R().
		SetContext(ctx).
		SetDoNotParseResponse(true).
		// SetDebug(true).
		Get(sourceURL.String())
	defer resp.RawBody().Close()
	if err != nil || resp.IsError() {
		return "", models.NewAPIError(resp.StatusCode(), err)
	}
	if resp.Header().Get("Content-Encoding") == "gzip" {
		// For gzipped response, uncompress first.
		reader, err := gzip.NewReader(resp.RawBody())
		if err != nil {
			return "", fmt.Errorf("read gzip response: %w", err)
		}
		defer reader.Close()
		const maxBodySize = 10 * 1024 * 1024 // 10 MB limit
		limitReader := io.LimitReader(reader, maxBodySize)
		if _, err := io.Copy(pageBuf, limitReader); err != nil {
			return "", models.NewAPIError(http.StatusUnprocessableEntity, fmt.Errorf("read response: %w", err))
		}
	} else {
		// Read response directly.
		if _, err := io.Copy(pageBuf, resp.RawBody()); err != nil {
			return "", models.NewAPIError(http.StatusUnprocessableEntity, fmt.Errorf("read response: %w", err))
		}
	}

	_, faviconURL, _, err := html.FindFavicon(pageBuf.Bytes(), rawURL)
	if err != nil {
		return "", fmt.Errorf("find favicon: %w", err)
	}

	// Parse the found URL.
	imgURL, err := url.Parse(faviconURL)
	if err != nil {
		return faviconURL, fmt.Errorf("parse favicon URL %q: %w", faviconURL, err)
	}

	// If it is not an absolute URL, resolve it relative to the page URL.
	if !imgURL.IsAbs() {
		sourceURL, _ := url.Parse(rawURL)
		return sourceURL.ResolveReference(imgURL).String(), nil
	}

	return imgURL.String(), nil
}

var bufPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}
