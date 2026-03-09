// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//nolint:revive
package models

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v9/typedapi/core/search"
	estypes "github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/operator"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/sortorder"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/goforj/godump"
	slogctx "github.com/veqryn/slog-context"
	"github.com/zeebo/xxh3"
	"golang.org/x/sync/errgroup"

	"github.com/immanent-tech/go-syndication/opengraph"
	"github.com/immanent-tech/go-syndication/types"

	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/aggregations"
	"github.com/immanent-tech/foragd/providers/elastic/bulk"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/validation"
)

// GetSubscriptionsForItems returns the subscriptions that the list of items belong to.
func GetSubscriptionsForItems(ctx context.Context, items Items) (Subscriptions, error) {
	user := UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get user data: %w", ErrCtxValueNotFound)
	}
	// Get user subscription details for the feeds the items belong to.
	subscriptions, _, err := elastic.Search[*Subscription](
		ctx,
		schema.SubscriptionsIndexRO,
		query.Bool(
			query.Filter(
				query.Term("user_id", user.GetID()),
			),
			query.Should(
				query.Terms("feed_data.feed_id", items.GetFeedIDs()...),
				query.Terms("email_data.feed_id", items.GetFeedIDs()...),
			),
		),
		len(items.GetFeedIDs()),
	)
	if err != nil {
		return nil, ElasticsearchToAPIError(err)
	}
	return subscriptions, nil
}

func GetCategoriesForSubscriptions(ctx context.Context, subscriptionIDs ...SubscriptionID) (CategoryCounts, error) {
	// Retrieve user object.
	user := UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get user data: %w", ErrCtxValueNotFound)
	}

	// Build query.
	var searchQuery query.Option
	if len(subscriptionIDs) == 0 {
		searchQuery = query.Bool(
			query.Filter(
				query.Term("user_id", user.GetID()),
			),
		)
	} else {
		searchQuery = query.Bool(
			query.Filter(
				query.Term("user_id", user.GetID()),
				query.Terms("subscription_id", subscriptionIDs...),
			),
		)
	}

	// Build aggregations.
	termsField := "customisation.categories.raw"
	termsCount := 200
	aggs := aggregations.Aggs{
		"CategoryCounts": estypes.Aggregations{
			Terms: &estypes.TermsAggregation{
				Field: &termsField,
				Size:  &termsCount,
			},
		},
	}

	resp, err := elastic.NewSearchRequest(
		elastic.WithRequestID[*search.Search, elastic.SearchRequest](middleware.GetReqID(ctx)),
		elastic.WithIndex[*search.Search, elastic.SearchRequest](schema.SubscriptionsIndexRO),
		elastic.WithQueryOptions[*search.Search, elastic.SearchRequest](searchQuery),
		elastic.WithSize[*search.Search, elastic.SearchRequest](0),
		elastic.WithSortOptions[*search.Search, elastic.SearchRequest](
			&estypes.SortOptions{Doc_: estypes.NewScoreSort()},
		),
		elastic.WithAggregations[*search.Search, elastic.SearchRequest](aggs),
	).Do(ctx)
	if err != nil {
		return nil, ElasticsearchToAPIError(err)
	}

	categoryCounts, ok := resp.Aggregations["CategoryCounts"].(*estypes.StringTermsAggregate)
	if !ok {
		return nil, fmt.Errorf(
			"category counts aggregation invalid: %w",
			ErrInvalidAPIResult,
		)
	}
	categoryCountsBuckets, ok := categoryCounts.Buckets.([]estypes.StringTermsBucket)
	if !ok {
		return nil, fmt.Errorf(
			"unable to get feed stats: UnreadCounts aggregations invalid: %w",
			ErrInvalidAPIResult,
		)
	}

	counts := make(CategoryCounts, 0, len(categoryCountsBuckets))

	// Loop through the aggregation results and extract the unread count for each feed.
	for bucket := range slices.Values(categoryCountsBuckets) {
		var category Category
		if category, ok = bucket.Key.(string); ok {
			counts = append(counts, CategoryCount{Category: category, Count: int(bucket.DocCount)})
		}
	}
	return counts, nil
}

// GetSubscriptionCategories retrieves a map of categories from user subscriptions by count.
func GetSubscriptionCategories(ctx context.Context, subscriptions Subscriptions) (CategoryCounts, error) {
	// Retrieve user object.
	user := UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get user data: %w", ErrCtxValueNotFound)
	}

	// Build query.
	var searchQuery query.Option
	if len(subscriptions) == 0 {
		searchQuery = query.Bool(
			query.Filter(
				query.Term("user_id", user.GetID()),
			),
		)
	} else {
		searchQuery = query.Bool(
			query.Filter(
				query.Term("user_id", user.GetID()),
				query.Terms("subscription_id", subscriptions.GetIDs()...),
			),
		)
	}

	// Build aggregations.
	termsField := "customisation.categories.raw"
	termsCount := 200
	aggs := aggregations.Aggs{
		"CategoryCounts": estypes.Aggregations{
			Terms: &estypes.TermsAggregation{
				Field: &termsField,
				Size:  &termsCount,
			},
		},
	}

	resp, err := elastic.NewSearchRequest(
		elastic.WithRequestID[*search.Search, elastic.SearchRequest](middleware.GetReqID(ctx)),
		elastic.WithIndex[*search.Search, elastic.SearchRequest](schema.SubscriptionsIndexRO),
		elastic.WithQueryOptions[*search.Search, elastic.SearchRequest](searchQuery),
		elastic.WithSize[*search.Search, elastic.SearchRequest](0),
		elastic.WithSortOptions[*search.Search, elastic.SearchRequest](
			&estypes.SortOptions{Doc_: estypes.NewScoreSort()},
		),
		elastic.WithAggregations[*search.Search, elastic.SearchRequest](aggs),
	).Do(ctx)
	if err != nil {
		return nil, ElasticsearchToAPIError(err)
	}

	categoryCounts, ok := resp.Aggregations["CategoryCounts"].(*estypes.StringTermsAggregate)
	if !ok {
		return nil, fmt.Errorf(
			"category counts aggregation invalid: %w",
			ErrInvalidAPIResult,
		)
	}
	categoryCountsBuckets, ok := categoryCounts.Buckets.([]estypes.StringTermsBucket)
	if !ok {
		return nil, fmt.Errorf(
			"unable to get feed stats: UnreadCounts aggregations invalid: %w",
			ErrInvalidAPIResult,
		)
	}

	counts := make(CategoryCounts, 0, len(categoryCountsBuckets))

	// Loop through the aggregation results and extract the unread count for each feed.
	for bucket := range slices.Values(categoryCountsBuckets) {
		var category Category
		if category, ok = bucket.Key.(string); ok {
			counts = append(counts, CategoryCount{Category: category, Count: int(bucket.DocCount)})
		}
	}
	return counts, nil
}

type subscriptionsRequest struct {
	filterFavorites  bool
	filterIDs        []SubscriptionID
	filterFeedIDs    []FeedID
	filterCategories []Category
	ignoreIDs        []SubscriptionID
	addDynamicInfo   bool
}

type subscriptionsRequestOption func(*subscriptionsRequest)

// GetSubscriptionsByFavorite option adds a filter to the query to get favorite subscriptions only.
func GetSubscriptionsByFavorite(value bool) subscriptionsRequestOption {
	return func(sr *subscriptionsRequest) {
		sr.filterFavorites = value
	}
}

// GetSubscriptionsByIDs option adds a filter to the query to get subscriptions by their ids.
func GetSubscriptionsByIDs(ids ...SubscriptionID) subscriptionsRequestOption {
	return func(sr *subscriptionsRequest) {
		sr.filterIDs = ids
	}
}

// GetSubscriptionsByFeedIDs option adds a filter to the query to get subscriptions by their feed ids.
func GetSubscriptionsByFeedIDs(ids ...FeedID) subscriptionsRequestOption {
	return func(sr *subscriptionsRequest) {
		sr.filterFeedIDs = ids
	}
}

// GetSubscriptionsByCategories option adds a filter to the query to get subscriptions by category.
func GetSubscriptionsByCategories(categories ...Category) subscriptionsRequestOption {
	return func(sr *subscriptionsRequest) {
		sr.filterCategories = categories
	}
}

// GetSubscriptionsDynamicInfo option will fill in the dynamic info (i.e., stats) after fetching.
func GetSubscriptionsDynamicInfo(value bool) subscriptionsRequestOption {
	return func(sr *subscriptionsRequest) {
		sr.addDynamicInfo = value
	}
}

func IgnoreSubscriptions(ids ...SubscriptionID) subscriptionsRequestOption {
	return func(sr *subscriptionsRequest) {
		sr.ignoreIDs = ids
	}
}

type searchSubscriptionsRequest struct {
	count          int
	sort           *Sort
	pagination     Pagination
	addDynamicInfo bool
	queries        []query.Option
}

// searchSubscriptionsOption is a functional option to apply to a search subscriptions request.
type searchSubscriptionsOption func(*searchSubscriptionsRequest)

