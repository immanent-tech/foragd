// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package service

import (
	"context"
	"fmt"
	"slices"

	estypes "github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/operator"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/providers/elastic/retriever"
)

// buildSearchResultsQuery generates a query that can be used to fetch appropriate results for a given SearchRequest
// criteria.
func buildSearchResultsQuery(
	ctx context.Context,
	user *models.User,
	request *models.SearchRequest,
	clause query.Option,
) query.Option {
	// Get user subscriptions.
	allSubscriptions := models.SubscriptionsFromCtx(ctx)
	if len(allSubscriptions) == 0 {
		slogctx.Warn(ctx, "Could not retrieve user subscriptions from context.")
		return nil
	}
	subscriptions := allSubscriptions.FilterByIDs(request.Subscriptions...)

	return query.Bool(
		query.Must(
			query.Bool(
				query.WithBoolQueryName("search-filters"),
				query.Filter(
					query.Bool(
						// Must satisfy user global filters.
						ArticleFiltersQueryClause(user.GetSettings().GlobalFilters),
						// Must be in the given user subscriptions.
						query.Should(BuildItemQueries(user, request.View, subscriptions)...),
					),
					// Must be published/updated since the given time.
					query.Bool(
						query.Should(
							query.Since("published", request.Since()),
							query.Since("updated", request.Since()),
						),
					),
				),
				query.Should(
					// Boost items that are from a favorite subscription.
					query.Terms(
						"feed_id",
						subscriptions.FilterByView(models.ViewFavorites).GetFeedIDs(),
						query.WithQueryName[*query.TermsQuery]("boost-favorites"),
						query.WithQueryBoost[*query.TermsQuery](2.0),
					),
					// Boost documents closer to the current time.
					query.Distance("published", request.Pivot(), "now"),
					query.Distance("updated", request.Pivot(), "now"),
				),
			),
			clause,
		),
	)
}

func standardSearchResultsClause(search *models.SearchRequest) query.Option {
	// Must match either: search term in any of the fields, or, matches directly as a search-as-you-type (same as
	// search suggestion).
	return query.Bool(
		query.Must(
			query.Bool(
				query.Should(
					// Boost match title exactly.
					query.Term("title.exact", search.Text, query.WithQueryBoost[*query.TermQuery](10.0)),
					// Simple query string across most fields. Boost title and description matches.
					query.SimpleQueryString(
						query.WithSimpleQueryStringText(&search.Text),
						query.WithSimpleQueryStringFields(
							"title^3",
							"description^2",
							"content",
							"categories",
							"authors",
							"contributors",
						),
						query.WithSimpleQueryStringOperator(&operator.And),
					),
				),
			),
		),
	)
}

func semanticSearchResultsClause(search *models.SearchRequest) query.Option {
	// Perform semantic search on content field for text.
	return query.Match("content_semantic", search.Text)
}

func searchSuggestionsClause(search *models.SearchRequest) query.Option {
	// Must match at least one of in title, description, content.
	return query.Bool(
		query.Must(
			query.Bool(
				query.Should(
					query.Term("title.exact", search.Text, query.WithQueryBoost[*query.TermQuery](10.0)),
					query.SearchAsYouType(search.Text, "title"),
					query.SearchAsYouType(search.Text, "description"),
					query.SimpleQueryString(
						query.WithSimpleQueryStringText(&search.Text),
						query.WithSimpleQueryStringFields("content"),
						query.WithSimpleQueryStringOperator(&operator.And),
					),
				),
			),
		),
	)
}

// SearchItems will search the items index for items matching the given query. Count, sort and pagination values are
// optional.
func QueryItems(
	ctx context.Context,
	query query.Option,
	count int,
	sort *models.Sort,
	pagination *string,
) (models.Items, string, error) {
	searchAfter, err := elastic.DecodePagination(pagination)
	if err != nil {
		return nil, "", models.ErrInvalidParams
	}
	// Perform search.
	resp, err := elastic.Search[*models.Item](ctx,
		schema.ItemsIndexRO(),
		elastic.WithQueryOptions[*elastic.SearchRequest](query),
		elastic.WithSort(NewItemSortOptions(sort)...),
		elastic.WithSearchAfter(searchAfter...),
		elastic.WithSize(count),
	)
	if err != nil {
		return nil, "", fmt.Errorf("search items: %w", err)
	}
	// Parse last search after value into pagination.
	newPagination, err := elastic.EncodePagination[string](resp.Pagination)
	if err != nil {
		return nil, "", models.ErrInvalidParams
	}
	// Update cache.
	for item := range slices.Values(resp.Results) {
		itemsCache.Invalidate(item.GetID())
		itemsCache.Set(item.GetID(), item)
	}
	return resp.Results, newPagination, nil
}

func SuggestItems(ctx context.Context, request *models.SearchRequest) (models.Items, error) {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get user details: %w", models.ErrCtxValueNotFound)
	}

	clause := buildSearchResultsQuery(ctx, user, request, searchSuggestionsClause(request))

	items, _, err := QueryItems(ctx, clause, request.Count, &request.Sort, nil)
	if err != nil {
		return nil, fmt.Errorf("query items: %w", err)
	}

	return items, nil
}

