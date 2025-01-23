// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/mmcdole/gofeed"
)

var ErrInvalidID = errors.New("error generating unique ID")

// Feed represents a feed. It embeds the gofeed.Feed object and adds additional
// fields required.
type Feed struct {
	CreatedAt CreatedAt `json:"created_at" validate:"required"`
	*gofeed.Feed
	ID FeedID `json:"feed_id" validate:"required"`
}

// Item represents an item of a feed. It embeds the gofeed.Item object and adds additional
// fields required.
type Item struct {
	CreatedAt CreatedAt `json:"@timestamp" validate:"required"`
	*gofeed.Item
	ID     ItemID `json:"item_id"`
	FeedID FeedID `json:"feed_id"`
}

type UserData struct {
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

func ReadItemIDs(items []ReadItem) []ItemID {
	itemIDs := make([]ItemID, len(items))

	for i, item := range items {
		itemIDs[i] = item.ItemID
	}

	return itemIDs
}
