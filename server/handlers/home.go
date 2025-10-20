// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/justinas/nosurf"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic/aggregations"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/providers/elastic/results"
	"github.com/immanent-tech/foragd/web/templates/layouts"
	"github.com/immanent-tech/foragd/web/templates/partials"
)

// Home handles displaying the user's home page.
func (a *API) Home() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		ctx := req.Context()
		ctx = models.CSRFTokenToCtx(ctx, nosurf.Token(req))
		user, err := models.UserFromCtx(ctx)
		if err != nil {
			return fmt.Errorf("unable to serve home page: %w", err)
		}
		if user.GetSettings().ShowOnboarding {
			template := layouts.NewUserHome()
			renderPage(template, "Home - "+config.AppName).ServeHTTP(res, req.WithContext(ctx))
			return nil
		}
		data, err := a.getHomePageData(ctx)
		if err != nil {
			renderPartial(partials.Notification(
				models.NewErrorMessage(
					"Unable to get home page data",
					"Something went wrong, please try again",
				), 0))
			return models.NewAPIError(
				fmt.Errorf("unable to mark subscription: %w", err),
				http.StatusInternalServerError)
		}
		template := data.Template()
		renderPage(template, "Home - "+config.AppName).ServeHTTP(res, req.WithContext(ctx))
		return nil
	})).ServeHTTP
}

// getHomePageData retrieves the data required to construct the homepage content.
//
//nolint:funlen // mostly aggregation definitions.
func (a *API) getHomePageData(ctx context.Context) (*layouts.Home, error) {
	data := &layouts.Home{}
	// Retrieve user object.
	user, err := models.UserFromCtx(ctx)
	if err != nil {
		return data, fmt.Errorf("unable to retrieve user: %w", err)
	}
	data.User = user
	data.Favorites = user.GetFavorites()
	// User has no subscriptions, show empty page
	if len(user.GetSubscriptionMetadata()) == 0 {
		return data, nil
	}
	// Get subscriptions.
	subscriptions, err := models.GetSubscriptions(ctx, a.Elastic, user.GetSubscriptionMetadata().GetIDs()...)
	if err != nil {
		return data, fmt.Errorf("unable to retrieve subscriptions: %w", err)
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
		return nil, fmt.Errorf("unable to retrieve articles: %w", err)
	}
	data.LatestArticles, err = models.GenerateArticles(ctx, latestItems)
	if err != nil {
		return nil, fmt.Errorf("unable to generate articles: %w", err)
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
							TopHits: &types.TopHitsAggregation{
								Size: &TopHitsCount,
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
	queryResult, err := a.DataAPI().ItemsAggregation(ctx, query, 0, aggs)
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
					topHitsAgg, ok := category.Aggregations["top_articles"].(*types.TopHitsAggregate)
					if !ok {
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
