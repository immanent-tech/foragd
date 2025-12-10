// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/count"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/deletebyquery"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/get"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/mget"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/calendarinterval"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/refresh"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/sortorder"
	"github.com/go-chi/chi/v5/middleware"
	slogctx "github.com/veqryn/slog-context"
	"golang.org/x/sync/errgroup"

	"github.com/immanent-tech/foragd/logging"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic/aggregations"
	"github.com/immanent-tech/foragd/providers/elastic/bulk"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/providers/elastic/results"
	"github.com/immanent-tech/foragd/server/session/store"
	"github.com/immanent-tech/foragd/validation"
)

const (
	defaultPaginationSize = 5000
	defaultRetries        = 5
)

var (
	errNotFound     = errors.New("not found")
	ErrNotFound     = models.NewAPIError(errNotFound, http.StatusNotFound)
	ErrNoIndexInCtx = models.NewAPIError(
		fmt.Errorf("get index from context: %w", errNotFound),
		http.StatusInternalServerError,
	)
	ErrNoUserInCtx = models.NewAPIError(
		fmt.Errorf("get user from context: %w", errNotFound),
		http.StatusInternalServerError,
	)
)

var (
	_ store.Datastore = (*API)(nil)

	_ types.FieldValueVariant = (*paginationValue[types.FieldValue])(nil)
)

// API is an object that provides access to the Elasticsearch API.
type API struct {
	*elasticsearch.TypedClient
}

// Object represents any kind of object that has an ID. Effectively the object can be indexed in Elasticsearch.
type Object[T ~string] interface {
	GetID() T
}

// Option is a generic type for functional options.
type Option[T any] func(T)

// GetAPI returns the raw API object.
func (a *API) GetAPI() *elasticsearch.TypedClient {
	return a.TypedClient
}

// GetSubscription returns the subscription that matches the given ID.
//
// Accepts the GetSubscriptionsDynamicInfo request option to generate dynamic info (i.e. stats) for the subscription.
func (a *API) GetSubscription(
	ctx context.Context,
	id models.SubscriptionID,
	options ...SubscriptionsRequestOption,
) (*models.Subscription, error) {
	// Parse and apply options.
	req := &subscriptionsRequest{}
	for option := range slices.Values(options) {
		option(req)
	}

	// Get user data.
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get subscription: get user data: %w", models.ErrNoUserCtx)
	}

	// Find matching subscription.
	subscriptions, _, err := a.searchSubscriptions(ctx,
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
		return nil, fmt.Errorf("get subscription: %w", err)
	case len(subscriptions) == 0:
		return nil, fmt.Errorf("get subscription: %w", ErrNotFound)
	case len(subscriptions) != 1:
		return nil, fmt.Errorf("get subscription: %w: too many subscriptions", models.ErrInvalidAPIResult)
	}

	// Add dynamic info if requested.
	if req.addDynamicInfo {
		err = a.addSubscriptionDynamicInfo(ctx, subscriptions)
		if err != nil {
			return nil, fmt.Errorf("get subscription: %w", err)
		}
	}

	return subscriptions[0], nil
}

// GetSubscriptionByFeedID returns the subscription that matches the given feed ID.
//
// Accepts the GetSubscriptionsDynamicInfo request option to generate dynamic info (i.e. stats) for the subscription.
func (a *API) GetSubscriptionByFeedID(
	ctx context.Context,
	id models.FeedID,
	options ...SubscriptionsRequestOption,
) (*models.Subscription, error) {
	// Parse and apply options.
	req := &subscriptionsRequest{}
	for option := range slices.Values(options) {
		option(req)
	}

	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get subscription by feed id: get user data: %w", models.ErrNoUserCtx)
	}
	subscriptions, _, err := a.searchSubscriptions(ctx,
		query.Bool(
			query.Filter(
				query.Term("user_id", user.GetID()),
				query.Term("type", models.SubscriptionTypeFeed),
				query.Term("feed_data.feed_id", id),
			),
		),
		searchSubscriptionsMaxResults(1),
	)
	switch {
	case err != nil:
		return nil, fmt.Errorf("get subscription by feed id: %w", err)
	case len(subscriptions) == 0:
		return nil, fmt.Errorf("get subscription by feed id: %w", ErrNotFound)
	case len(subscriptions) != 1:
		return nil, fmt.Errorf("get subscription by feed id: %w: too many subscriptions", models.ErrInvalidAPIResult)
	}

	// Add dynamic info if requested.
	if req.addDynamicInfo {
		err = a.addSubscriptionDynamicInfo(ctx, subscriptions)
		if err != nil {
			return nil, fmt.Errorf("get subscription by feed id: %w", err)
		}
	}

	return subscriptions[0], nil
}

// GetSubscriptionSuggestions returns subscriptions that match the given text.
//
// Accepts the GetSubscriptionsDynamicInfo request option to generate dynamic info (i.e. stats) for the subscription
// suggestions.
func (a *API) GetSubscriptionSuggestions(
	ctx context.Context,
	text string,
	count int,
	options ...SubscriptionsRequestOption,
) (models.Subscriptions, error) {
	// Parse and apply options.
	req := &subscriptionsRequest{}
	for option := range slices.Values(options) {
		option(req)
	}

	// Get subscriptions by ID.
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get subscription suggestions: get user data: %w", models.ErrNoUserCtx)
	}

	sort := models.SortMostRelevant
	subscriptions, _, err := a.searchSubscriptions(ctx,
		query.Bool(
			query.Filter(
				query.Term("user_id", user.GetID()),
			),
			query.Must(
				query.Bool(
					query.Should(
						query.SearchAsYouType(text, "customisation.nickname"),
					),
				),
			),
		),
		searchSubscriptionsMaxResults(count),
		searchSubscriptionsSortResults(&sort),
	)
	if err != nil {
		return nil, fmt.Errorf("get subscription suggestions: api request failed: %w", err)
	}
	if len(subscriptions) == 0 {
		return nil, fmt.Errorf("get subscription suggestions: %w", ErrNotFound)
	}

	// Add dynamic info if requested.
	if req.addDynamicInfo {
		err = a.addSubscriptionDynamicInfo(ctx, subscriptions)
		if err != nil {
			return nil, fmt.Errorf("get subscription suggestions: %w", err)
		}
	}

	return subscriptions, nil
}

// AddSubscriptions adds the given subscriptions to a user.
func (a *API) AddSubscriptions(ctx context.Context, subscriptions ...*models.Subscription) error {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return fmt.Errorf("add subscriptions: get user data: %w", ErrNoUserInCtx)
	}
	if _, err := a.UpdateSubscriptions(ctx, subscriptions...); err != nil {
		return fmt.Errorf("add subscriptions: %w", err)
	}
	// Disable onboarding once a subscription has been added.
	if settings := user.GetSettings(); settings.ShowOnboarding {
		settings.ShowOnboarding = false
		// Update the user object.
		if err := a.UpdateUser(ctx, user.GetID(), map[string]any{
			"settings": settings,
		}); err != nil {
			return fmt.Errorf("add subscriptions: update user: %w", err)
		}
	}
	return nil
}

// RemoveSubscriptions removes subscriptions with the given ID from a user.
func (a *API) RemoveSubscriptions(ctx context.Context, ids ...models.SubscriptionID) error {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return fmt.Errorf("remove subscriptions: %w", ErrNoUserInCtx)
	}
	index, err := SubscriptionsWriteIndexFromCtx(ctx)
	if err != nil {
		return fmt.Errorf("remove subscriptions: %w", ErrNoIndexInCtx)
	}
	err = DeleteDocs(ctx, a.GetAPI(), index,
		query.Bool(
			query.Filter(
				query.Term("user_id", user.GetID()),
				query.Terms("subscription_id", ids...),
			),
		),
	)
	if err != nil {
		return fmt.Errorf("remove subscriptions: %w", err)
	}
	return nil
}

// UpdateSubscriptions will bulk update the given subscriptions in Elasticsearch.
func (a *API) UpdateSubscriptions(
	ctx context.Context,
	subscriptions ...*models.Subscription,
) (map[models.SubscriptionID]*bulk.OperationResponse, error) {
	index, err := SubscriptionsWriteIndexFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("update subscriptions: %w", ErrNoIndexInCtx)
	}
	resp, err := BulkUpdate(ctx, a, index, subscriptions...)
	if err != nil {
		return nil, fmt.Errorf("update subscriptions: %w", err)
	}
	return resp, nil
}

// UpdateFavoriteSubscription changes the favorite status of a subscription by updating the user object to flag the
// subscription as appropriate.
func (a *API) UpdateFavoriteSubscription(ctx context.Context, id models.SubscriptionID, favorite bool) error {
	subscription, err := a.GetSubscription(ctx, id)
	if err != nil {
		return fmt.Errorf("update favorite subscription: get subscription: %w", err)
	}

	subscription.Favorite = favorite

	_, err = a.UpdateSubscriptions(ctx, subscription)
	if err != nil {
		return fmt.Errorf("update favorite subscription: update subscription: %w", err)
	}

	return nil
}

// CreateFeedSubscriptions will create new FeedSubscriptions for the user from the given requests.
func (a *API) CreateFeedSubscriptions(ctx context.Context, results ...*models.AddFeedSubscriptionResult) error {
	if len(results) == 0 {
		return nil
	}
	subscriptions := make(models.Subscriptions, 0, len(results))
	for result := range slices.Values(results) {
		slogctx.FromCtx(ctx).Debug("Creating new subscription.",
			slog.String("feed", result.Feed.GetTitle()),
			slog.String("url", result.Feed.GetLink()),
		)
		// Generate metadata.
		subscription, err := models.NewFeedSubscription(ctx, &result.Feed, &result.Request)
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
		result.Subscription = *subscription
		subscriptions = append(subscriptions, subscription)
		result.Message = *models.NewSuccessMessage("Subscription Created: "+result.Feed.GetTitle(), "Articles will be fetched shortly...")
	}
	// Add subscriptions
	if err := a.AddSubscriptions(ctx, subscriptions...); err != nil {
		return fmt.Errorf("unable to create subscriptions: %w", err)
	}
	return nil
}

