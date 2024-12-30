// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
)

var ErrInvalidID = errors.New("error generating unique ID")

// Feed represents a feed. It embeds the gofeed.Feed object and adds additional
// fields required.
type Feed struct {
	CreatedAt time.Time `json:"@timestamp" validate:"required"`
	*gofeed.Feed
	ID string `json:"feed_id" validate:"required"`
}

func (f *Feed) DocID() string {
	return f.ID
}

// Item represents an item of a feed. It embeds the gofeed.Item object and adds additional
// fields required.
type Item struct {
	CreatedAt time.Time `json:"@timestamp" validate:"required"`
	*gofeed.Item
	ID     string `json:"item_id"`
	FeedID string `json:"feed_id"`
}

func (i *Item) DocID() string {
	return i.ID
}

type UserSession struct {
	*Tokens
	*User
}

// GenerateURL generates a new URL using the basePath provided with any non-zero
// filters.
func (f APISearchFilters) GenerateURL(basePath string) (*url.URL, error) {
	newURL, err := url.Parse(basePath)
	if err != nil {
		return nil, fmt.Errorf("cannot generate URL: %w", err)
	}

	params := newURL.Query()

	if len(f.FeedIDs) > 0 {
		params.Add("feeds", strings.Join(f.FeedIDs, ","))
	}

	if len(f.ItemIDs) > 0 {
		params.Add("items", strings.Join(f.ItemIDs, ","))
	}

	if len(f.Categories) > 0 {
		params.Add("categories", strings.Join(f.Categories, ","))
	}

	if f.Pagination != nil {
		params.Add("pagination", string(f.Pagination))
	}

	newURL.RawQuery = params.Encode()

	return newURL, nil
}
