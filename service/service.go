// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package service

import (
	"bytes"
	"errors"
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
	sourceURL *url.URL,
	source []byte,
) (*htmlx.OpenGraph, *readability.Article, error) {
	var errs []error
	// Extract any Opengraph data.
	opengraphData, err := htmlx.DecodeOpengraph(bytes.NewReader(source))
	if err != nil {
		errs = append(errs, err)
	}
	// Extract readability data.
	readabilityData, err := readability.FromReader(bytes.NewReader(source), sourceURL)
	if err != nil {
		errs = append(errs, err)
	}

	return opengraphData, &readabilityData, errors.Join(errs...)
}