// searchSubscriptionsQueries option appends the given queries into the request.
func searchSubscriptionsQueries(queries ...query.Option) searchSubscriptionsOption {
	return func(ssr *searchSubscriptionsRequest) {
		ssr.queries = append(ssr.queries, queries...)
	}
}

// searchSubscriptionsMaxResults option controls the max results returned by the request.
func searchSubscriptionsMaxResults(count int) searchSubscriptionsOption {
	return func(ssr *searchSubscriptionsRequest) {
		ssr.count = count
	}
}

func searchSubscriptionsSortResults(sort *Sort) searchSubscriptionsOption {
	return func(ssr *searchSubscriptionsRequest) {
		ssr.sort = sort
	}
}

func searchSubscriptionsPaginate(paginate Pagination) searchSubscriptionsOption {
	return func(ssr *searchSubscriptionsRequest) {
		ssr.pagination = paginate
	}
}

func searchSubscriptionsAddDynamicInfo(value bool) searchSubscriptionsOption {
	return func(ssr *searchSubscriptionsRequest) {
		ssr.addDynamicInfo = value
	}
}

// GetSubscription returns the subscription that matches the given ID.
//
// Accepts the GetSubscriptionsDynamicInfo request option to generate dynamic info (i.e. stats) for the subscription.
func GetSubscription(
	ctx context.Context,
	id SubscriptionID,
	options ...subscriptionsRequestOption,
) (*Subscription, error) {
	// Parse and apply options.
	req := &subscriptionsRequest{}
	for option := range slices.Values(options) {
		option(req)
	}

	// Get user data.
	user := UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get user data: %w", ErrCtxValueNotFound)
	}

	// Find matching subscription.
	subscriptions, _, err := SearchSubscriptions(ctx,
		query.Bool(
			query.Filter(
				query.Term("user_id", user.GetID()),
				query.Term("subscription_id", id),
			),
		),
		searchSubscriptionsMaxResults(1),
	)
	switch {
	case err != nil:
		return nil, fmt.Errorf("search subscriptions: %w", err)
	case len(subscriptions) == 0:
		return nil, fmt.Errorf("search subscriptions: %w", ErrNotFound)
	case len(subscriptions) != 1:
		return nil, fmt.Errorf("search subscriptions: %w: too many subscriptions", ErrInvalidAPIResult)
	}

	// Add dynamic info if requested.
	if req.addDynamicInfo {
		err = addSubscriptionDynamicInfo(ctx, subscriptions)
		if err != nil {
			return nil, fmt.Errorf("add dynamic info: %w", err)
		}
	}

	return subscriptions[0], nil
}

// GetSubscriptions performs a search request to fetch subscriptions. Accepts all request options to filter/enrich the
// results.
func GetSubscriptions(
	ctx context.Context,
	options ...subscriptionsRequestOption,
) (Subscriptions, error) {
	req := &subscriptionsRequest{}
	for option := range slices.Values(options) {
		option(req)
	}

	// Get user data.
	user := UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get user data: %w", ErrCtxValueNotFound)
	}

	// Build query optional parts.
	queries := []query.Option{query.Term("user_id", user.GetID())}
	if req.filterFavorites {
		queries = append(queries, query.Term("favorite", true))
	}
	if len(req.filterIDs) > 0 {
		queries = append(queries, query.Terms("subscription_id", req.filterIDs...))
	}
	if len(req.filterCategories) > 0 {
		queries = append(queries, query.Terms("customisation.categories", req.filterCategories...))
	}

	// Construct query.
	subscriptions, err := getAllSubscriptionsByQuery(ctx,
		query.Bool(
			query.Filter(
				queries...,
			),
		))
	if err != nil {
		return nil, fmt.Errorf("get all subscriptions: %w", err)
	}
	if len(subscriptions) == 0 {
		return nil, fmt.Errorf("get all subscriptions: %w", ErrNotFound)
	}

	if req.addDynamicInfo {
		if err = addSubscriptionDynamicInfo(ctx, subscriptions); err != nil {
			return nil, fmt.Errorf("add dynamic info: %w", err)
		}
	}

	return subscriptions, nil
}

// GetSubscriptionByFeedID returns the subscription that matches the given feed ID.
//
// Accepts the GetSubscriptionsDynamicInfo request option to generate dynamic info (i.e. stats) for the subscription.
func GetSubscriptionByFeedID(
	ctx context.Context,
	id FeedID,
	options ...subscriptionsRequestOption,
) (*Subscription, error) {
	// Parse and apply options.
	req := &subscriptionsRequest{}
	for option := range slices.Values(options) {
		option(req)
	}

	user := UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get user data: %w", ErrCtxValueNotFound)
	}
	subscriptions, _, err := SearchSubscriptions(ctx,
		query.Bool(
			query.Filter(
				query.Term("user_id", user.GetID()),
				query.Term("type", SubscriptionTypeFeed),
				query.Term("feed_data.feed_id", id),
			),
		),
		searchSubscriptionsMaxResults(1),
	)
	switch {
	case err != nil:
		return nil, fmt.Errorf("search subscriptions: %w", err)
	case len(subscriptions) == 0:
		return nil, fmt.Errorf("search subscriptions: %w", ErrNotFound)
	case len(subscriptions) != 1:
		return nil, fmt.Errorf("search subscriptions: %w: too many subscriptions", ErrInvalidAPIResult)
	}

	// Add dynamic info if requested.
	if req.addDynamicInfo {
		err = addSubscriptionDynamicInfo(ctx, subscriptions)
		if err != nil {
			return nil, fmt.Errorf("add dynamic info: %w", err)
		}
	}

	return subscriptions[0], nil
}

// GetSubscriptionSuggestions returns subscriptions that match the given text.
//
// Accepts the GetSubscriptionsDynamicInfo request option to generate dynamic info (i.e. stats) for the subscription
// suggestions.
func GetSubscriptionSuggestions(
	ctx context.Context,
	text string,
	count int,
	options ...subscriptionsRequestOption,
) (Subscriptions, error) {
	// Parse and apply options.
	req := &subscriptionsRequest{}
	for option := range slices.Values(options) {
		option(req)
	}

	// Get subscriptions by ID.
	user := UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get user data: %w", ErrCtxValueNotFound)
	}

	sort := SortMostRelevant
	subscriptions, _, err := SearchSubscriptions(ctx,
		query.Bool(
			query.Filter(
				query.Term("user_id", user.GetID()),
				query.Bool(
					query.Should(
						query.Term("type", SubscriptionTypeEmail),
						query.Term("type", SubscriptionTypeFeed),
					),
				),
			),
			query.Must(
				query.Bool(
					query.Should(
						query.SearchAsYouType(text, "customisation.nickname"),
					),
				),
			),
			query.MustNot(
				query.Terms("subscription_id", req.ignoreIDs...),
			),
		),
		searchSubscriptionsMaxResults(count),
		searchSubscriptionsSortResults(&sort),
	)
	if err != nil {
		return nil, fmt.Errorf("search subscriptions: %w", err)
	}
	if len(subscriptions) == 0 {
		return nil, fmt.Errorf("search subscriptions: %w", ErrNotFound)
	}

	// Add dynamic info if requested.
	if req.addDynamicInfo {
		err = addSubscriptionDynamicInfo(ctx, subscriptions)
		if err != nil {
			return nil, fmt.Errorf("add dynamic info: %w", err)
		}
	}

	return subscriptions, nil
}

func SearchSubscriptions(
	ctx context.Context,
	query query.Option,
	options ...searchSubscriptionsOption,
) (Subscriptions, Pagination, error) {
	req := &searchSubscriptionsRequest{}

	// Build request with options.
	for option := range slices.Values(options) {
		option(req)
	}

	// Add sane values where not supplied.
	if req.count == 0 {
		req.count = 10
	}

	searchAfter, err := elastic.DecodePagination(&req.pagination)
	if err != nil {
		return nil, "", ErrInvalidParams
	}

	// Perform search.
	subscriptions, newSearchAfter, err := elastic.Search[*Subscription](
		ctx,
		schema.SubscriptionsIndexRO,
		query,
		req.count,
		elastic.WithSortOptions[*search.Search, elastic.SearchRequest](newSubscriptionSortOptions(req.sort)...),
		elastic.WithSearchAfter[*search.Search, elastic.SearchRequest](searchAfter...),
	)
	if err != nil {
		return nil, "", ElasticsearchToAPIError(err)
	}
	// Parse search after into pagination.
	if req.pagination != "" {
		req.pagination, err = elastic.EncodePagination[Pagination](newSearchAfter)
		if err != nil {
			return nil, "", ErrInvalidParams
		}
		return subscriptions, req.pagination, nil
	}

	return subscriptions, "", nil
}

