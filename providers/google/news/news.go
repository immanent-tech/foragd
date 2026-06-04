// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package news

import (
	"fmt"
	"net/url"
)

// GenerateRSSURL takes a query string and generates the Google News RSS URL for it.
func GenerateRSSURL(query string) (*url.URL, error) {
	feedURL, err := url.Parse("https://news.google.com/rss/search")
	if err != nil {
		return nil, fmt.Errorf("parse google feed URL: %w", err)
	}

	params := url.Values{}
	params.Set("q", query)
	params.Set("hl", "en-US")
	params.Set("gl", "US")
	params.Set("ceid", "US:en")

	feedURL.RawQuery = params.Encode()

	return feedURL, nil
}
