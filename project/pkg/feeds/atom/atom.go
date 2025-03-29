// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package atom

import (
	"bytes"
	"encoding/xml"
	"fmt"

	"golang.org/x/net/html/charset"

	"github.com/joshuar/go-feed-me/pkg/feeds/types"
)

// From generates an Atom object from the given byte array.
func From(b []byte) (*Feed, error) {
	var feed Feed

	reader := bytes.NewReader(b)
	decoder := xml.NewDecoder(reader)
	decoder.CharsetReader = charset.NewReaderLabel
	err := decoder.Decode(&feed)
	if err != nil {
		return nil, fmt.Errorf("could not decode byte array to RSS: %w", err)
	}

	return &feed, nil
}

func (a *Feed) Metadata() (*FeedMetadata, error) {
	data, err := types.Encode(a)
	if err != nil {
		return nil, err
	}
	return types.Decode[*FeedMetadata](data)
}