// CreateSearchSubscriptions will create new SearchSubscriptions for the user from the given requests.
func (a *API) CreateSearchSubscriptions(ctx context.Context, requests ...*models.SearchSubscriptionRequest) error {
	subscriptions := make(models.Subscriptions, 0, len(requests))
	for request := range slices.Values(requests) {
		slogctx.FromCtx(ctx).Debug("Creating new search subscription.",
			slog.String("feed", request.Customisation.Nickname),
		)
		// Generate metadata.
		subscription, err := models.NewSearchSubscription(ctx, request)
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
	if err := a.AddSubscriptions(ctx, subscriptions...); err != nil {
		return fmt.Errorf("create search subscription: add subscriptions failed: %w", err)
	}
	return nil
}

type subscriptionsRequest struct {
	filterFavorites  bool
	filterIDs        []models.SubscriptionID
	filterFeedIDs    []models.FeedID
	filterCategories []models.Category
	addDynamicInfo   bool
}

type SubscriptionsRequestOption func(*subscriptionsRequest)

// GetSubscriptionsByFavorite option adds a filter to the query to get favorite subscriptions only.
func GetSubscriptionsByFavorite(value bool) SubscriptionsRequestOption {
	return func(sr *subscriptionsRequest) {
		sr.filterFavorites = value
	}
}

// GetSubscriptionsByIDs option adds a filter to the query to get subscriptions by their ids.
func GetSubscriptionsByIDs(ids ...models.SubscriptionID) SubscriptionsRequestOption {
	return func(sr *subscriptionsRequest) {
		sr.filterIDs = ids
	}
}

// GetSubscriptionsByFeedIDs option adds a filter to the query to get subscriptions by their feed ids.
func GetSubscriptionsByFeedIDs(ids ...models.FeedID) SubscriptionsRequestOption {
	return func(sr *subscriptionsRequest) {
		sr.filterFeedIDs = ids
	}
}

// GetSubscriptionsByCategories option adds a filter to the query to get subscriptions by category.
func GetSubscriptionsByCategories(categories ...models.Category) SubscriptionsRequestOption {
	return func(sr *subscriptionsRequest) {
		sr.filterCategories = categories
	}
}

// GetSubscriptionsDynamicInfo option will fill in the dynamic info (i.e., stats) after fetching.
func GetSubscriptionsDynamicInfo(value bool) SubscriptionsRequestOption {
	return func(sr *subscriptionsRequest) {
		sr.addDynamicInfo = value
	}
}

// GetSubscriptions performs a search request to fetch subscriptions. Accepts all request options to filter/enrich the
// results.
func (a *API) GetSubscriptions(
	ctx context.Context,
	options ...SubscriptionsRequestOption,
) (models.Subscriptions, error) {
	req := &subscriptionsRequest{}
	for option := range slices.Values(options) {
		option(req)
	}

	// Get user data.
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get subscriptions: %w", ErrNoUserInCtx)
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
		queries = append(queries, query.Terms("subscription_id", req.filterCategories...))
	}

	// Construct query.
	subscriptions, err := a.getAllSubscriptionsByQuery(ctx,
		query.Bool(
			query.Filter(
				queries...,
			),
		))
	if err != nil {
		return nil, fmt.Errorf("get subscriptions: %w", err)
	}
	if len(subscriptions) == 0 {
		return nil, fmt.Errorf("get subscriptions: %w", ErrNotFound)
	}

	if req.addDynamicInfo {
		if err = a.addSubscriptionDynamicInfo(ctx, subscriptions); err != nil {
			return nil, fmt.Errorf("get subscriptions: %w", err)
		}
	}

	return subscriptions, nil
}

// FilterSubscriptions returns subscriptions filtered by the given filters and paginated by the given pagination.
// Dynamic information for subscriptions will also be added.
func (a *API) FilterSubscriptions(
	ctx context.Context,
	filters *models.ListDisplayFilters,
	pagination models.Pagination,
) (models.Subscriptions, models.Pagination, error) {
	// Get subscriptions by ID.
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, "", fmt.Errorf("filter subscriptions: get user data: %w", models.ErrNoUserCtx)
	}
	subscriptionQuery := query.Bool(
		query.Filter(
			query.Term("user_id", user.GetID()),
			query.Terms("subscription_id", filters.Subscriptions...),
			// query.Term("favorite", filters.OnlyFavorites),
			query.Terms("customisation.categories.raw", filters.GetCategories()...),
		),
	)
	subscriptions, err := a.getAllSubscriptionsByQuery(ctx, subscriptionQuery)
	if err != nil {
		return nil, "", fmt.Errorf("filter subscriptions: api request failed: %w", err)
	}
	if len(subscriptions) == 0 {
		return nil, "", fmt.Errorf("filter subscriptions: %w", ErrNotFound)
	}
	// Add dynamic info.
	err = a.addSubscriptionDynamicInfo(ctx, subscriptions)
	if err != nil {
		return nil, "", fmt.Errorf("filter subscriptions: could not add dynamic info: %w", err)
	}
	// Sort and paginate.
	subscriptions, pagination = subscriptions.
		FilterByView(filters.GetView()).
		FilterByFavorites(filters.OnlyFavorites).
		Sort(filters.Sort).
		Paginate(pagination, filters.GetCount())
	return subscriptions, pagination, nil
}

// ProcessSubscriptionRequest manages parsing a subscription request and turning it into a subscription that can be
// added to a user. It handles finding a existing matching feed or creating a new one, checking the user isn't already
// subscribed and generating an appropriate subscription object. It returns an object that includes the request and new
// subscription data, or the request and an error if subscription data could not be generated.
//
//nolint:funlen
func (a *API) ProcessSubscriptionRequest(
	ctx context.Context,
	request *models.AddFeedSubscriptionRequest,
	resultsCh chan models.AddFeedSubscriptionResult,
) {
	result := models.AddFeedSubscriptionResult{
		Request: *request,
	}
	// Try to match request URL to an existing feed
	var feed *models.Feed
	feeds, _, err := a.SearchFeeds(ctx, query.Term("source_urls", request.GetURL()), 1, nil, nil)
	if err != nil {
		result.Error = err
		result.Message = *models.NewErrorMessage("Unable to determine existing subscription status", "The backend produced an error. This might be temporary, please try again.")
		resultsCh <- result
		return
	}
	if len(feeds) == 1 {
		feed = feeds[0]
	}

	// If no existing feed, create a new one.
	if feed == nil {
		slogctx.FromCtx(ctx).Debug("Parsing url", slog.String("url", request.GetURL()))
		var newFeed *models.Feed
		newFeed, err = models.NewFeedFromURL(ctx, request.GetURL())
		if err != nil {
			result.Error = err
			result.Message = *models.NewErrorMessage("Unable to create subscription", fmt.Sprintf("The feed URL %q cannot be parsed as a feed source or is not a valid URL.", request.GetURL()))
			resultsCh <- result
			return
		}
		err = validation.Validate.Struct(newFeed)
		if err != nil {
			result.Error = err
			result.Message = *models.NewErrorMessage("Unable to create subscription", fmt.Sprintf("The feed URL %q cannot be parsed as a feed source or is not a valid URL.", request.GetURL()))
			resultsCh <- result
			return
		}
		err = models.CreateFeed(ctx, a, newFeed)
		if err != nil {
			result.Error = err
			result.Message = *models.NewErrorMessage("Unable to create new feed for subscription", "The backend produced an error. This might be temporary, please try again.")
			resultsCh <- result
			return
		}
		slogctx.FromCtx(ctx).Debug("Created new feed for request.",
			slog.String("name", newFeed.GetTitle()),
			slog.String("urls", strings.Join(newFeed.GetSourceURLs(), ",")),
		)
		feed = newFeed
	}

	user := models.UserFromCtx(ctx)
	if user == nil {
		result.Error = ErrNoUserInCtx
		result.Message = *models.NewErrorMessage("Unable to check for existing subscription for "+request.GetURL(), "The backend produced an error. This might be temporary, please try again.")
		resultsCh <- result
		return
	}
	subscription, err := a.GetSubscriptionByFeedID(ctx, feed.GetID())
	if err != nil && models.HTTPStatus(err) != http.StatusNotFound {
		result.Error = err
		result.Message = *models.NewErrorMessage("Unable to check for existing subscription for "+request.GetURL(), "The backend produced an error. This might be temporary, please try again.")
		resultsCh <- result
		return
	}
	if subscription != nil {
		result.Error = models.NewAPIError(errors.New("already subscribed"), http.StatusConflict)
		result.Message = *models.NewWarningMessage("Already subscribed to feed", feed.GetTitle()+" ("+request.URL+")")
		resultsCh <- result
		return
	}

	// Add the feed details to the result.
	result.Feed = *feed
	// Send the result back through the channel.
	resultsCh <- result
}

