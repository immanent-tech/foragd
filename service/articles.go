// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"sync"

	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/providers/google/gcs"
	"github.com/immanent-tech/foragd/providers/zyte"
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

// FilterArticles returns Articles filtered by the given filters and paginated by the given pagination.
func FilterArticles(
	ctx context.Context,
	request *models.ListRequest,
) (models.Articles, models.Pagination, error) {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, "", fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound)
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
		return nil, "", fmt.Errorf("get subscriptions: %w", err)
	case len(subscriptions) == 0:
		return nil, "", models.ErrNotFound
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
				query.Should(BuildItemQueries(user, request.Filters.GetView(), subscriptions)...),
			),
			request.Query,
		),
	)

	sort := request.Filters.GetSort()

	// Find items matching filters.
	items, pagination, err := SearchItems(
		ctx,
		articleQuery,
		request.Filters.GetCount(),
		&sort,
		request.Pagination,
	)
	if err != nil {
		return nil, "", fmt.Errorf("could not retrieve filtered items: %w", err)
	}

	// Filter by favorites if requested.
	if request.Filters.OnlyFavorites {
		items = items.FilterByIDs(user.ItemFavorites...)
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

// GetArticleRemoteContent attempts to fetch remote content for an article. It will check if the remote content has
// already been fetched and cached in GCS and use that content. Otherwise, it uses the Zyte API to fetch the remote
// content and then cache it for reuse.
func GetArticleRemoteContent(ctx context.Context, article *models.Article) error {
	var cached bool

	// Try to load content from the article cache.
	if err := loadArticleCache(); err != nil {
		slogctx.FromCtx(ctx).Warn("Unable to load article cache.",
			slog.Any("error", err),
		)
		cached = false
	} else {
		if articleBuf, ok := bufPool.Get().(*bytes.Buffer); !ok {
			slogctx.FromCtx(ctx).Warn("Unable to create buffer for cached article.")
			cached = false
		} else {
			articleBuf.Reset()
			defer bufPool.Put(articleBuf)
			if err := articleCache.Copy(ctx, article.GetID(), articleBuf); err != nil {
				slogctx.FromCtx(ctx).Warn("Unable to copy article data from cache.",
					slog.Any("error", err),
				)
				cached = false
			} else {
				cached = true
				article.Content = new(articleBuf.String())
			}
		}
	}

	// Fetch article from remote.
	if !cached {
		extracted, err := zyte.ExtractArticle(ctx,
			article.GetLink(),
			zyte.WithTag("item_id", article.GetID()),
			zyte.WithTag("feed_id", article.GetFeedID()),
		)
		switch {
		case err != nil:
			if zyteErr, ok := errors.AsType[*zyte.ResponseError](err); ok {
				return models.NewAPIError(zyteErr.HTTPStatus(), zyteErr)
			}
			return models.NewAPIError(http.StatusInternalServerError, err)
		case extracted == nil:
			return models.NewAPIError(http.StatusInternalServerError, errors.New("no content extracted"))
		default:
			article.Content = new(extracted.GetHTML())
		}

		// Cache the content.
		articleCache.Set(ctx, article.GetID(), []byte(extracted.GetHTML()))
	}

	return nil
}

var articleCache objectCache

var loadArticleCache = sync.OnceValue(func() error {
	switch config.GetEnvironment() {
	case config.EnvProduction:
		bucketName := os.Getenv("FORAGD_SERVER_BUCKET")
		var err error
		articleCache, err = gcs.Connect(context.Background(), bucketName, "articles")
		if err != nil {
			return fmt.Errorf("connect to gcs: %w", err)
		}
	default:
		var err error
		articleCache, err = newDirCache("articles")
		if err != nil {
			return fmt.Errorf("create dir cache: %w", err)
		}
	}

	return nil
})
