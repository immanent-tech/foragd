// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package service

import (
	"context"
	"fmt"
	"net/http"
	"slices"

	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/opensearch/search"
	"github.com/immanent-tech/foragd/schema"
)

// buildSearchResultsQuery generates a query that can be used to fetch appropriate results for a given SearchRequest
// criteria.
func buildSearchResultsQuery(
	ctx context.Context,
	user *models.User,
	request *models.SearchRequest,
	clause search.QueryOption,
) search.QueryOption {
	// Get user subscriptions.
	allSubscriptions := models.SubscriptionsFromCtx(ctx)
	if len(allSubscriptions) == 0 {
		slogctx.Warn(ctx, "Could not retrieve user subscriptions from context.")
		return nil
	}
	subscriptions := allSubscriptions.FilterByIDs(request.Subscriptions...)

	return search.BoolQuery(
		search.WithBoolMust(
			search.BoolQuery(
				search.WithBoolFilter(
					search.BoolQuery(
						// Must satisfy user global filters.
						ArticleFiltersQueryClause(user.GetSettings().GlobalFilters),
						// Must be in the given user subscriptions.
						search.WithBoolShould(
							BuildItemQueries(user, request.View, subscriptions)...,
						),
					),
				),
				search.WithBoolShould(
					// Boost items that are from a favorite subscription.
					search.TermsQuery(
						"feed_id",
						subscriptions.FilterByView(models.ViewFavorites).GetFeedIDs(),
						search.WithQueryBoost(2.0),
					),
					search.DistanceFeatureDateQuery("published", "now", request.Pivot()),
					search.DistanceFeatureDateQuery("updated", "now", request.Pivot()),
				),
			),
			clause,
		),
	)
}

func standardSearchResultsClause(request *models.SearchRequest) search.QueryOption {
	// Must match either: search term in any of the fields, or, matches directly as a search-as-you-type (same as
	// search suggestion).
	return search.BoolQuery(
		search.WithBoolMust(
			search.BoolQuery(
				search.WithBoolShould(
					// Boost match title exactly.
					search.TermQuery("title.exact", request.Text, search.WithQueryBoost(10.0)),
					// Simple query string across most fields. Boost title and description matches.
					search.SimpleQueryStringQuery(
						request.Text,
						[]string{
							"title^3",
							"description^2",
							"content",
							"categories",
							"authors",
							"contributors",
						},
						search.WithSimpleQueryStringQueryOperator(search.OperatorAnd),
					),
				),
			),
		),
	)
}

func semanticSearchResultsClause(request *models.SearchRequest) search.QueryOption {
	// Perform semantic search on content field for text.
	return search.MatchQuery("content_semantic", request.Text)
}

func searchSuggestionsClause(request *models.SearchRequest) search.QueryOption {
	// Must match at least one of in title, description, content.
	return search.BoolQuery(
		search.WithBoolMust(
			search.BoolQuery(
				search.WithBoolShould(
					search.TermQuery("title.exact", request.Text, search.WithQueryBoost(10.0)),
					search.SearchAsYouTypeQuery(request.Text, "title"),
					search.SearchAsYouTypeQuery(request.Text, "description"),
				),
			),
		),
	)
}

// QueryItems will search the items index for items matching the given [query.Option]. coutn, [models.Sort] and
// [models.Pagination] values are optional.
func (s *ItemService) QueryItems(
	ctx context.Context,
	query search.QueryOption,
	count int,
	sort *models.Sort,
	pagination *string,
) (models.Items, string, error) {
	searchAfter, err := search.DecodePagination(pagination)
	if err != nil {
		return nil, "", models.ErrInvalidParams
	}
	// Perform search.
	resp, err := search.Search(ctx, s.osc, s.osc.IndexReadAlias(schema.ItemsIndex),
		search.WithQuery(query),
		search.WithSize(count),
		search.WithSort(ItemSortCombinations(sort)),
		search.WithSearchAfter(searchAfter),
	)
	if err != nil {
		return nil, "", models.NewAPIError(http.StatusInternalServerError, fmt.Errorf("search items: %w", err))
	}
	items, lastSortValue, err := resp.ExtractHits[*models.Item]()
	if err != nil {
		return nil, "", models.NewAPIError(http.StatusInternalServerError, fmt.Errorf("extract items: %w", err))
	}

	// Parse last search after value into pagination.
	newPagination, err := search.EncodePagination[string](lastSortValue)
	if err != nil {
		return nil, "", models.ErrInvalidParams
	}

	// Update cache.
	for item := range slices.Values(items) {
		s.Invalidate(item.GetID())
		s.Set(item.GetID(), item)
	}
	return items, newPagination, nil
}

