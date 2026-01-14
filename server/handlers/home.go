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
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic/aggregations"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/providers/elastic/results"
	"github.com/immanent-tech/foragd/web/templates"
)

// Home handles displaying the user's home page.
func Home() http.HandlerFunc {
	return defaultHandlerChain.Append(setCacheControl).
		ThenFunc(showOnError(func(res http.ResponseWriter, req *http.Request) error {
			user, err := models.UserFromCtx(req.Context())
			if err != nil {
				return &models.APIError{
					InternalError: fmt.Errorf("get user data: %w", err),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Could not display home page",
						"This might be temporary, please try again.",
					),
				}
			}
			if user.GetSettings().ShowOnboarding {
				renderPage(
					templates.NewPage(
						wrapContent(req, templates.NewUserHome()),
						templates.WithPageTitle("Home"),
					),
				).ServeHTTP(res, req)
				return nil
			}

			data, err := getHomePageData(req.Context())
			if err != nil {
				return &models.APIError{
					InternalError: fmt.Errorf("run data collection: %w", err),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Could not display home page",
						"This might be temporary, please try again.",
					),
				}
			}
			renderPage(
				templates.NewPage(
					wrapContent(req, data.Template()),
					templates.WithPageTitle("Home"),
				),
			).ServeHTTP(res, req)
			return nil
		})).
		ServeHTTP
}

// WatchHome handles watching the home page content (namely, latest articles) for updates.
func WatchHome() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		filters := models.PageFiltersFromCtx(req.Context(), req.URL.Path)
		query, err := models.BuildItemsQuery(req.Context(), filters)
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Cannot generate query for updates.",
				slog.Any("error", err))
			res.WriteHeader(http.StatusInternalServerError)
			return
		}
		// Watch for updates to home page articles.
		watchForUpdates(query).ServeHTTP(res, req)
	}).ServeHTTP
}

// getHomePageData retrieves the data required to construct the homepage content.
func getHomePageData(ctx context.Context) (*templates.Home, error) {
	data := &templates.Home{}
	// Retrieve user object.
	user, err := models.UserFromCtx(ctx)
	if err != nil {
		return data, fmt.Errorf("unable to retrieve user: %w", err)
	}
	data.User = user

	// Retrieve unread subscriptions and articles.
	subscriptions, articles, err := getHomePageObjects(ctx, user)
	if err != nil {
		return data, fmt.Errorf("unable to retrieve subscriptions and/or articles: %w", err)
	}

	data.Subscriptions = subscriptions
	data.LatestArticles = articles

	// Retrieve statistics.
	if err := performHomePageAggs(ctx, user, data); err != nil {
		return data, fmt.Errorf("unable to perform aggregations: %w", err)
	}

	return data, nil
}

// getHomePageObjects retrieves all unread subscriptions and the latest (max 10) unread articles.
func getHomePageObjects(ctx context.Context, user *models.User) (models.Subscriptions, models.Articles, error) {
	// Use default filters.
	filters := models.NewListDisplayFilters()

	// Get all user subscriptions.
	subscriptions, err := models.GetSubscriptions(ctx,
		models.GetSubscriptionsDynamicInfo(true),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("get subscriptions: %w", err)
	}
	subscriptions = subscriptions.
		FilterByView(filters.GetView())

	// Generate a query for getting articles.
	articleQuery := query.Bool(
		query.Filter(
			// Must match any of the given categories.
			query.Terms("feed_id", subscriptions.GetFeedIDs()...),
			query.Terms("categories.raw", filters.GetCategories()...),
			query.Bool(
				query.Should(models.BuildItemQueries(user, filters.GetView(), subscriptions)...),
			),
		),
	)

	sort := filters.GetSort()

	// Find items matching filters.
	items, _, err := models.SearchItems(ctx, articleQuery, filters.GetCount(), &sort, "")
	if err != nil {
		return nil, nil, fmt.Errorf("get items: %w", err)
	}

	// Generate articles.
	articles, err := models.GenerateArticles(ctx, items)
	if err != nil {
		return nil, nil, fmt.Errorf("generate articles: %w", err)
	}

	return subscriptions, articles, nil
}

