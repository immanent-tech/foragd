// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"slices"

	"github.com/immanent-tech/go-syndication/opml"
	"github.com/immanent-tech/go-syndication/types"

	"github.com/immanent-tech/foragd/validation"
)

// OPMLFile represents an OPML file.
type OPMLFile struct {
	data multipart.File
	hdr  *multipart.FileHeader
}

// Load will load the OPMLFile object with the data representing an OPML file contained in the given multipart form values.
func (f *OPMLFile) Load(data multipart.File, hdr *multipart.FileHeader) error {
	f.data = data
	f.hdr = hdr
	return nil
}

// Valid returns a boolean indicating if the OPML file is valid. If not valid, a non-nil error is also returned which
// will contain details about validation failures.
func (f *OPMLFile) Valid() (bool, error) {
	mediaType, _, err := mime.ParseMediaType(f.hdr.Header.Get("Content-Type"))
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrInvalidMimeType, err)
	}
	if mediaType != types.MimeTypeOPML && mediaType != "application/octet-stream" {
		return false, fmt.Errorf("%w: got %s, want "+types.MimeTypeOPML, ErrInvalidMimeType, mediaType)
	}
	return true, nil
}

// GenerateRequests extracts the feed outlines from the OPML file and returns a slice of subscription requests.
func (f *OPMLFile) GenerateRequests() ([]*SubscriptionRequest, error) {
	importfile, err := f.parse()
	if err != nil {
		return nil, fmt.Errorf("could not generate requests from opml file: %w", err)
	}
	requests := GenerateRequestsFromOutlines(importfile.Body...)
	return requests, nil
}

func (f *OPMLFile) parse() (*opml.OPML, error) {
	// Read the OPML file data into a byte array.
	data, err := io.ReadAll(f.data)
	if err != nil {
		return nil, fmt.Errorf("decode OPML file data failed: %w", err)
	}
	// Parse and create an OPML object from the byte array.
	opmlImport, err := opml.NewOPMLFromBytes(data)
	if err != nil {
		return nil, fmt.Errorf("decode OPML file data failed: %w", err)
	}

	return opmlImport, nil
}

func GenerateRequestsFromOutlines(outlines ...opml.Outline) []*SubscriptionRequest {
	requests := make([]*SubscriptionRequest, 0, len(outlines))
	for outline := range slices.Values(outlines) {
		if outline.Type == "rss" {
			requests = append(requests, &SubscriptionRequest{URL: outline.XMLURL})
		}
		if len(outline.Outlines) > 0 {
			requests = append(requests, GenerateRequestsFromOutlines(outline.Outlines...)...)
		}
	}
	return requests
}

// Valid returns a boolean indicating whether the SubscriptionRequest is valid,
// and any validation errors if applicable.
func (r *AddFeedsetRequest) Valid() (bool, error) {
	valid, err := validation.ValidateStruct(r)
	if err != nil {
		return false, fmt.Errorf("add feedset validation error: %w", err)
	}
	if !valid {
		return false, nil
	}
	return true, nil
}

// Sanitise will sanitise the input values of the SubscriptionRequest.
func (r *AddFeedsetRequest) Sanitise() error {
	if r == nil {
		return nil
	}
	sets := make([]string, 0, len(r.Feedset))
	for set := range slices.Values(r.Feedset) {
		set = validation.SanitizeString(set)
		sets = append(sets, set)
	}
	r.Feedset = sets
	return nil
}
