// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package service

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/operator"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
)

// GetArticles generates Article objects from the Items with the given IDs.
func GetArticles(ctx context.Context, itemIDs ...models.ItemID) (models.Articles, error) {
	items, err := GetItems(ctx, itemIDs...)
	if err != nil {
		return nil, fmt.Errorf("get items: %w", err)
	}
	articles, err := GenerateArticles(ctx, items)
	if err != nil {
		return nil, fmt.Errorf("generate articles: %w", err)
	}

	return articles, nil
}

func ArticleFiltersQueryClause(filters *models.ArticleFilters) query.BoolOption {
	if filters == nil {
		return nil
	}
	if filters.IsEmpty() {
		return nil
	}
	return query.Must(
		query.SimpleQueryString(
			query.WithSimpleQueryStringText(filters.Text),
			query.WithSimpleQueryStringFields("title", "description", "content"),
			query.WithSimpleQueryStringOperator(&operator.And),
		),
		query.SimpleQueryString(
			query.WithSimpleQueryStringText(filters.Authors),
			query.WithSimpleQueryStringFields("authors", "contributors"),
			query.WithSimpleQueryStringOperator(&operator.And),
		),
		query.SimpleQueryString(
			query.WithSimpleQueryStringText(filters.Categories),
			query.WithSimpleQueryStringFields("categories"),
			query.WithSimpleQueryStringOperator(&operator.And),
		),
	)
}

// GetNextArticle returns the "next" article from the given article, based on the given article timestamp. The direction
// defines what the next article will be (previous or next). If a subscription is given, it will filter to that
// subscription only. Otherwise, the results are also filtered to the given view.
func GetNextArticle(
	ctx context.Context,
	currentID models.ItemID,
	subscriptionID *models.SubscriptionID,
	view models.View,
	direction models.NextArticleRequestDirection,
	ts time.Time,
) (*models.Article, error) {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get user: %w", models.ErrCtxValueNotFound)
	}

	// filters are query clauses that filter the results.
	filters := make([]query.Option, 0)
	// exclusions are query clauses that exclude some results.
	exclusions := make([]query.Option, 0)
	exclusions = append(exclusions, query.Term("item_id", currentID))

	// Define filters/exclusions based on subscription(s).
	if subscriptionID != nil {
		subscription, err := GetSubscription(ctx, *subscriptionID)
		if err != nil {
			return nil, fmt.Errorf("get subscription: %w", err)
		}
		filters = append(filters, query.Term("feed_id", subscription.GetFeedID()))
		filters = append(filters,
			query.Bool(
				ArticleFiltersQueryClause(user.GetSettings().GlobalFilters),
				query.Should(BuildItemQueries(user, view, models.Subscriptions{subscription})...)),
		)
	} else {
		allSubscriptions, err := GetAllSubscriptions(ctx)
		if err != nil {
			return nil, fmt.Errorf("get all subscriptions: %w", err)
		}
		filters = append(filters, query.Terms("feed_id", allSubscriptions.GetFeedIDs()))
		filters = append(filters,
			query.Bool(
				ArticleFiltersQueryClause(user.GetSettings().GlobalFilters),
				query.Should(BuildItemQueries(user, view, allSubscriptions)...)),
		)
	}

	// Define filters and sorting based on direction.
	var sort models.Sort
	switch direction {
	case models.NextArticleRequestDirectionNext:
		filters = append(filters, query.Bool(
			query.Should(
				query.Since("published", ts),
				query.Since("updated", ts),
			),
		))
		sort = models.SortOldestFirst
	case models.NextArticleRequestDirectionPrevious:
		filters = append(filters, query.Bool(
			query.Should(
				query.Before("published", ts),
				query.Before("updated", ts),
			),
		))
		sort = models.SortNewestFirst
	}

	// Find the next item and generate an article.
	items, _, err := SearchItems(
		ctx,
		query.Bool(
			query.Filter(filters...),
			query.MustNot(exclusions...),
		),
		1,
		&sort,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("search items: %w", err)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("search items: %w", elastic.ErrNotFound)
	}
	articles, err := GenerateArticles(ctx, items)
	if err != nil {
		return nil, fmt.Errorf("generate article: %w", err)
	}

	return articles[0], nil
}

