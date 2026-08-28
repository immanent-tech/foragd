// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package zyte

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

// GetHTML gets the article content as HTML.
func (a *Article) GetHTML() string {
	if a.ArticleBodyHtml != nil {
		return *a.ArticleBodyHtml
	}
	return ""
}

// GetText gets the article content as text.
func (a *Article) GetText() string {
	if a.ArticleBody != nil {
		return *a.ArticleBody
	}
	return ""
}

func (a *Article) GetContent() string {
	if a.GetHTML() != "" {
		return a.GetHTML()
	}
	if a.GetText() != "" {
		return a.GetText()
	}
	return ""
}

var dateOnlyLayouts = []string{
	"2006-01-02T15:04:05+00:00",
	"2006-01-02T15:04:05+0000",
	"2006-01-02T15:04:05",
	"2006-01-02T15:04:05.999999",
	time.RFC3339,
	time.RFC3339Nano,
}

func ParseTimestamp(ts string) (time.Time, error) {
	var lastErr error
	for layout := range slices.Values(dateOnlyLayouts) {
		if t, err := time.Parse(layout, ts); err == nil {
			return t, nil
		} else {
			lastErr = err
		}
	}
	return time.Time{}, fmt.Errorf("zyte timestamp: could not parse %q: %w", ts, lastErr)
}

func (a *Article) GetPublishedDate() (time.Time, error) {
	if a.DatePublished != nil {
		return ParseTimestamp(*a.DatePublished)
	}
	return time.Time{}, errors.New("no datePublished timestamp")
}

func (a *Article) GetUpdatedDate() (time.Time, error) {
	if a.DateModified != nil {
		return ParseTimestamp(*a.DateModified)
	}
	return time.Time{}, errors.New("no dateModified timestamp")
}
