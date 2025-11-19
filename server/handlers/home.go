// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/aggregations"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/providers/elastic/results"
	"github.com/immanent-tech/foragd/web/templates"
)

// Home handles displaying the user's home page.
func (a *API) Home() http.HandlerFunc {
	return defaultHandlerChain.Append(setCacheControl).ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		pageTitle := templates.GeneratePageTitle("Home")
		ctx := req.Context()
		user, err := models.UserFromCtx(ctx)
		if err != nil {
			msg := models.NewErrorMessage("Unable to complete request!", "This might be temporary, please try again.")
			renderPage(templates.ErrorPage(msg), pageTitle).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to retrieve user data: %w", err), http.StatusInternalServerError)
		}
		if user.GetSettings().ShowOnboarding {
			template := templates.NewUserHome()
			renderPage(template, pageTitle).ServeHTTP(res, req.WithContext(ctx))
			return nil
		}
		data, err := a.getHomePageData(ctx)
		if err != nil {
			msg := models.NewErrorMessage("Unable to complete request!", "This might be temporary, please try again.")
			renderPage(templates.ErrorPage(msg), pageTitle).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to retrieve home page data: %w", err), http.StatusInternalServerError)
		}
		template := data.Template()
		renderPage(template, pageTitle).ServeHTTP(res, req.WithContext(ctx))
		return nil
	})).ServeHTTP
}

// WatchHome handles watching the home page content (namely, latest articles) for updates.
func WatchHome(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		filters := models.PageFiltersFromCtx(req.Context(), req.URL.Path)
		query, err := models.BuildItemsQuery(req.Context(), api, filters)
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Cannot generate query for updates.",
				slog.Any("error", err))
			res.WriteHeader(http.StatusInternalServerError)
			return
		}
		// Watch for updates to home page articles.
		watchForUpdates(api, query).ServeHTTP(res, req)
	}).ServeHTTP
}

// getHomePageData retrieves the data required to construct the homepage content.
//
//nolint:funlen,gocognit,nestif // mostly aggregation definitions.
func (a *API) getHomePageData(ctx context.Context) (*templates.Home, error) {
	data := &templates.Home{}
	// Retrieve user object.
	user, err := models.UserFromCtx(ctx)
	if err != nil {
		return data, fmt.Errorf("unable to retrieve user: %w", err)
	}
	data.User = user

	subscriptionQuery := query.Bool(
		query.Filter(
			query.Term("user_id", user.GetID()),
		),
	)
	subscriptions, err := a.DataAPI().GetAllSubscriptions(ctx, subscriptionQuery)
	if err != nil {
		return nil, fmt.Errorf("filter articles: get subscriptions: %w", err)
	}
	// Return early if there the user has no subscriptions (i.e., new user).
	if len(subscriptions) == 0 {
		return nil, models.ErrNotFound
	}

	data.SubscriptionsCount = len(subscriptions)
	// Query definition for fetching unread items for all subscriptions.
	articlesQuery := query.Bool(
		query.BoolQueryName("item_filters"),
		query.Filter(
			// Must match any of the given feed IDs.
			query.Terms("feed_id", subscriptions.GetFeedIDs()...),
			query.Bool(
				query.Should(models.BuildSubscriptionQueries(user, models.ViewUnread, subscriptions)...),
			),
		),
	)

	// Fetch latest articles.
	sort := models.SortNewestFirst
	latestItems, _, err := a.DataAPI().SearchItems(ctx, articlesQuery, 12, &sort, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve articles: %w", err)
	}
	data.LatestArticles, err = models.GenerateArticles(ctx, a.DataAPI(), latestItems)
	if err != nil && !errors.Is(err, models.ErrNotFound) {
		return nil, fmt.Errorf("unable to generate articles: %w", err)
	}

	if errors.Is(err, models.ErrNotFound) {
		return data, nil
	}

	// Fetch aggregation data.
	TermsField := "categories.raw"
	// Aggregation definition for fetching the top 10 item categories across all subscriptions.
	SampleField := "feed_id"
	DefaultMaxDocsPerValue := 10
	ShardSize := 1000
	TopHitsCount := 1
	MaxDocCount := int64(5)
	aggs := aggregations.Aggs{
		// top_categories_sample: diversified sampler to ensure top categories not dominated by single overwhelming
		// source.
		"top_categories_sample": types.Aggregations{
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
							Filter: query.Build(query.Bool(
								query.MustNot(
									query.Terms("feed_id", data.LatestArticles.GetFeedIDs()...),
								),
							)),
							Aggregations: map[string]types.Aggregations{
								"top_article_hits": {
									TopHits: &types.TopHitsAggregation{
										Size: &TopHitsCount,
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
				Field:       &TermsField,
				MaxDocCount: &MaxDocCount,
				Exclude:     models.CommonCategoryFilters,
			},
		},
	}

	// Perform the request.
	queryResult, err := a.DataAPI().ItemsAggregation(ctx, articlesQuery, 0, aggs)
	if err != nil {
		return nil, fmt.Errorf("unable to calculate aggregations: %w", err)
	}

	// Get the top categories.
	topCategoriesSamplerAgg, ok := queryResult.Aggregations["top_categories_sample"].(*types.SamplerAggregate)
	if ok {
		topCategoriesAgg, ok := topCategoriesSamplerAgg.Aggregations["top_categories"].(*types.StringTermsAggregate)
		if ok {
			categoryBuckets, ok := topCategoriesAgg.Buckets.([]types.StringTermsBucket)
			if ok {
				// Generate categories from aggregation.
				data.TopCategories = make(models.CategoryCounts, 0)
				data.TopArticles = make(models.Articles, 0)
				for category := range slices.Values(categoryBuckets) {
					value, ok := category.Key.(string)
					if !ok {
						continue
					}
					data.TopCategories = append(data.TopCategories, models.CategoryCount{Category: value, Count: int(category.DocCount)})
					// Get top article.
					topArticlesAgg, ok := category.Aggregations["top_articles"].(*types.FilterAggregate)
					if !ok {
						continue
					}
					topHitsAgg, ok := topArticlesAgg.Aggregations["top_article_hits"].(*types.TopHitsAggregate)
					if !ok {
						continue
					}
					items, _, err := results.ExtractSourceFromHits[*models.Item](topHitsAgg.Hits.Hits)
					if err != nil {
						continue
					}
					articles, err := models.GenerateArticles(ctx, a.DataAPI(), items)
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
		}
	}
	// Get the rare categories.
	rareCategoriesAgg, ok := queryResult.Aggregations["rare_categories"].(*types.StringRareTermsAggregate)
	if ok {
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
	}

	return data, nil
}