type searchSubscriptionsRequest struct {
	count          int
	sort           *models.Sort
	pagination     *models.Pagination
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

func searchSubscriptionsSortResults(sort *models.Sort) searchSubscriptionsOption {
	return func(ssr *searchSubscriptionsRequest) {
		ssr.sort = sort
	}
}

func searchSubscriptionsPaginate(paginate *models.Pagination) searchSubscriptionsOption {
	return func(ssr *searchSubscriptionsRequest) {
		ssr.pagination = paginate
	}
}

func searchSubscriptionsAddDynamicInfo(value bool) searchSubscriptionsOption {
	return func(ssr *searchSubscriptionsRequest) {
		ssr.addDynamicInfo = value
	}
}

func (a *API) searchSubscriptions(
	ctx context.Context,
	query query.Option,
	options ...searchSubscriptionsOption,
) (models.Subscriptions, models.Pagination, error) {
	req := &searchSubscriptionsRequest{}

	// Build request with options.
	for option := range slices.Values(options) {
		option(req)
	}

	// Get user data.
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, "", fmt.Errorf("search subscriptions: %w", ErrNoUserInCtx)
	}

	// // Append any optional queries.
	// queries := []query.Option{query.Term("user_id", user.GetID())}
	// if len(req.queries) > 0 {
	// 	queries = append(queries, req.queries...)
	// }

	// Add sane values where not supplied.
	if req.count == 0 {
		req.count = 10
	}

	index, err := SubscriptionsReadIndexFromCtx(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("search subscriptions: %w", ErrNoIndexInCtx)
	}

	searchAfter, err := decodePagination(req.pagination)
	if err != nil {
		return nil, "", models.NewAPIError( //nolint:wrapcheck
			fmt.Errorf("search subscriptions: decode pagination failed: %w", err),
			http.StatusInternalServerError,
		)
	}

	// Perform search.
	subscriptions, newSearchAfter, err := Search[*models.Subscription](
		ctx,
		a.GetAPI(),
		index,
		query,
		req.count,
		WithSortOptions[*search.Search, SearchRequest](newSubscriptionSortOptions(req.sort)...),
		WithSearchAfter[*search.Search, SearchRequest](searchAfter...),
	)
	if err != nil {
		return nil, "", fmt.Errorf("search subscriptions: %w", err)
	}
	// Parse search after into pagination.
	if req.pagination != nil {
		*req.pagination, err = encodePagination(newSearchAfter)
		if err != nil {
			return nil, "", models.NewAPIError( //nolint:wrapcheck
				fmt.Errorf("search subscriptions: encode pagination failed: %w", err),
				http.StatusInternalServerError,
			)
		}
		return subscriptions, *req.pagination, nil
	}

	return subscriptions, "", nil
}

// addSubscriptionDynamicInfo adds dynamically generated information (e.g., unread count, stats, etc.) to subscriptions.
// At the least, all subscriptions will have an unread count and last updated info generated. Other stats will also be
// generated if the user has set the display option ShowSubscriptionStats in their account settings.
//
//nolint:gocognit,funlen
func (a *API) addSubscriptionDynamicInfo(ctx context.Context, subscriptions models.Subscriptions) error {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return fmt.Errorf("add subscription dynamic info: get user data: %w", models.ErrNoUserCtx)
	}

	var extraIDs []models.SubscriptionID
	for subscription := range slices.Values(subscriptions) {
		// Get any additional subscription info for subscriptions in group subscriptions that we didn't already fetch.
		if subscription.GetSubscriptionType() == models.SubscriptionTypeGroup {
			for id := range slices.Values(subscription.GroupData.Subscriptions) {
				if !slices.ContainsFunc(subscriptions, func(e *models.Subscription) bool {
					return e.GetID() == id
				}) {
					extraIDs = append(extraIDs, id)
				}
			}
		}
		// Add the show stats setting from the user settings.
		subscription.Settings.ShowSubscriptionStats = user.GetSettings().ShowSubscriptionStats
	}
	if len(extraIDs) > 0 {
		extraSubscriptions, err := a.GetSubscriptions(ctx,
			GetSubscriptionsByIDs(extraIDs...),
		)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return fmt.Errorf("add subscription dynamic info: get additional subscriptions: %w", err)
		}
		subscriptions = append(subscriptions, extraSubscriptions...)
	}

	fetchJobs, ctx := errgroup.WithContext(ctx)

	// Get unread count per feed.
	var unreadCounts map[models.FeedID]int64
	fetchJobs.Go(func() error {
		var err error
		unreadCounts, err = a.getFeedUnreadCounts(ctx, subscriptions)
		if err != nil {
			return fmt.Errorf("get unread counts: %w", err)
		}
		return nil
	})

	// For search subscriptions, run queries directly to add unread count and last update.
	fetchJobs.Go(func() error {
		for subscription := range slices.Values(subscriptions.FilterByType(models.SubscriptionTypeSearch)) {
			search := subscription.SearchData.Search
			// Build query to get unread count.
			query, err := a.BuildSearchResultsQuery(ctx, user, &search)
			if err != nil {
				return fmt.Errorf(
					"add subscription dynamic info: build search subscription %s query: %w",
					subscription.GetID(),
					err,
				)
			}
			count, err := a.CountItems(ctx, query)
			if err == nil {
				subscription.Stats.UnreadCount = int(count)
			} else {
				slogctx.FromCtx(ctx).Warn("Add subscription dynamic info, could not get unread count for search subscription.",
					slog.String("subscription_id", subscription.GetID()),
					slog.Any("error", err),
				)
			}
			// Update query for getting last updated item (view: all, sort: newest first).
			search.View = models.ViewAll
			sort := models.SortNewestFirst
			query, err = a.BuildSearchResultsQuery(ctx, user, &search)
			if err != nil {
				return fmt.Errorf(
					"add subscription dynamic info: build search subscription %s query: %w",
					subscription.GetID(),
					err,
				)
			}
			if items, _, err := a.SearchItems(ctx, query, 1, &sort, nil); err == nil && len(items) > 0 {
				subscription.Stats.LastUpdate = items[0].GetTimestamp()
			} else {
				slogctx.FromCtx(ctx).Warn("Add subscription dynamic info, could not get last update for search subscription.",
					slog.String("subscription_id", subscription.GetID()),
					slog.Any("error", err),
				)
			}
		}
		return nil
	})

	// Get last update (latest item timestamp) per feed.
	var lastUpdate map[models.FeedID]time.Time
	fetchJobs.Go(func() error {
		var err error
		lastUpdate, err = a.getFeedLastUpdates(ctx, subscriptions.GetFeedIDs()...)
		if err != nil {
			return fmt.Errorf("get last update: %w", err)
		}
		return nil
	})

	var avgDailyUpdates map[models.FeedID]float64
	if user.GetSettings().ShowSubscriptionStats {
		// Get average daily updates per feed
		fetchJobs.Go(func() error {
			var err error
			avgDailyUpdates, err = a.getFeedAverageDailyUpdates(ctx, subscriptions.GetFeedIDs()...)
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
	for subscription := range slices.Values(subscriptions.FilterByType(models.SubscriptionTypeFeed)) {
		// Add stats for feed subscriptions.
		subscription.Stats.UnreadCount = int(unreadCounts[subscription.FeedData.FeedID])
		subscription.Stats.LastUpdate = lastUpdate[subscription.FeedData.FeedID]
		if user.GetSettings().ShowSubscriptionStats {
			subscription.Stats.AvgDailyUpdates = avgDailyUpdates[subscription.FeedData.FeedID]
		}
	}

	// For group subscriptions, calculate stats from other subscriptions.
	for subscription := range slices.Values(subscriptions.FilterByType(models.SubscriptionTypeGroup)) {
		var avgDailyUpdates []float64
		var unreadCount int
		var lastUpdates []time.Time
		for groupSubscription := range slices.Values(subscriptions) {
			if slices.Contains(subscription.GroupData.Subscriptions, groupSubscription.GetID()) {
				if user.GetSettings().ShowSubscriptionStats {
					avgDailyUpdates = append(avgDailyUpdates, groupSubscription.Stats.AvgDailyUpdates)
				}
				unreadCount += groupSubscription.Stats.UnreadCount
				lastUpdates = append(lastUpdates, groupSubscription.Stats.LastUpdate)
			}
		}
		if user.GetSettings().ShowSubscriptionStats {
			slices.Sort(avgDailyUpdates)
			slices.Reverse(avgDailyUpdates)
			subscription.Stats.AvgDailyUpdates = avgDailyUpdates[0]
		}
		subscription.Stats.UnreadCount = unreadCount
		// Sort by date ascending, with favorites before non-favorites.
		slices.SortFunc(lastUpdates, func(timeA, timeB time.Time) int {
			return timeA.Compare(timeB)
		})
		slices.Reverse(lastUpdates)
		subscription.Stats.LastUpdate = lastUpdates[0]
	}

	return nil
}

// GetFeedSubscriptionStats fetches the stats for FeedSubscriptions and returns a map of the SubscriptionID to
// SubscriptionStats that can be used to lookup the stats pertaining to a particular subscription.
func (a *API) getFeedAverageDailyUpdates(ctx context.Context, ids ...models.FeedID) (map[models.FeedID]float64, error) {
	// Build query.
	query := query.Bool(
		query.BoolQueryName("feed_stats_query"),
		query.Filter(
			// Must match any of the given feed IDs.
			query.Terms("feed_id", ids...),
			// Must be published within last month.
			query.Since("@timestamp", time.Now().UTC().Add(-24*30*time.Hour)),
		),
	)
	// Build aggregations.
	termsField := "feed_id"
	termsCount := len(ids)
	dateHistoField := "@timestamp"
	dateFormat := "yyyy-MM-dd"
	aggs := aggregations.Aggs{
		"feed": types.Aggregations{
			Terms: &types.TermsAggregation{
				Field: &termsField,
				Size:  &termsCount,
			},
			Aggregations: map[string]types.Aggregations{
				"updates_per_day": {
					DateHistogram: &types.DateHistogramAggregation{
						Field:            &dateHistoField,
						CalendarInterval: &calendarinterval.Day,
						Format:           &dateFormat,
					},
				},
				"avg_daily_updates": {
					AvgBucket: &types.AverageBucketAggregation{
						BucketsPath: "updates_per_day._count",
					},
				},
			},
		},
	}

	results, err := a.ItemsAggregation(ctx, query, len(ids), aggs)
	if err != nil {
		return nil, fmt.Errorf("unable to get feed stats: Feed aggregation invalid: %w", models.ErrInvalidAPIResult)
	}
	feedStats, ok := results.Aggregations["feed"].(*types.StringTermsAggregate)
	if !ok {
		return nil, fmt.Errorf("unable to get feed stats: Feed aggregation invalid: %w", models.ErrInvalidAPIResult)
	}
	feedStatsBuckets, ok := feedStats.Buckets.([]types.StringTermsBucket)
	if !ok {
		return nil, fmt.Errorf("unable to get feed stats: Feed aggregation invalid: %w", models.ErrInvalidAPIResult)
	}

	stats := make(map[models.FeedID]float64)

	// Loop through the aggregation results and extract the daily updates metric for each feed.
	for feed := range slices.Values(feedStatsBuckets) {
		var feedID models.FeedID
		feedID, ok = feed.Key.(string)
		if !ok {
			slogctx.FromCtx(ctx).Debug("Unable to extract feed ID for aggregation", slog.Any("feed_id", feed.Key))
			continue
		}
		var updatesResult *types.SimpleValueAggregate
		updatesResult, ok = feed.Aggregations["avg_daily_updates"].(*types.SimpleValueAggregate)
		if !ok {
			slogctx.FromCtx(ctx).
				Debug("Unable to extract avg_daily_updates agg for subscription", slog.String("feed_id", feedID))
			continue
		}
		stats[feedID] = float64(*updatesResult.Value)
	}

	return stats, nil
}

func (a *API) getFeedUnreadCounts(
	ctx context.Context,
	subscriptions models.Subscriptions,
) (map[models.FeedID]int64, error) {
	// Retrieve user object.
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("unable to get subscription unread counts: %w", models.ErrNoUserCtx)
	}
	// Generate unread count query.
	subscriptionQueries := make([]query.Option, 0, len(subscriptions))
	for subscription := range slices.Values(subscriptions) {
		if subscription.GetSubscriptionType() != models.SubscriptionTypeFeed {
			continue
		}
		subscriptionQueries = append(subscriptionQueries, queryUnreadItems(user, subscription))
	}
	// Build query.
	query := query.Bool(
		query.Filter(
			query.Bool(
				query.Should(subscriptionQueries...),
			),
		),
	)
	// Build aggregations.
	termsField := "feed_id"
	termsCount := len(subscriptions)
	aggs := aggregations.Aggs{
		"UnreadCounts": types.Aggregations{
			Terms: &types.TermsAggregation{
				Field: &termsField,
				Size:  &termsCount,
			},
		},
	}
	// Perform aggregation.
	results, err := a.ItemsAggregation(ctx, query, 0, aggs)
	if err != nil {
		return nil, fmt.Errorf("unable to get subscription unread counts: %w", err)
	}

	unreadCounts, ok := results.Aggregations["UnreadCounts"].(*types.StringTermsAggregate)
	if !ok {
		return nil, fmt.Errorf(
			"unable to get feed stats: UnreadCounts aggregations invalid: %w",
			models.ErrInvalidAPIResult,
		)
	}
	unreadCountsBuckets, ok := unreadCounts.Buckets.([]types.StringTermsBucket)
	if !ok {
		return nil, fmt.Errorf(
			"unable to get feed stats: UnreadCounts aggregations invalid: %w",
			models.ErrInvalidAPIResult,
		)
	}

	stats := make(map[models.SubscriptionID]int64)

	// Loop through the aggregation results and extract the unread count for each feed.
	for feed := range slices.Values(unreadCountsBuckets) {
		var feedID models.FeedID
		if feedID, ok = feed.Key.(string); ok {
			stats[feedID] = feed.DocCount
		}
	}
	return stats, nil
}

func (a *API) getFeedLastUpdates(ctx context.Context, ids ...models.FeedID) (map[models.FeedID]time.Time, error) {
	index, err := ItemsReadIndexFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("get last updated items: %w", ErrNoIndexInCtx)
	}

	sort := models.SortNewestFirst

	items, _, err := Search[*models.Item](
		ctx,
		a.GetAPI(),
		index,
		query.Terms("feed_id", ids...),
		len(ids),
		WithCollapseField("feed_id"),
		WithSortOptions[*search.Search, SearchRequest](newItemSortOptions(&sort)...),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to get feed last updates: %w", err)
	}

	updates := make(map[models.FeedID]time.Time)

	for item := range slices.Values(items) {
		updates[item.GetFeedID()] = item.GetTimestamp()
	}

	return updates, nil
}

