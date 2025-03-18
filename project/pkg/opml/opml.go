// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package opml

import (
	"bytes"
	"encoding/xml"
	"fmt"

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
func (o *OPML) ExtractRSS() []RSSOutline {
	return extractRSSOutlines(o.Body...)
}

// extractRSSOutlines will recursively collect all outlines that are a single
// RSS feed into a slice.
func extractRSSOutlines(outlines ...RSSOutline) []RSSOutline {
	var requests []RSSOutline

	for _, outline := range outlines {
		switch {
		case outline.isFeed():
			requests = append(requests, outline)
		case outline.isGroup():
			requests = append(requests, extractRSSOutlines(outline.Outline...)...)
		}
	}

	return requests
}

// isFeed returns a boolean indicating whether this outline represents an RSS
// feed.
func (r *RSSOutline) isFeed() bool {
	return r.Type == "rss"
}

// isGroup returns a boolean indicating whether this outline represents a group
// of RSS feeds (i.e., has children outlines).
func (r *RSSOutline) isGroup() bool {
	return r.Type != "rss" && len(r.Outline) > 0
}
