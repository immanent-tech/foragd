// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/joshuar/go-feed-me/models/feeds"
	"github.com/joshuar/go-feed-me/models/feeds/types"
)

var _ types.ObjectCommon = (*Item)(nil)

var ErrGetItem = errors.New("could not retrieve item")

type Items []*Item

// FilterSince filters the given slice of Items to ones which are newer than the given timestamp.
func (i Items) FilterSince(since time.Time) Items {
	return slices.Collect(FilterSlice(i, func(v *Item) bool {
		return v.IsNewer(since)
	}))
}

// GetFeedIDs retrieves a list of all FeedIDs for all items.
func (i Items) GetFeedIDs() []FeedID {
	feedIDs := make([]FeedID, 0, len(i))
	for item := range slices.Values(i) {
		feedIDs = append(feedIDs, item.GetFeedID())
	}
	return feedIDs
}

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

func (i *Item) GetID() ItemID {
	return i.ItemID
}

func (i *Item) GetFeedID() FeedID {
	return i.FeedID
}

func (i *Item) GetLink() URL {
	return i.URL
}

func (i *Item) GetTitle() string {
	return i.Title
}

func (i *Item) GetDescription() string {
	return i.Description
}

func (i *Item) GetAuthors() []string {
	return i.Authors
}

func (i *Item) GetContributors() []string {
	return i.Contributors
}

func (i *Item) GetCategories() []string {
	return i.Categories
}

func (i *Item) GetImage() *types.Image {
	return i.Image
}

func (i *Item) GetLanguage() string {
	return i.Language
}

func (i *Item) GetPublishedDate() time.Time {
	return i.Published
}

func (i *Item) GetUpdatedDate() time.Time {
	return i.Updated
}

func (i *Item) GetRights() string {
	return i.Copyright
}

func (i *Item) GetContent() string {
	return i.Content
}

func (i *Item) HasState() bool {
	return i.State != nil
}

func (i *Item) IsUnread() bool {
	if !i.HasState() {
		return true
	}
	return *i.State == StateUnread
}

func (i *Item) GetUserState() State {
	if i.HasState() {
		return *i.State
	}
	return StateUnread
}

func (i *Item) SetUserItemState(state State) {
	i.State = &state
}

func (i *Item) GetTimestamp() time.Time {
	if valid, _ := ValidateDatetime(i.GetUpdatedDate()); valid {
		return i.GetUpdatedDate()
	} else if valid, _ := ValidateDatetime(i.GetPublishedDate()); valid {
		return i.GetUpdatedDate()
	} else {
		return i.Timestamp
	}
}

// IsNewer returns a boolean indicating whether this item has been updated or
// published after the given time.
func (i *Item) IsNewer(since time.Time) bool {
	return i.GetTimestamp().After(since)
}

func (s ItemState) IsUnread() bool {
	return s.State == StateUnread
}

func (s ItemState) IsRead() bool {
	return s.State == StateRead
}

func GetFeedItems(ctx context.Context, id FeedID, url string) (Items, error) {
	var items Items

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	results := feeds.NewItemsFromURLs(ctx, url)
	for result := range slices.Values(results) {
		if result.Err != nil {
			return nil, fmt.Errorf("unable to fetch feed items: %w", result.Err)
		}
		for item := range slices.Values(result.Items) {
			items = append(items, newItemFromSource(item.ItemSource, id, string(item.SourceType)))
		}
	}
	return items, nil
}

// newFeedFromSource converts the raw types.FeedSource into a Feed object.
func newItemFromSource[T types.ItemSource](source T, feedID FeedID, sourceType string) *Item {
	item := &Item{
		ItemID:       NewID(ItemPFX),
		FeedID:       feedID,
		Timestamp:    time.Now().UTC(),
		Published:    source.GetPublishedDate(),
		Updated:      source.GetUpdatedDate(),
		Title:        source.GetTitle(),
		Description:  source.GetDescription(),
		SourceType:   ItemSourceType(sourceType),
		URL:          source.GetLink(),
		Authors:      source.GetAuthors(),
		Contributors: source.GetContributors(),
		Copyright:    source.GetRights(),
		Language:     source.GetLanguage(),
		Categories:   source.GetCategories(),
		Image:        source.GetImage(),
		Content:      source.GetContent().String(),
	}

	return item
}