// MarkSubscriptions will mark as appropriate all the given subscriptions. Marking a subscription includes updating the
// subscription data in the user object and clearing any individual item states for a subscription.
func (a *API) MarkSubscriptions(
	ctx context.Context,
	mark models.Mark,
	subscriptionIDs ...models.SubscriptionID,
) error {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return fmt.Errorf("mark subscriptions: get user data: %w", models.ErrNoUserCtx)
	}

	subscriptions, err := a.GetSubscriptions(ctx,
		GetSubscriptionsByIDs(subscriptionIDs...),
	)
	if err != nil {
		return fmt.Errorf("mark subscriptions: %w", err)
	}

	for subscription := range slices.Values(subscriptions) {
		if subscription.GetSubscriptionType() == models.SubscriptionTypeGroup {
			if err = a.MarkSubscriptions(ctx, mark, subscription.GroupData.Subscriptions...); err != nil {
				return fmt.Errorf("mark subscriptions: mark group subscription: %w", err)
			}
		} else {
			subscription.Mark(user, mark)
			if _, err = a.UpdateSubscriptions(ctx, subscriptions...); err != nil {
				return fmt.Errorf("mark subscriptions: update subscription data: %w", err)
			}
		}
	}

	return nil
}

// getAllSubscriptionsByQuery returns all subscriptions that match the given query.
func (a *API) getAllSubscriptionsByQuery(ctx context.Context, query query.Option) (models.Subscriptions, error) {
	index, err := SubscriptionsReadIndexFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all subscriptions by query: %w", ErrNoIndexInCtx)
	}

	var subscriptions models.Subscriptions
	subscriptions, err = SearchAll[*models.Subscription](ctx, a.GetAPI(), index, query, defaultPaginationSize)
	if err != nil {
		return nil, fmt.Errorf("get all subscriptions by query: %w", err)
	}
	return subscriptions, nil
}

// BuildSubscriptionQueries generates a slices of queries for the given subscriptions, based on the given filters.
func (a *API) BuildSubscriptionQueries(
	user *models.User,
	view models.View,
	subscriptions models.Subscriptions,
) []query.Option {
	queries := make([]query.Option, 0, len(subscriptions))
	// Work out what query to use based on the state filter.
	if len(subscriptions) == 0 {
		return nil
	}
	for subscription := range slices.Values(subscriptions) {
		if subscription.GetSubscriptionType() != models.SubscriptionTypeFeed {
			continue
		}
		switch view {
		case models.ViewRead:
			queries = append(queries, queryReadItems(user, subscription))
		case models.ViewAll:
			queries = append(queries, queryAllItems(user, subscription))
		case models.ViewUnread:
			fallthrough
		default:
			queries = append(queries, queryUnreadItems(user, subscription))
		}
	}
	return queries
}

// queryReadItems generates a query for finding read items for the given subscription.
func queryReadItems(user *models.User, subscription *models.Subscription) query.Option {
	if subscription.GetSubscriptionType() != models.SubscriptionTypeFeed {
		return nil
	}
	return query.Bool(
		query.BoolQueryName(subscription.FeedData.FeedID+"_read_items"),
		query.Filter(
			// Must match this feed.
			query.Term("feed_id", subscription.FeedData.FeedID),
			// And should be between the user max history and last read time.
			query.Bool(
				query.Should(
					query.Between("published", user.GetMaxHistory(), subscription.MarkedReadAt),
					query.Between("updated", user.GetMaxHistory(), subscription.MarkedReadAt),
					query.Terms("item_id", subscription.FeedData.GetReadItems()...),
				),
				// Must not match any unread items for the feed
				query.MustNot(
					query.Terms("item_id", subscription.FeedData.GetUnreadItems()...),
				),
			),
		),
		// User-specified field-level filtering.
		query.Must(
			query.SimpleQueryString(subscription.FeedData.ArticleFilters.Text, "", "title", "description", "content"),
			query.SimpleQueryString(subscription.FeedData.ArticleFilters.Authors, "", "authors", "contributors"),
			query.SimpleQueryString(subscription.FeedData.ArticleFilters.Categories, "", "categories"),
		),
	)
}

// QueryUnreadItems generates a query for finding unread items for the given subscription.
func queryUnreadItems(user *models.User, subscription *models.Subscription) query.Option {
	if subscription.GetSubscriptionType() != models.SubscriptionTypeFeed {
		return nil
	}
	return query.Bool(
		query.BoolQueryName(subscription.FeedData.GetFeedID()+"_unread_items"),
		query.Filter(
			// Must match this feed.
			query.Term("feed_id", subscription.FeedData.GetFeedID()),
			query.Bool(
				query.Should(
					query.Since("published", subscription.MarkedReadAt),
					query.Since("updated", subscription.MarkedReadAt),
					query.Terms("item_id", subscription.FeedData.GetUnreadItems()...),
				),
				// Must not match any read items for the feed
				query.MustNot(
					query.Terms("item_id", subscription.FeedData.GetReadItems()...),
				),
			),
		),
		// User-specified field-level filtering.
		query.Must(
			query.SimpleQueryString(subscription.FeedData.ArticleFilters.Text, "", "title", "description", "content"),
			query.SimpleQueryString(subscription.FeedData.ArticleFilters.Authors, "", "authors", "contributors"),
			query.SimpleQueryString(subscription.FeedData.ArticleFilters.Categories, "", "categories"),
		),
	)
}

