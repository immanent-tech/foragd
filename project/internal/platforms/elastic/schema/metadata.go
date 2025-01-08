// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package schema

import (
	"encoding/json"
	"strconv"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

// WithMetadataField inserts the given field and value as metadata.
func WithMetadataField(name, value string) Option[types.Metadata] {
	return func(metadata types.Metadata) types.Metadata {
		metadata[name] = json.RawMessage(strconv.Quote(value))
		return metadata
	}
}

// NewMetadata creates a new metadata object with the given options.
func NewMetadata(options ...Option[types.Metadata]) types.Metadata {
	metadata := types.Metadata{}

	for _, option := range options {
		metadata = option(metadata)
	}

	return metadata
}