// RetrieveItems will retrieve a [models.Items] slice of items matching the given [*models.SearchRequest]. The request
// supports pagination and will return the next set of results as appropriate.
func RetrieveItems(ctx context.Context, request *models.SearchRequest) (models.Items, models.Pagination, error) {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, models.Pagination{}, fmt.Errorf("retrieve items: get user: %w", models.ErrCtxValueNotFound)
	}

	var from int
	if request.GetPagination().From == nil {
		from = 0
	} else {
		from = *request.GetPagination().From
	}

	filter := buildSearchResultsQuery(ctx, user, request, nil)

	// Perform search.
	resp, err := elastic.Search[*models.Item](ctx,
		schema.ItemsIndexRO(),
		elastic.WithRetriever(
			retriever.WithReciprocalRankFusionRetriever(
				retriever.WithRankWindowSize(150),
				retriever.WithQueryFilters(filter),
				retriever.WithChildRetrievers(
					retriever.WithStandardRetriever(
						"retriever-regular",
						standardSearchResultsClause(request),
					),
					retriever.WithStandardRetriever(
						"retriver-semantic",
						semanticSearchResultsClause(request),
					),
				),
			),
		),
		// elastic.WithSort(NewItemSortOptions(sort)...),
		elastic.WithFrom(from),
		elastic.WithSize(request.Count),
	)
	if err != nil {
		return nil, models.Pagination{}, fmt.Errorf("search items: %w", err)
	}
	// Update cache.
	for item := range slices.Values(resp.Results) {
		itemsCache.Invalidate(item.GetID())
		itemsCache.Set(item.GetID(), item)
	}
	// Parse last search after value into pagination.
	return resp.Results, models.Pagination{From: new(from + request.Count)}, nil
}

// CountSearchResults will return an approximate count of the number of items that would match the given
// [*models.SearchRequest], published within the last 5 minutes.
func CountSearchResults(ctx context.Context, request *models.SearchRequest) (int64, error) {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return 0, fmt.Errorf("get user details: %w", models.ErrCtxValueNotFound)
	}

	// Override the published within on the search request to last 5 minutes for updates.
	request.PublishedWithin = models.SearchRequestPublishedWithinLast5mins

	filter := buildSearchResultsQuery(ctx, user, request, nil)

	// Perform search.
	resp, err := elastic.Search[*models.Item](ctx,
		schema.ItemsIndexRO(),
		elastic.WithRetriever(
			retriever.WithReciprocalRankFusionRetriever(
				retriever.WithRankWindowSize(150),
				retriever.WithQueryFilters(filter),
				retriever.WithChildRetrievers(
					retriever.WithStandardRetriever(
						"retriever-regular",
						standardSearchResultsClause(request),
					),
					retriever.WithStandardRetriever(
						"retriver-semantic",
						semanticSearchResultsClause(request),
					),
				),
			),
		),
		elastic.WithSize(0),
		elastic.WithTrackTotalHits(true),
	)
	if err != nil {
		return 0, fmt.Errorf("search items: %w", err)
	}

	return resp.Hits.Total.Value, nil
}

// GetTopItemCategories returns a [models.Categories] slice containing the top categories from items that match the
// given [*models.SearchRequest].
func GetTopItemCategories(ctx context.Context, request *models.SearchRequest) (models.Categories, error) {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("retrieve items: get user: %w", models.ErrCtxValueNotFound)
	}

	filter := buildSearchResultsQuery(ctx, user, request, standardSearchResultsClause(request))

	// Perform aggregation.
	resp, err := elastic.Search[*models.Item](ctx,
		schema.ItemsIndexRO(),
		elastic.WithRetriever(
			retriever.WithReciprocalRankFusionRetriever(
				retriever.WithRankWindowSize(150),
				retriever.WithQueryFilters(filter),
				retriever.WithChildRetrievers(
					retriever.WithStandardRetriever(
						"retriever-regular",
						standardSearchResultsClause(request),
					),
					retriever.WithStandardRetriever(
						"retriver-semantic",
						semanticSearchResultsClause(request),
					),
				),
			),
		),
		// elastic.WithQueryOptions[*elastic.SearchRequest](
		// 	// Use the original search query but filter out common categories.
		// 	query.Bool(
		// 		query.Must(filter),
		// 		query.MustNot(
		// 			query.Terms(
		// 				"categories.raw",
		// 				slices.Concat(models.CommonCategoryFilters, []string{""}),
		// 			),
		// 		),
		// 	),
		// ),
		elastic.WithAggregations(
			elastic.Aggs{
				"TopCategories": estypes.Aggregations{
					Terms: &estypes.TermsAggregation{
						Field: new("categories.raw"),
						Size:  new(10),
					},
				},
			},
		),
		elastic.WithSize(0),
	)
	if err != nil {
		return nil, fmt.Errorf("aggregate articles: %w", err)
	}

	topCategoriesAgg, isTopCategoriesAgg := resp.Aggregations["TopCategories"].(*estypes.StringTermsAggregate)
	if !isTopCategoriesAgg {
		return nil, fmt.Errorf("extract aggregation: %w", models.ErrInvalidAPIResult)
	}
	topCategoriesBuckets, isTopCategoriesBuckets := topCategoriesAgg.Buckets.([]estypes.StringTermsBucket)
	if !isTopCategoriesBuckets {
		return nil, fmt.Errorf("extract buckets: %w", models.ErrInvalidAPIResult)
	}

	categories := make(models.Categories, 0, len(topCategoriesBuckets))
	for bucket := range slices.Values(topCategoriesBuckets) {
		if category, isCategoryBucket := bucket.Key.(models.Category); isCategoryBucket {
			categories = append(categories, category)
		}
	}
	return categories, nil
}
