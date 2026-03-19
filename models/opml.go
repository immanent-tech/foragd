// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/immanent-tech/go-syndication/opml"
	"github.com/immanent-tech/go-syndication/types"

	"github.com/immanent-tech/foragd/validation"
)

// ErrInvalidMimeType indicates that the mime type is not valid.
var ErrInvalidMimeType = errors.New("invalid mime type")

// OPMLFile is an opml file used for importing/exporting subscriptions.
type OPMLFile struct {
	*FileUpload
}

// Valid returns a boolean indicating if the OPML file is valid. If not valid, a non-nil error is also returned which
// will contain details about validation failures.
func (f *OPMLFile) Valid() (bool, error) {
	mediaType, err := f.ParseMimetype()
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrInvalidMimeType, err)
	}
	if mediaType != types.MimeTypeOPML && mediaType != "application/octet-stream" {
		return false, fmt.Errorf("%w: got %s, want "+types.MimeTypeOPML, ErrInvalidMimeType, mediaType)
	}
	return true, nil
}

// GenerateRequests extracts the feed outlines from the OPML file and returns a slice of subscription requests.
func (f *OPMLFile) GenerateRequests() ([]NewFeedSubscriptionRequest, error) {
	importfile, err := f.parse()
	if err != nil {
		return nil, fmt.Errorf("could not generate requests from opml file: %w", err)
	}
	requests := GenerateRequestsFromOutlines(importfile.Body...)
	return requests, nil
}

func (f *OPMLFile) parse() (*opml.OPML, error) {
	// Read the OPML file data into a byte array.
	data, err := io.ReadAll(f.Data)
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

func GenerateRequestsFromOutlines(outlines ...opml.Outline) []NewFeedSubscriptionRequest {
	requests := make([]NewFeedSubscriptionRequest, 0, len(outlines))
	for outline := range slices.Values(outlines) {
		if outline.Type == "rss" {
			requests = append(requests, NewFeedSubscriptionRequest{URL: outline.XMLURL})
		}
		if len(outline.Outlines) > 0 {
			requests = append(requests, GenerateRequestsFromOutlines(outline.Outlines...)...)
		}
	}
	return requests
}

// Valid returns a boolean indicating whether the SubscriptionRequest is valid,
// and any validation errors if applicable.
func (r *AddFeedsetRequest) Valid() error {
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("add feedset validation error: %w", err)
	}
	return nil
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
