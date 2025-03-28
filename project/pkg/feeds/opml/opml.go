// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package opml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"slices"

	"golang.org/x/net/html/charset"
)

// New generates an OPML object from the given byte array.
func New(b []byte) (*OPML, error) {
	var root OPML

	reader := bytes.NewReader(b)
	decoder := xml.NewDecoder(reader)
	decoder.CharsetReader = charset.NewReaderLabel
	err := decoder.Decode(&root)
	if err != nil {
		return nil, fmt.Errorf("could not decode byte array to OPML: %w", err)
	}

	return &root, nil
}

// ExtractRSS extracts all RSS outlines from the OPML.
func (o *OPML) ExtractRSS() []Outline {
	return extractRSSOutlines(o.Body...)
}

// extractRSSOutlines will recursively collect all outlines that are a single
// RSS feed into a slice.
func extractRSSOutlines(outlines ...Outline) []Outline {
	var requests []Outline

	for outline := range slices.Values(outlines) {
		switch {
		case outline.isFeed():
			requests = append(requests, outline)
		case outline.isGroup():
			requests = append(requests, extractRSSOutlines(outline.Outlines...)...)
		}
	}

	return requests
}

// isFeed returns a boolean indicating whether this outline represents an RSS
// feed.
func (r *Outline) isFeed() bool {
	if r.Type == nil {
		return false
	}
	return *r.Type == "rss"
}

// isGroup returns a boolean indicating whether this outline represents a group
// of RSS feeds (i.e., has children outlines).
func (r *Outline) isGroup() bool {
	return len(r.Outlines) > 0
}
