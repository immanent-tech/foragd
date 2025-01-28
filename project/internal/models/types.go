// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"errors"

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

func ReadItemIDs(items []ReadItem) []ItemID {
	itemIDs := make([]ItemID, len(items))

	for i, item := range items {
		itemIDs[i] = item.ItemID
	}

	return itemIDs
}

// func (p *ShowUnread) MarshalJSON() ([]byte, error) {
// 	return json.Marshal(string(*p))
// }

// func (p *ShowUnread) UnmarshalJSON(data []byte) error {
// 	spew.Dump("unmarshal", *p)
// 	return nil
// }
