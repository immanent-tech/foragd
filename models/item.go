// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	feeds "github.com/immanent-tech/go-syndication"
	"github.com/immanent-tech/go-syndication/types"
	"github.com/spaolacci/murmur3"
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

// GetID returns the item ID.
func (i *Item) GetID() ItemID {
	return i.ItemID
}

// GetFeedID returns the ID of the feed the item belongs to.
func (i *Item) GetFeedID() FeedID {
	return i.FeedID
}

// GetLink returns the URL that should point to a page containing the full item content.
func (i *Item) GetLink() URL {
	return i.URL
}

// GetTitle returns the item's title.
func (i *Item) GetTitle() string {
	return i.Title
}

// GetDescription returns the summary of the item content, if any.
func (i *Item) GetDescription() string {
	return i.Description
}

// GetAuthors returns a slice of the item's authors, if any.
func (i *Item) GetAuthors() []string {
	return i.Authors
}

// GetContributors returns a slice of the item's contributors, if any.
func (i *Item) GetContributors() []string {
	return i.Contributors
}

// GetCategories returns a slice of the item's categories, if any.
func (i *Item) GetCategories() []string {
	return i.Categories
}

// GetImage returns an image that can represent the item, if any.
func (i *Item) GetImage() *types.ImageInfo {
	return &i.Image
}

// GetLanguage returns the language of the item, if set.
func (i *Item) GetLanguage() string {
	return i.Language
}

// GetRights returns the copyright associated with the item, if any.
func (i *Item) GetRights() string {
	return i.Copyright
}

// GetContent returns the full item content, if set.
func (i *Item) GetContent() string {
	return i.Content
}

// GetTimestamp returns a timestamp indicating when the item was last updated. This will be either, the updated
// timestamp, or, the published timestamp, or the indexing timestamp, whichever is found and
// is a valid value, in that order.
func (i *Item) GetTimestamp() time.Time {
	if valid, _ := ValidateDatetime(i.Updated); valid {
		return i.Updated
	} else if valid, _ = ValidateDatetime(i.Published); valid {
		return i.Published
	}
	return i.Timestamp
}

// IsNewer returns a boolean indicating whether this item has been updated or
// published after the given time and before now (to ignore potentially incorrect dates in the future).
func (i *Item) IsNewer(since time.Time) bool {
	return i.GetTimestamp().After(since) && i.GetTimestamp().Before(time.Now().UTC())
}

// NewItemFromSource generates an Item from the underlying feed data.
func NewItemFromSource(source *feeds.Item, feed *Feed) *Item {
	// Generate a consistent document ID from either the item ID (if it has one) or the item URL.
	var itemID ItemID
	if sourceID := source.GetID(); sourceID != "" {
		itemID = strings.Join(
			[]string{ItemPFX.String(), strconv.FormatUint(murmur3.Sum64([]byte(feed.GetID()+"-"+sourceID)), 10)},
			"_",
		)
	} else {
		itemID = strings.Join([]string{ItemPFX.String(), strconv.FormatUint(murmur3.Sum64([]byte(feed.GetID()+"-"+source.GetLink())), 10)}, "_")
	}
	item := &Item{
		ItemID:       itemID,
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
		Content:      source.GetContent(),
		FeedTitle:    source.FeedTitle,
	}

	if source.GetImage() != nil {
		item.Image = *source.GetImage()
	}

	// Check for a valid published timestamp. If not valid, set the published timestamp to the feed's updated timestamp.
	if valid, _ := ValidateDatetime(item.Published); !valid {
		item.Published = feed.GetTimestamp()
	}

	return item
}
