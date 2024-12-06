// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
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

func (f *ItemsFilters) Valid(_ context.Context) (bool, ValidationErrors) {
	return true, nil
}