// FilterSubscriptions returns subscriptions filtered by the given filters and paginated by the given pagination.
// Dynamic information for subscriptions will also be added.
func FilterSubscriptions(
	ctx context.Context,
	request *ListRequest,
) (Subscriptions, Pagination, error) {
	// Get subscriptions by ID.
	user := UserFromCtx(ctx)
	if user == nil {
		return nil, "", fmt.Errorf("get user data: %w", ErrCtxValueNotFound)
	}
	subscriptionQuery := query.Bool(
		query.Filter(
			query.Term("user_id", user.GetID()),
			query.Terms("subscription_id", request.Filters.Subscriptions...),
			// query.Term("favorite", filters.OnlyFavorites),
			query.Terms("customisation.categories.raw", request.Filters.GetCategories()...),
		),
	)
	subscriptions, err := getAllSubscriptionsByQuery(ctx, subscriptionQuery)
	if err != nil {
		return nil, "", fmt.Errorf("get all subscriptions: %w", err)
	}
	if len(subscriptions) == 0 {
		return nil, "", fmt.Errorf("get all subscriptions: %w", ErrNotFound)
	}
	// Add dynamic info.
	err = addSubscriptionDynamicInfo(ctx, subscriptions)
	if err != nil {
		return nil, "", fmt.Errorf("add dynamic info: %w", err)
	}
	// Sort and paginate.
	var pagination Pagination
	if request.Pagination != nil {
		pagination = *request.Pagination
	}
	subscriptions, pagination = subscriptions.
		FilterByView(request.Filters.GetView()).
		FilterByFavorites(request.Filters.OnlyFavorites).
		Sort(request.Filters.Sort).
		Paginate(pagination, request.Filters.GetCount())

	return subscriptions, pagination, nil
}

// ProcessSubscriptionRequest manages parsing a subscription request and turning it into a subscription that can be
// added to a user. It handles finding a existing matching feed or creating a new one, checking the user isn't already
// subscribed and generating an appropriate subscription object. It returns an object that includes the request and new
// subscription data, or the request and an error if subscription data could not be generated.
func ProcessSubscriptionRequest(
	ctx context.Context,
	request *AddFeedSubscriptionRequest,
	resultsCh chan AddFeedSubscriptionResult,
) {
	result := AddFeedSubscriptionResult{
		Request: *request,
	}

	var (
		newFeed *Feed
		err     error
	)

	feedURL, err := FeedURLParser(ctx, request.GetURL())
	if err != nil {
		result.Error = err
		result.Message = NewErrorMessage(
			"Unable to create subscription",
			fmt.Sprintf(
				"The feed URL %q cannot be parsed as a feed source or is not a valid URL.",
				request.GetURL(),
			),
		)
		resultsCh <- result
		return
	}

	newFeed, err = NewFeedFromURL(ctx, feedURL.String(), "", true)
	if err != nil {
		result.Error = err
		result.Message = NewErrorMessage(
			"Unable to create subscription",
			fmt.Sprintf(
				"The feed URL %q cannot be parsed as a feed source or is not a valid URL.",
				request.GetURL(),
			),
		)
		resultsCh <- result
		return
	}

	// Create terms queries to match the new feed to an existing feed.
	var terms []query.Option
	for url := range slices.Values(newFeed.SourceURLs) {
		terms = append(terms, query.Term("source_urls", url))
	}
	terms = append(terms, query.Term("url", newFeed.URL))
	// Find any existing feed.
	feeds, _, err := elastic.Search[*Feed](ctx,
		schema.FeedsIndexRO,
		query.Bool(
			query.Filter(
				query.Bool(
					query.Should(terms...),
				),
			),
		),
		1,
	)
	if err != nil {
		result.Error = err
		result.Message = NewErrorMessage(
			"Unable to determine existing subscription status",
			"The backend produced an error. This might be temporary, please try again.",
		)
		resultsCh <- result
		return
	}
	if len(feeds) == 1 {
		// If an existing feed is found, use that feed.
		result.Feed = feeds[0]
	} else {
		// Otherwise create a new feed.

		// Try to find an image for the feed if it does not supply one.
		if newFeed.GetImage() == nil {
			og, err := opengraph.ParseURL(ctx, newFeed.GetLink())
			if err != nil {
				slogctx.FromCtx(ctx).WarnContext(ctx, "No image for feed.",
					slog.String("feed", newFeed.GetTitle()),
				)
			}
			if err := og.Valid(); err != nil {
				newFeed.Image = &types.ImageInfo{
					URL:   os.Getenv("FORAGD_BASEURL") + "/content/images/placeholder.webp",
					Title: newFeed.GetDescription(),
				}
			} else {
				newFeed.Image = &types.ImageInfo{
					URL:   og.Image,
					Title: newFeed.GetDescription(),
				}
			}
			godump.Dump(og)
		}
		godump.Dump(newFeed)
		// Validate the new feed data.
		err = validation.Validate.Struct(newFeed)
		if err != nil {
			result.Error = err
			result.Message = NewErrorMessage(
				"Unable to create subscription",
				fmt.Sprintf(
					"The feed URL %q cannot be parsed as a feed source or is not a valid URL.",
					request.GetURL(),
				),
			)
			resultsCh <- result
			return
		}
		// Index the new feed.
		if err := elastic.CreateDoc(ctx, schema.FeedsIndexRW, newFeed.GetID(), newFeed); err != nil {
			result.Error = err
			result.Message = NewErrorMessage(
				"Unable to create new feed for subscription",
				"The backend produced an error. This might be temporary, please try again.",
			)
			resultsCh <- result
			return
		}
		slogctx.FromCtx(ctx).Debug("Created new feed for request.",
			slog.String("name", newFeed.GetTitle()),
			slog.String("urls", strings.Join(newFeed.GetSourceURLs(), ",")),
		)
		result.Feed = newFeed
	}

	// Check for an existing subscription.
	subscription, err := GetSubscriptionByFeedID(ctx, result.Feed.GetID())
	if err != nil && HTTPStatus(err) != http.StatusNotFound {
		result.Error = err
		result.Message = NewErrorMessage(
			"Unable to check for existing subscription for "+result.Feed.GetTitle(),
			"The backend produced an error. This might be temporary, please try again.",
		)
		resultsCh <- result
		return
	}
	if subscription != nil {
		// Existing subscription to feed found, ignore new feed.
		result.Error = &APIError{
			InternalError: errors.New("already subscribed"),
			StatusCode:    http.StatusConflict,
		}
		result.Message = NewWarningMessage("Already subscribed to feed", result.Feed.GetTitle())
		resultsCh <- result
		return
	}

	slogctx.FromCtx(ctx).Debug("New subscription required.",
		slog.String("feed_title", result.Feed.GetTitle()),
		slog.String("feed_id", result.Feed.GetID()),
	)

	// New subscription required.
	resultsCh <- result
}

// AddSubscriptions adds the given subscriptions to a user.
func AddSubscriptions(ctx context.Context, subscriptions ...*Subscription) error {
	user := UserFromCtx(ctx)
	if user == nil {
		return fmt.Errorf("get user data: %w", ErrCtxValueNotFound)
	}
	if _, err := UpdateSubscriptions(ctx, subscriptions...); err != nil {
		return fmt.Errorf("update subscriptions: %w", err)
	}
	// Disable onboarding once a subscription has been added.
	if settings := user.GetSettings(); settings.ShowOnboarding {
		settings.ShowOnboarding = false
		// Update the user object.
		if err := UpdateUser(ctx, user.GetID(), map[string]any{
			"settings": settings,
		}); err != nil {
			return fmt.Errorf("update user: %w", err)
		}
	}
	return nil
}

// RemoveSubscriptions removes subscriptions with the given ID from a user.
func RemoveSubscriptions(ctx context.Context, ids ...SubscriptionID) error {
	user := UserFromCtx(ctx)
	if user == nil {
		return fmt.Errorf("get user data: %w", ErrCtxValueNotFound)
	}
	if err := elastic.DeleteDocs(ctx, schema.SubscriptionsIndexRW,
		query.Bool(
			query.Filter(
				query.Term("user_id", user.GetID()),
				query.Terms("subscription_id", ids...),
			),
		),
	); err != nil {
		return ElasticsearchToAPIError(err)
	}
	return nil
}

// UpdateSubscriptions will bulk update the given subscriptions in Elasticsearch.
func UpdateSubscriptions(
	ctx context.Context,
	subscriptions ...*Subscription,
) (map[SubscriptionID]*bulk.OperationResponse, error) {
	resp, err := elastic.BulkUpdate(ctx, schema.SubscriptionsIndexRW, subscriptions...)
	if err != nil {
		return nil, ElasticsearchToAPIError(err)
	}
	return resp, nil
}

// UpdateFavoriteSubscription changes the favorite status of a subscription by updating the user object to flag the
// subscription as appropriate.
func UpdateFavoriteSubscription(ctx context.Context, id SubscriptionID, favorite bool) error {
	subscription, err := GetSubscription(ctx, id)
	if err != nil {
		return fmt.Errorf("get subscription: %w", err)
	}

	subscription.Favorite = favorite

	_, err = UpdateSubscriptions(ctx, subscription)
	if err != nil {
		return ElasticsearchToAPIError(err)
	}

	return nil
}