// performHomePageAggs performs aggregations to get statistics and samples of the top unread subscriptions and articles.
func performHomePageAggs(ctx context.Context, user *models.User, data *templates.Home) error {
	// Fetch aggregation data.
	termsField := "categories.raw"
	// Aggregation definition for fetching the top 10 item categories across all subscriptions.
	sampleField := "feed_id"
	defaultMaxDocsPerValue := 8
	shardSize := 1000
	topHitsCount := 1
	maxDocCount := int64(5)
	aggs := aggregations.Aggs{
		// top_categories_sample: diversified sampler to ensure top categories not dominated by single overwhelming
		// source.
		"top_categories_sample": types.Aggregations{
			DiversifiedSampler: &types.DiversifiedSamplerAggregation{
				Field:           &sampleField,
				MaxDocsPerValue: &defaultMaxDocsPerValue,
				ShardSize:       &shardSize,
			},
			Aggregations: map[string]types.Aggregations{
				// top_categories: the top categories across all subscriptions.
				"top_categories": {
					Terms: &types.TermsAggregation{
						Field:   &termsField,
						Exclude: models.CommonCategoryFilters,
					},
					Aggregations: map[string]types.Aggregations{
						// top_articles: the top scoring article for each top category.
						"top_articles": {
							Filter: query.Build(query.Bool(
								query.MustNot(
									query.Terms("item_id", data.LatestArticles.GetIDs()...),
								),
							)),
							Aggregations: map[string]types.Aggregations{
								"top_article_hits": {
									TopHits: &types.TopHitsAggregation{
										Size: &topHitsCount,
									},
								},
							},
						},
					},
				},
			},
		},
		// Aggregation definition for fetching the rare item categories across all subscriptions.
		"rare_categories": types.Aggregations{
			RareTerms: &types.RareTermsAggregation{
				Field:       &termsField,
				MaxDocCount: &maxDocCount,
				Exclude:     models.CommonCategoryFilters,
			},
		},
	}

	// Perform the request.
	filters := models.NewListDisplayFilters()
	queryResult, err := models.ItemsAggregation(ctx, query.Bool(
		query.Filter(
			// Must match any of the given categories.
			query.Terms("feed_id", data.Subscriptions.GetFeedIDs()...),
			query.Terms("categories.raw", filters.GetCategories()...),
			query.Bool(
				query.Should(models.BuildItemQueries(user, filters.GetView(), data.Subscriptions)...),
			),
		),
	), 0, aggs)
	if err != nil {
		return fmt.Errorf("unable to calculate aggregations: %w", err)
	}

	// Get the top categories.
	if topCategoriesSamplerAgg, ok := queryResult.Aggregations["top_categories_sample"].(*types.SamplerAggregate); ok {
		var topCategoriesAgg *types.StringTermsAggregate
		if topCategoriesAgg, ok = topCategoriesSamplerAgg.Aggregations["top_categories"].(*types.StringTermsAggregate); ok {
			var categoryBuckets []types.StringTermsBucket
			if categoryBuckets, ok = topCategoriesAgg.Buckets.([]types.StringTermsBucket); ok {
				// Generate categories from aggregation.
				data.TopCategories = make(models.CategoryCounts, 0)
				data.TopArticles = make(models.Articles, 0)
				for category := range slices.Values(categoryBuckets) {
					var value string
					if value, ok = category.Key.(string); !ok {
						continue
					}
					data.TopCategories = append(
						data.TopCategories,
						models.CategoryCount{Category: value, Count: int(category.DocCount)},
					)
					// Get top article.
					var topArticlesAgg *types.FilterAggregate
					if topArticlesAgg, ok = category.Aggregations["top_articles"].(*types.FilterAggregate); !ok {
						continue
					}
					var topHitsAgg *types.TopHitsAggregate
					if topHitsAgg, ok = topArticlesAgg.Aggregations["top_article_hits"].(*types.TopHitsAggregate); !ok {
						continue
					}
					var items models.Items
					if items, _, err = results.ExtractSourceFromHits[*models.Item](topHitsAgg.Hits.Hits); err != nil {
						continue
					}
					var articles models.Articles
					if articles, err = models.GenerateArticles(ctx, items); err != nil {
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
		}
	}
	// Get the rare categories.
	if rareCategoriesAgg, ok := queryResult.Aggregations["rare_categories"].(*types.StringRareTermsAggregate); ok {
		// Generate category counts from buckets.
		var rareCategoryBuckets []types.StringRareTermsBucket
		if rareCategoryBuckets, ok = rareCategoriesAgg.Buckets.([]types.StringRareTermsBucket); ok {
			data.RareCategories = make(models.CategoryCounts, 0)
			for category := range slices.Values(rareCategoryBuckets) {
				data.RareCategories = append(
					data.RareCategories,
					models.CategoryCount{Category: category.Key, Count: int(category.DocCount)},
				)
			}
			data.RareCategories.Sort()
			maxRareCategoriesCount := 10
			if len(data.RareCategories) > maxRareCategoriesCount {
				data.RareCategories = data.RareCategories[:maxRareCategoriesCount]
			}
		}
	}

	return nil
}
