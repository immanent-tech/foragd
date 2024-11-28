// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"errors"

	"github.com/mmcdole/gofeed"
)

var ErrInvalidID = errors.New("error generating unique ID")

type Feed struct {
	*gofeed.Feed
	ID string `json:"feed_id"`
}

type Item struct {
	*gofeed.Item
	ID     string `json:"item_id"`
	FeedID string `json:"feed_id"`
}

type UserSession struct {
	*Tokens
	*User
}