// subscriptionQueryReadItems generates a query for finding all items for the given subscription.
func queryAllItems(user *models.User, subscription *models.Subscription) query.Option {
	if subscription.GetSubscriptionType() != models.SubscriptionTypeFeed {
		return nil
	}
	maxHistory := user.GetMaxHistory()
	return query.Bool(
		query.BoolQueryName(subscription.FeedData.GetFeedID()+"_all_items"),
		query.Filter(
			// Must match this feed.
			query.Term("feed_id", subscription.FeedData.GetFeedID()),
			// And be published/updated since the user max history.
			query.Bool(
				query.Should(
					query.Since("published", maxHistory),
					query.Since("updated", maxHistory),
				),
			),
		),
		// User-specified field-level filtering.
		query.Must(
			query.SimpleQueryString(subscription.FeedData.ArticleFilters.Text, "", "title", "description", "content"),
			query.SimpleQueryString(subscription.FeedData.ArticleFilters.Authors, "", "authors", "contributors"),
			query.SimpleQueryString(subscription.FeedData.ArticleFilters.Categories, "", "categories"),
		),
	)
}

// GetArticles generates Article objects from the Items with the given IDs.
func (a *API) GetArticles(ctx context.Context, itemIDs ...models.ItemID) (models.Articles, error) {
	// Search through items matching any given feeds filters, excluding any read
	// items.
	query := query.Bool(
		query.Filter(
			// Must match any of the given item IDs,
			query.Terms("item_id", itemIDs...),
		),
	)
	items, _, err := a.SearchItems(ctx, query, len(itemIDs), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("get articles failed: %w", err)
	}
	articles, err := a.GenerateArticles(ctx, items)
	if err != nil {
		return nil, fmt.Errorf("get articles failed: %w", err)
	}

	return articles, nil
}

// FilterArticles returns Articles filtered by the given filters and paginated by the given pagination.
func (a *API) FilterArticles(
	ctx context.Context,
	filters *models.ListDisplayFilters,
	pagination models.Pagination,
) (models.Articles, models.Pagination, error) {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, "", fmt.Errorf("filter articles: get user data: %w", models.ErrNoUserCtx)
	}

	subscriptions, err := a.GetSubscriptions(ctx,
		GetSubscriptionsByIDs(filters.GetSubscriptions()...),
	)
	if err != nil {
		return nil, "", fmt.Errorf("filter articles: get subscriptions: %w", err)
	}
	// Return early if there the user has no subscriptions (i.e., new user).
	if len(subscriptions) == 0 {
		return nil, "", fmt.Errorf("filter articles: %w", ErrNotFound)
	}
	// Search through items matching any given feeds filters, excluding any read
	// items.
	articleQuery := query.Bool(
		query.Filter(
			// Must match any of the given categories.
			query.Terms("categories.raw", filters.GetCategories()...),
			query.Bool(
				query.Should(a.BuildSubscriptionQueries(user, filters.GetView(), subscriptions)...),
			),
		),
	)

	sort := filters.GetSort()

	// Find items matching filters.
	items, pagination, err := a.SearchItems(ctx, articleQuery, filters.GetCount(), &sort, &pagination)
	if err != nil {
		return nil, "", fmt.Errorf("could not retrieve filtered items: %w", err)
	}
	// Generate articles.
	articles, err := a.GenerateArticles(ctx, items)
	if err != nil {
		return nil, "", fmt.Errorf("could not generate articles from items: %w", err)
	}

	return articles, pagination, nil
}

// FindSimilarArticles performs a "more like this" search to find other Articles that are similar to the Items with the
// given IDs.
func (a *API) FindSimilarArticles(ctx context.Context, itemIDs ...models.ItemID) (models.Articles, error) {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("find similar articles: get user data: %w", models.ErrNoUserCtx)
	}
	subscriptions, err := a.GetSubscriptions(ctx)
	switch {
	case err != nil:
		return nil, fmt.Errorf("find similar articles: get subscriptions: %w", err)
	case len(subscriptions) == 0:
		return nil, fmt.Errorf("find similar articles: get subscriptions: %w", ErrNotFound)
	}
	// Build the More Like This query.
	// TODO: tweak values and fields for optimum results matching...
	var (
		minTermFreq   = 1
		maxQueryTerms = 12
	)
	mlt := query.NewMoreLikeThisQuery("similar_articles")
	mlt.LikeDocs(itemIDs...)
	mlt.Fields = []string{"title", "categories.raw", "author"}
	mlt.MinTermFreq = &minTermFreq
	mlt.MaxQueryTerms = &maxQueryTerms
	// Build query
	similarQuery := query.Bool(
		query.Filter(
			query.Bool(
				query.Should(a.BuildSubscriptionQueries(user, models.ViewUnread, subscriptions)...),
			),
		),
		query.Must(
			mlt.ToQueryOption(),
		),
	)
	// Query for similar articles.
	sort := models.SortMostRelevant
	items, _, err := a.SearchItems(ctx, similarQuery, 15, &sort, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to find similar articles: %w", err)
	}
	// Generate article data.
	articles, err := a.GenerateArticles(ctx, items)
	if err != nil {
		return nil, fmt.Errorf("unable to find similar articles: %w", err)
	}
	return articles, nil
}