func (s *ItemService) SuggestItems(
	ctx context.Context,
	request *models.SearchRequest,
) (models.Items, error) {
	ctx, span := tracer.Start(ctx, "SuggestItems")
	defer span.End()

	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, models.NewAPIError(
			http.StatusForbidden,
			fmt.Errorf("get user details: %w", models.ErrCtxValueNotFound),
		)
	}

	clause := buildSearchResultsQuery(ctx, user, request, searchSuggestionsClause(request))

	items, _, err := s.QueryItems(ctx, clause, 9, &request.Sort, nil)
	if err != nil {
		return nil, fmt.Errorf("query items: %w", err)
	}

	return items, nil
}

// RetrieveItems will retrieve a [models.Items] slice of items matching the given [*models.SearchRequest]. The request
// supports pagination and will return the next set of results as appropriate.
func (s *ItemService) RetrieveItems(
	ctx context.Context,
	request *models.SearchRequest,
) (models.Items, models.Pagination, error) {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, models.Pagination{}, fmt.Errorf("retrieve items: get user: %w", models.ErrCtxValueNotFound)
	}

	filter := buildSearchResultsQuery(ctx, user, request, nil)

	resp, err := search.Search(ctx, s.osc, s.osc.IndexReadAlias(schema.ItemsIndex),
		search.WithQuery(
			search.HybridQuery(
				filter,
				standardSearchResultsClause(request),
				// semanticSearchResultsClause(request),
			),
		),
		search.WithFrom(request.GetFrom()),
		search.WithSize(request.GetCount()),
	)
	if err != nil {
		return nil, models.Pagination{}, fmt.Errorf("search items: %w", err)
	}
	items, _, err := resp.ExtractHits[*models.Item]()
	if err != nil {
		return nil, models.Pagination{}, fmt.Errorf("extract items: %w", err)
	}
	// Update cache.
	for item := range slices.Values(items) {
		s.Invalidate(item.GetID())
		s.Set(item.GetID(), item)
	}
	// Parse last search after value into pagination.
	return items, models.Pagination{From: request.GetFrom() + request.GetCount()}, nil
}

// CountSearchResults will return an approximate count of the number of items that would match the given
// [*models.SearchRequest], published within the last 5 minutes.
func (s *ItemService) CountSearchResults(ctx context.Context, request *models.SearchRequest) (int64, error) {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return 0, fmt.Errorf("get user details: %w", models.ErrCtxValueNotFound)
	}

	// Override the published within on the search request to last 5 minutes for updates.
	request.PublishedWithin = models.SearchRequestPublishedWithinLast5mins

	filter := buildSearchResultsQuery(ctx, user, request, nil)

	// Perform search.
	resp, err := search.Search(ctx, s.osc, s.osc.IndexReadAlias(schema.ItemsIndex),
		search.WithQuery(
			search.HybridQuery(
				filter,
				standardSearchResultsClause(request),
				// semanticSearchResultsClause(request),
			),
		),
		search.WithSize(0),
		search.WithTrackTotalHits(true),
	)
	if err != nil {
		return 0, fmt.Errorf("search items: %w", err)
	}

	hits, err := resp.Hits.Total.Int64()
	if err != nil {
		return 0, fmt.Errorf("get total hits: %w", err)
	}

	return hits, nil
}

// GetTopItemCategoriesForSearchResults returns a [models.Categories] slice containing the top categories from items
// that match the given [*models.SearchRequest].
func (s *ItemService) GetTopItemCategoriesForSearchResults(
	ctx context.Context,
	request *models.SearchRequest,
) (models.Categories, error) {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("retrieve items: get user: %w", models.ErrCtxValueNotFound)
	}

	filter := buildSearchResultsQuery(ctx, user, request, standardSearchResultsClause(request))

	// Perform aggregation.
	resp, err := search.Search(ctx, s.osc, s.osc.IndexReadAlias(schema.ItemsIndex),
		search.WithQuery(
			search.HybridQuery(
				filter,
				standardSearchResultsClause(request),
				// semanticSearchResultsClause(request),
			),
		),
		search.WithSize(0),
		search.WithAggregation(
			"TopCategories",
			search.TermsAggregation(
				search.WithTermsAggregationField("categories.raw"),
				search.WithTermsAggregationSize(10),
			),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("aggregate articles: %w", err)
	}

	categoriesAgg, err := resp.ExtractAggregation[opensearchapi.CommonAggregationsStringTermsAggregate]("TopCategories")
	if err != nil {
		return nil, fmt.Errorf("extract aggregation: %w", err)
	}

	categories := make(models.Categories, 0, len(categoriesAgg.Buckets))
	for bucket := range slices.Values(categoriesAgg.Buckets) {
		if bucket.Key == "" {
			continue
		}
		categories = append(categories, bucket.Key)
	}
	return nil, nil
}
