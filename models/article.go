// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"time"

	slogctx "github.com/veqryn/slog-context"
	"golang.org/x/net/html"

	"golang.org/x/net/html/atom"

	"github.com/immanent-tech/go-syndication/types"

	estypes "github.com/elastic/go-elasticsearch/v9/typedapi/types"

	"github.com/immanent-tech/foragd/providers/elastic/aggregations"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/validation"
)

// GetArticles generates Article objects from the Items with the given IDs.
func GetArticles(ctx context.Context, itemIDs ...ItemID) (Articles, error) {
	// Search through items matching any given feeds filters, excluding any read
	// items.
	query := query.Bool(
		query.Filter(
			// Must match any of the given item IDs,
			query.Terms("item_id", itemIDs...),
		),
	)
	items, _, err := SearchItems(ctx, query, len(itemIDs), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("get articles failed: %w", err)
	}
	articles, err := GenerateArticles(ctx, items)
	if err != nil {
		return nil, fmt.Errorf("get articles failed: %w", err)
	}

	return articles, nil
}

// FilterArticles returns Articles filtered by the given filters and paginated by the given pagination.
func FilterArticles(
	ctx context.Context,
	request *ListRequest,
) (Articles, Pagination, error) {
	user := UserFromCtx(ctx)
	if user == nil {
		return nil, "", fmt.Errorf("get user data: %w", ErrCtxValueNotFound)
	}

	// Get all user subscriptions.
	subscriptions, err := GetSubscriptions(ctx,
		GetSubscriptionsByIDs(request.Filters.GetSubscriptions()...),
	)
	if err != nil {
		return nil, "", fmt.Errorf("filter articles: get subscriptions: %w", err)
	}

	// Return early if there the user has no subscriptions (i.e., new user).
	if len(subscriptions) == 0 {
		return nil, "", ErrNotFound
	}

	// Build article query.
	articleQuery := query.Bool(
		query.Filter(
			query.Terms("feed_id", subscriptions.GetFeedIDs()...),
			query.Terms("categories.raw", request.Filters.GetCategories()...),
			query.Bool(
				query.Should(BuildItemQueries(user, request.Filters.GetView(), subscriptions)...),
			),
			request.Query,
		),
	)

	sort := request.Filters.GetSort()

	// Find items matching filters.
	items, pagination, err := SearchItems(ctx, articleQuery, request.Filters.GetCount(), &sort, request.Pagination)
	if err != nil {
		return nil, "", fmt.Errorf("could not retrieve filtered items: %w", err)
	}

	// Generate articles.
	articles, err := GenerateArticles(ctx, items)
	if err != nil {
		return nil, "", fmt.Errorf("could not generate articles from items: %w", err)
	}

	return articles, pagination, nil
}

// FindSimilarArticles performs a "more like this" search to find other Articles that are similar to the Items with the
// given IDs.
func FindSimilarArticles(ctx context.Context, count int, itemIDs ...ItemID) (Articles, error) {
	user := UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get user data: %w", ErrCtxValueNotFound)
	}
	subscriptions, err := GetSubscriptions(ctx)
	switch {
	case err != nil:
		return nil, fmt.Errorf("find similar articles: get subscriptions: %w", err)
	case len(subscriptions) == 0:
		return nil, fmt.Errorf("find similar articles: get subscriptions: %w", ErrNotFound)
	}
	// Build the More Like This query.
	// TODO: tweak values and fields for optimum results matching...
	var (
		minTermFreq   = 1
		maxQueryTerms = 12
	)
	mlt := query.NewMoreLikeThisQuery("similar_articles")
	mlt.LikeDocs(itemIDs...)
	mlt.Fields = []string{"title", "categories.raw", "author"}
	mlt.MinTermFreq = &minTermFreq
	mlt.MaxQueryTerms = &maxQueryTerms
	// Build query
	similarQuery := query.Bool(
		query.Filter(
			query.Bool(
				query.Should(BuildItemQueries(user, ViewUnread, subscriptions)...),
			),
		),
		query.Must(
			mlt.ToQueryOption(),
		),
	)
	// Query for similar articles.
	sort := SortMostRelevant
	items, _, err := SearchItems(ctx, similarQuery, count, &sort, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to find similar articles: %w", err)
	}
	// Generate article data.
	articles, err := GenerateArticles(ctx, items)
	if err != nil {
		return nil, fmt.Errorf("unable to find similar articles: %w", err)
	}
	return articles, nil
}

