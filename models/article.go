// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/joshuar/go-feed-me/validation"
)

// Articles is a slices of individual articles.
type Articles []*Article

// GetCategoryCounts returns a count of the occurrence of a Category across all
// the Articles in the slice.
func (a Articles) GetCategoryCounts() CategoryCounts {
	countsMap := make(map[Category]int)
	for article := range slices.Values(a) {
		for category := range slices.Values(article.GetCategories()) {
			countsMap[category]++
		}
	}
	var counts CategoryCounts
	for category, count := range maps.All(countsMap) {
		counts = append(counts, CategoryCount{Category: category, Count: count})
	}

	return counts
}

// GetSubscriptionIDs retrieves the subscription ids for all articles in the slice.
func (a Articles) GetSubscriptionIDs() []SubscriptionID {
	ids := make([]SubscriptionID, 0, len(a))
	for article := range slices.Values(a) {
		ids = append(ids, article.SubscriptionID)
	}
	return slices.Compact(ids)
}

// GenerateArticle creates an article from the given data: an item, subscription state and customisation. Only the item
// and state is required.
func GenerateArticle(item *Item, state *SubscriptionState) (*Article, error) {
	article := &Article{
		Item:                      *item,
		SubscriptionID:            state.GetID(),
		State:                     *state.GetItemState(item.GetID()),
		SubscriptionCustomisation: state.Customisation,
	}
	if item.GetPublishedDate().After(article.State.UpdatedAt) {
		article.State.MarkUnread(item.GetPublishedDate())
	}

	// Validate the article.
	if valid, err := article.Valid(); !valid {
		return nil, fmt.Errorf("article data is invalid: %w", err)
	}

	return article, nil
}

// Valid returns a boolean indicating if the article contains valid data (true). If it contains invalid data
// (false) a non-nil error is also returned which contains validation issues.
func (a *Article) Valid() (bool, error) {
	if valid, err := validation.ValidateStruct(a); err != nil || !valid {
		return false, fmt.Errorf("article is invalid: %w", err)
	}
	return true, nil
}

func (a *Article) GetID() ItemID {
	return a.Item.GetID()
}

func (a *Article) GetSubscriptionID() SubscriptionID {
	return a.SubscriptionID
}

func (a *Article) GetFeedID() FeedID {
	return a.Item.GetFeedID()
}

func (a *Article) GetTitle() string {
	return a.Item.GetTitle()
}

func (a *Article) GetDescription() string {
	return a.Item.GetDescription()
}

func (a *Article) GetContent() string {
	return a.Item.GetContent()
}

func (a *Article) GetImage() *ObjectImage {
	return a.Item.GetImage()
}

func (a *Article) GetAuthors() []string {
	return a.Item.GetAuthors()
}

func (a *Article) GetUpdatedDate() time.Time {
	return a.Item.GetUpdatedDate()
}

func (a *Article) GetLink() string {
	return a.Item.GetLink()
}

func (a *Article) GetCategories() []string {
	return a.Item.GetCategories()
}

func (a *Article) GetFeedTitle() string {
	if a.SubscriptionCustomisation.Title != "" {
		return a.SubscriptionCustomisation.Title
	}
	return a.Item.FeedTitle
}

func (a *Article) IsUnread() bool {
	return !a.State.IsRead()
}

// Sanitise will sanitise the input values.
func (r *MarkArticlesRequest) Sanitise() error {
	return nil
}

// Valid returns a boolean indicating whether the object is valid.
func (r *MarkArticlesRequest) Valid() (bool, error) {
	if r == nil {
		return false, fmt.Errorf("request is invalid: %w", validation.ErrNilObject)
	}
	valid, err := validation.ValidateStruct(r)
	if !valid || err != nil {
		return false, fmt.Errorf("request is invalid: %w", err)
	}
	return true, nil
}

// NewArchivedArticle creates a new archived article for long-term storage.
func NewArchivedArticle(userID UserID, subscriptionID SubscriptionID, item *Item) (*ArticleArchive, error) {
	archive := &ArticleArchive{}
	data, err := json.Marshal(item)
	if err != nil {
		return nil, fmt.Errorf("unable to archive article: %w", err)
	}
	err = json.Unmarshal(data, archive)
	if err != nil {
		return nil, fmt.Errorf("unable to archive article: %w", err)
	}
	archive.SubscriptionID = subscriptionID
	archive.UserID = userID
	return archive, nil
}
