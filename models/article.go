// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"time"

	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-syndication/types"

	"github.com/immanent-tech/foragd/validation"
)

// Articles is a slices of Article objects.
type Articles []*Article

// GetSubscriptionIDs retrieves the subscription ids for all articles in the slice.
func (a Articles) GetSubscriptionIDs() []SubscriptionID {
	ids := make([]SubscriptionID, 0, len(a))
	for article := range slices.Values(a) {
		ids = append(ids, article.SubscriptionID)
	}
	return slices.Compact(ids)
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
func GenerateArticle(user *User, item *Item, state *FeedSubscription) (*Article, error) {
	article := &Article{
		Item:           *item,
		SubscriptionID: state.GetID(),
		State:          *state.GetItemState(item.GetID()),
	}
	// If there is favorite data, mark article as a favorite.
	if user.IsFavorite(article.GetID()) {
		article.Favorite = true
	}
	// Add any appropriate feed customisation data.
	if state.Customisation.Nickname != "" {
		article.Item.FeedTitle = state.Customisation.Nickname
	}
	// 	Update read status.
	if item.GetTimestamp().Before(state.MarkedReadAt) {
		article.State.MarkRead(state.MarkedReadAt)
	}
	// Toggle showing remote article content.
	if state.Settings.ShowFullArticleContent {
		article.ShowFullContent = true
	}
	// Validate the article.
	valid, err := article.Valid()
	if err != nil || !valid {
		return nil, fmt.Errorf("could not generate article: %w", err)
	}

	return article, nil
}

// GenerateArticles takes a slice of items and creates articles from them, grabbing the necessary data from the user
// object.
func GenerateArticles(ctx context.Context, items Items) (Articles, error) {
	user, err := UserFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to generate articles: %w", err)
	}
	// Retrieve subscription customisations for feed subscriptions.
	subscriptions := user.GetSubscriptions().GetFeedSubscriptions().FilterByFeedIDs(items.GetFeedIDs()...)
	// Create articles from the items.
	articles := make(Articles, 0, len(items))
	for item := range slices.Values(items) {
		article, err := GenerateArticle(user, item, subscriptions.GetByFeedID(item.GetFeedID()))
		if err != nil {
			slogctx.FromCtx(ctx).WarnContext(ctx, "Could not generate article from data.",
				slog.Any("error", err),
				slog.String("item_id", item.GetID()),
			)
			continue
		}
		articles = append(articles, article)
	}
	return articles, nil
}

// Valid returns a boolean indicating if the article contains valid data (true). If it contains invalid data
// (false) a non-nil error is also returned which contains validation issues.
func (a *Article) Valid() (bool, error) {
	valid, err := validation.ValidateStruct(a)
	if err != nil || !valid {
		return false, fmt.Errorf("article is invalid: %w", err)
	}
	return true, nil
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

func (a *Article) GetImage() *types.ImageInfo {
	if a.Item.GetImage() != nil && a.Item.GetImage().GetURL() != "" {
		return a.Item.GetImage()
	}
	return nil
}

func (a *Article) GetAuthors() []string {
	return a.Item.GetAuthors()
}

func (a *Article) GetUpdatedDate() time.Time {
	return a.Item.GetTimestamp()
}

func (a *Article) GetLink() string {
	return a.Item.GetLink()
}

func (a *Article) GetCategories(max int) Categories {
	categories := a.Item.GetCategories()
	if max != 0 {
		if len(categories) > max {
			return categories[:max]
		} else {
			return categories
		}
	}
	return categories
}

func (a *Article) GetFeedTitle() string {
	return a.Item.FeedTitle
}

// IsUnread returns a boolean indicating whether the user has not read this article.
func (a *Article) IsUnread() bool {
	return !a.State.Read
}

// IsFavorite returns a boolean indicating whether the article has been favorited.
func (s *Article) IsFavorite() bool {
	return s.Favorite
}

// Type returns the type of the object, in this case, "article".
func (a *Article) GetObjectType() ObjectType {
	return ObjectTypeArticle
}

// Sanitise will sanitise the input values.
func (r *MarkArticleRequest) Sanitise() error {
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

// Valid returns a boolean indicating whether the object is valid.
func (r *MarkArticleRequest) Valid() (bool, error) {
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