// GenerateArticles takes a slice of items and creates articles from them, grabbing the necessary data from the user
// object.
func (a *API) GenerateArticles(ctx context.Context, items models.Items) (models.Articles, error) {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("generate articles: get user data: %w", models.ErrNoUserCtx)
	}
	subscriptions, _, err := a.searchSubscriptions(ctx,
		query.Bool(
			query.Filter(
				query.Term("user_id", user.GetID()),
				query.Terms("type", models.SubscriptionTypeFeed),
				query.Terms("feed_data.feed_id", items.GetFeedIDs()...),
			),
		),
		searchSubscriptionsMaxResults(len(items.GetFeedIDs())),
	)
	switch {
	case err != nil:
		return nil, fmt.Errorf("generate articles: get subscriptions: %w", err)
	case len(subscriptions) == 0:
		return nil, fmt.Errorf("generate articles: get subscriptions: %w", ErrNotFound)
	}
	// Create articles from the items.
	articles := make(models.Articles, 0, len(items))
	for item := range slices.Values(items) {
		var article *models.Article
		article, err = models.GenerateArticle(user, item, subscriptions.GetByFeedID(item.GetFeedID()))
		if err != nil {
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

// UpdateFavoriteArticle changes the favorite status of an article. For adding a favorite article, the content is stored
// in a separate and the user object is updated with a link to the content. For removing a favorite, the stored content
// is removed and user object updated appropriately.
func (a *API) UpdateFavoriteArticle(ctx context.Context, user *models.User, id models.ItemID, favorite bool) error {
	switch favorite {
	case true:
		// Don't do anything if article is already a favorite.
		if slices.Contains(user.ItemFavorites, id) {
			return models.ErrUserAlreadyFavorited
		}
		// Get the article details.
		articles, err := a.GetArticles(ctx, id)
		if err != nil {
			return fmt.Errorf("unable to add favorite article: %w", err)
		}
		if len(articles) != 1 {
			return models.ErrInvalidAPIResult
		}
		article := articles[0]
		// Archive the article.
		archive, err := models.NewArchivedArticle(user.GetID(), article.GetSubscriptionID(), &article.Item)
		if err != nil {
			return fmt.Errorf("unable to add favorite article: %w", err)
		}
		err = a.ArchiveArticle(ctx, archive)
		if err != nil {
			return fmt.Errorf("unable to add favorite article: %w", err)
		}
		// Update the list of favorites items in the user object
		user.ItemFavorites = append(user.ItemFavorites, id)
		err = a.UpdateUser(ctx, user.GetID(), map[string]any{
			"item_favorites": user.ItemFavorites,
		})
		if err != nil {
			return fmt.Errorf("unable to add favorite article: %w", err)
		}
	case false:
		err := a.UnarchiveArticle(ctx, user.GetID(), id)
		if err != nil {
			return fmt.Errorf("unable to remove favorite article: %w", err)
		}
		newFavorites := slices.DeleteFunc(user.ItemFavorites, func(e models.ItemID) bool {
			return e == id
		})
		err = a.UpdateUser(ctx, user.GetID(), map[string]any{
			"item_favorites": newFavorites,
		})
		if err != nil {
			return fmt.Errorf("unable to remove favorite article: %w", err)
		}
	}
	return nil
}

// BuildItemsQuery generates a query to fetch the Items that match the given Filters from the given Subscriptions.
func (a *API) BuildItemsQuery(
	ctx context.Context,
	filters models.Filters,
	subscriptionIDs ...models.SubscriptionID,
) (query.Option, error) {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("unable to build items query: %w", models.ErrNoUserCtx)
	}

	subscriptions, err := a.GetSubscriptions(ctx,
		GetSubscriptionsByIDs(subscriptionIDs...),
	)
	switch {
	case err != nil:
		return nil, fmt.Errorf("get suggestions: get subscriptions: %w", err)
	case len(subscriptions) == 0:
		return nil, fmt.Errorf("get suggestions: get subscriptions: %w", ErrNotFound)
	}

	// Search through items matching any given feeds filters, excluding any read
	// items.
	return query.Bool(
		query.BoolQueryName("get_items"),
		query.Filter(
			// Must match any of the given feed IDs.
			query.Terms("feed_id", subscriptions.GetFeedIDs()...),
			// Must match any of the given categories.
			query.Terms("categories.raw", filters.GetCategories()...),
			// And should match one feed clause.
			query.Bool(
				query.Should(a.BuildSubscriptionQueries(user, filters.GetView(), subscriptions)...),
			),
		),
	), nil
}

// BuildSearchResultsQuery generates a query that can be used to fetch appropriate results for a given SearchRequest
// criteria.
func (a *API) BuildSearchResultsQuery(
	ctx context.Context,
	user *models.User,
	request *models.SearchRequest,
) (query.Option, error) {
	// var err error
	var loc *time.Location
	var err error
	if request.Timezone != "" {
		loc, err = time.LoadLocation(request.Timezone)
		if err != nil {
			return nil, fmt.Errorf("build search query: load timezone: %w", err)
		}
	} else {
		loc, err = time.LoadLocation("UTC")
		if err != nil {
			return nil, fmt.Errorf("build search query: load timezone: %w", err)
		}
	}
	var since time.Time
	var pivot string
	switch request.PublishedWithin {
	case models.SearchRequestPublishedWithinLastHour:
		since, _ = time.ParseInLocation(time.Layout, time.Now().Add(-time.Hour).Format(time.Layout), loc)
		pivot = "30m"
	case models.SearchRequestPublishedWithinLast12hours:
		since, _ = time.ParseInLocation(time.Layout, time.Now().Add(-12*time.Hour).Format(time.Layout), loc)
		pivot = "6h"
	case models.SearchRequestPublishedWithinLastDay:
		since, _ = time.ParseInLocation(time.Layout, time.Now().Add(-24*time.Hour).Format(time.Layout), loc)
		pivot = "12h"
	case models.SearchRequestPublishedWithinLastWeek:
		since, _ = time.ParseInLocation(time.Layout, time.Now().Add(-7*24*time.Hour).Format(time.Layout), loc)
		pivot = "3d"
	case models.SearchRequestPublishedWithinLastMonth:
		since, _ = time.ParseInLocation(time.Layout, time.Now().Add(-30*24*time.Hour).Format(time.Layout), loc)
		pivot = "14d"
	default: // default to one week.
		since, _ = time.ParseInLocation(time.Layout, time.Now().Add(-7*24*time.Hour).Format(time.Layout), loc)
		pivot = "3d"
	}

	subscriptions, err := a.GetSubscriptions(ctx,
		GetSubscriptionsByIDs(request.Subscriptions...),
	)
	switch {
	case err != nil:
		return nil, fmt.Errorf("build search query: get subscriptions: %w", err)
	case len(subscriptions) == 0:
		return nil, fmt.Errorf("build search query: get subscriptions: %w", ErrNotFound)
	}

	return query.Bool(
		query.Filter(
			// Must be in the given user subscriptions.
			query.Bool(
				query.Should(a.BuildSubscriptionQueries(user, request.View, subscriptions)...),
			),
			// Must be published/updated since the given time.
			query.Bool(
				query.Should(
					query.Since("published", since),
					query.Since("updated", since),
				),
			),
		),
		// Boost documents closer to the current time.
		query.Should(
			query.Distance("published", pivot, "now"),
			query.Distance("updated", pivot, "now"),
		),
		// Must match either: search term in any of the fields, or, matches directly as a search-as-you-type (same as
		// search suggestion).
		query.Must(
			// Search across title, description and content fields, with preference for match in that order (via field
			// boosting).
			query.SimpleQueryString(request.Text, "", "title^6", "description^3", "content"),
			// Search in categories.
			query.SimpleQueryString(request.Categories, "", "categories"),
			// Search in authors, contributors.
			query.SimpleQueryString(request.Authors, "", "authors", "contributors"),
		),
	), nil
}

// func (e *API) MultiSearchFeeds(ctx context.Context, queries ...*models.MultiSearchQuery) (results.MSearchResults, error) {
// 	return MultiSearch(ctx, e.GetAPI(), queries...)
// }

// SearchItems will search the items index for items matching the given query. Count, sort and pagination values are
// optional.
func (a *API) SearchItems(
	ctx context.Context,
	query query.Option,
	count int,
	sort *models.Sort,
	pagination *models.Pagination,
) (models.Items, models.Pagination, error) {
	index, err := ItemsReadIndexFromCtx(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("search items: %w", ErrNoIndexInCtx)
	}

	searchAfter, err := decodePagination(pagination)
	if err != nil {
		return nil, "", models.NewAPIError(
			fmt.Errorf("search items: decode pagination failed: %w", err),
			http.StatusInternalServerError,
		) //nolint:wrapcheck
	}
	// Perform search.
	items, newSearchAfter, err := Search[*models.Item](ctx, a.GetAPI(), index, query, count,
		WithSortOptions[*search.Search, SearchRequest](newItemSortOptions(sort)...),
		WithSearchAfter[*search.Search, SearchRequest](searchAfter...),
	)
	if err != nil {
		return nil, "", fmt.Errorf("search items: %w", err)
	}
	// Parse last search after value into pagination.
	newPagination, err := encodePagination(newSearchAfter)
	if err != nil {
		return nil, "", models.NewAPIError(
			fmt.Errorf("search items: encode pagination failed: %w", err),
			http.StatusInternalServerError,
		) //nolint:wrapcheck
	}
	return items, newPagination, nil
}

// ItemsAggregation performs an aggregation-only (i.e., search request with no hits returned) using the given query as
// the set of documents and performing the given aggregations across the documents.
func (a *API) ItemsAggregation(
	ctx context.Context,
	query query.Option,
	size int,
	aggregations aggregations.Aggs,
) (*search.Response, error) {
	index, err := ItemsReadIndexFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("items aggregation: %w", ErrNoIndexInCtx)
	}

	req := NewSearchRequest(a.GetAPI(),
		WithRequestID[*search.Search, SearchRequest](middleware.GetReqID(ctx)),
		WithIndex[*search.Search, SearchRequest](index),
		WithQueryOptions[*search.Search, SearchRequest](query),
		WithSize[*search.Search, SearchRequest](size),
		WithSortOptions[*search.Search, SearchRequest](&types.SortOptions{Doc_: types.NewScoreSort()}),
		WithAggregations[*search.Search, SearchRequest](aggregations),
	)
	resp, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("items aggregation: %w", err)
	}

	return resp, nil
}

// CountItems returns a count of items that match the given query.
func (a *API) CountItems(ctx context.Context, query query.Option) (int64, error) {
	index, err := ItemsReadIndexFromCtx(ctx)
	if err != nil {
		return 0, ErrNoIndexInCtx //nolint:wrapcheck
	}

	count, err := Count(ctx, a.GetAPI(), index, query)
	if err != nil {
		return 0, fmt.Errorf("count items: %w", err)
	}

	return count, nil
}

// AddItems will bulk index the given items.
func (a *API) AddItems(ctx context.Context, items ...*models.Item) (map[models.ItemID]*bulk.OperationResponse, error) {
	index, err := ItemsWriteIndexFromCtx(ctx)
	if err != nil {
		return nil, ErrNoIndexInCtx //nolint:wrapcheck
	}
	return BulkUpdate(ctx, a, index, items...)
}

// ArchiveArticle will index the given article content to the article archive for permanent storage.
func (a *API) ArchiveArticle(ctx context.Context, article *models.ArticleArchive) error {
	index, err := FavoriteItemsWriteIndexFromCtx(ctx)
	if err != nil {
		return ErrNoIndexInCtx //nolint:wrapcheck
	}
	err = CreateDoc(ctx, a.GetAPI(), index, article.ItemID, article)
	if err != nil {
		return fmt.Errorf("archive article: %w", err)
	}
	return nil
}

// UnarchiveArticle will delete an article from the archive.
func (a *API) UnarchiveArticle(ctx context.Context, userID models.UserID, itemID models.ItemID) error {
	index, err := FavoriteItemsWriteIndexFromCtx(ctx)
	if err != nil {
		return ErrNoIndexInCtx //nolint:wrapcheck
	}
	// Set up the query to match the user's favorited article.
	query := query.Bool(
		query.Filter(
			query.Term("user_id", userID),
			query.Term("item_id", itemID),
		),
	)
	err = DeleteDocs(ctx, a.GetAPI(), index, query)
	if err != nil {
		return fmt.Errorf("unarchive article: %w", err)
	}
	return nil
}

// BulkAdd will create documents for the given list of objects. Responses are returned as a map of doc id to response.
// If the request itself fails, a non-nil error is returned.
func BulkAdd[T ~string, O Object[T]](
	ctx context.Context,
	api *API,
	index string,
	objects ...O,
) (map[T]*bulk.OperationResponse, error) {
	bulkOps, respCh := bulk.NewRequest(ctx, api)

	go func() {
		defer close(bulkOps)

		for object := range slices.Values(objects) {
			bulkOps <- bulk.NewOperation(object,
				bulk.SetDocID(string(object.GetID())),
				bulk.ToIndex(index),
			)
		}
	}()

	bulkOpResponse := <-respCh
	// If the request failed, return an error.
	if bulkOpResponse.Err != nil {
		return nil, toAPIError("bulk add", bulkOpResponse.Err)
	}
	// Create a map of responses by object id.
	responses := make(map[T]*bulk.OperationResponse)
	// Map responses to object id.
	for opResp := range slices.Values(bulkOpResponse.Responses) {
		if opResp.Id_ == nil {
			continue
		}
		if idx := slices.IndexFunc(objects, func(o O) bool {
			return string(o.GetID()) == *opResp.Id_
		}); idx != -1 {
			responses[objects[idx].GetID()] = opResp
		}
	}

	return responses, nil
}

