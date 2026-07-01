// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/pkg/formats/html"

	estypes "github.com/elastic/go-elasticsearch/v9/typedapi/types"

	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/validation"
)

var ErrInvalidArticleContent = errors.New("invalid article content")

// GetArticleTopCategories performs an aggregation to return the top Item categories across the given Feeds.
func GetArticleTopCategories(ctx context.Context, searchQuery query.Option) ([]Category, error) {
	// Build elastic.
	termsField := "categories.raw"
	termsCount := 10
	aggs := elastic.Aggs{
		"TopCategories": estypes.Aggregations{
			Terms: &estypes.TermsAggregation{
				Field: &termsField,
				Size:  &termsCount,
			},
		},
	}
	// Perform aggregation.
	resp, err := elastic.Search[*Item](ctx,
		schema.ItemsIndexRO(),
		elastic.WithQueryOptions[*elastic.SearchRequest](searchQuery),
		elastic.WithAggregations(aggs),
		elastic.WithSize(0),
		elastic.WithDocSorting(),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to get top categories: %w", err)
	}

	topCategoriesAgg, ok := resp.Aggregations["TopCategories"].(*estypes.StringTermsAggregate)
	if !ok {
		return nil, fmt.Errorf("unable to get top categories: aggregations invalid: %w", ErrInvalidAPIResult)
	}
	topCategoriesBuckets, ok := topCategoriesAgg.Buckets.([]estypes.StringTermsBucket)
	if !ok {
		return nil, fmt.Errorf("unable to get top categories: aggregations invalid: %w", ErrInvalidAPIResult)
	}

	topCategories := make([]Category, 0)

	for bucket := range slices.Values(topCategoriesBuckets) {
		if category, ok := bucket.Key.(Category); ok {
			topCategories = append(topCategories, category)
		}
	}

	return topCategories, nil
}

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

// FilterByView returns a slice containing the subscription which match the given view state.
func (a Articles) FilterByView(view View) Articles {
	switch view {
	case ViewRead:
		return slices.Collect(FilterSlice(a, func(article *Article) bool {
			return !article.IsUnread()
		}))
	case ViewUnread:
		return slices.Collect(FilterSlice(a, func(article *Article) bool {
			return article.IsUnread()
		}))
	default:
		return a
	}
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

func (a *Article) IsYoutubeVideo() bool {
	return a.Item.ExtensionType != nil && *a.Item.ExtensionType == ItemExtensionTypeYoutube
}

func (a *Article) AsYoutubeVideo() (*ItemExtensionYoutube, error) {
	if !a.IsYoutubeVideo() {
		return nil, ErrInvalidArticleContent
	}
	data, err := a.Item.ExtensionData.AsItemExtensionYoutube()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidArticleContent, err)
	}
	return &data, nil
}

func (a *Article) IsEmail() bool {
	return a.SourceType == SourceTypeEmail
}

// GetContent returns the main content of the article. This will be either the full content fetched remotely (if
// requested), the "content" field of the item (if not empty), or the description (if any).
func (a *Article) GetContent() string {
	switch {
	case a.IsEmail():
		return a.formatContent()
	case a.IsYoutubeVideo():
		return ""
	case a.ShowFullContent && a.Content != nil:
		return *a.Content
	case a.Item.HasContent():
		return a.formatContent()
	default:
		return a.Item.GetDescription()
	}
}

func (a *Article) formatContent() string {
	switch content := a.Item.GetContent(); {
	case strings.Contains(a.Item.GetLink(), "reddit.com"):
		return html.CleanRedditHTML(a.Item.GetContent())
	case a.SourceType == SourceTypeEmail:
		// For emails, perform extra content cleanup.
		return stripPreheaderPadding(content)
	default:
		return content
	}
}

// IsRemoteContent returns a boolean indicating whether the full content of the article should be shown.
func (a *Article) IsRemoteContent() bool {
	return a.ShowFullContent
}

// GetImage returns the image associated with the article.
func (a *Article) GetImage() *RemoteImage {
	if a.Item.GetImage() != nil && a.Item.GetImage().GetURL() != "" {
		return a.Item.GetImage()
	}

	// Try to extract an image from the content.
	switch url, alt, err := html.ExtractImageFromHTML(a.GetContent()); {
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
func (a *Article) GetAuthors() []Author {
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
	for key, value := range r.DisplayedArticles {
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
	return nil
}

func (r *MarkArticleRequest) Valid() error {
	return validation.Validate.Struct(r)
}

func (r *MarkArticleRequest) Sanitise() error {
	r.Mark = setValidMark(r.Mark)
	return nil
}

func (r *FavoriteArticleRequest) Valid() error {
	return validation.Validate.Struct(r)
}

func (r *FavoriteArticleRequest) Sanitise() error {
	return nil
}

func (r *ShareArticleRequest) Valid() error {
	return validation.Validate.Struct(r)
}

func (r *ShareArticleRequest) Sanitise() error {
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

var preheaderPadding = regexp.MustCompile(`[\x{00A0}\x{200C}\s]{10,}`)

func stripPreheaderPadding(html string) string {
	return preheaderPadding.ReplaceAllString(html, "")
}
