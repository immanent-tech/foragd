/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package service

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sync"

	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/providers/elastic/results"
)

type Home struct{}

func (s *Home) AggregateSubscriptions(
	ctx context.Context,
	user *models.User,
	subscriptions models.Subscriptions,
) (models.CategoryCounts, models.CategoryCounts, models.Articles, error) {
	// Don't perform aggregations if there is no data to aggregate.
	if len(subscriptions.GetFeedIDs()) == 0 {
		return nil, nil, nil, nil
	}
	// Fetch aggregation data.
	termsField := "categories.raw"
	// Aggregation definition for fetching the top 10 item categories across all subscriptions.
	sampleField := "feed_id"
	defaultMaxDocsPerValue := 1
	shardSize := 200
	topCategoryHitsCount := 3
	lastestArticlesCount := 10
	maxDocCount := int64(3)

	// Perform the request.
	resp, err := elastic.Search[*models.Item](
		ctx,
		schema.ItemsIndexRO(),
		// Query is adapted from service.FilterArticles query to boost favorites and return unread only.
		elastic.WithQueryOptions[*elastic.SearchRequest](
			query.Bool(
				query.Filter(
					query.Terms(
						"feed_id",
						subscriptions.GetFeedIDs(),
						query.WithQueryName[*query.TermsQuery]("match-feed-id"),
					),
					query.Bool(
						query.Should(
							BuildItemQueries(user, models.ViewUnread, subscriptions)...),
					),
				),
			),
		),
		elastic.WithAggregations(
			elastic.Aggs{
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
								Exclude: slices.Concat(models.CommonCategoryFilters, []string{""}),
							},
							Aggregations: map[string]types.Aggregations{
								"top_article_hits": {
									TopHits: &types.TopHitsAggregation{
										Size: &topCategoryHitsCount,
									},
								},
								// top_articles: the top scoring article for each top category.
								// "top_articles": {
								// 	Filter: query.Build(query.Bool(
								// 		query.MustNot(
								// 			query.Terms("item_id", data.LatestArticles.GetIDs()...),
								// 		),
								// 	)),
								// 	Aggregations: map[string]types.Aggregations{
								// 		"top_article_hits": {
								// 			TopHits: &types.TopHitsAggregation{
								// 				Size: &topHitsCount,
								// 			},
								// 		},
								// 	},
								// },
							},
						},
						"latest_articles_sample": {
							TopHits: &types.TopHitsAggregation{
								Size: &lastestArticlesCount,
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
			},
		),
		elastic.WithSize(0),
		elastic.WithDocSorting(),
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("aggregate: %w", err)
	}

	// slogctx.FromCtx(req.Context()).Debug("Performed home aggregations.",
	// 	slog.Duration("took", time.Since(start)))

	topCategoriesSamplerAgg, found, err := elastic.ExtractAggregation[*types.SamplerAggregate](
		resp.Aggregations,
		"top_categories_sample",
	)
	if !found || err != nil {
		return nil, nil, nil, fmt.Errorf("extract aggregation: %w", err)
	}

	var wg sync.WaitGroup

	// Get the top categories.
	topCategories := make(models.CategoryCounts, 0)
	wg.Go(func() {
		if topCategoriesAgg, foundTopCategoriesAgg, err := elastic.ExtractAggregation[*types.StringTermsAggregate](
			topCategoriesSamplerAgg.Aggregations,
			"top_categories",
		); foundTopCategoriesAgg &&
			err == nil {
			if categoryBuckets, ok := topCategoriesAgg.Buckets.([]types.StringTermsBucket); ok {
				// Generate categories from aggregation.
				for category := range slices.Values(categoryBuckets) {
					var value string
					if value, ok = category.Key.(string); !ok {
						continue
					}
					// // Get top articles.
					// topHitsAgg, found, err := elastic.ExtractAggregation[*types.TopHitsAggregate](
					// 	category.Aggregations,
					// 	"top_article_hits",
					// )
					// if !found || err != nil {
					// 	continue
					// }
					// var items models.Items
					// if items, _, err = results.ExtractSourceFromHits[*models.Item](
					// 	topHitsAgg.Hits.Hits,
					// ); err != nil {
					// 	continue
					// }
					// var articles models.Articles
					// if articles, err = service.GenerateArticles(req.Context(), items); err != nil {
					// 	continue
					// }
					newCategory := models.CategoryCount{Category: value, Count: int(category.DocCount)}
					topCategories = append(topCategories, newCategory)
				}
				// // Remove duplicate articles.
				// slices.SortFunc(data.TopArticles, func(a, b *models.Article) int {
				// 	return strings.Compare(a.GetID(), b.GetID())
				// })
			}
		}
	})

	// Get the rare categories.
	rareCategories := make(models.CategoryCounts, 0)
	wg.Go(func() {
		if rareCategoriesAgg, found, err := elastic.ExtractAggregation[*types.StringRareTermsAggregate](
			resp.Aggregations,
			"rare_categories",
		); found && err == nil {
			// Generate category counts from buckets.
			if rareCategoryBuckets, ok := rareCategoriesAgg.Buckets.([]types.StringRareTermsBucket); ok {
				for category := range slices.Values(rareCategoryBuckets) {
					rareCategories = append(
						rareCategories,
						models.CategoryCount{Category: category.Key, Count: int(category.DocCount)},
					)
				}
				rareCategories.Sort()
				if len(rareCategories) > 10 {
					rareCategories = rareCategories[:10]
				}
			}
		}
	})

	// Get the latest articles.
	latestArticles := make(models.Articles, 0)
	wg.Go(func() {
		if latestArticlesSampleAgg, found, err := elastic.ExtractAggregation[*types.TopHitsAggregate](
			topCategoriesSamplerAgg.Aggregations,
			"latest_articles_sample",
		); found && err == nil {
			var items models.Items
			if items, _, err = results.ExtractSourceFromHits[*models.Item](
				latestArticlesSampleAgg.Hits.Hits,
			); err != nil {
				slogctx.Warn(ctx, "Could not extract latest items from aggregation.",
					slog.Any("error", err),
				)
			}
			var articles models.Articles
			if articles, err = GenerateArticles(ctx, items); err != nil {
				slogctx.Warn(ctx, "Could not generate articles from items.",
					slog.Any("error", err),
				)
			}
			latestArticles = articles
		}
	})

	wg.Wait()

	return topCategories, rareCategories, latestArticles, nil
}