// MarkSubscriptions will mark as appropriate all the given subscriptions. Marking a subscription includes updating the
// subscription data in the user object and clearing any individual item states for a subscription.
func MarkSubscriptions(
	ctx context.Context,
	mark Mark,
	subscriptionIDs ...SubscriptionID,
) error {
	user := UserFromCtx(ctx)
	if user == nil {
		return fmt.Errorf("get user data: %w", ErrCtxValueNotFound)
	}

	subscriptions, err := GetSubscriptions(ctx,
		GetSubscriptionsByIDs(subscriptionIDs...),
	)
	if err != nil {
		return fmt.Errorf("mark subscriptions: %w", err)
	}

	for subscription := range slices.Values(subscriptions) {
		if subscription.GetSubscriptionType() == SubscriptionTypeGroup {
			if err = MarkSubscriptions(ctx, mark, subscription.GroupData.Subscriptions...); err != nil {
				return fmt.Errorf("mark subscriptions: mark group subscription: %w", err)
			}
		} else {
			subscription.Mark(user, mark)
			if _, err = UpdateSubscriptions(ctx, subscriptions...); err != nil {
				return fmt.Errorf("mark subscriptions: update subscription data: %w", err)
			}
		}
	}

	return nil
}

// CreateFeedSubscriptions will create new FeedSubscriptions for the user from the given requests.
func CreateFeedSubscriptions(ctx context.Context, results ...*AddFeedSubscriptionResult) error {
	if len(results) == 0 {
		return nil
	}
	subscriptions := make(Subscriptions, 0, len(results))
	for result := range slices.Values(results) {
		slogctx.FromCtx(ctx).Debug("Creating new subscription.",
			slog.String("feed", result.Feed.GetTitle()),
			slog.String("url", result.Feed.GetLink()),
		)
		// Generate metadata.
		subscription, err := NewFeedSubscription(ctx, result.Feed, &result.Request)
		if err != nil {
			slogctx.FromCtx(ctx).Error("Could not create subscription",
				slog.Any("error", err))
			result.Error = fmt.Errorf("unable to create subscription: invalid metadata: %w", err)
			continue
		}
		err = subscription.Valid()
		if err != nil {
			slogctx.FromCtx(ctx).Error("Could not create subscription",
				slog.Any("error", err))
			result.Error = fmt.Errorf("unable to create subscription: invalid metadata: %w", err)
			continue
		}
		result.Subscription = subscription
		subscriptions = append(subscriptions, subscription)
		result.Message = NewSuccessMessage(
			"Subscription Created: "+result.Feed.GetTitle(),
			"Articles will be fetched shortly...",
		)
	}
	// Add subscriptions
	if err := AddSubscriptions(ctx, subscriptions...); err != nil {
		return fmt.Errorf("unable to create subscriptions: %w", err)
	}
	return nil
}

// CreateSearchSubscriptions will create new SearchSubscriptions for the user from the given requests.
func CreateSearchSubscriptions(ctx context.Context, requests ...*SearchSubscriptionRequest) error {
	subscriptions := make(Subscriptions, 0, len(requests))
	for request := range slices.Values(requests) {
		slogctx.FromCtx(ctx).Debug("Creating new search subscription.",
			slog.String("feed", request.Customisation.Nickname),
		)
		// Generate metadata.
		subscription, err := NewSearchSubscription(ctx, request)
		if err != nil {
			return fmt.Errorf("create search subscription: generate subscription failed: %w", err)
		}
		err = subscription.Valid()
		if err != nil {
			return fmt.Errorf("create search subscription: invalid data: %w", err)
		}
		subscriptions = append(subscriptions, subscription)
	}
	// Add subscriptions
	if err := AddSubscriptions(ctx, subscriptions...); err != nil {
		return fmt.Errorf("create search subscription: add subscriptions failed: %w", err)
	}
	return nil
}

// addSubscriptionDynamicInfo adds dynamically generated information (e.g., unread count, stats, etc.) to subscriptions.
// At the least, all subscriptions will have an unread count and last updated info generated. Other stats will also be
// generated if the user has set the display option ShowSubscriptionStats in their account settings.
//
//nolint:gocognit,funlen
func addSubscriptionDynamicInfo(ctx context.Context, subscriptions Subscriptions) error {
	user := UserFromCtx(ctx)
	if user == nil {
		return fmt.Errorf("get user data: %w", ErrCtxValueNotFound)
	}

	var extraIDs []SubscriptionID
	for subscription := range slices.Values(subscriptions) {
		// Get any additional subscription info for subscriptions in group subscriptions that we didn't already fetch.
		if subscription.GetSubscriptionType() == SubscriptionTypeGroup {
			for id := range slices.Values(subscription.GroupData.Subscriptions) {
				if !slices.ContainsFunc(subscriptions, func(e *Subscription) bool {
					return e.GetID() == id
				}) {
					extraIDs = append(extraIDs, id)
				}
			}
		}
	}
	if len(extraIDs) > 0 {
		extraSubscriptions, err := GetSubscriptions(ctx,
			GetSubscriptionsByIDs(extraIDs...),
		)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return fmt.Errorf("add subscription dynamic info: get additional subscriptions: %w", err)
		}
		subscriptions = append(subscriptions, extraSubscriptions...)
	}

	fetchJobs, jobCtx := errgroup.WithContext(ctx)
	defer jobCtx.Done()

	// Get unread count per feed.
	var unreadCounts map[FeedID]int64
	fetchJobs.Go(func() error {
		var err error
		unreadCounts, err = getFeedUnreadCounts(jobCtx, subscriptions)
		if err != nil {
			return fmt.Errorf("get unread counts: %w", err)
		}
		return nil
	})

	// For search subscriptions, run queries directly to add unread count and last update.
	fetchJobs.Go(func() error {
		for subscription := range slices.Values(subscriptions.FilterByType(SubscriptionTypeSearch)) {
			request := subscription.SearchData.Search
			// Build query to get unread count.
			query, err := BuildSearchResultsQuery(jobCtx, user, &request, SearchResultsClause(&request))
			if err != nil {
				return fmt.Errorf(
					"add subscription dynamic info: build search subscription %s query: %w",
					subscription.GetID(),
					err,
				)
			}
			count, err := CountItems(jobCtx, query)
			if err == nil {
				subscription.GetStats().UnreadCount = int(count)
			} else {
				slogctx.FromCtx(jobCtx).
					Warn("Add subscription dynamic info, could not get unread count for search subscription.",
						slog.String("subscription_id", subscription.GetID()),
						slog.Any("error", err),
					)
			}
			// Update query for getting last updated item (view: all, sort: newest first).
			request.View = ViewAll
			sort := SortNewestFirst
			query, err = BuildSearchResultsQuery(jobCtx, user, &request, SearchResultsClause(&request))
			if err != nil {
				return fmt.Errorf(
					"add subscription dynamic info: build search subscription %s query: %w",
					subscription.GetID(),
					err,
				)
			}
			if items, _, err := SearchItems(jobCtx, query, 1, &sort, nil); err == nil && len(items) > 0 {
				subscription.GetStats().LastUpdate = items[0].GetTimestamp()
			} else {
				slogctx.FromCtx(jobCtx).
					Warn("Add subscription dynamic info, could not get last update for search subscription.",
						slog.String("subscription_id", subscription.GetID()),
						slog.Any("error", err),
					)
			}
		}
		return nil
	})

	// Get last update (latest item timestamp) per feed.
	var lastUpdate map[FeedID]time.Time
	fetchJobs.Go(func() error {
		var err error
		lastUpdate, err = getFeedLastUpdates(jobCtx, subscriptions.GetFeedIDs()...)
		if err != nil {
			return fmt.Errorf("get last update: %w", err)
		}
		return nil
	})

	var avgDailyUpdates map[FeedID]float64
	if user.GetSettings().ShowSubscriptionStats {
		// Get average daily updates per feed
		fetchJobs.Go(func() error {
			var err error
			avgDailyUpdates, err = getFeedAverageDailyUpdates(jobCtx, subscriptions.GetFeedIDs()...)
			if err != nil {
				return fmt.Errorf("get average daily updates: %w", err)
			}
			return nil
		})
	}

	if err := fetchJobs.Wait(); err != nil {
		return fmt.Errorf("add subscription dynamic info: run jobs: %w", err)
	}

	// For feed subscriptions, add stats.
	for subscription := range slices.Values(subscriptions.FilterByType(SubscriptionTypeFeed)) {
		subscription.GetStats().UnreadCount = int(unreadCounts[subscription.GetFeedID()])
		subscription.GetStats().LastUpdate = lastUpdate[subscription.GetFeedID()]
		if user.GetSettings().ShowSubscriptionStats {
			subscription.GetStats().AvgDailyUpdates = avgDailyUpdates[subscription.GetFeedID()]
		}
	}

	// For email subscriptions, add stats.
	for subscription := range slices.Values(subscriptions.FilterByType(SubscriptionTypeEmail)) {
		subscription.GetStats().UnreadCount = int(unreadCounts[subscription.GetFeedID()])
		subscription.GetStats().LastUpdate = lastUpdate[subscription.GetFeedID()]
		if user.GetSettings().ShowSubscriptionStats {
			subscription.GetStats().AvgDailyUpdates = avgDailyUpdates[subscription.GetFeedID()]
		}
	}

	// For group subscriptions, calculate stats from other subscriptions.
	for subscription := range slices.Values(subscriptions.FilterByType(SubscriptionTypeGroup)) {
		var avgDailyUpdates []float64
		var unreadCount int
		var lastUpdates []time.Time
		for groupSubscription := range slices.Values(subscriptions) {
			if slices.Contains(subscription.GroupData.Subscriptions, groupSubscription.GetID()) {
				if user.GetSettings().ShowSubscriptionStats {
					avgDailyUpdates = append(avgDailyUpdates, groupSubscription.GetStats().AvgDailyUpdates)
				}
				unreadCount += groupSubscription.GetStats().UnreadCount
				lastUpdates = append(lastUpdates, groupSubscription.GetStats().LastUpdate)
			}
		}
		if user.GetSettings().ShowSubscriptionStats && len(avgDailyUpdates) > 0 {
			slices.Sort(avgDailyUpdates)
			slices.Reverse(avgDailyUpdates)
			subscription.GetStats().AvgDailyUpdates = avgDailyUpdates[0]
		}
		subscription.GetStats().UnreadCount = unreadCount
		// Sort by date ascending, with favorites before non-favorites.
		slices.SortFunc(lastUpdates, func(timeA, timeB time.Time) int {
			return timeA.Compare(timeB)
		})
		slices.Reverse(lastUpdates)
		subscription.GetStats().LastUpdate = lastUpdates[0]
	}

	return nil
}