// BulkUpdate will update documents for the given list of objects. Responses are returned as a map of doc id to response.
// If the request itself fails, a non-nil error is returned.
func BulkUpdate[T ~string, O Object[T]](
	ctx context.Context,
	api *API,
	index string,
	objects ...O,
) (map[T]*bulk.OperationResponse, error) {
	bulkOps, respCh := bulk.NewRequest(ctx, api)

	go func() {
		defer close(bulkOps)

		for object := range slices.Values(objects) {
			bulkOps <- bulk.NewOperation(object,
				bulk.AsOperationType(bulk.BulkUpdate),
				bulk.SetDocID(string(object.GetID())),
				bulk.ToIndex(index),
				bulk.Upsert(true),
			)
		}
	}()

	bulkOpResponse := <-respCh
	// If the request failed, return an error.
	if bulkOpResponse.Err != nil {
		return nil, toAPIError("bulk update", bulkOpResponse.Err)
	}
	// Create  a map of responses by object id.
	responses := make(map[T]*bulk.OperationResponse)
	// Map responses to object id.
	for opResp := range slices.Values(bulkOpResponse.Responses) {
		if opResp.Id_ == nil {
			continue
		}
		if idx := slices.IndexFunc(objects, func(o O) bool {
			return string(o.GetID()) == *opResp.Id_
		}); idx != -1 {
			responses[objects[idx].GetID()] = opResp
		}
	}

	return responses, nil
}

// exists checks if the document with the given id exists in the given index.
func exists[T ~string](ctx context.Context, api *elasticsearch.TypedClient, index string, id T) (bool, error) {
	found, err := api.Exists(index, string(id)).
		Header(ReqIDHeader, middleware.GetReqID(ctx)).
		Do(ctx)
	if err != nil {
		return false, toAPIError("exists", err)
	}
	return found, nil
}

// Count will return the number of docs matching the given queries in the given index.
func Count(ctx context.Context, api *elasticsearch.TypedClient, index string, queries ...query.Option) (int64, error) {
	resp, err := NewCountRequest(api,
		WithRequestID[*count.Count, CountRequest](middleware.GetReqID(ctx)),
		WithIndex[*count.Count, CountRequest](index),
		WithQueryOptions[*count.Count, CountRequest](queries...),
	).Do(ctx)
	if err != nil {
		return 0, toAPIError("count", err)
	}

	return resp.Count, nil
}

// GetDocs performs an `_mget` request to fetch the documents from the given index with the given ids. A non-nil error
// is returned on a failure.
func GetDocs[T ~string, O any](
	ctx context.Context,
	api *elasticsearch.TypedClient,
	index string,
	ids ...T,
) ([]O, error) {
	docIDs := make([]string, 0, len(ids))
	for id := range slices.Values(ids) {
		docIDs = append(docIDs, string(id))
	}
	resp, err := NewMGetRequest(api,
		WithRequestID[*mget.Mget, MgetRequest](middleware.GetReqID(ctx)),
		WithIndex[*mget.Mget, MgetRequest](index),
		WithIDs[*mget.Mget, MgetRequest](docIDs...),
	).Do(ctx)
	if err != nil {
		return nil, toAPIError("get docs", err)
	}
	objects, warnings := results.ExtractSourceFromDocs[O](resp.Docs)
	if warnings != nil {
		slogctx.FromCtx(ctx).WarnContext(ctx, "Some docs could not be extracted.",
			slog.Any("warnings", warnings))
	}
	return objects, nil
}

// GetDoc retrieves the doc with the given id from the given index. A non-nil error is returned on a failure.
func GetDoc[T ~string, O any](ctx context.Context, api *elasticsearch.TypedClient, index string, id T) (O, error) {
	var doc O
	resp, err := NewGetRequest(api, index, string(id),
		WithRequestID[*get.Get, RequestCommon[*get.Get]](middleware.GetReqID(ctx)),
	).Do(ctx)
	if err != nil {
		return doc, toAPIError("get doc", err)
	}
	if !resp.Found {
		return doc, ErrNotFound //nolint:wrapcheck
	}
	doc, err = results.ExtractSource[O](resp.Source_)
	if err != nil {
		return doc, toAPIError("get doc: extract doc failed", err)
	}
	return doc, nil
}

// CreateDoc will create the given document, with given id, in the given index.
func CreateDoc[T ~string, O any](ctx context.Context, api *elasticsearch.TypedClient, index string, id T, doc O) error {
	resp, err := api.Create(index, string(id)).
		Document(doc).
		Header(ReqIDHeader, middleware.GetReqID(ctx)).
		Refresh(refresh.True).
		Do(ctx)
	if err != nil {
		return toAPIError("create doc", err)
	}
	if resp != nil {
		slogctx.FromCtx(ctx).Log(ctx, logging.LevelTrace, "Created document.",
			slog.String("id", resp.Id_),
			slog.String("result", resp.Result.String()),
		)
	}
	return nil
}

// UpdateDoc performs a partial doc update on the document with the given id in the given index. A non-nil error is
// returned on a failure.
func UpdateDoc[T ~string](
	ctx context.Context,
	api *elasticsearch.TypedClient,
	index string,
	id T,
	updates map[string]any,
	options ...Option[UpdateDocRequest],
) error {
	resp, err := NewUpdateDocRequest(api, index, string(id), updates, options...).Do(ctx)
	if err != nil {
		return toAPIError("update doc", err)
	}
	if resp != nil {
		slogctx.FromCtx(ctx).Log(ctx, logging.LevelTrace, "Updated document.",
			slog.String("id", resp.Id_),
			slog.String("result", resp.Result.String()),
		)
	}

	return nil
}

// DeleteDoc deletes the document with the given id from the given index.
func DeleteDoc[T ~string](ctx context.Context, api *elasticsearch.TypedClient, index string, id T) error {
	resp, err := api.Delete(index, string(id)).
		Header(ReqIDHeader, middleware.GetReqID(ctx)).
		Refresh(refresh.True).
		Do(ctx)
	if err != nil {
		return toAPIError("delete doc", err)
	}
	if resp != nil {
		slogctx.FromCtx(ctx).Log(ctx, logging.LevelTrace, "Deleted document.",
			slog.String("id", resp.Id_),
			slog.String("result", resp.Result.String()),
		)
	}
	return nil
}

// DeleteDocs performs a delete by query request on the given index to delete documents matching the given queries.
func DeleteDocs(ctx context.Context, api *elasticsearch.TypedClient, index string, queries ...query.Option) error {
	resp, err := NewDeleteByQueryRequest(
		api,
		index,
		WithRequestID[*deletebyquery.DeleteByQuery, RequestCommon[*deletebyquery.DeleteByQuery]](
			middleware.GetReqID(ctx),
		),
		WithQueryOptions[*deletebyquery.DeleteByQuery, RequestWithQuery[*deletebyquery.DeleteByQuery]](queries...),
	).Do(ctx)
	if err != nil {
		return toAPIError("delete docs", err)
	}
	if resp != nil {
		slogctx.FromCtx(ctx).Log(ctx, logging.LevelTrace, "Delete documents.",
			slog.Int64("count", *resp.Deleted),
		)
	}
	return nil
}

// Search performs a _search request to find documents matching the given query.
func Search[O any](
	ctx context.Context,
	api *elasticsearch.TypedClient,
	index string,
	query query.Option,
	count int,
	options ...Option[SearchRequest],
) ([]O, []types.FieldValue, error) {
	defaultOptions := []Option[SearchRequest]{
		WithRequestID[*search.Search, SearchRequest](middleware.GetReqID(ctx)),
		WithIndex[*search.Search, SearchRequest](index),
		WithQueryOptions[*search.Search, SearchRequest](query),
		WithSize[*search.Search, SearchRequest](count),
	}
	defaultOptions = append(defaultOptions, options...)
	req := NewSearchRequest(api, defaultOptions...)
	resp, err := req.Do(ctx)
	if err != nil {
		return nil, nil, toAPIError("search", err)
	}
	var warnings error
	var docs []O
	var newSearchAfter []types.FieldValue

	docs, newSearchAfter, warnings = results.ExtractSourceFromHits[O](resp.Hits.Hits)
	if warnings != nil {
		slogctx.FromCtx(ctx).WarnContext(ctx, "Some docs could not be extracted.",
			slog.Any("warnings", warnings))
	}

	return docs, newSearchAfter, nil
}

// SearchAll performs a paginated search request to retrieve *all* documents matching the given query. Unlike Search, it
// does not stop when the request hits count is reached.
func SearchAll[O any](
	ctx context.Context,
	api *elasticsearch.TypedClient,
	index string,
	query query.Option,
	paginationSize int,
	options ...Option[SearchRequest],
) ([]O, error) {
	if paginationSize == 0 {
		paginationSize = 1000
	}
	allResults := make([]O, 0)
	var searchAfter []types.FieldValueVariant

	// Loop until we've paginated through all results.
	var loops int
	for {
		resultsPage, nextSearchAfter, err := Search[O](ctx, api, index, query, paginationSize,
			WithSortOptions[*search.Search, SearchRequest](&types.SortOptions{Doc_: types.NewScoreSort()}),
			WithSearchAfter[*search.Search, SearchRequest](searchAfter...),
			WithTrackTotalHits(false),
		)
		if err != nil {
			return nil, toAPIError("search all", err)
		}
		pagination, err := encodePagination(nextSearchAfter)
		if err != nil {
			return nil, toAPIError("search all: encode pagination failed", err)
		}
		searchAfter, err = decodePagination(&pagination)
		if err != nil {
			return nil, toAPIError("search all: decode pagination failed", err)
		}

		allResults = append(allResults, resultsPage...)
		// Stop if the number of hits is less than the search size (i.e., last set of hits).
		if len(resultsPage) < paginationSize {
			break
		}
		loops++
	}
	slogctx.FromCtx(ctx).Log(ctx, logging.LevelTrace, "Paginated search finished.",
		slog.Int("loops", loops),
	)
	return allResults, nil
}

// // MultiSearch performs an msearch request.
// func MultiSearch(ctx context.Context, api *elasticsearch.TypedClient, searches ...*models.MultiSearchQuery) (results.MSearchResults, error) {
// 	// subscriptionsIndex, err := FeedsReadIndexFromCtx(ctx)
// 	// if err != nil {
// 	// 	return nil, errors.Join(ErrUpdateFailed, ErrFetchCtx)
// 	// }
// 	// itemsIndex, err := ItemsReadIndexFromCtx(ctx)
// 	// if err != nil {
// 	// 	return nil, fmt.Errorf("unable to perform multi-search: %w", err)
// 	// }

