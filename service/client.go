/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

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
	"strconv"

	"github.com/go-resty/resty/v2"
	"github.com/immanent-tech/go-base/pkg/zyte"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
)

func DirectFetch(ctx context.Context, httpClient *resty.Client, sourceURL *url.URL, buf *bytes.Buffer) error {
	resp, err := httpClient.R().
		SetContext(ctx).
		SetDoNotParseResponse(true).
		// SetDebug(true).
		Get(sourceURL.String())
	switch {
	case err != nil:
		return models.NewAPIError(
			http.StatusInternalServerError,
			fmt.Errorf("fetch failed: %q: %w", sourceURL.String(), err),
			models.WithUserMessage(models.NewErrorMessage("Could not fetch feed", sourceURL.String())),
		)
	case resp.IsError():
		if resp.StatusCode() == http.StatusForbidden || resp.StatusCode() == http.StatusTooManyRequests {
			slogctx.FromCtx(ctx).Debug("Potentially blocked. Retrying request through proxy.",
				slog.String("feed_url", sourceURL.String()),
			)
			return ProxyFetch(ctx, sourceURL, buf)
		}
		return models.NewAPIError(
			resp.StatusCode(),
			errors.New(resp.Status()),
			models.WithUserMessage(
				models.NewErrorMessage("Could not fetch feed from URL: "+resp.Status(), sourceURL.String()),
			),
		)
	}
	defer resp.RawBody().Close()

	if resp.Header().Get("Content-Encoding") == "gzip" {
		// For gzipped response, uncompress first.
		reader, err := gzip.NewReader(resp.RawBody())
		if err != nil {
			return models.NewAPIError(
				http.StatusUnprocessableEntity,
				fmt.Errorf("read gzip response: %w", err),
				models.WithUserMessage(models.NewErrorMessage(
					"Feed URL returned invalid or malformed data",
					sourceURL.String(),
				)),
			)
		}
		defer reader.Close()
		const maxBodySize = 10 * 1024 * 1024 // 10 MB limit
		limitReader := io.LimitReader(reader, maxBodySize)
		if _, err := io.Copy(buf, limitReader); err != nil {
			return models.NewAPIError(
				http.StatusUnprocessableEntity,
				fmt.Errorf("read response: %w", err),
				models.WithUserMessage(models.NewErrorMessage(
					"Feed URL returned invalid or malformed data",
					sourceURL.String(),
				)),
			)
		}
	} else {
		// Read response directly.
		if _, err := io.Copy(buf, resp.RawBody()); err != nil {
			return models.NewAPIError(
				http.StatusUnprocessableEntity,
				fmt.Errorf("read response: %w", err),
				models.WithUserMessage(models.NewErrorMessage(
					"Feed URL returned invalid or malformed data",
					sourceURL.String(),
				)),
			)
		}
	}
	return nil
}

func ProxyFetch(ctx context.Context, sourceURL *url.URL, buf *bytes.Buffer) error {
	resp, err := zyte.Proxy(
		ctx,
		sourceURL.String(),
		zyte.WithResponseBody(true),
		zyte.WithFollowRedirects(true),
	)
	if err != nil {
		if zyteErr, isZyteErr := errors.AsType[*zyte.ResponseError](err); isZyteErr {
			return models.NewAPIError(
				zyteErr.HTTPStatus(),
				fmt.Errorf("proxy request: %w", zyteErr),
				models.WithUserMessage(models.NewErrorMessage(
					"Could not fetch feed from URL: "+strconv.Itoa(zyteErr.HTTPStatus()),
					sourceURL.String(),
				)),
			)
		}
		return models.NewAPIError(
			http.StatusInternalServerError,
			fmt.Errorf("proxy request: %w", err),
			models.WithUserMessage(models.NewErrorMessage(
				"Could not fetch feed from URL",
				sourceURL.String(),
			)),
		)
	}
	body, err := resp.GetHTMLResponse()
	if err != nil {
		return models.NewAPIError(
			http.StatusUnprocessableEntity,
			fmt.Errorf("get response body: %w", err),
			models.WithUserMessage(models.NewErrorMessage(
				"Feed URL returned invalid or malformed data",
				sourceURL.String(),
			)),
		)
	}
	if _, err := buf.Write(body); err != nil {
		return models.NewAPIError(
			http.StatusUnprocessableEntity, fmt.Errorf("read response: %w", err),
			models.WithUserMessage(models.NewErrorMessage(
				"Feed URL returned invalid or malformed data",
				sourceURL.String(),
			)),
		)
	}
	return nil
}