// GenerateArticles takes a slice of items and creates articles from them, grabbing the necessary data from the user
// object.
func GenerateArticles(ctx context.Context, items Items) (Articles, error) {
	user := UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get user data: %w", ErrCtxValueNotFound)
	}

	// Get the subscriptions associated with the items.
	subscriptions, err := GetSubscriptionsForItems(ctx, items)
	if err != nil {
		return nil, fmt.Errorf("get subscriptions for items: %w", err)
	}
	if len(subscriptions) == 0 {
		return nil, fmt.Errorf("get subscriptions for items: %w", ErrNotFound)
	}

	// Create articles from the items.
	articles := make(Articles, 0, len(items))
	for item := range slices.Values(items) {
		subscription := subscriptions.GetByFeedID(item.GetFeedID())
		if subscription == nil {
			slogctx.FromCtx(ctx).WarnContext(ctx, "Could not match item to subscription.",
				slog.Any("error", err),
				slog.String("item_id", item.GetID()),
				slog.String("feed_id", item.GetFeedID()),
			)
			continue
		}
		article := &Article{
			Item:           *item,
			SubscriptionID: subscription.GetID(),
			State:          *subscription.GetItemState(item.GetID()),
			SourceType:     item.SourceType,
		}
		// If there is favorite data, mark article as a favorite.
		if slices.Contains(user.ItemFavorites, item.GetID()) {
			article.Favorite = true
		}
		// Add any appropriate feed customisation data.
		article.Item.FeedTitle = subscription.GetTitle()
		// 	Update read status.
		if item.GetTimestamp().Before(subscription.GetMarkedReadAt()) {
			article.State.MarkRead(subscription.GetMarkedReadAt())
		}
		// Toggle showing remote article content.
		article.ShowFullContent = subscription.Settings.ShowFullArticleContent
		// Toggle marking read on view.
		article.MarkArticleReadOnView = user.GetSettings().MarkArticleReadOnView
		// Validate the article.
		if err := article.Valid(); err != nil {
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

// GetArticleTopCategories performs an aggregation to return the top Item categories across the given Feeds.
func GetArticleTopCategories(ctx context.Context, searchQuery query.Option) ([]Category, error) {
	// Build aggregations.
	termsField := "categories.raw"
	termsCount := 10
	aggs := aggregations.Aggs{
		"TopCategories": estypes.Aggregations{
			Terms: &estypes.TermsAggregation{
				Field: &termsField,
				Size:  &termsCount,
			},
		},
	}
	// Perform aggregation.
	results, err := ItemsAggregation(ctx, searchQuery, 0, aggs)
	if err != nil {
		return nil, fmt.Errorf("unable to get top categories: %w", err)
	}

	topCategoriesAgg, ok := results.Aggregations["TopCategories"].(*estypes.StringTermsAggregate)
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

// GetContent returns the main content of the article. This will be either the full content fetched remotely (if
// requested), the "content" field of the item (if not empty), or the description (if any).
func (a *Article) GetContent() string {
	switch {
	case a.ShowFullContent && a.Content != nil:
		return *a.Content
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
	// Try to extract an image from the content.
	img, err := ExtractImageFromContent(a.GetContent())
	switch {
	case err != nil:
		return nil
	case img != nil:
		if img.Title == "" {
			img.Title = a.GetTitle()
		}
		return img
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

func ExtractImageFromContent(content string) (*types.ImageInfo, error) {
	doc, err := html.Parse(strings.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("parse content: %w", err)
	}
	for n := range doc.Descendants() {
		if n.Type == html.ElementNode && n.DataAtom == atom.Img {
			img := &types.ImageInfo{}

			for a := range slices.Values(n.Attr) {
				switch a.Key {
				case "src":
					img.URL = a.Val
				case "alt":
					img.Title = a.Val
				}
			}

			if img.URL != "" {
				return img, nil
			}
		}
	}

	return nil, errors.New("no img element found")
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
