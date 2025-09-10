// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"maps"
	"slices"
	"time"

	feeds "github.com/immanent-tech/go-syndication"
)

// Items is a slice of items.
type Items []*Item

// FilterSince filters items to ones which are newer than the given timestamp.
func (i Items) FilterSince(since time.Time) Items {
	return slices.Collect(FilterSlice(i, func(v *Item) bool {
		return v.IsNewer(since)
	}))
}

// FilterByFeed filters items to ones which match the given feed ID.
func (i Items) FilterByFeed(feedID FeedID) Items {
	return slices.Collect(FilterSlice(i, func(v *Item) bool {
		return v.GetFeedID() == feedID
	}))
}

// GetFeedIDs retrieves a list of all FeedIDs from all items.
func (i Items) GetFeedIDs() []FeedID {
	feedIDs := make([]FeedID, 0, len(i))
	for item := range slices.Values(i) {
		feedIDs = append(feedIDs, item.GetFeedID())
	}
	return slices.Compact(feedIDs)
}

// GetIDs retrieves a list of all ItemIDs from all items.
func (i Items) GetIDs() []ItemID {
	itemIDs := make([]ItemID, 0, len(i))
	for item := range slices.Values(i) {
		itemIDs = append(itemIDs, item.GetID())
	}
	return slices.Compact(itemIDs)
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

func (i *Item) GetImage() *RemoteImage {
	return &i.Image
}

func (i *Item) GetLanguage() string {
	return i.Language
}

// GetUpdatedDate returns a timestamp indicating when the item was last updated. This will be either, the updated
// timestamp, or, the published timestamp, or the indexing timestamp, whichever is found and
// is a valid value, in that order.
func (i *Item) GetUpdatedDate() time.Time {
	if valid, _ := ValidateDatetime(i.Updated); valid {
		return i.Updated
	} else if valid, _ := ValidateDatetime(i.Published); valid {
		return i.Published
	}
	return i.Timestamp
}

// IsNewer returns a boolean indicating whether this item has been updated or
// published after the given time.
func (i *Item) IsNewer(since time.Time) bool {
	return i.GetUpdatedDate().After(since)
}

func (i *Item) GetRights() string {
	return i.Copyright
}

func (i *Item) GetContent() string {
	return i.Content
}

// NewItemFromSource generates an Item from the underlying feed data.
func NewItemFromSource(source *feeds.Item, feed *Feed) *Item {
	item := &Item{
		ItemID:       NewID(ItemPFX),
		FeedID:       feed.GetID(),
		Timestamp:    time.Now().UTC(),
		Published:    source.GetPublishedDate(),
		Updated:      source.GetUpdatedDate(),
		Title:        source.GetTitle(),
		Description:  source.GetDescription(),
		SourceType:   ItemSourceType(feed.SourceType),
		URL:          source.GetLink(),
		Authors:      source.GetAuthors(),
		Contributors: source.GetContributors(),
		Copyright:    source.GetRights(),
		Language:     source.GetLanguage(),
		Categories:   source.GetCategories(),
		Image: RemoteImage{
			URL:   source.GetImage().URL(),
			Title: source.GetImage().String(),
		},
		Content:   source.GetContent(),
		FeedTitle: source.FeedTitle,
	}

	// Check for a valid published timestamp. If not valid, set the published timestamp to the feed's updated timestamp.
	if valid, _ := ValidateDatetime(item.Published); !valid {
		item.Published = feed.GetUpdatedDate()
	}

	return item
}
