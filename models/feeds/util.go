// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package feeds

import (
	"bytes"
	"encoding/xml"
	"fmt"

	"golang.org/x/net/html/charset"
)

// Decode will decode the byte array into the given type T, and assign values without a namespace with the given
// namespace.
func Decode[T any](namespace string, b []byte) (T, error) {
	var feed T

	reader := bytes.NewReader(b)
	decoder := xml.NewDecoder(reader)
	decoder.DefaultSpace = namespace
	decoder.CharsetReader = charset.NewReaderLabel
	err := decoder.Decode(&feed)
	if err != nil {
		return feed, fmt.Errorf("could not decode byte array: %w", err)
	}

	return feed, nil
}

// Encode will encode the given type T into a byte array.
func Encode[T any](feed T) ([]byte, error) {
	var b []byte

	reader := bytes.NewBuffer(b)
	encoder := xml.NewEncoder(reader)
	err := encoder.Encode(&feed)
	if err != nil {
		return nil, fmt.Errorf("could not encode byte array: %w", err)
	}

	return reader.Bytes(), nil
}