// getAllSubscriptionsByQuery returns all subscriptions that match the given query.
func getAllSubscriptionsByQuery(ctx context.Context, query query.Option) (Subscriptions, error) {
	var (
		subscriptions Subscriptions
		err           error
	)
	subscriptions, err = elastic.SearchAll[*Subscription](
		ctx,
		schema.SubscriptionsIndexRO,
		query,
		DefaultPaginationSize,
	)
	if err != nil {
		return nil, ElasticsearchToAPIError(err)
	}
	return subscriptions, nil
}

// BuildItemQueries generates a slices of queries for the given subscriptions, based on the given filters.
func BuildItemQueries(
	user *User,
	view View,
	subscriptions Subscriptions,
) []query.Option {
	queries := make([]query.Option, 0, len(subscriptions))
	// Work out what query to use based on the state filter.
	if len(subscriptions) == 0 {
		return nil
	}
	for subscription := range slices.Values(subscriptions) {
		// Ignore subscriptions that aren't based on a feed object.
		if subscription.GetFeedID() == "" {
			continue
		}

		switch view {
		case ViewRead:
			queries = append(queries, queryReadItems(user, subscription))
		case ViewAll:
			queries = append(queries, queryAllItems(user, subscription))
		case ViewUnread:
			fallthrough
		default:
			queries = append(queries, queryUnreadItems(user, subscription))
		}
	}
	return queries
}

// queryReadItems generates a query for finding read items for the given subscription.
func queryReadItems(user *User, source ItemSource) query.Option {
	// if subscription.GetSubscriptionType() != SubscriptionTypeFeed {
	// 	return nil
	// }
	return query.Bool(
		query.WithBoolQueryName(source.GetFeedID()+"_read_items"),
		query.Filter(
			// Must match this feed.
			query.Term("feed_id", source.GetFeedID()),
			// And should be between the user max history and last read time.
			query.Bool(
				query.Should(
					query.Between("published", user.GetMaxHistory(), source.GetMarkedReadAt()),
					query.Between("updated", user.GetMaxHistory(), source.GetMarkedReadAt()),
					query.Terms("item_id", source.GetReadItems()...),
				),
				// Must not match any unread items for the feed
				query.MustNot(
					query.Terms("item_id", source.GetUnreadItems()...),
				),
			),
		),
		// User-specified field-level filtering.
		ArticleFiltersQueryClause(source),
	)
}

// QueryUnreadItems generates a query for finding unread items for the given subscription.
func queryUnreadItems(_ *User, source ItemSource) query.Option {
	return query.Bool(
		query.WithBoolQueryName(source.GetFeedID()+"_unread_items"),
		query.Filter(
			// Must match this feed.
			query.Term("feed_id", source.GetFeedID()),
			query.Bool(
				query.Should(
					query.Since("published", source.GetMarkedReadAt()),
					query.Since("updated", source.GetMarkedReadAt()),
					query.Terms("item_id", source.GetUnreadItems()...),
				),
			),
		),
		// Must not match any read items for the feed
		query.MustNot(
			query.Terms("item_id", source.GetReadItems()...),
		),
		// User-specified field-level filtering.
		ArticleFiltersQueryClause(source),
	)
}

// subscriptionQueryReadItems generates a query for finding all items for the given subscription.
func queryAllItems(user *User, source ItemSource) query.Option {
	maxHistory := user.GetMaxHistory()
	return query.Bool(
		query.WithBoolQueryName(source.GetFeedID()+"_all_items"),
		query.Filter(
			// Must match this feed.
			query.Term("feed_id", source.GetFeedID()),
			// And be published/updated since the user max history.
			query.Bool(
				query.Should(
					query.Since("published", maxHistory),
					query.Since("updated", maxHistory),
				),
			),
		),
		// User-specified field-level filtering.
		ArticleFiltersQueryClause(source),
	)
}

func ArticleFiltersQueryClause(source ItemSource) query.BoolOption {
	return query.Must(
		query.SimpleQueryString(
			query.WithSimpleQueryStringText(source.GetArticleFilters().Text),
			query.WithSimpleQueryStringFields("title", "description", "content"),
			query.WithSimpleQueryStringOperator(&operator.And),
		),
		query.SimpleQueryString(
			query.WithSimpleQueryStringText(source.GetArticleFilters().Authors),
			query.WithSimpleQueryStringFields("authors", "contributors"),
			query.WithSimpleQueryStringOperator(&operator.And),
		),
		query.SimpleQueryString(
			query.WithSimpleQueryStringText(source.GetArticleFilters().Categories),
			query.WithSimpleQueryStringFields("categories"),
			query.WithSimpleQueryStringOperator(&operator.And),
		),
	)
}

// NewFeedSubscription creates a new subscription for a feed from the request and feed details.
func NewFeedSubscription(ctx context.Context, feed *Feed, request *AddFeedSubscriptionRequest) (*Subscription, error) {
	// Create state based on feed and user data.
	feedSubscription := &FeedSubscription{
		URL:           feed.GetLink(),
		FeedID:        feed.GetID(),
		ArticleStates: make(map[ItemID]ArticleState),
	}

	settings := newSubscriptionSettings()

	customisation := newSubscriptionCustomisation(feed.GetTitle(), feed.GetImage(), feed.GetCategories())
	// Override with any user customisations.
	if request != nil {
		if request.Nickname != nil {
			customisation.Nickname = *request.Nickname
		}
		if len(request.Categories) > 0 {
			customisation.Categories = request.Categories
		}
	}

	subscription, err := newSubscription(ctx, *customisation, settings, feedSubscription)
	if err != nil {
		return nil, fmt.Errorf("new feed subscription: %w", err)
	}

	return subscription, nil
}

// Valid returns a boolean indicating if the Subscription contains valid data (true). If it contains invalid data
// (false) a non-nil error is also returned which contains validation issues.
func (s *FeedSubscription) Valid() error {
	if err := validation.Validate.Struct(s); err != nil {
		return fmt.Errorf("feed subscription is invalid: %w", err)
	}
	return nil
}

// GetFeedID returns the feed ID.
func (s *FeedSubscription) GetFeedID() FeedID {
	return s.FeedID
}

// NewSearchSubscription creates a new SearchSubscription. A SearchSubscription collates articles that match a search
// into a single custom subscription.
func NewSearchSubscription(ctx context.Context, request *SearchSubscriptionRequest) (*Subscription, error) {
	searchSubscription := &SearchSubscription{
		Search: request.Search,
	}
	subscription, err := newSubscription(ctx, *request.Customisation, request.Settings, searchSubscription)
	if err != nil {
		return nil, fmt.Errorf("new search subscription: %w", err)
	}
	subscription.Favorite = true
	return subscription, nil
}

// Valid returns a boolean indicating if the Subscription contains valid data (true). If it contains invalid data
// (false) a non-nil error is also returned which contains validation issues.
func (s *SearchSubscription) Valid() error {
	if err := validation.Validate.Struct(s); err != nil {
		return fmt.Errorf("search subscription is invalid: %w", err)
	}
	return nil
}

// NewGroupSubscription creates a GroupSubscription. A GroupSubscription is a kind of meta-subscription that aggregates
// all articles from multiple individual subscriptions into a single custom subscription.
func NewGroupSubscription(ctx context.Context, request *GroupSubscriptionRequest) (*Subscription, error) {
	groupSubscription := &GroupSubscription{
		Subscriptions: slices.Collect(maps.Keys(request.Subscriptions)),
	}
	subscription, err := newSubscription(ctx, *request.Customisation, request.Settings, groupSubscription)
	if err != nil {
		return nil, fmt.Errorf("new group subscription: %w", err)
	}
	subscription.Favorite = true
	return subscription, nil
}

