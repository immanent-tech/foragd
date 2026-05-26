// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/zeebo/xxh3"

	feeds "github.com/immanent-tech/go-syndication"
	"github.com/immanent-tech/go-syndication/atom"

	"github.com/immanent-tech/foragd/pkg/formats/html"
	"github.com/immanent-tech/foragd/pkg/formats/markdown"
	"github.com/immanent-tech/foragd/providers/zyte"
	"github.com/immanent-tech/foragd/validation"
)

// Items is a slice of items.
type Items []*Item

// FilterSince filters items to ones which are newer than the given timestamp.
func (i Items) FilterSince(since time.Time) Items {
	return slices.Collect(FilterSlice(i, func(item *Item) bool {
		return item.IsNewer(since)
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

// SortByTimestamp sorts the items by their timestamps, in descending order.
func (i Items) SortByTimestamp() Items {
	slices.SortFunc(i, func(itemA, itemB *Item) int {
		return itemA.GetTimestamp().Compare(itemB.GetTimestamp())
	})
	slices.Reverse(i)
	return i
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
	if i.Title == "" {
		return "(no title)"
	}
	return i.Title
}

// GetDescription returns the summary of the item content, if any.
func (i *Item) GetDescription() string {
	if i.Description != nil {
		switch {
		case html.IsHTML(*i.Description):
			sanitizedDesc, err := html.SanitizeHTMLString(*i.Description)
			if err != nil {
				return ""
			}
			return sanitizedDesc
		default:
			formatted, err := markdown.ToHTML([]byte(*i.Description))
			if err != nil {
				return *i.Description
			}
			return string(formatted)
		}
	}
	return ""
}

// GetAuthors returns a slice of the item's authors, if any.
func (i *Item) GetAuthors() []Author {
	return generateAuthors(i.Authors)
}

// GetContributors returns a slice of the item's contributors, if any.
func (i *Item) GetContributors() []Author {
	return generateAuthors(i.Contributors)
}

// GetCategories returns a slice of the item's categories, if any.
func (i *Item) GetCategories() []string {
	// Just in case there are duplicate categories, avoids a validation error.
	return slices.Compact(i.Categories)
}

// GetImage returns an image that can represent the item, if any.
func (i *Item) GetImage() *RemoteImage {
	return i.Image
}

// GetLanguage returns the language of the item, if set.
func (i *Item) GetLanguage() string {
	if i.Language != nil {
		return *i.Language
	}
	return ""
}

// GetRights returns the copyright associated with the item, if any.
func (i *Item) GetRights() string {
	if i.Copyright != nil {
		return *i.Copyright
	}
	return ""
}

// GetContent returns the full item content, if set.
func (i *Item) GetContent() string {
	if i.Content == nil {
		return ""
	}
	switch {
	case html.IsHTML(*i.Content):
		sanitizedDesc, err := html.SanitizeHTMLString(*i.Content)
		if err != nil {
			return ""
		}
		return sanitizedDesc
	default:
		formatted, err := markdown.ToHTML([]byte(*i.Content))
		if err != nil {
			return *i.Content
		}
		return string(formatted)
	}
}

// HasContent returns a boolean indicating whether the item has full or partial content.
func (i *Item) HasContent() bool {
	return i.Content != nil && *i.Content != ""
}

// GetTimestamp returns a timestamp indicating when the item was last updated. This will be either, the published
// timestamp, or, the updated timestamp, or the indexing timestamp, whichever is found and is a valid value, in that
// order.
func (i *Item) GetTimestamp() time.Time {
	if i.Updated != nil {
		if valid, _ := validateDatetime(*i.Updated); valid {
			return i.Updated.UTC()
		}
	}
	if valid, _ := validateDatetime(i.Published); valid {
		return i.Published.UTC()
	}
	return i.Timestamp.UTC()
}

// WasUpdated returns a boolean indicating whether the item has been updated since being published.
func (i *Item) WasUpdated() bool {
	if i.Updated != nil {
		upd := *i.Updated
		return !upd.IsZero()
	}
	return false
}

// IsNewer returns a boolean indicating whether this item has been updated or
// published after the given time and before now (to ignore potentially incorrect dates in the future).
func (i *Item) IsNewer(since time.Time) bool {
	return i.GetTimestamp().After(since) && i.GetTimestamp().Before(time.Now().UTC())
}

// NewFeedItem generates an Item from the underlying feed data.
func NewFeedItem(ctx context.Context, source *feeds.Item, feed *Feed) *Item {
	// Generate a consistent document ID from either the item ID (if it has one) or the item URL.
	var itemID ItemID
	if sourceID := source.GetID(); sourceID != "" {
		itemID = "item_" + strconv.FormatUint(xxh3.Hash([]byte(feed.GetID()+sourceID)), 10)
	} else {
		itemID = "item_" + strconv.FormatUint(xxh3.Hash([]byte(feed.GetID()+source.GetLink())), 10)
	}
	item := &Item{
		ItemID:       itemID,
		FeedID:       feed.GetID(),
		Timestamp:    time.Now().UTC(),
		Title:        source.GetTitle(),
		Description:  new(validation.SanitizeString(source.GetDescription())),
		SourceType:   feed.SourceType,
		URL:          source.GetLink(),
		Authors:      source.GetAuthors(),
		Contributors: source.GetContributors(),
		Copyright:    source.GetRights(),
		Language:     source.GetLanguage(),
		Categories:   source.GetCategories(),
		FeedTitle:    feed.GetTitle(),
	}
	if content := source.GetContent(); content != nil {
		item.Content = new(validation.SanitizeString(*content))
	}
	if pubDate := source.GetPublishedDate(); pubDate != nil {
		item.Published = pubDate.UTC()
	} else {
		item.Published = item.Timestamp
	}
	if updDate := source.GetUpdatedDate(); updDate != nil {
		item.Updated = new(updDate.UTC())
	}

	// Add youtube extension data if found.
	addYoutubeExtension(source, item)

	// Set the image.
	if sourceImg := source.GetImage(); sourceImg != nil {
		// Source has an image, use that.
		item.Image = NewRemoteImage(sourceImg.GetURL(), sourceImg.GetTitle())
	} else {
		// Find an appropriate image for the item and use it.
		if imgURL, err := zyte.ExtractMainImage(ctx, item.GetLink()); err == nil && imgURL != "" {
			item.Image = NewRemoteImage(imgURL, item.GetTitle())
		}
	}

	// Check for a valid published timestamp. If not valid, set the published timestamp to the feed's updated timestamp.
	if valid, _ := validateDatetime(item.Published); !valid {
		item.Published = feed.GetTimestamp()
	}

	return item
}

// NewEmailItem generates a new Item from an email.
func NewEmailItem(email Email, subscription *Subscription) *Item {
	// Generate a consistent document ID from either the item ID (if it has one) or the item URL.
	itemID := "item_" + strconv.FormatUint(xxh3.Hash([]byte(email.GetID())), 10)
	item := &Item{
		ItemID:     itemID,
		FeedID:     subscription.GetFeedID(),
		Timestamp:  email.Timestamp(),
		Published:  email.Timestamp(),
		Updated:    new(email.Timestamp()),
		Title:      email.GetSubject(),
		SourceType: SourceTypeEmail,
		Authors:    []string{email.GetFrom().String()},
		Content:    new(email.GetBody()),
		FeedTitle:  subscription.GetTitle(),
	}

	return item
}

func addYoutubeExtension(source *feeds.Item, item *Item) {
	// Extract and add additional information for youtube feeds.
	if strings.Contains(item.GetLink(), "youtube.com") && strings.HasPrefix(source.GetID(), "yt:video:") {
		if entry, isValidEntry := source.ItemSource.(*atom.Entry); isValidEntry {
			if len(entry.MediaGroup.Content) > 0 {
				width := entry.MediaGroup.Content[0].Width
				height := entry.MediaGroup.Content[0].Height
				if videoID, isValidVideoID := strings.CutPrefix(source.GetID(), "yt:video:"); isValidVideoID {
					item.ExtensionType = new(ItemExtensionTypeYoutube)
					item.ExtensionData = &Item_ExtensionData{}
					item.ExtensionData.FromItemExtensionYoutube(ItemExtensionYoutube{
						VideoId: videoID,
						Width:   &width,
						Height:  &height,
					})
				}
			}

		}
	}
}
