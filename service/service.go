// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package service

import (
	"bytes"
	"context"
	"log/slog"
	"net/url"
	"sync"

	"codeberg.org/readeck/go-readability/v2"
	"github.com/immanent-tech/go-base/pkg/htmlx"
	slogctx "github.com/veqryn/slog-context"
)

var bufPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

func extractMetadataFromHTML(
	ctx context.Context,
	sourceURL *url.URL,
	source []byte,
) (*htmlx.OpenGraph, *readability.Article) {
	// Extract any Opengraph data.
	opengraphData, err := htmlx.DecodeOpengraph(bytes.NewReader(source))
	if err != nil {
		slogctx.Debug(ctx, "Unable to extract opengraph data for item.",
			slog.Any("error", err),
		)
	}
	// Extract readability data.
	readabilityData, err := readability.FromReader(bytes.NewReader(source), sourceURL)
	if err != nil {
		slogctx.Debug(ctx, "Unable to extract readability data for item.",
			slog.Any("error", err),
		)
	}

	return opengraphData, &readabilityData
}