// Valid returns a boolean indicating if the Subscription contains valid data (true). If it contains invalid data
// (false) a non-nil error is also returned which contains validation issues.
func (s *GroupSubscription) Valid() error {
	if err := validation.Validate.Struct(s); err != nil {
		return fmt.Errorf("feed subscription is invalid: %w", err)
	}
	return nil
}

func NewEmailSubscription(
	ctx context.Context,
	userID UserID,
	from *mail.Address,
) (*Subscription, error) {
	// Validate sender address.
	if from.Address == "" {
		return nil, fmt.Errorf("%w: blank sender address", validation.ErrInvalid)
	}
	if err := validation.Validate.Var(from.Address, "required,email"); err != nil {
		return nil, fmt.Errorf("%w: sender address: %w", validation.ErrInvalid, err)
	}

	emailSubscription := &EmailSubscription{
		EmailSenderID: from.Address,
	}
	customisation := newSubscriptionCustomisation(from.String(), nil, nil)
	settings := newSubscriptionSettings()

	subscription, err := newSubscription(ctx, *customisation, settings, emailSubscription)
	if err != nil {
		return nil, fmt.Errorf("new group subscription: %w", err)
	}

	// Override the default SubscriptionID generation
	subscription.SubscriptionID = "sub_" + strconv.FormatUint(
		xxh3.Hash([]byte(userID+from.Address)),
		10,
	)

	// Generate a "virtual" FeedID.
	subscription.EmailData.FeedID = strings.ReplaceAll(subscription.GetID(), "sub_", "feed_")

	return subscription, nil
}

// GetEmailSubscription retrieves an EmailSubscription for the given user ID and email sender.
func GetEmailSubscription(ctx context.Context, userID UserID, from *mail.Address) (*Subscription, error) {
	// Check for an existing email subscription for the user.
	subscriptions, _, err := SearchSubscriptions(ctx,
		query.Bool(
			query.Filter(
				query.Term("user_id", userID),
				query.Term("email_data.email_sender_id", from.Address),
			),
		))
	if err != nil {
		return nil, fmt.Errorf("search subscriptions: %w", err)
	}

	var subscription *Subscription
	switch {
	case len(subscriptions) > 1:
		// Ambiguous subscription match for sender.
		return nil, fmt.Errorf("%w: ambiguous subscription match for sender", ErrInvalidAPIResult)
	case len(subscriptions) == 0:
		// Create a new email subscription for this sender.
		subscription, err = NewEmailSubscription(ctx, userID, from)
		if err != nil {
			return nil, fmt.Errorf("create email subscription: %w", err)
		}
		if err := AddSubscriptions(ctx, subscription); err != nil {
			return nil, fmt.Errorf("add email subscription: %w", err)
		}
	default:
		// Use the existing email subscription.
		subscription = subscriptions[0]
	}

	return subscription, nil
}

// Valid returns a non-nil error if the EmailSubscription contains invalid data.
func (s *EmailSubscription) Valid() error {
	if err := validation.Validate.Struct(s); err != nil {
		return fmt.Errorf("email subscription is invalid: %w", err)
	}
	return nil
}

// Valid returns a boolean indicating whether the SubscriptionRequest is valid,
// and any validation errors if applicable.
func (r *EditEmailSubscriptionRequest) Valid() error {
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("email subscription validation error: %w", err)
	}
	return nil
}

// Sanitise will sanitise the input values of the SubscriptionRequest.
func (r *EditEmailSubscriptionRequest) Sanitise() error {
	if r.Customisation.Nickname != "" {
		r.Customisation.Nickname = validation.SanitizeString(r.Customisation.Nickname)
	}
	categories := make([]Category, 0, len(r.Customisation.Categories))
	for category := range slices.Values(r.Customisation.Categories) {
		category = validation.SanitizeString(category)
		categories = append(categories, category)
	}
	r.Customisation.Categories = categories
	return nil
}

// Valid returns a boolean indicating if the Subscription contains valid data (true). If it contains invalid data
// (false) a non-nil error is also returned which contains validation issues.
func (s *Subscription) Valid() error {
	if err := validation.Validate.Struct(s); err != nil {
		return fmt.Errorf("subscription is invalid: %w", err)
	}
	return nil
}

// GetID returns the unqiue ID of the subscription.
func (s *Subscription) GetID() SubscriptionID {
	return s.SubscriptionID
}

// GetFeedID returns the unqiue FeedID of the subscription. Not all subscription types will have a FeedID.
func (s *Subscription) GetFeedID() FeedID {
	switch s.Type {
	case SubscriptionTypeFeed:
		return s.FeedData.FeedID
	case SubscriptionTypeEmail:
		return s.EmailData.FeedID
	default:
		return ""
	}
}

// GetSubscriptionType returns the type of subscription (i.e., feed, search, group, etc.).
func (s *Subscription) GetSubscriptionType() SubscriptionType {
	return s.Type
}

// GetUpdatedDate returns the timestamp that represents when the subscription was last updated. Usually, this means the
// timestamp of the newest article in the subscription.
func (s *Subscription) GetUpdatedDate() time.Time {
	return s.Stats.LastUpdate
}

// GetTitle returns the title (or user nickname if assigned) of the subscription.
func (s *Subscription) GetTitle() string {
	if s.Customisation != nil {
		return s.Customisation.Nickname
	}
	return ""
}

// GetCategories returns the categories of the subscription. It is the combined list of any user-assigned categories and
// the categories in the feed content.
func (s *Subscription) GetCategories(maxCount int) Categories {
	var all []Category
	if s.Customisation.Categories != nil {
		all = s.Customisation.Categories
		if maxCount != 0 {
			if len(all) > maxCount {
				return all[:maxCount]
			}
			return all
		}
	}
	return all
}

// GetImage retrieves the image that represents the subscription, or nil if no image is available.
func (s *Subscription) GetImage() URL {
	if s.Customisation != nil {
		if s.Customisation.ImageURL != nil {
			return *s.Customisation.ImageURL
		}
	}
	return ""
}

// GetLink returns the source feed link. For a search subscription, there is no source so this returns an empty string.
func (s *Subscription) GetLink() string {
	switch s.Type {
	case SubscriptionTypeFeed:
		return s.FeedData.URL
	default:
		return ""
	}
}

func (s *Subscription) GetArticleFilters() SubscriptionArticleFilters {
	switch {
	case s.Type == SubscriptionTypeFeed && s.FeedData.ArticleFilters != nil:
		return *s.FeedData.ArticleFilters
	case s.Type == SubscriptionTypeEmail && s.EmailData.ArticleFilters != nil:
		return *s.EmailData.ArticleFilters
	case s.Type == SubscriptionTypeGroup && s.GroupData.ArticleFilters != nil:
		return *s.GroupData.ArticleFilters
	default:
		return SubscriptionArticleFilters{}
	}
}

// GetStats returns the stats object containing the dynamically generated stats (i.e., unread count, article rate) of
// the subscription.
func (s *Subscription) GetStats() *SubscriptionStats {
	if s.Stats != nil {
		return s.Stats
	}
	s.Stats = &SubscriptionStats{}
	return s.Stats
}

// IsFavorite returns a boolean indicating whether the user has marked this subscription as a favorite. Note for some
// subscription types, this will always return true.
func (s *Subscription) IsFavorite() bool {
	return s.Favorite
}

func (s *Subscription) GetMarkedReadAt() time.Time {
	if s.MarkedReadAt != nil {
		return *s.MarkedReadAt
	}
	return UnixEpoch
}

// GetReadItems retrieves a list of ItemIDs for the feed subscription that
// user has explicitly marked as read.
func (s *Subscription) GetReadItems() []ItemID {
	ids := make([]ItemID, 0)
	states := make(map[ItemID]ArticleState)
	switch s.Type {
	case SubscriptionTypeFeed:
		maps.Copy(states, s.FeedData.ArticleStates)
	case SubscriptionTypeEmail:
		maps.Copy(states, s.EmailData.ArticleStates)
	default:
		return nil
	}

	for id, state := range states {
		if state.Read {
			ids = append(ids, id)
		}
	}
	return ids
}

// GetUnreadItems retrieves a list of ItemIDs for the feed subscription that
// user has explicitly marked as unread.
func (s *Subscription) GetUnreadItems() []ItemID {
	ids := make([]ItemID, 0)
	states := make(map[ItemID]ArticleState)
	switch s.Type {
	case SubscriptionTypeFeed:
		maps.Copy(states, s.FeedData.ArticleStates)
	case SubscriptionTypeEmail:
		maps.Copy(states, s.EmailData.ArticleStates)
	default:
		return nil
	}

	for id, state := range states {
		if !state.Read {
			ids = append(ids, id)
		}
	}
	return ids
}