// 	options := make([]Option[MsearchRequest], 0, len(searches)+1)
// 	options = append(options, WithRequestID[*msearch.Msearch, MsearchRequest](middleware.GetReqID(ctx)))
// 	for search := range slices.Values(searches) {
// 		options = append(options, WithSearch(search))
// 	}

// 	req := NewMSearchRequest(api, options...)
// 	resp, err := req.Do(ctx)
// 	if err != nil {
// 		return nil, fmt.Errorf("multisearch: msearch request failed: %w", err)
// 	}

// 	results := make(map[string]*types.MultiSearchItem)
// 	for idx, search := range searches {
// 		if result, ok := resp.Responses[idx].(*types.MultiSearchItem); ok {
// 			results[search.Name] = result
// 		}
// 	}

// 	return results, nil
// }

//nolint:wrapcheck,err113
func toAPIError(msg string, err error) error {
	var esErr *types.ElasticsearchError
	if errors.As(err, &esErr) {
		return models.NewAPIError(
			fmt.Errorf("%s: %s: %s", msg, esErr.ErrorCause.Type, *esErr.ErrorCause.Reason),
			esErr.Status,
		)
	}
	return models.NewAPIError(fmt.Errorf("%s: %w", msg, err), http.StatusInternalServerError)
}

// paginationValue is a value that can be used as a sort value as a search after option.
type paginationValue[T types.FieldValue] struct {
	value T
}

func newPaginationValue[T any](value T) *paginationValue[T] {
	return &paginationValue[T]{value: value}
}

func (v *paginationValue[T]) FieldValueCaster() *types.FieldValue {
	casted := types.FieldValue(v)
	return &casted
}

func (v *paginationValue[T]) MarshalJSON() ([]byte, error) {
	data, err := json.Marshal(v.value)
	if err != nil {
		return data, fmt.Errorf("failed to marshal pagination value: %w", err)
	}
	return data, nil
}

// encodePagination will take sort values returned from a query, marshal them to
// JSON, then HTML-escape the string into a models.Pagination object, which is
// safe for use in API query parameters.
func encodePagination(sortValues []types.FieldValue) (models.Pagination, error) {
	if len(sortValues) == 0 {
		return "", nil
	}
	// Marshal sort values into json.
	data, err := json.Marshal(sortValues)
	if err != nil {
		return "", fmt.Errorf("could not encode pagination values: %w", err)
	}
	// Return as HTML encoded string.
	return url.QueryEscape(string(data)), nil
}

// decodePagination will take a models.Pagination object, HTML-unescape the
// string then unmarshal it back into sort values.
func decodePagination(pagination *models.Pagination) ([]types.FieldValueVariant, error) {
	if pagination == nil {
		return nil, nil
	}
	if *pagination == "" {
		return nil, nil
	}
	// Unescape HTML encoded data.
	data, err := url.QueryUnescape(*pagination)
	if err != nil {
		return nil, fmt.Errorf("could not decode pagination values: %w", err)
	}
	// Unmarshal sort values.
	var values []any
	err = json.Unmarshal([]byte(data), &values)
	if err != nil {
		return nil, fmt.Errorf("could not decode pagination values: %w", err)
	}
	casted := make([]types.FieldValueVariant, 0, len(values))
	for v := range slices.Values(values) {
		switch r := v.(type) {
		case string:
			casted = append(casted, newPaginationValue(r))
		case int64:
			casted = append(casted, newPaginationValue(r))
		case float64:
			casted = append(casted, newPaginationValue(r))
		default:
			casted = nil
		}
	}

	// Return sort values.
	return casted, nil
}

// ItemSorting contains the sort options for sorting item search results.
type ItemSorting struct {
	Updated   string `json:"updated"`
	Published string `json:"published"`
	ItemID    string `json:"item_id"`
}

// SortCombinationsCaster is required to allow ItemSorting to be used as Elasticsearch sort values.
func (s *ItemSorting) SortCombinationsCaster() *types.SortCombinations {
	c := types.SortCombinations(s)
	return &c
}

func newItemSortOptions(sort *models.Sort) []types.SortCombinationsVariant {
	if sort == nil {
		return []types.SortCombinationsVariant{&types.SortOptions{Doc_: types.NewScoreSort()}}
	}
	var opts []types.SortCombinationsVariant
	switch *sort {
	case models.SortNewestFirst:
		opts = append(opts, &ItemSorting{
			Updated:   "desc",
			Published: "desc",
			ItemID:    "desc",
		})
	case models.SortOldestFirst:
		opts = append(opts, &ItemSorting{
			Updated:   "asc",
			Published: "asc",
			ItemID:    "asc",
		})
	case models.SortMostRelevant:
		opts = append(opts, &types.SortOptions{
			Score_: &types.ScoreSort{
				Order: &sortorder.Desc,
			},
		})
		opts = append(opts,
			&ItemSorting{
				Updated:   "asc",
				Published: "asc",
				ItemID:    "asc",
			},
		)
	default:
		opts = append(opts, &types.SortOptions{
			Doc_: &types.ScoreSort{},
		})
	}
	return opts
}

func newItemSortCombinations(sort *models.Sort) []types.SortCombinations {
	var opts []types.SortCombinations
	switch *sort {
	case models.SortNewestFirst:
		opts = append(opts, &ItemSorting{
			Updated:   "desc",
			Published: "desc",
			ItemID:    "desc",
		})
	case models.SortOldestFirst:
		opts = append(opts, &ItemSorting{
			Updated:   "asc",
			Published: "asc",
			ItemID:    "asc",
		})
	case models.SortMostRelevant:
		opts = append(opts, &types.SortOptions{
			Score_: &types.ScoreSort{
				Order: &sortorder.Desc,
			},
		})
		opts = append(opts,
			&ItemSorting{
				Updated:   "asc",
				Published: "asc",
				ItemID:    "asc",
			},
		)
	default:
		opts = append(opts, &types.SortOptions{
			Doc_: types.NewScoreSort(),
		})
	}
	return opts
}

// FeedSorting contains the sort options for sorting item search results.
type FeedSorting struct {
	Updated   string `json:"updated"`
	Published string `json:"published"`
	FeedID    string `json:"feed_id"`
}

// SortCombinationsCaster is required to allow FeedSorting to be used as Elasticsearch sort values.
func (s *FeedSorting) SortCombinationsCaster() *types.SortCombinations {
	c := types.SortCombinations(s)
	return &c
}

func newFeedSortOptions(sort *models.Sort) []types.SortCombinationsVariant {
	if sort == nil {
		return []types.SortCombinationsVariant{&types.SortOptions{Doc_: types.NewScoreSort()}}
	}
	var opts []types.SortCombinationsVariant
	switch *sort {
	case models.SortNewestFirst:
		opts = append(opts, &FeedSorting{
			Updated:   "desc",
			Published: "desc",
			FeedID:    "desc",
		})
	case models.SortOldestFirst:
		opts = append(opts, &FeedSorting{
			Updated:   "asc",
			Published: "asc",
			FeedID:    "asc",
		})
	case models.SortMostRelevant:
		opts = append(opts, &types.SortOptions{
			Score_: &types.ScoreSort{
				Order: &sortorder.Desc,
			},
		})
		opts = append(opts,
			&FeedSorting{
				Updated:   "asc",
				Published: "asc",
				FeedID:    "asc",
			},
		)
	default:
		opts = append(opts, &types.SortOptions{
			Doc_: types.NewScoreSort(),
		})
	}
	return opts
}

func newFeedSortCombinations(sort *models.Sort) []types.SortCombinations {
	var opts []types.SortCombinations
	switch *sort {
	case models.SortNewestFirst:
		opts = append(opts, &FeedSorting{
			Updated:   "desc",
			Published: "desc",
			FeedID:    "desc",
		})
	case models.SortOldestFirst:
		opts = append(opts, &FeedSorting{
			Updated:   "asc",
			Published: "asc",
			FeedID:    "asc",
		})
	case models.SortMostRelevant:
		opts = append(opts, &types.SortOptions{
			Score_: &types.ScoreSort{
				Order: &sortorder.Desc,
			},
		})
		opts = append(opts,
			&FeedSorting{
				Updated:   "asc",
				Published: "asc",
				FeedID:    "asc",
			},
		)
	default:
		opts = append(opts, &types.SortOptions{
			Doc_: types.NewScoreSort(),
		})
	}
	return opts
}

// SubscriptionSorting contains the sort options for sorting subscription results.
type SubscriptionSorting struct {
	MarkedReadAt   string `json:"marked_read_at"`
	SubscriptionID string `json:"subscription_id"`
}

// SortCombinationsCaster is required to allow FeedSorting to be used as Elasticsearch sort values.
func (s *SubscriptionSorting) SortCombinationsCaster() *types.SortCombinations {
	c := types.SortCombinations(s)
	return &c
}

func newSubscriptionSortOptions(sort *models.Sort) []types.SortCombinationsVariant {
	if sort == nil {
		return []types.SortCombinationsVariant{&types.SortOptions{Doc_: types.NewScoreSort()}}
	}
	var opts []types.SortCombinationsVariant
	switch *sort {
	case models.SortNewestFirst:
		opts = append(opts, &SubscriptionSorting{
			MarkedReadAt:   "asc",
			SubscriptionID: "desc",
		})
	case models.SortOldestFirst:
		opts = append(opts, &SubscriptionSorting{
			MarkedReadAt:   "desc",
			SubscriptionID: "asc",
		})
	case models.SortMostRelevant:
		opts = append(opts, &types.SortOptions{
			Score_: &types.ScoreSort{
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
		opts = append(opts, &types.SortOptions{
			Doc_: types.NewScoreSort(),
		})
	}
	return opts
}
