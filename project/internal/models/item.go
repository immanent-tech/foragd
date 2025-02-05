// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"errors"
	"html"
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/joshuar/go-feed-me/internal/id"
)

var ErrGetItem = errors.New("could not retrieve item")

func (i *APIItem) GetTitle() string {
	return html.UnescapeString(safePrinter.Sanitize(i.Title))
}

func (i *APIItem) GetID() string {
	return i.ID
}

func (i *APIItem) GetFeedID() string {
	return i.FeedID
}

func (i *APIItem) GetLink() string {
	return i.ItemURL
}

func (i *APIItem) GetImage() *gofeed.Image {
	return i.Image
}

func (i *APIItem) GetCategories() []string {
	cleaned := make([]string, 0, len(i.Categories))

	for _, category := range i.Categories {
		cleaned = append(cleaned, CleanCategory(category))
	}

	return cleaned
}

func (i *APIItem) GetContent() string {
	return safePrinter.Sanitize(i.Description)
}

func (i *APIItem) GetTimestamp() time.Time {
	var itemTime time.Time

	if !i.Published.IsZero() {
		itemTime = i.Published
	} else {
		itemTime = i.Updated
	}

	return itemTime
}

func (i *APIItem) GetUserState() State {
	if i.UserProperties != nil {
		if i.UserProperties.State != nil {
			return *i.UserProperties.State
		}
	}

	return StateUnread
}

func (i *APIItem) SetUserItemState(state State) {
	if i.UserProperties == nil {
		i.UserProperties = &UserItemProperties{}
	}

	i.UserProperties.State = &state
}

// isNewer returns a boolean indicating whether this item has been updated or
// published after the given time.
func (i *Item) isNewer(since time.Time) bool {
	var itemTime time.Time

	if i.UpdatedParsed != nil {
		itemTime = *i.UpdatedParsed
	} else {
		itemTime = *i.PublishedParsed
	}

	return itemTime.After(since)
}

// NewFeedItem creates a new Feed object from the given item details, using the
// given feed ID.
func NewFeedItem(feedID string, details *gofeed.Item) (*Item, error) {
	var err error

	itemID, err := id.NewID(id.Item)
	if err != nil {
		return nil, errors.Join(ErrInvalidID, err)
	}

	return &Item{
			CreatedAt: time.Now().UTC(),
			ID:        itemID,
			FeedID:    feedID,
			Item:      details,
		},
		nil
}
