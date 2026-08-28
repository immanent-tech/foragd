// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package service

import (
	"bytes"
	"context"
	"net/url"
	"sync"

	"codeberg.org/readeck/go-readability/v2"
	"github.com/immanent-tech/go-base/pkg/htmlx"
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
	opengraphData, _ := htmlx.DecodeOpengraph(bytes.NewReader(source))
	// Extract readability data.
	readabilityData, _ := readability.FromReader(bytes.NewReader(source), sourceURL)

	return opengraphData, &readabilityData
}