// FilterArticles returns Articles filtered by the given filters and paginated by the given pagination.
func FilterArticles(
	ctx context.Context,
	request *models.ListRequest,
) (models.Articles, models.Pagination, error) {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, models.Pagination{}, fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound)
	}

	var (
		subscriptions models.Subscriptions
		err           error
	)
	// Get subscriptions.
	if len(request.Filters.GetSubscriptions()) > 0 {
		subscriptions, err = GetSubscriptionsByID(ctx, request.Filters.GetSubscriptions()...)
	} else {
		subscriptions, err = GetAllSubscriptions(ctx)
	}
	switch {
	case err != nil:
		return nil, models.Pagination{}, fmt.Errorf("get subscriptions: %w", err)
	case len(subscriptions) == 0:
		return nil, models.Pagination{}, models.ErrNotFound
	}

	// Build article query.
	articleQuery := query.Bool(
		query.Filter(
			query.Terms(
				"feed_id",
				subscriptions.GetFeedIDs(),
				query.WithQueryName[*query.TermsQuery]("match-feed-id"),
			),
			query.Terms("categories.raw", request.Filters.GetCategories(),
				query.WithQueryName[*query.TermsQuery]("match-categories"),
			),
			query.Bool(
				ArticleFiltersQueryClause(user.GetSettings().GlobalFilters),
				query.Should(BuildItemQueries(user, request.Filters.GetView(), subscriptions)...),
			),
			request.Query,
		),
	)

	// Set sorting of results.
	sort := request.Filters.GetSort()

	// Set number of items to fetch.
	var count int
	if request.Filters.UpTo != nil {
		count = *request.Filters.UpTo
	} else {
		count = request.Filters.Count
	}

	// Find items matching filters.
	items, pagination, err := SearchItems(
		ctx,
		articleQuery,
		count,
		&sort,
		request.Filters.SearchAfter,
	)
	if err != nil {
		return nil, models.Pagination{}, fmt.Errorf("could not retrieve filtered items: %w", err)
	}

	// Generate articles.
	articles, err := GenerateArticles(ctx, items)
	if err != nil {
		return nil, models.Pagination{}, fmt.Errorf("could not generate articles from items: %w", err)
	}

	return articles, models.Pagination{SearchAfter: &pagination}, nil
}

// FindSimilarArticles performs a "more like this" search to find other Articles that are similar to the Items with the
// given IDs.
func FindSimilarArticles(ctx context.Context, count int, itemIDs ...models.ItemID) (models.Articles, error) {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound)
	}
	subscriptions, err := GetAllSubscriptions(ctx)
	switch {
	case err != nil:
		return nil, fmt.Errorf("get subscriptions: %w", err)
	case len(subscriptions) == 0:
		return nil, fmt.Errorf("get subscriptions: %w", models.ErrNotFound)
	}
	// Build the More Like This query.
	// TODO: tweak values and fields for optimum results matching...
	var (
		minTermFreq   = 1
		minDocFreq    = 2
		minWordLen    = 3
		maxQueryTerms = 25
	)
	mlt := query.NewMoreLikeThisQuery("similar_articles")
	mlt.LikeDocs(itemIDs...)
	mlt.Fields = []string{"title", "description", "content", "categories.raw", "author"}
	mlt.MinTermFreq = &minTermFreq
	mlt.MaxQueryTerms = &maxQueryTerms
	mlt.MinDocFreq = &minDocFreq
	mlt.MinWordLength = &minWordLen
	// Build query
	similarQuery := query.Bool(
		query.Filter(
			query.Bool(
				query.Should(BuildItemQueries(user, models.ViewUnread, subscriptions)...),
			),
		),
		query.Must(
			mlt.ToQueryOption(),
		),
	)
	// Query for similar articles.
	sort := models.SortMostRelevant
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
func GenerateArticles(ctx context.Context, items models.Items) (models.Articles, error) {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound)
	}

	// Get the subscriptions associated with the items.
	subscriptions, err := GetAllSubscriptions(ctx)
	if err != nil {
		return nil, fmt.Errorf("get subscriptions for items: %w", err)
	}
	subscriptions = subscriptions.FilterByFeedIDs(items.GetFeedIDs()...)
	if len(subscriptions) == 0 {
		return nil, fmt.Errorf("get subscriptions for items: %w", models.ErrNotFound)
	}

	// Create articles from the items.
	articles := make(models.Articles, 0, len(items))
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
		article := &models.Article{
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
		if err := article.Validate(); err != nil {
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

// GetArticleRemoteContent populates the article content with the item source.
func GetArticleRemoteContent(ctx context.Context, article *models.Article) error {
	// Get the complete item HTML source, either from the cache or fetch fresh.
	itemPageBuf, err := getItemContent(ctx, &article.Item)
	if err != nil {
		return models.NewAPIError(http.StatusInternalServerError, fmt.Errorf("get item content: %w", err))
	}

	// Parse the item URL.
	articleURL, err := url.Parse(article.GetLink())
	if err != nil {
		return models.NewAPIError(http.StatusInternalServerError, fmt.Errorf("parse article URL: %w", err))
	}

	// Extract opengraph and readability data from item HTML source.
	_, readabilityData, err := extractMetadataFromHTML(articleURL, itemPageBuf.Bytes())
	if err != nil {
		logGeneralError(ctx, err, article.GetLink(), article.Item.GetFeedID())
	}

	// Extract article content using readability.
	var articleBuf bytes.Buffer
	if err := readabilityData.RenderHTML(&articleBuf); err != nil {
		return models.NewAPIError(http.StatusInternalServerError, fmt.Errorf("render article HTML: %w", err))
	}

	// Set the article content to the extracted content.
	article.Content = new(articleBuf.String())

	return nil
}
