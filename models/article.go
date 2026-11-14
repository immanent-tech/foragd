/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package models

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/immanent-tech/go-base/pkg/htmlx"
	"github.com/immanent-tech/go-base/validation"
)

var ErrInvalidArticleContent = errors.New("invalid article content")

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

// GetLanguageCounts returns a count of the occurrence of a [models.Language] across all the [models.Article] in the
// slice.
func (a Articles) GetLanguageCounts() map[string]int64 {
	counts := make(map[string]int64)
	for article := range slices.Values(a) {
		if lang := article.Item.GetLanguage(); lang != "" {
			counts[lang]++
		}
	}
	return counts
}

// FilterByView returns a slice containing the subscription which match the given view state.
func (a Articles) FilterByView(view View) Articles {
	switch view {
	case ViewRead:
		return slices.Collect(Filter(slices.Values(a), func(article *Article) bool {
			return !article.IsUnread()
		}))
	case ViewUnread:
		return slices.Collect(Filter(slices.Values(a), func(article *Article) bool {
			return article.IsUnread()
		}))
	default:
		return a
	}
}

// SortByTimestamp sorts the items by their timestamps, in descending order.
func (a Articles) SortByTimestamp() Articles {
	slices.SortFunc(a, func(a, b *Article) int {
		return a.Item.GetTimestamp().Compare(b.Item.GetTimestamp())
	})
	slices.Reverse(a)
	return a
}

// Validate returns a boolean indicating if the article contains valid data (true). If it contains invalid data
// (false) a non-nil error is also returned which contains validation issues.
func (a Article) Validate() error {
	if err := validation.Validate.Struct(a); err != nil {
		return fmt.Errorf("article is invalid: %w", err)
	}
	return nil
}

// GetID returns the article ID.
func (a Article) GetID() string {
	return a.Item.GetID()
}

// GetSubscriptionID returns the ID of the user subscription the article belongs to.
func (a Article) GetSubscriptionID() SubscriptionID {
	return a.Subscription.GetID()
}

// GetSubscriptionTitle returns the title of the subscription the article belongs to.
func (a Article) GetSubscriptionTitle() string {
	return a.Subscription.GetTitle()
}

// GetSubscriptionLink returns the link of the subscription the article belongs to.
func (a Article) GetSubscriptionLink() string {
	return a.Subscription.GetLink()
}

func (a Article) GetSubscription() Subscription {
	return a.Subscription
}

// GetFeedID returns the ID of the feed the article belongs to.
func (a Article) GetFeedID() FeedID {
	return a.Item.GetFeedID()
}

// GetTitle returns the title of the article.
func (a Article) GetTitle() string {
	if a.Item.GetTitle() == "" {
		return "(no title)"
	}
	return a.Item.GetTitle()
}

// GetDescription returns the description of the article.
func (a Article) GetDescription() string {
	return a.Item.GetDescription()
}

func (a Article) IsYoutubeVideo() bool {
	return a.Item.ExtensionType != nil && *a.Item.ExtensionType == ItemExtensionTypeYoutube
}

func (a Article) AsYoutubeVideo() (*ItemExtensionYoutube, error) {
	if !a.IsYoutubeVideo() {
		return nil, ErrInvalidArticleContent
	}
	data, err := a.Item.ExtensionData.AsItemExtensionYoutube()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidArticleContent, err)
	}
	return &data, nil
}

func (a Article) IsEmail() bool {
	return a.Item.SourceType == SourceTypeEmail
}

// GetContent returns the main content of the article. This will be either the full content fetched remotely (if
// requested), the "content" field of the item (if not empty), or the description (if any).
func (a Article) GetContent() string {
	switch {
	case a.ShowFullContent && a.Content != nil:
		sanitized, err := htmlx.Lint(*a.Content)
		if err != nil {
			return ""
		}
		return sanitized
	case a.IsEmail():
		sanitized, err := htmlx.Lint(
			a.Item.GetContent(),
			htmlx.UnwrapLayoutTables,
			htmlx.StripAttributesOption,
			htmlx.StripWhitespaceOnlyNodesOption,
		)
		if err != nil {
			return ""
		}
		return htmlx.StripEmailPreheaderPadding(sanitized)
	case a.IsYoutubeVideo():
		return ""
	case strings.Contains(a.Item.GetLink(), "reddit.com"):
		sanitized, err := htmlx.Lint(
			a.Item.GetContent(),
			htmlx.UnwrapLayoutTables,
			htmlx.StripWhitespaceOnlyNodesOption,
		)
		if err != nil {
			return ""
		}
		return sanitized
	case a.Item.HasContent():
		sanitized, err := htmlx.Lint(a.Item.GetContent())
		if err != nil {
			return ""
		}
		return sanitized
	default:
		return a.Item.GetDescription()
	}
}

// IsRemoteContent returns a boolean indicating whether the full content of the article should be shown.
func (a Article) IsRemoteContent() bool {
	return a.ShowFullContent
}

// GetImage returns the image associated with the article.
func (a Article) GetImage() *RemoteImage {
	if a.Item.GetImage() != nil && a.Item.GetImage().GetURL() != "" {
		return a.Item.GetImage()
	}

	// Try to extract an image from the content.
	switch url, alt, err := htmlx.ExtractImage(a.GetContent(), a.GetLink()); {
	case err != nil:
		return nil
	case url != "":
		img := NewRemoteImage(url, alt)
		if img.GetTitle() == "" {
			img.Title = new(a.GetTitle())
		}
		return img
	}

	return nil
}

// GetAuthors returns a slice of authors (if any) of the article.
func (a Article) GetAuthors() []Author {
	return a.Item.GetAuthors()
}

// GetUpdatedDate returns the timestamp when the article was last updated (or created if no updates).
func (a Article) GetUpdatedDate() time.Time {
	return a.Item.GetTimestamp()
}

// GetLink returns the URL pointing to the original article content.
func (a Article) GetLink() string {
	return a.Item.GetLink()
}

// GetCategories returns the categories of the article (if any).
func (a Article) GetCategories(num int) Categories {
	categories := slices.Compact(a.Item.GetCategories())
	if num != 0 {
		if len(categories) > num {
			return categories[:num]
		}
		return categories
	}
	return categories
}

// GetFeedTitle returns the title of the feed the article belongs to.
func (a Article) GetFeedTitle() string {
	return a.Item.FeedTitle
}

// IsUnread returns a boolean indicating whether the user has not read this article.
func (a Article) IsUnread() bool {
	if a.State != nil {
		return !a.State.Read
	}
	return true
}

// IsFavorite returns a boolean indicating whether the article has been favorited.
func (a Article) IsFavorite() bool {
	if a.State != nil {
		return a.State.Favorite
	}
	return false
}

// GetObjectType returns the type of the object, in this case, "article".
func (a Article) GetObjectType() ObjectType {
	return ObjectTypeArticle
}

const articleCtxKey contextKey = "article"

func ArticleToCtx(ctx context.Context, article *Article) context.Context {
	return context.WithValue(ctx, articleCtxKey, *article)
}

func ArticleFromCtx(ctx context.Context) *Article {
	if article, ok := ctx.Value(articleCtxKey).(Article); ok {
		return &article
	}
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

func (r *ListArticlesRequest) Sanitise() error {
	return nil
}

func (r *ListArticlesRequest) Validate() error {
	if r == nil {
		return nil
	}
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("validate list articles: %w", err)
	}
	return nil
}

func (r *ViewArticleRequest) Sanitise() error {
	return nil
}

func (r *ViewArticleRequest) Validate() error {
	if r == nil {
		return nil
	}
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("validate view article: %w", err)
	}
	return nil
}
