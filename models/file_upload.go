// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"errors"
	"fmt"
	"mime"

	"github.com/immanent-tech/foragd/validation"
)

// ErrFileTooLarge can be used to indicate a file upload had a size greater than a set limit.
var ErrFileTooLarge = errors.New("file is too large")

// Valid will return a non-nill error containing details of any validation issues with the file upload data. Otherwise,
// a nil error is returned if the data is valid.
func (f *FileUpload) Valid() error {
	if f.Header == nil || f.Data == nil {
		return errors.New("empty file upload")
	}
	if err := validation.Validate.Struct(f); err != nil {
		return fmt.Errorf("invalid file upload: %w", err)
	}
	return nil
}

// GetSize returns the file size.
func (f *FileUpload) GetSize() int64 {
	return f.Header.Size
}

// ParseMimetype attempts to parse and return the mimetype of the file from its mime header.
func (f *FileUpload) ParseMimetype() (string, error) {
	mediaType, _, err := mime.ParseMediaType(f.Header.Header.Get("Content-Type"))
	if err != nil {
		return "unknown", fmt.Errorf("cannot parse mime type of file %s: %w", f.Header.Filename, err)
	}
	return mediaType, nil
}
