// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"

	"github.com/joshuar/go-feed-me/pkg/opml"
)

type OPMLFile struct {
	data multipart.File
	hdr  *multipart.FileHeader
}

func (f *OPMLFile) Load(data multipart.File, hdr *multipart.FileHeader) error {
	f.data = data
	f.hdr = hdr
	return nil
}

func (f *OPMLFile) Parse() (*opml.OPML, error) {
	// Read the OPML file data into a byte array.
	data, err := io.ReadAll(f.data)
	if err != nil {
		return nil, fmt.Errorf("decode OPML file data failed: %w", err)
	}
	// Parse and create an OPML object from the byte array.
	opmlImport, err := opml.New(data)
	if err != nil {
		return nil, fmt.Errorf("decode OPML file data failed: %w", err)
	}

	return opmlImport, nil
}

func (f *OPMLFile) Valid() (bool, error) {
	mediaType, _, err := mime.ParseMediaType(f.hdr.Header.Get("Content-Type"))
	if err != nil {
		return false, errors.New("invalid media type")
	}
	if mediaType != "text/x-opml+xml" {
		return false, errors.New("invalid media type")
	}
	return true, nil
}
