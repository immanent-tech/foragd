// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/immanent-tech/go-base/validation"
	"github.com/immanent-tech/go-syndication/opml"
)

// ErrInvalidMimeType indicates that the mime type is not valid.
var ErrInvalidMimeType = errors.New("invalid mime type")

// OPMLFile is an opml file used for importing/exporting subscriptions.
type OPMLFile struct {
	*FileUpload
}

func NewOPMLFile(file *FileUpload) *OPMLFile {
	return &OPMLFile{FileUpload: file}
}

// Valid returns a boolean indicating if the OPML file is valid. If not valid, a non-nil error is also returned which
// will contain details about validation failures.
func (f *OPMLFile) Validate() (bool, error) {
	mediaType, err := f.ParseMimetype()
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrInvalidMimeType, err)
	}
	if mediaType != opml.MimeType && mediaType != "application/octet-stream" {
		return false, fmt.Errorf("%w: got %s, want "+opml.MimeType, ErrInvalidMimeType, mediaType)
	}
	return true, nil
}

// GenerateRequests extracts the feed outlines from the OPML file and returns a slice of subscription requests.
func (f *OPMLFile) GenerateRequests() ([]ImportRequest, error) {
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

func GenerateRequestsFromOutlines(outlines ...opml.Outline) []ImportRequest {
	requests := make([]ImportRequest, 0)
	for feed := range slices.Values(flatten(outlines, nil)) {
		request := ImportRequest{
			Categories: feed.Categories,
			URL:        feed.XMLURL,
		}
		requests = append(requests, request)
	}
	return requests
}

type OPMLFeedEntry struct {
	Title      string
	XMLURL     string
	HTMLURL    string
	Categories []string // ancestor folder names, outermost first
}

// flatten walks the tree, carrying the ancestor folder names down.
func flatten(outlines []opml.Outline, parents []string) []OPMLFeedEntry {
	var feeds []OPMLFeedEntry
	for o := range slices.Values(outlines) {
		var name string
		if title := o.GetAttr("title"); title == "" {
			name = title
		} else {
			name = o.Text
		}

		if xmlURL := o.GetAttr("xmlUrl"); xmlURL != "" { // it's a feed
			htmlURL := o.GetAttr("htmlUrl")
			// copy so sibling feeds never share a backing array
			cats := append([]string(nil), parents...)
			if len(o.Category) > 0 {
				cats = append(cats, o.Category...)
			}
			feeds = append(feeds, OPMLFeedEntry{
				Title:      name,
				XMLURL:     xmlURL,
				HTMLURL:    htmlURL,
				Categories: cats,
			})
		}

		if len(o.Outlines) > 0 { // it's a folder: recurse with it added
			next := append(append([]string(nil), parents...), name)
			feeds = append(feeds, flatten(o.Outlines, next)...)
		}
	}
	return feeds
}

// Valid returns a boolean indicating whether the SubscriptionRequest is valid,
// and any validation errors if applicable.
func (r *AddFeedsetRequest) Validate() error {
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
