// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package atom

import (
	"bytes"
	"encoding/xml"
	"fmt"

	"golang.org/x/net/html/charset"
)

// New generates an Atom object from the given byte array.
func New(b []byte) (*Feed, error) {
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
