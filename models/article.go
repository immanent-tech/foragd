// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/immanent-tech/go-syndication/types"

	"github.com/immanent-tech/foragd/validation"
)

// Articles is a slices of Article objects.
type Articles []*Article

// GetSubscriptionIDs retrieves the subscription ids for all articles in the slice.
func (a Articles) GetSubscriptionIDs() []SubscriptionID {
	ids := make([]SubscriptionID, 0, len(a))
	for article := range slices.Values(a) {
		ids = append(ids, article.GetSubscriptionID())
	}
	return slices.Compact(ids)
}

// GetFeedIDs retrieves the feed ids for all articles in the slice.
func (a Articles) GetFeedIDs() []FeedID {
	ids := make([]FeedID, 0, len(a))
	for article := range slices.Values(a) {
		ids = append(ids, article.GetFeedID())
	}
	return slices.Compact(ids)
}

// GetIDs retrieves the item ids for all articles in the slice.
func (a Articles) GetIDs() []ItemID {
	ids := make([]ItemID, 0, len(a))
	for article := range slices.Values(a) {
		ids = append(ids, article.GetID())
	}
	return ids
}

// GetCategoryCounts returns a count of the occurrence of a Category across all
// the Articles in the slice.
func (a Articles) GetCategoryCounts() CategoryCounts {
	countsMap := make(map[Category]int)
	for article := range slices.Values(a) {
		for category := range slices.Values(article.GetCategories(0)) {
			countsMap[category]++
		}
	}
	var counts CategoryCounts
	for category, count := range maps.All(countsMap) {
		counts = append(counts, CategoryCount{Category: category, Count: count})
	}

	return counts
}

// GenerateArticle creates an article from the given data: an item, subscription state and customisation. Only the item
// and state is required.
func GenerateArticle(user *User, item *Item, subscription *Subscription) (*Article, error) {
	article := &Article{
		Item:           *item,
		SubscriptionID: subscription.GetID(),
		State:          *subscription.FeedData.GetItemState(item.GetID()),
	}
	// If there is favorite data, mark article as a favorite.
	if slices.Contains(user.ItemFavorites, item.GetID()) {
		article.Favorite = true
	}
	// Add any appropriate feed customisation data.
	article.Item.FeedTitle = subscription.GetTitle()
	// 	Update read status.
	if item.GetTimestamp().Before(subscription.MarkedReadAt) {
		article.State.MarkRead(subscription.MarkedReadAt)
	}
	// Toggle showing remote article content.
	article.ShowFullContent = subscription.Settings.ShowFullArticleContent
	// Toggle marking read on view.
	article.MarkArticleReadOnView = user.GetSettings().MarkArticleReadOnView
	// Validate the article.
	if err := article.Valid(); err != nil {
		return nil, fmt.Errorf("could not generate article: %w", err)
	}

	return article, nil
}

// Valid returns a boolean indicating if the article contains valid data (true). If it contains invalid data
// (false) a non-nil error is also returned which contains validation issues.
func (a *Article) Valid() error {
	if err := validation.Validate.Struct(a); err != nil {
		return fmt.Errorf("article is invalid: %w", err)
	}
	return nil
}

// GetID returns the article ID.
func (a *Article) GetID() string {
	return a.Item.GetID()
}

// GetSubscriptionID returns the ID of the user subscription the article belongs to.
func (a *Article) GetSubscriptionID() SubscriptionID {
	return a.SubscriptionID
}

// GetFeedID returns the ID of the feed the article belongs to.
func (a *Article) GetFeedID() FeedID {
	return a.Item.GetFeedID()
}

// GetTitle returns the title of the article.
func (a *Article) GetTitle() string {
	if a.Item.GetTitle() == "" {
		return "(no title)"
	}
	return a.Item.GetTitle()
}

// GetDescription returns the description of the article.
func (a *Article) GetDescription() string {
	return a.Item.GetDescription()
}

// GetContent returns the main content of the article. This will be either the full content fetched remotely (if
// requested), the "content" field of the item (if not empty), or the description (if any).
func (a *Article) GetContent() string {
	switch {
	case a.ShowFullContent:
		return a.Content
	case a.Item.GetContent() != "":
		return a.Item.GetContent()
	default:
		return a.Item.GetDescription()
	}
}

// IsRemoteContent returns a boolean indicating whether the full content of the article should be shown.
func (a *Article) IsRemoteContent() bool {
	return a.ShowFullContent
}

// GetImage returns the image associated with the article.
func (a *Article) GetImage() *types.ImageInfo {
	if a.Item.GetImage() != nil && a.Item.GetImage().GetURL() != "" {
		return a.Item.GetImage()
	}
	return nil
}

// GetAuthors returns a slice of authors (if any) of the article.
func (a *Article) GetAuthors() []string {
	return a.Item.GetAuthors()
}

// GetUpdatedDate returns the timestamp when the article was last updated (or created if no updates).
func (a *Article) GetUpdatedDate() time.Time {
	return a.Item.GetTimestamp()
}

// GetLink returns the URL pointing to the original article content.
func (a *Article) GetLink() string {
	return a.Item.GetLink()
}

// GetCategories returns the categories of the article (if any).
func (a *Article) GetCategories(num int) Categories {
	categories := a.Item.GetCategories()
	if num != 0 {
		if len(categories) > num {
			return categories[:num]
		}
		return categories
	}
	return categories
}

// GetFeedTitle returns the title of the feed the article belongs to.
func (a *Article) GetFeedTitle() string {
	return a.Item.FeedTitle
}

// IsUnread returns a boolean indicating whether the user has not read this article.
func (a *Article) IsUnread() bool {
	return !a.State.Read
}

// IsFavorite returns a boolean indicating whether the article has been favorited.
func (a *Article) IsFavorite() bool {
	return a.Favorite
}

// GetObjectType returns the type of the object, in this case, "article".
func (a *Article) GetObjectType() ObjectType {
	return ObjectTypeArticle
}

// Valid ensures that the MarkArticlesRequest contains valid data.
func (r *MarkArticlesRequest) Valid() error {
	for key, value := range r.Metadata {
		err := validation.Validate.Var(key, "startswith=sub_")
		if err != nil {
			return fmt.Errorf("mark articles request: invalid subscription ID: %w", err)
		}
		err = validation.Validate.Var(value, "dive,startswith=item_")
		if err != nil {
			return fmt.Errorf("mark articles request: invalid item IDs: %w", err)
		}
	}
	return nil
}

// Sanitise will alter MarkArticlesRequest data to ensure safety, where needed.
func (r *MarkArticlesRequest) Sanitise() error {
	r.Mark = setValidMark(r.Mark)
	r.View = setValidView(r.View)
	return nil
}

// MarkRead will set the article state to read.
func (s *ArticleState) MarkRead(markedAt time.Time) {
	s.Read = true
	s.UpdatedAt = markedAt
}

// MarkUnread will set the article state to unread.
func (s *ArticleState) MarkUnread(markedAt time.Time) {
	s.Read = false
	s.UpdatedAt = markedAt
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
