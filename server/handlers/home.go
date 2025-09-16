// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-feed-me/models"
	"github.com/immanent-tech/go-feed-me/providers/elastic/aggregations"
	"github.com/immanent-tech/go-feed-me/providers/elastic/query"
	"github.com/immanent-tech/go-feed-me/providers/elastic/results"
	"github.com/immanent-tech/go-feed-me/web/templates/layouts"
	"github.com/immanent-tech/go-feed-me/web/templates/partials"
)

// Home handles displaying the user's home page.
func (a *API) Home() http.HandlerFunc {
	return alice.New(
		routeLogger,
	).ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		ctx := req.Context()
		ctx = context.WithValue(ctx, titleCtxKey, "Go Feed Me Home")
		user, found := models.UserFromCtx(ctx)
		if !found {
			return models.ErrUserNotFound
		}
		if user.GetSettings().ShowOnboarding {
			template := layouts.NewUserHome()
			renderPage(layouts.Drawer(template), "Home - Go Feed Me").ServeHTTP(res, req)
			return nil
		}
		data, err := a.getHomePageData(ctx)
		if err != nil {
			renderPartial(partials.Notification(
				models.NewErrorMessage(
					"Unable to get home page data",
					"Something went wrong, please try again",
				),
			), "")
			return models.NewAPIError(
				fmt.Errorf("unable to mark subscription: %w", err),
				http.StatusInternalServerError)
		}
		template := data.Template()
		renderPage(layouts.Drawer(template), "Home - Go Feed Me").ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// getHomePageData retrieves the data required to construct the homepage content.
//
//nolint:funlen // mostly aggregation definitions.
func (a *API) getHomePageData(ctx context.Context) (*layouts.Home, error) {
	data := &layouts.Home{}
	// Retrieve user object.
	user, found := models.UserFromCtx(ctx)
	if !found {
		return data, fmt.Errorf("getHomePageData: could not fetch data: %w", ErrNoCtxData)
	}
	data.User = user
	data.Favorites = user.GetFavorites()
	// User has no subscriptions, show empty page
	if len(user.GetSubscriptionMetadata()) == 0 {
		return data, nil
	}
	// Get subscriptions.
	subscriptions, err := a.getSubscriptions(ctx)
	if err != nil {
		return data, models.RespErrBackend(err)
	}
	data.Subscriptions = subscriptions.FilterByView(models.ViewUnread)
	if len(data.Subscriptions) == 0 {
		return data, nil
	}
	// Query definition for fetching unread items for all subscriptions.
	query := query.Bool(
		query.BoolQueryName("item_filters"),
		query.Filter(
			// Must match any of the given feed IDs.
			query.Terms("feed_id", data.Subscriptions.GetFeedIDs()...),
			query.Bool(
				query.Should(buildSubscriptionQueries(user, models.ViewUnread, data.Subscriptions.GetSubscriptionMetadata()...)...),
			),
		),
	)

	// Fetch latest articles.
	latestItems, _, err := a.DataAPI().SearchItems(ctx, query, 10, &models.SortLastUpdatedDesc, nil)
	if err != nil {
		return nil, fmt.Errorf("getHomePageData: could not fetch latest articles: %w", err)
	}
	data.LatestArticles, err = models.GenerateArticles(ctx, latestItems)
	if err != nil {
		return nil, fmt.Errorf("getHomePageData: could not fetch latest articles: %w", err)
	}

	// Fetch aggregation data.
	var aggs []aggregations.Aggregation
	TermsField := "categories.raw"
	// Aggregation definition for fetching the top 10 item categories across all subscriptions.
	SampleField := "feed_id"
	DefaultMaxDocsPerValue := 10
	ShardSize := 1000
	TopHitsCount := 1
	aggs = append(aggs,
		aggregations.Aggregation{
			// top_categories_sample: diversified sampler to ensure top categories not dominated by single overwhelming
			// source.
			Name: "top_categories_sample",
			Definition: types.Aggregations{
				DiversifiedSampler: &types.DiversifiedSamplerAggregation{
					Field:           &SampleField,
					MaxDocsPerValue: &DefaultMaxDocsPerValue,
					ShardSize:       &ShardSize,
				},
				Aggregations: map[string]types.Aggregations{
					// top_categories: the top categories across all subscriptions.
					"top_categories": {
						Terms: &types.TermsAggregation{
							Field:   &TermsField,
							Exclude: models.CommonCategoryFilters,
						},
						Aggregations: map[string]types.Aggregations{
							// top_articles: the top scoring article for each top category.
							"top_articles": {
								TopHits: &types.TopHitsAggregation{
									Size: &TopHitsCount,
								},
							},
						},
					},
				},
			},
		},
	)
	// Aggregation definition for fetching the rare item categories across all subscriptions.
	MaxDocCount := int64(5)
	aggs = append(aggs,
		aggregations.Aggregation{
			Name: "rare_categories",
			Definition: types.Aggregations{
				RareTerms: &types.RareTermsAggregation{
					Field:       &TermsField,
					MaxDocCount: &MaxDocCount,
					Exclude:     models.CommonCategoryFilters,
				},
			},
		},
	)

	// Perform the request.
	queryResult, resp := a.DataAPI().ItemsAggregation(ctx, query, 0, aggs...)
	if resp != nil {
		return nil, resp
	}

	// Get the top categories.
	topCategoriesSamplerAgg, err := aggregations.ExtractAggregation[*types.SamplerAggregate](queryResult.Aggregations, "top_categories_sample")
	if err != nil {
		slogctx.FromCtx(ctx).WarnContext(ctx, "Unable to show top categories.",
			slog.Any("error", err))
	}
	topCategoriesAgg, err := aggregations.ExtractAggregation[*types.StringTermsAggregate](topCategoriesSamplerAgg.Aggregations, "top_categories")
	if err != nil {
		slogctx.FromCtx(ctx).WarnContext(ctx, "Unable to show top categories.",
			slog.Any("error", err))
	}
	categoryBuckets, ok := topCategoriesAgg.Buckets.([]types.StringTermsBucket)
	if ok {
		// Generate categories from aggregation.
		data.TopCategories = make(models.CategoryCounts, 0)
		data.TopArticles = make(models.Articles, 0)
		for category := range slices.Values(categoryBuckets) {
			data.TopCategories = append(data.TopCategories, models.CategoryCount{Category: category.Key.(string), Count: int(category.DocCount)})
			// Get top article.
			topHitsAgg, err := aggregations.ExtractAggregation[*types.TopHitsAggregate](category.Aggregations, "top_articles")
			if err != nil {
				continue
			}
			items, _, err := results.ExtractSourceFromHits[*models.Item](topHitsAgg.Hits.Hits)
			if err != nil {
				continue
			}
			articles, err := models.GenerateArticles(ctx, items)
			if err != nil {
				continue
			}
			data.TopArticles = append(data.TopArticles, articles...)

		}
		// Sort categories by count.
		data.TopCategories.Sort()
		// Remove duplicate articles.
		slices.SortFunc(data.TopArticles, func(a, b *models.Article) int {
			return strings.Compare(a.GetID(), b.GetID())
		})
		data.TopArticles = slices.CompactFunc(data.TopArticles, func(a, b *models.Article) bool {
			return a.GetID() == b.GetID()
		})
	}

	// Get the rare categories.
	rareCategoriesAgg, err := aggregations.ExtractAggregation[*types.StringRareTermsAggregate](queryResult.Aggregations, "rare_categories")
	if err != nil {
		slogctx.FromCtx(ctx).WarnContext(ctx, "Unable to show rare categories.",
			slog.Any("error", err))
	}
	// Generate category counts from buckets.
	rareCategoryBuckets, ok := rareCategoriesAgg.Buckets.([]types.StringRareTermsBucket)
	if ok {
		data.RareCategories = make(models.CategoryCounts, 0)
		for category := range slices.Values(rareCategoryBuckets) {
			data.RareCategories = append(data.RareCategories, models.CategoryCount{Category: category.Key, Count: int(category.DocCount)})
		}
		data.RareCategories.Sort()
		if len(data.RareCategories) > 10 {
			data.RareCategories = data.RareCategories[:10]
		}
	}

	return data, nil
}
