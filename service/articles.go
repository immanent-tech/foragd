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
	"github.com/go-resty/resty/v2"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-syndication/sanitization"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/server/cache"
)

// GetArticles generates Article objects from the Items with the given IDs.
func (s *ItemService) GetArticles(
	ctx context.Context,
	itemIDs ...models.ItemID,
) (models.Articles, error) {
	items, err := s.GetItems(ctx, itemIDs...)
	if err != nil {
		return nil, fmt.Errorf("get items: %w", err)
	}
	articles, err := GenerateArticles(ctx, items)
	if err != nil {
		return nil, fmt.Errorf("generate articles: %w", err)
	}

	return articles, nil
}

// GetNextArticle returns the "next" article from the given article, based on the given article timestamp. The direction
// defines what the next article will be (previous or next). If a subscription is given, it will filter to that
// subscription only. Otherwise, the results are also filtered to the given view.
func (s *ItemService) GetNextArticle(
	ctx context.Context,
	currentID models.ItemID,
	subscriptionID models.SubscriptionID,
	view models.View,
	direction string,
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
	allSubscriptions := models.SubscriptionsFromCtx(ctx)
	if allSubscriptions == nil {
		return nil, fmt.Errorf("get user subscriptions: %w", models.ErrCtxValueNotFound)
	}
	subscription := allSubscriptions.GetByID(subscriptionID)
	filters = append(filters, query.Term("feed_id", subscription.GetFeedID()))
	filters = append(filters,
		query.Bool(
			ArticleFiltersQueryClause(user.GetSettings().GlobalFilters),
			query.Should(BuildItemQueries(user, view, models.Subscriptions{subscription})...)),
	)

	// Define filters and sorting based on direction.
	var sort models.Sort
	switch direction {
	case "next":
		filters = append(filters, query.Bool(
			query.Should(
				query.Since("published", ts),
				query.Since("updated", ts),
			),
		))
		sort = models.SortOldestFirst
	case "prev":
		filters = append(filters, query.Bool(
			query.Should(
				query.Before("published", ts),
				query.Before("updated", ts),
			),
		))
		sort = models.SortNewestFirst
	}

	// Find the next item and generate an article.
	items, _, err := s.QueryItems(
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

// FilterArticles returns a [models.Articles] slice (and a [models.Subscriptions] slice) of articles that match the
// given [models.ListFilters]. An additional array of [query.Option] can be passed to further filter items matched.
func (s *ItemService) FilterArticles(
	ctx context.Context,
	filters *models.ListFilters,
	extraQueries ...query.Option,
) (models.Articles, models.Subscriptions, models.Pagination, error) {
	user, subscriptions, err := getUserAndSubscriptions(ctx)
	if err != nil {
		return nil, nil, models.Pagination{}, models.NewAPIError(
			http.StatusInternalServerError,
			fmt.Errorf("get user and subscriptions: %w", err),
		)
	}

	if len(filters.GetSubscriptions()) > 0 {
		subscriptions = subscriptions.FilterByIDs(filters.GetSubscriptions()...)
	}

	// Set sorting of results.
	sort := filters.GetSort()

	// Set number of items to fetch.
	var count int
	if filters.UpTo != nil {
		count = *filters.UpTo
	} else {
		count = filters.Count
	}

	// Find items matching filters.
	items, pagination, err := s.QueryItems(
		ctx,
		generateArticlesQuery(user, subscriptions, filters, extraQueries...),
		count,
		&sort,
		filters.SearchAfter,
	)
	if err != nil {
		return nil, nil, models.Pagination{}, fmt.Errorf("could not retrieve filtered items: %w", err)
	}

	// Generate articles.
	articles, err := GenerateArticles(ctx, items)
	if err != nil {
		return nil, nil, models.Pagination{}, fmt.Errorf("could not generate articles from items: %w", err)
	}

	return articles, subscriptions, models.Pagination{SearchAfter: &pagination}, nil
}

// FilterGeoArticles executes FilterArticles with an additional filter clause to only match articles with geo data.
func (s *ItemService) FilterGeoArticles(
	ctx context.Context,
	filters *models.ListFilters,
) (models.Articles, models.Subscriptions, models.Pagination, error) {
	// Filter to items with geo data.
	return s.FilterArticles(ctx, filters, query.Bool(
		query.Filter(
			query.Exists("geo"),
		),
	))
}

// CountArticles returns a count of items that match the given [models.ListFilters]. An additional array of
// [query.Option] can be passed to further filter items matched.
func (s *ItemService) CountArticles(
	ctx context.Context,
	filters *models.ListFilters,
	extraQueries ...query.Option,
) (int64, error) {
	user, subscriptions, err := getUserAndSubscriptions(ctx)
	if err != nil {
		return 0, models.NewAPIError(
			http.StatusInternalServerError,
			fmt.Errorf("get user and subscriptions: %w", err),
		)
	}

	if len(filters.GetSubscriptions()) > 0 {
		subscriptions = subscriptions.FilterByIDs(filters.GetSubscriptions()...)
	}

	count, err := elastic.Count(
		ctx,
		s.store.GetIndexRO(ItemsIndex),
		generateArticlesQuery(
			user,
			subscriptions,
			filters,
			slices.Concat([]query.Option{
				query.Bool(
					query.Should(
						query.Since("published", time.Now().UTC().Add(-5*time.Minute)),
						query.Since("updated", time.Now().UTC().Add(-5*time.Minute)),
					),
				),
			},
				extraQueries,
			)...,
		),
	)
	if err != nil {
		return 0, fmt.Errorf("count items: %w", err)
	}

	return count, nil
}

// CountGeoArticles executes CountArticles with an additional filter clause to only match articles with geo data.
func (s *ItemService) CountGeoArticles(
	ctx context.Context,
	filters *models.ListFilters,
) (int64, error) {
	return s.CountArticles(ctx, filters, query.Bool(
		query.Filter(
			query.Exists("geo"),
		),
	))
}

// generateArticlesQuery creates the base query for finding articles matching the given [models.ListFilters]. An
// additional list of [query.Option] can be passed, which will be added to the filter clause of the overall bool query.
func generateArticlesQuery(
	user *models.User,
	subscriptions models.Subscriptions,
	filters *models.ListFilters,
	extraQueries ...query.Option,
) query.Option {
	// Build article query.
	return query.Bool(
		query.Filter(
			slices.Concat(
				[]query.Option{
					// Must match these feed IDs.
					query.Terms("feed_id", subscriptions.GetFeedIDs()),
					// Must match these categories.
					query.Terms("categories.raw", filters.GetCategories()),
					// Must have matching language.
					query.Term("language", filters.GetLanguage()),
					// Must match these global article filters.
					query.Bool(ArticleFiltersQueryClause(user.GetSettings().GlobalFilters)),
					// Must match this additional query.
				},
				// Must match one of the subscription filter clauses.
				BuildItemQueries(user, filters.GetView(), subscriptions),
				// Any additional queries passed in.
				extraQueries,
			)...,
		),
	)
}

// FindSimilarArticles performs a "more like this" search to find other Articles that are similar to the Items with the
// given IDs.
func (s *ItemService) FindSimilarArticles(
	ctx context.Context,
	count int,
	itemIDs ...models.ItemID,
) (models.Articles, error) {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get user details: %w", models.ErrCtxValueNotFound)
	}
	allSubscriptions := models.SubscriptionsFromCtx(ctx)
	if allSubscriptions == nil {
		return nil, fmt.Errorf("get user subscriptions: %w", models.ErrCtxValueNotFound)
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
				query.Should(BuildItemQueries(user, models.ViewUnread, allSubscriptions)...),
			),
		),
		query.Must(
			mlt.ToQueryOption(),
		),
	)
	// Query for similar articles.
	sort := models.SortMostRelevant
	items, _, err := s.QueryItems(ctx, similarQuery, count, &sort, nil)
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