// GetItemState retrieves the item state (read/unread/saved) from the
// subscription. By default it will return unread unless the user has explicitly
// marked or saved the item.
func (s *Subscription) GetItemState(itemID ItemID) *ArticleState {
	states := make(map[ItemID]ArticleState)
	switch s.Type {
	case SubscriptionTypeFeed:
		maps.Copy(states, s.FeedData.ArticleStates)
	case SubscriptionTypeEmail:
		maps.Copy(states, s.EmailData.ArticleStates)
	default:
		return &ArticleState{
			Read: false,
		}
	}

	// If the subscription has no article states, return unread state.
	if len(states) == 0 {
		return &ArticleState{
			Read: false,
		}
	}

	// If a state is found return that.
	if state, found := states[itemID]; found {
		return &state
	}

	// Return unread state if no state found.
	return &ArticleState{
		Read: false,
	}
}

// SetItemState will set the state of the item to the given state.
func (s *Subscription) SetItemState(itemID ItemID, state *ArticleState) {
	switch s.Type {
	case SubscriptionTypeFeed:
		if s.FeedData.ArticleStates == nil {
			s.FeedData.ArticleStates = make(map[ItemID]ArticleState)
		}
		s.FeedData.ArticleStates[itemID] = *state
	case SubscriptionTypeEmail:
		if s.EmailData.ArticleStates == nil {
			s.EmailData.ArticleStates = make(map[ItemID]ArticleState)
		}
		s.EmailData.ArticleStates[itemID] = *state
	}
}

// Mark applies the given mark (read/unread) to a subscription.
func (s *Subscription) Mark(user *User, mark Mark) {
	var ts time.Time
	switch mark {
	case MarkRead:
		// Set marked at to now when marking read.
		ts = time.Now().UTC()
	case MarkUnread:
		// Set marked at to max history when marking unread.
		ts = user.GetMaxHistory()
	}
	s.MarkedReadAt = &ts
	// Reset article states for appropriate subscription types.
	switch s.GetSubscriptionType() {
	case SubscriptionTypeFeed:
		s.FeedData.ArticleStates = nil
	case SubscriptionTypeEmail:
		s.EmailData.ArticleStates = nil
	}
}

// MarkItemsRead will mark the given items as read for the subscription.
func (s *Subscription) MarkItemsRead(itemIDs ...ItemID) {
	for itemID := range slices.Values(itemIDs) {
		if !s.GetItemState(itemID).Read {
			s.SetItemState(itemID, &ArticleState{Read: true, UpdatedAt: time.Now().UTC()})
		}
	}
}

// MarkItemsUnread will mark the given items as unread for the subscription.
func (s *Subscription) MarkItemsUnread(itemIDs ...ItemID) {
	for itemID := range slices.Values(itemIDs) {
		if s.GetItemState(itemID).Read {
			s.SetItemState(itemID, &ArticleState{Read: false, UpdatedAt: time.Now().UTC()})
		}
	}
}

// MarkItems marks the given items in a user subscription the given mark.
func (s *Subscription) MarkItems(mark Mark, itemIDs ...ItemID) {
	switch mark {
	case MarkRead:
		s.MarkItemsRead(itemIDs...)
	case MarkUnread:
		s.MarkItemsUnread(itemIDs...)
	}
}

// Subscriptions is a slice of subscriptions of any type.
type Subscriptions []*Subscription

// GetIDs returns the subscription ids for all subscription states in the slice.
func (s Subscriptions) GetIDs() []SubscriptionID {
	ids := make([]SubscriptionID, 0, len(s))
	for subscription := range slices.Values(s) {
		ids = append(ids, subscription.GetID())
	}
	return ids
}

// GetFeedIDs returns the IDs of feeds the subscriptions are for. This may return an empty slice if the subscriptions
// are only of type search, for example as those subscriptions do not represent any particular feed.
func (s Subscriptions) GetFeedIDs() []FeedID {
	ids := make([]FeedID, 0)
	for subscription := range slices.Values(s) {
		switch subscription.Type {
		case SubscriptionTypeFeed, SubscriptionTypeEmail:
			ids = append(ids, subscription.GetFeedID())
		case SubscriptionTypeGroup:
			ids = append(ids, subscription.GroupData.Subscriptions...)
		}
	}
	return slices.Compact(ids)
}

// GetCategories returns all categories across all the subscriptions. Duplicates are removed.
func (s Subscriptions) GetCategories() Categories {
	categories := make(Categories, 0)
	for subscription := range slices.Values(s) {
		categories = append(categories, subscription.GetCategories(0)...)
	}
	return slices.Compact(categories)
}

// GetByFeedID will return the subscription that matches the given feed ID, if any.
func (s Subscriptions) GetByFeedID(id FeedID) *Subscription {
	if idx := slices.IndexFunc(s, func(e *Subscription) bool {
		return e.GetFeedID() == id
	}); idx != -1 {
		return s[idx]
	}
	return nil
}

// FilterByCategories returns a new slice containing the subscriptions which have a category matching the given
// categories.
func (s Subscriptions) FilterByCategories(categories ...Category) Subscriptions {
	if len(categories) == 0 {
		return s
	}
	return slices.Collect(FilterSlice(s, func(subscription *Subscription) bool {
		for category := range slices.Values(categories) {
			return slices.Contains(subscription.GetCategories(0), category)
		}
		return false
	}))
}

// FilterByView returns a slice containing the subscription which match the given view state.
func (s Subscriptions) FilterByView(view View) Subscriptions {
	switch view {
	case ViewRead:
		return slices.Collect(FilterSlice(s, func(subscription *Subscription) bool {
			return !subscription.GetStats().IsUnread()
		}))
	case ViewUnread:
		return slices.Collect(FilterSlice(s, func(subscription *Subscription) bool {
			return subscription.GetStats().IsUnread()
		}))
	default:
		return s
	}
}

// FilterByFavorites returns a slice containing only favorite subscriptions.
func (s Subscriptions) FilterByFavorites(value bool) Subscriptions {
	if !value {
		return s
	}
	return slices.Collect(FilterSlice(s, func(subscription *Subscription) bool {
		return subscription.IsFavorite()
	}))
}

// FilterByType returns a slice containing subscriptions of the specified type.
func (s Subscriptions) FilterByType(t ...SubscriptionType) Subscriptions {
	return slices.Collect(FilterSlice(s, func(subscription *Subscription) bool {
		return slices.Contains(t, subscription.GetSubscriptionType())
	}))
}

// Sort will sort the slice of subscriptions by the given sort option. Favorite subscriptions are always sorted before
// other subscriptions, and the sort option is used as a tiebreaker.
func (s Subscriptions) Sort(sort Sort) Subscriptions {
	sort = setValidSort(sort)
	switch sort {
	case SortNewestFirst, SortOldestFirst:
		slices.SortFunc(s, func(subscriptionA, subscriptionB *Subscription) int {
			switch {
			case subscriptionA.IsFavorite() && !subscriptionB.IsFavorite(): // Favorites before non-favorites.
				return 1
			case !subscriptionA.IsFavorite() && subscriptionB.IsFavorite(): // Favorites before non-favorites.
				return -1
			default:
				return subscriptionA.GetUpdatedDate().Compare(subscriptionB.GetUpdatedDate())
			}
		})
		// Reverse sort for newest first.
		if sort == SortNewestFirst {
			slices.Reverse(s)
		}
	case SortMostUnread, SortLeastUnread:
		// Sort by unread count, with favorite or search subscriptions before non-favorites/non-search subscriptions.
		slices.SortFunc(s, func(subscriptionA, subscriptionB *Subscription) int {
			switch {
			case subscriptionA.IsFavorite() && !subscriptionB.IsFavorite(): // Favorites before non-favorites.
				return 1
			case !subscriptionA.IsFavorite() && subscriptionB.IsFavorite(): // Favorites before non-favorites.
				return -1
			case subscriptionA.GetSubscriptionType() != SubscriptionTypeFeed && subscriptionB.GetSubscriptionType() == SubscriptionTypeFeed: // Non-feed type before feed type.
				return 1
			case subscriptionA.GetSubscriptionType() == SubscriptionTypeFeed && subscriptionB.GetSubscriptionType() != SubscriptionTypeFeed: // Non-feed type before feed type.
				return -1
			case subscriptionA.GetSubscriptionType() != SubscriptionTypeFeed && subscriptionB.GetSubscriptionType() != SubscriptionTypeFeed: // Use title is tiebreaker where both non feed type.
				return cmp.Compare(subscriptionA.GetTitle(), subscriptionB.GetTitle())
			default:
				cmpValue := cmp.Compare(subscriptionA.GetStats().UnreadTotal(), subscriptionB.GetStats().UnreadTotal())
				if cmpValue == 0 { // Use date as tiebreaker for equal unread counts.
					return subscriptionA.GetUpdatedDate().Compare(subscriptionB.GetUpdatedDate())
				}
				return cmpValue
			}
		})
		// Reverse sort for most unread.
		if sort == SortMostUnread {
			slices.Reverse(s)
		}
	}
	return s
}

