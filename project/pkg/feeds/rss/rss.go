// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package rss

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"

	"golang.org/x/net/html/charset"
)

// New generates an RSS object from the given byte array.
func New(b []byte) (*RSS, error) {
	var feed RSS

	reader := bytes.NewReader(b)
	decoder := xml.NewDecoder(reader)
	decoder.CharsetReader = charset.NewReaderLabel
	err := decoder.Decode(&feed)
	if err != nil {
		return nil, fmt.Errorf("could not decode byte array to RSS: %w", err)
	}

	return &feed, nil
}

func (r *RSS) Metadata() (*ChannelMetadata, error) {
	data, err := json.Marshal(r.Channel)
	if err != nil {
		return nil, err
	}
	var metadata ChannelMetadata
	err = json.Unmarshal(data, &metadata)
	if err != nil {
		return nil, err
	}
	return &metadata, nil
}
