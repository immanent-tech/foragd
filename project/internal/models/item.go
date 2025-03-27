// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"errors"
	"html"
	"maps"
	"slices"
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/joshuar/go-feed-me/internal/id"
)

var ErrGetItem = errors.New("could not retrieve item")

type Items []*APIItem

// GetCategoryCounts returns a count of the occurrence of a Category across all
// the Items.
func (i Items) GetCategoryCounts() CategoryCounts {
	countsMap := make(map[Category]int)
	for item := range slices.Values(i) {
		for category := range slices.Values(item.GetCategories()) {
			countsMap[category]++
		}
	}
	var counts CategoryCounts
	for category, count := range maps.All(countsMap) {
		counts = append(counts, CategoryCount{Category: category, Count: count})
	}

	return counts
}

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

// GetCategories retrieves a list of Categories assigned to the Item.
func (i *APIItem) GetCategories() []string {
	categories := slices.Clone(i.Categories)
	slices.Sort(categories)
	return categories
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

func (i *APIItem) HasState() bool {
	return i.State != nil
}

func (i *APIItem) GetUserState() State {
	if i.HasState() {
		return i.State.State
	}
	return StateUnread
}

func (i *APIItem) SetUserItemState(state State) {
	newState := &ItemState{
		State: state,
	}
	i.State = newState
}

// IsNewer returns a boolean indicating whether this item has been updated or
// published after the given time.
func (i *Item) IsNewer(since time.Time) bool {
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
	return &Item{
			CreatedAt: time.Now().UTC(),
			ID:        id.NewID(id.Item),
			FeedID:    feedID,
			Item:      details,
		},
		nil
}