// Paginate will paginate through a slice of subscriptions, returning a new slice of subscriptions and the next
// pagination value (if any).
func (s Subscriptions) Paginate(pagination Pagination, count int) (Subscriptions, Pagination) {
	var from, to int
	if pagination != "" {
		if value, err := strconv.Atoi(pagination); err == nil {
			from = value
		}
	}
	to = min(from+count, len(s))
	newPagination := strconv.Itoa(to)
	return s[from:to], newPagination
}

// GetCategoryCounts returns a count of the occurrence of a Category across all
// the Subscriptions.
func GetCategoryCounts(subscriptions ...*Subscription) CategoryCounts {
	countsMap := make(map[Category]int)
	for object := range slices.Values(subscriptions) {
		for category := range slices.Values(object.GetCategories(0)) {
			countsMap[category]++
		}
	}
	var counts CategoryCounts
	for category, count := range maps.All(countsMap) {
		counts = append(counts, CategoryCount{Category: category, Count: count})
	}

	return counts
}

// Valid will return an error if the request object does not pass validation.
func (r *ListRequest) Valid() error {
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("validate list subscription request: %w", err)
	}
	if err := r.Filters.Valid(); err != nil {
		return fmt.Errorf("validate filters: %w", err)
	}
	return nil
}

// Valid returns a boolean indicating whether the SubscriptionRequest is valid,
// and any validation errors if applicable.
func (r *AddFeedSubscriptionRequest) Valid() error {
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("subscription validation error: %w", err)
	}
	return nil
}

// Sanitise will sanitise the input values of the SubscriptionRequest.
func (r *AddFeedSubscriptionRequest) Sanitise() error {
	r.URL = validation.SanitizeString(r.URL) // sanitise string.
	r.URL = strings.TrimPrefix(r.URL, "/")   // remove trailing slash.
	rssURL, err := url.Parse(r.URL)
	if err != nil {
		return fmt.Errorf("unable to parse url: %w", err)
	}
	// Ensure scheme is set.
	if rssURL.Scheme != "https" && rssURL.Scheme != "http" {
		rssURL.Scheme = "https"
		r.URL = rssURL.String()
	}
	if r.Nickname != nil {
		sanitizedNickname := validation.SanitizeString(*r.Nickname)
		r.Nickname = &sanitizedNickname
	}
	categories := make([]Category, 0, len(r.Categories))
	for category := range slices.Values(r.Categories) {
		category = validation.SanitizeString(category)
		categories = append(categories, category)
	}
	r.Categories = categories
	return nil
}

// GetURL returns the (feed) URL for the request.
func (r *AddFeedSubscriptionRequest) GetURL() string {
	return strings.TrimSpace(r.URL)
}

// GetNickname returns the nickname chosen for the subscription.
func (r *AddFeedSubscriptionRequest) GetNickname() string {
	if r.Nickname != nil {
		return *r.Nickname
	}
	return ""
}

// Valid checks that the MarkSubscriptionRequest contains valid data.
func (s *MarkSubscriptionRequest) Valid() error {
	if err := validation.Validate.Struct(s); err != nil {
		return fmt.Errorf("mark subscription request is invalid: %w", err)
	}
	return nil
}

// Sanitise will sanitise the MarkSubscriptionRequest, ensuring it contains valid field values.
func (s *MarkSubscriptionRequest) Sanitise() error {
	return nil
}

// Valid checks that the MarkSubscriptionsRequest contains valid data.
func (s *MarkSubscriptionsRequest) Valid() error {
	if err := validation.Validate.Struct(s); err != nil {
		return fmt.Errorf("mark subscriptions request is invalid: %w", err)
	}
	return nil
}

// Sanitise will sanitise the MarkSubscriptionsRequest, ensuring it contains valid field values.
func (s *MarkSubscriptionsRequest) Sanitise() error {
	for idx, id := range s.Subscriptions {
		s.Subscriptions[idx] = validation.SanitizeString(id)
	}
	s.View = setValidView(s.View)
	return nil
}

// Valid checks that the FavoriteSubscriptionsRequest contains valid data.
func (s *FavoriteSubscriptionRequest) Valid() error {
	if err := validation.Validate.Struct(s); err != nil {
		return fmt.Errorf("favorite subscriptions request is invalid: %w", err)
	}
	return nil
}

// Sanitise will sanitise the FavoriteSubscriptionsRequest, ensuring it contains valid field values.
func (s *FavoriteSubscriptionRequest) Sanitise() error {
	return nil
}

// Valid checks that the RemoveSubscriptionRequest contains valid data.
func (r *RemoveSubscriptionRequest) Valid() error {
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("remove subscription request is invalid: %w", err)
	}
	return nil
}

// AddSubscriptionResult represents the result of creating a new subscription.
type AddSubscriptionResult struct {
	Subscription *FeedSubscription
	Message      *UserMessage
}

// UpdateFrequency returns a string that roughly indicates how often the subscription is updated.
func (s *SubscriptionStats) UpdateFrequency() string {
	switch {
	case s.AvgDailyUpdates > 1:
		return fmt.Sprintf("%.0f articles/day", s.AvgDailyUpdates)
	case s.AvgDailyUpdates < 1 && s.AvgDailyUpdates > 0.5:
		return "A few times a week"
	case s.AvgDailyUpdates < 0.5 && s.AvgDailyUpdates > 0.25:
		return "About weekly"
	default:
		return "Infrequent"
	}
}

// UnreadTotal returns the unread count of items in the subscription.
func (s *SubscriptionStats) UnreadTotal() int {
	return s.UnreadCount
}

// IsUnread returns a boolean indicating whether the subscription is considered unread.
func (s *SubscriptionStats) IsUnread() bool {
	return s.UnreadCount > 0
}

func newSubscription(
	ctx context.Context,
	customisation SubscriptionCustomisation,
	settings *SubscriptionSettings,
	data any,
) (*Subscription, error) {
	user := UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get user data: %w", ErrCtxValueNotFound)
	}
	ts := time.Now().UTC()
	mr := user.GetMaxHistory()
	subscription := &Subscription{
		SubscriptionID: "sub_" + strconv.FormatUint(xxh3.Hash([]byte(user.GetID()+customisation.Nickname)), 10),
		UserID:         user.GetID(),
		UpdatedAt:      &ts,
		CreatedAt:      ts,
		MarkedReadAt:   &mr,
		Customisation:  &customisation,
		Settings:       *newSubscriptionSettings(),
		Favorite:       false,
	}
	if settings != nil {
		subscription.Settings = *settings
	}

	switch typeData := data.(type) {
	case *FeedSubscription:
		subscription.Type = SubscriptionTypeFeed
		subscription.FeedData = typeData
	case *SearchSubscription:
		subscription.Type = SubscriptionTypeSearch
		subscription.SearchData = typeData
		subscription.Favorite = true
	case *GroupSubscription:
		subscription.Type = SubscriptionTypeGroup
		subscription.GroupData = typeData
		subscription.Favorite = true
	case *EmailSubscription:
		subscription.Type = SubscriptionTypeEmail
		subscription.EmailData = typeData
	default:
		return nil, fmt.Errorf("new subscription: %w", ErrInvalidAPIResult)
	}

	return subscription, nil
}

func newSubscriptionCustomisation(
	nickname string,
	image *types.ImageInfo,
	categories Categories,
) *SubscriptionCustomisation {
	customisation := &SubscriptionCustomisation{
		Nickname:   nickname,
		Categories: categories,
	}
	if image != nil {
		customisation.ImageURL = new(image.GetURL())
	}
	return customisation
}

func newSubscriptionSettings() *SubscriptionSettings {
	return &SubscriptionSettings{
		ShowFullArticleContent: false,
	}
}

// SubscriptionSorting contains the sort options for sorting subscription results.
type SubscriptionSorting struct {
	MarkedReadAt   string `json:"marked_read_at"`
	SubscriptionID string `json:"subscription_id"`
}

// SortCombinationsCaster is required to allow FeedSorting to be used as Elasticsearch sort values.
func (s *SubscriptionSorting) SortCombinationsCaster() *estypes.SortCombinations {
	c := estypes.SortCombinations(s)
	return &c
}

func newSubscriptionSortOptions(sort *Sort) []estypes.SortCombinationsVariant {
	if sort == nil {
		return []estypes.SortCombinationsVariant{&estypes.SortOptions{Doc_: estypes.NewScoreSort()}}
	}
	var opts []estypes.SortCombinationsVariant
	switch *sort {
	case SortNewestFirst:
		opts = append(opts, &SubscriptionSorting{
			MarkedReadAt:   "asc",
			SubscriptionID: "desc",
		})
	case SortOldestFirst:
		opts = append(opts, &SubscriptionSorting{
			MarkedReadAt:   "desc",
			SubscriptionID: "asc",
		})
	case SortMostRelevant:
		opts = append(opts, &estypes.SortOptions{
			Score_: &estypes.ScoreSort{
				Order: &sortorder.Desc,
			},
		})
		opts = append(opts,
			&SubscriptionSorting{
				MarkedReadAt:   "asc",
				SubscriptionID: "asc",
			},
		)
	default:
		opts = append(opts, &estypes.SortOptions{
			Doc_: estypes.NewScoreSort(),
		})
	}
	return opts
}
