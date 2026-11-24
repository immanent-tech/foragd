/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"sync"

	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/opensearch"
	"github.com/immanent-tech/foragd/providers/opensearch/search"
	"github.com/immanent-tech/foragd/schema"
)

type Home struct {
	osc *opensearch.Client
}

func NewHomeService(osc *opensearch.Client) *Home {
	return &Home{osc: osc}
}

func (s *Home) AggregateSubscriptions(
	ctx context.Context,
	user *models.User,
	subscriptions models.Subscriptions,
) (models.CategoryCounts, models.CategoryCounts, models.Articles, error) {
	// Don't perform aggregations if there is no data to aggregate.
	if len(subscriptions.GetFeedIDs()) == 0 {
		return nil, nil, nil, nil
	}

	// Perform the request.
	resp, err := search.Search(ctx, s.osc, s.osc.IndexReadAlias(schema.ItemsIndex),
		search.WithQuery(
			search.BoolQuery(
				search.WithBoolFilter(
					search.TermsQuery("feed_id", subscriptions.GetFeedIDs()),
					search.BoolQuery(
						search.WithBoolShould(
							BuildItemQueries(user, models.ViewUnread, subscriptions)...),
					),
				),
			),
		),
		search.WithAggregation(
			"top_categories_sample",
			search.DiversifiedSamplerAggregation(
				search.WithDiversifiedSamplerAggregationField("feed_id"),
				search.WithDiversifiedSamplerAggregationMaxDocsPerValue(1),
				search.WithDiversifiedSamplerAggregationShardSize(200),
			),
			search.WithChildAggregation(
				"top_categories",
				search.TermsAggregation(
					search.WithTermsAggregationField("categories.raw"),
					search.WithTermsAggregationExclude(models.CommonCategoryFilters...),
				),
				search.WithChildAggregation(
					"top_article_hits",
					search.TopHitsAggregation(
						search.WithTopHitsAggregationSize(3),
					),
				),
			),
			search.WithChildAggregation(
				"latest_articles_sample",
				search.TopHitsAggregation(
					search.WithTopHitsAggregationSize(10),
				),
			),
		),
		search.WithAggregation(
			"rare_categories",
			search.RareTermsAggregation(
				search.WithRareTermsAggregationField("categories.raw"),
				search.WithRareTermsAggregationMaxDocCount(3),
				search.WithRareTermsAggregationExclude(models.CommonCategoryFilters...),
			),
		),
		search.WithSize(0),
		search.WithTrackTotalHits(false),
		search.WithTrackScores(false),
		search.WithSort(search.NewSortOptions(search.WithSortField("_doc"))),
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("aggregate: %w", err)
	}
	// Extract the feed aggregation. We need to use a custom type as we cannot access nested aggregations from the typed
	// API at the moment.
	type homeAggs struct {
		Aggregations struct {
			TopCategoriesSample struct {
				DocCount             int64 `json:"doc_count"`
				LatestArticlesSample struct {
					Hits struct {
						Hits []struct {
							ID     string          `json:"_id"`
							Source json.RawMessage `json:"_source"`
						} `json:"hits"`
					} `json:"hits"`
				} `json:"latest_articles_sample"`
				TopCategories struct {
					Buckets []struct {
						Key            string `json:"key"`
						DocCount       int64  `json:"doc_count"`
						TopArticleHits struct {
							Hits struct {
								Hits []struct {
									ID     string          `json:"_id"`
									Source json.RawMessage `json:"_source"`
								} `json:"hits"`
							} `json:"hits"`
						} `json:"top_article_hits"`
					} `json:"buckets"`
				} `json:"top_categories"`
			} `json:"top_categories_sample"`
			RareCategories struct {
				Buckets []struct {
					Key      string `json:"key"`
					DocCount int64  `json:"doc_count"`
				} `json:"buckets"`
			} `json:"rare_categories"`
		} `json:"aggregations"`
	}
	var homeAggsResults homeAggs
	if err := json.NewDecoder(resp.RawBody()).Decode(&homeAggsResults); err != nil {
		return nil, nil, nil, fmt.Errorf("extract aggregation results: %w", err)
	}

	var wg sync.WaitGroup

	// Get the top categories.
	topCategories := make(models.CategoryCounts, 0)
	wg.Go(func() {
		for categoryBucket := range slices.Values(homeAggsResults.Aggregations.TopCategoriesSample.TopCategories.Buckets) {
			if categoryBucket.Key == "" {
				continue
			}
			newCategory := models.CategoryCount{Category: categoryBucket.Key, Count: int(categoryBucket.DocCount)}
			topCategories = append(topCategories, newCategory)
		}
	})

	// Get the rare categories.
	rareCategories := make(models.CategoryCounts, 0)
	wg.Go(func() {
		for rareCategoryBucket := range slices.Values(homeAggsResults.Aggregations.RareCategories.Buckets) {
			if rareCategoryBucket.Key == "" {
				continue
			}
			rareCategories = append(
				rareCategories,
				models.CategoryCount{Category: rareCategoryBucket.Key, Count: int(rareCategoryBucket.DocCount)},
			)
		}
		rareCategories.Sort()
		if len(rareCategories) > 10 {
			rareCategories = rareCategories[:10]
		}
	})

	// Get the latest articles.
	latestArticles := make(models.Articles, 0)
	wg.Go(func() {
		var (
			items models.Items
		)
		for hit := range slices.Values(homeAggsResults.Aggregations.TopCategoriesSample.LatestArticlesSample.Hits.Hits) {
			source, err := opensearch.ExtractSource[*models.Item](hit.Source)
			if err != nil {
				slogctx.Warn(
					ctx,
					"Could not extract latest item.",
					slog.Any("error", err),
				)
				continue
			}
			items = append(items, source)
		}
		items = items.SortByTimestamp()
		var articles models.Articles
		if articles, err = GenerateArticles(ctx, items); err != nil {
			slogctx.Warn(ctx, "Could not generate articles from items.",
				slog.Any("error", err),
			)
		}
		latestArticles = articles
	})

	wg.Wait()

	return topCategories, rareCategories, latestArticles, nil
}