// archiveArticle will index the given article content to the article archive for permanent storage.
func (s *ItemService) ArchiveArticle(ctx context.Context, article *models.ArticleArchive) error {
	if err := elastic.CreateDoc(
		ctx,
		s.store.GetIndexRW(FavoritesIndex),
		article.ItemID,
		article,
	); err != nil {
		return fmt.Errorf("archive article: %w", err)
	}
	return nil
}

// unarchiveArticle will delete an article from the archive.
func (s *ItemService) UnarchiveArticle(
	ctx context.Context,
	userID models.UserID,
	itemID models.ItemID,
) error {
	// Set up the query to match the user's favorited article.
	query := query.Bool(
		query.Filter(
			query.Term("user_id", userID),
			query.Term("item_id", itemID),
		),
	)
	if err := elastic.DeleteDocs(ctx, s.store.GetIndexRW(FavoritesIndex), query); err != nil {
		return fmt.Errorf("unarchive article: %w", err)
	}
	return nil
}

// GenerateArticles takes a slice of items and creates articles from them, grabbing the necessary data from the user
// object.
func GenerateArticles(ctx context.Context, items models.Items) (models.Articles, error) {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get user details: %w", models.ErrCtxValueNotFound)
	}

	// Get the subscriptions associated with the items.
	allSubscriptions := models.SubscriptionsFromCtx(ctx)
	if allSubscriptions == nil {
		return nil, fmt.Errorf("get user subscriptions: %w", models.ErrCtxValueNotFound)
	}
	subscriptions := allSubscriptions.FilterByFeedIDs(items.GetFeedIDs()...)
	if len(subscriptions) == 0 {
		return nil, fmt.Errorf("get subscriptions for items: %w", models.ErrNotFound)
	}

	// Create articles from the items.
	articles := make(models.Articles, 0, len(items))
	for item := range slices.Values(items) {
		subscription := subscriptions.GetByFeedID(item.GetFeedID())
		if subscription == nil {
			slogctx.FromCtx(ctx).WarnContext(ctx, "Could not match item to subscription.",
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
func GetArticleRemoteContent(
	ctx context.Context,
	httpClient *resty.Client,
	itemPageCache cache.ObjectCache,
	article *models.Article,
) error {
	// Get the complete item HTML source, either from the cache or fetch fresh.
	sourceURL, err := url.Parse(article.GetLink())
	if err != nil {
		return models.NewAPIError(http.StatusUnprocessableEntity, fmt.Errorf("parse article URL: %w", err))
	}

	itemPageBuf, err := getItemContent(ctx, httpClient, itemPageCache, article.GetID(), sourceURL)
	if err != nil {
		return models.NewAPIError(http.StatusInternalServerError, fmt.Errorf("get item content: %w", err))
	}

	if itemPageBuf.Len() != 0 {
		// Extract opengraph and readability data from item HTML source.
		_, readabilityData, err := extractMetadataFromHTML(sourceURL, itemPageBuf.Bytes())
		if err != nil {
			logGeneralError(ctx, err, article.GetLink(), article.Item.GetFeedID())
		}

		// Extract article content using readability.
		var articleBuf bytes.Buffer
		if err := readabilityData.RenderHTML(&articleBuf); err != nil {
			return models.NewAPIError(http.StatusInternalServerError, fmt.Errorf("render article HTML: %w", err))
		}

		// Set the article content to the extracted content.
		article.Content = new(sanitization.SanitizeString(articleBuf.String()))
	}

	return nil
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
