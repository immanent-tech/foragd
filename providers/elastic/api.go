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
	feeds "github.com/immanent-tech/go-syndication"
	slogctx "github.com/veqryn/slog-context"
	"golang.org/x/sync/errgroup"

	"github.com/immanent-tech/foragd/logging"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic/aggregations"
	"github.com/immanent-tech/foragd/providers/elastic/bulk"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/providers/elastic/results"
	"github.com/immanent-tech/foragd/providers/elastic/schema"
	"github.com/immanent-tech/foragd/server/session/store"
)

var (
	errNotFound     = errors.New("not found")
	ErrNotFound     = models.NewAPIError(errNotFound, http.StatusNotFound)
	ErrNoIndexInCtx = models.NewAPIError(
		fmt.Errorf("get index from context: %w", errNotFound),
		http.StatusInternalServerError,
	)
	errParseFailed = errors.New("parsing value failed")
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

// GetSession retrieves session data with the given token.
func (a *API) GetSession(ctx context.Context, token string) (*models.UserSession, error) {
	index := schema.SessionsSchemaPrefix + schema.IndexReadSuffix
	session, err := GetDoc[string, models.UserSession](ctx, a.GetAPI(), index, token)
	if err != nil {
		return nil, toAPIError(err)
	}
	return &session, nil
}

// DeleteSession removes the session data for the given token.
func (a *API) DeleteSession(ctx context.Context, token string) error {
	index := schema.SessionsSchemaPrefix + schema.IndexWriteSuffix
	err := DeleteDoc(ctx, a.GetAPI(), index, token)
	if err != nil {
		return toAPIError(err)
	}
	return nil
}

// UpdateSession updates the session data.
func (a *API) UpdateSession(ctx context.Context, token string, data map[string]any) error {
	index := schema.SessionsSchemaPrefix + schema.IndexWriteSuffix
	err := UpdateDoc(ctx, a.GetAPI(), index,
		token,
		data,
		UpdateDocAsUpsert(),
	)
	if err != nil {
		return toAPIError(err)
	}
	return nil
}

// FindAllSessions returns all active (non-expired) sessions.
func (a *API) FindAllSessions(ctx context.Context) ([]models.UserSession, error) {
	index := schema.SessionsSchemaPrefix + schema.IndexReadSuffix
	sessions, err := SearchAll[models.UserSession](
		ctx,
		a.GetAPI(),
		index,
		query.Since("expiry", time.Now().UTC()),
		1000,
	)
	if err != nil {
		return nil, toAPIError(err)
	}
	return sessions, nil
}

// GetAPI returns the raw API object.
func (a *API) GetAPI() *elasticsearch.TypedClient {
	return a.TypedClient
}

// UserExists checks if a user doc with the given ID exists.
func (a *API) UserExists(ctx context.Context, id models.UserID) (bool, error) {
	index, err := UserReadIndexFromCtx(ctx)
	if err != nil {
		return false, ErrNoIndexInCtx //nolint:wrapcheck
	}
	found, err := exists(ctx, a.TypedClient, index, id)
	if err != nil {
		return false, toAPIError(err)
	}
	return found, nil
}

// CreateUser creates a new user doc in Elasticsearch.
func (a *API) CreateUser(ctx context.Context, user *models.User) error {
	index, err := UserWriteIndexFromCtx(ctx)
	if err != nil {
		return ErrNoIndexInCtx //nolint:wrapcheck
	}
	err = CreateDoc(ctx, a.GetAPI(), index, user.GetID(), user)
	if err != nil {
		return toAPIError(err)
	}
	return nil
}

// GetUser retrieves the user doc with the given id.
func (a *API) GetUser(ctx context.Context, id models.UserID) (*models.User, error) {
	index, err := UserReadIndexFromCtx(ctx)
	if err != nil {
		return nil, ErrNoIndexInCtx //nolint:wrapcheck
	}
	user, err := GetDoc[models.UserID, *models.User](ctx, a.GetAPI(), index, id)
	if err != nil {
		return nil, toAPIError(err)
	}
	return user, nil
}

// DeleteUser removes the user doc with the given ID.
func (a *API) DeleteUser(ctx context.Context, id models.UserID) error {
	index, err := UserWriteIndexFromCtx(ctx)
	if err != nil {
		return ErrNoIndexInCtx //nolint:wrapcheck
	}
	err = DeleteDoc(ctx, a.GetAPI(), index, id)
	if err != nil {
		return toAPIError(err)
	}
	return nil
}

// UpdateUser will apply the given updates to the user.
func (a *API) UpdateUser(ctx context.Context, userID models.UserID, updates map[string]any) error {
	updates["updated_at"] = time.Now().UTC()
	index, err := UserWriteIndexFromCtx(ctx)
	if err != nil {
		return ErrNoIndexInCtx //nolint:wrapcheck
	}
	err = UpdateDoc(ctx, a.GetAPI(), index, userID, updates,
		WithRefresh("true"),
		WithRetryOnConflict(5),
	)
	if err != nil {
		return toAPIError(err)
	}
	return nil
}

// FindUserByExternalID will search for and return a user that matches the given external ID, if exists.
func (a *API) FindUserByExternalID(ctx context.Context, externalID string) (*models.User, error) {
	index, err := UserReadIndexFromCtx(ctx)
	if err != nil {
		return nil, ErrNoIndexInCtx //nolint:wrapcheck
	}
	// Get the user.
	users, _, err := Search[*models.User](ctx, a.GetAPI(), index, query.Term("external_user_id", externalID), 1,
		WithSortOptions[*search.Search, SearchRequest](&types.SortOptions{Doc_: types.NewScoreSort()}),
		WithTrackTotalHits(false),
	)
	if err != nil {
		return nil, toAPIError(err)
	}
	if len(users) == 0 {
		return nil, ErrNotFound //nolint:wrapcheck
	}
	return users[0], nil
}

// GetSubscriptionsByIDs returns the subscriptions that match the given IDs. Note: no dynamic info is generated for the
// subscriptions (use AddSubscriptionDynamicInfo after calling this method if needed).
func (e *API) GetSubscriptionsByIDs(
	ctx context.Context,
	ids ...models.SubscriptionID,
) (models.Subscriptions, error) {
	// Get subscriptions by ID.
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get subscriptions by ids: get user data: %w", models.ErrNoUserCtx)
	}

	// Suggestions query will match in title/description/categories across all feed subscriptions.
	subscriptionQuery := query.Bool(
		query.Filter(
			query.Term("user_id", user.GetID()),
			query.Terms("subscription_id", ids...),
		),
	)

	subscriptions, err := e.getAllSubscriptionsByQuery(ctx, subscriptionQuery)
	if err != nil {
		return nil, fmt.Errorf("get subscriptions by ids: api request failed: %w", err)
	}
	if len(subscriptions) == 0 {
		return nil, models.ErrNotFound
	}
	return subscriptions, nil
}

// GetSubscriptionsByIDs returns the subscriptions that match the given IDs. Note: no dynamic info is generated for the
// subscriptions (use AddSubscriptionDynamicInfo after calling this method if needed).
func (e *API) GetAllSubscriptions(
	ctx context.Context,
) (models.Subscriptions, error) {
	// Get subscriptions by ID.
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get all subscriptions: get user data: %w", models.ErrNoUserCtx)
	}

	// Suggestions query will match in title/description/categories across all feed subscriptions.
	subscriptionQuery := query.Bool(
		query.Filter(
			query.Term("user_id", user.GetID()),
		),
	)

	subscriptions, err := e.getAllSubscriptionsByQuery(ctx, subscriptionQuery)
	if err != nil {
		return nil, fmt.Errorf("get all subscriptions: api request failed: %w", err)
	}
	if len(subscriptions) == 0 {
		return nil, models.ErrNotFound
	}
	return subscriptions, nil
}

// GetFavoriteSubscriptions returns the favorite subscriptions of the user.
func (e *API) GetFavoriteSubscriptions(
	ctx context.Context,
) (models.Subscriptions, error) {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get favorite subscriptions: get user data: %w", models.ErrNoUserCtx)
	}
	// Get favorite subscriptions.
	subscriptions, err := e.getAllSubscriptionsByQuery(ctx, query.Bool(
		query.Filter(
			query.Term("user_id", user.GetID()),
			query.Term("favorite", true),
		),
	))
	if err != nil {
		return nil, fmt.Errorf("get favorite subscriptions: %w", err)
	}
	err = e.AddSubscriptionDynamicInfo(ctx, subscriptions)
	if err != nil {
		return nil, fmt.Errorf("list favorites: get favorite subscriptions: %w", err)
	}
	return subscriptions, nil
}

// GetSubscriptionSuggestions returns subscriptions that match the given text. Note: no dynamic info is generated for the
// subscriptions (use AddSubscriptionDynamicInfo after calling this method if needed).
func (e *API) GetSubscriptionSuggestions(ctx context.Context, text string) (models.Subscriptions, error) {
	// Get subscriptions by ID.
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get subscription suggestions: get user data: %w", models.ErrNoUserCtx)
	}

	// Suggestions query will match in title/description/categories across all feed subscriptions.
	subscriptionQuery := query.Bool(
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
	)

	subscriptions, err := e.getAllSubscriptionsByQuery(ctx, subscriptionQuery)
	if err != nil {
		return nil, fmt.Errorf("get subscription suggestions: api request failed: %w", err)
	}
	if len(subscriptions) == 0 {
		return nil, ErrNotFound
	}
	return subscriptions, nil
}

// FilterSubscriptions returns subscriptions filtered by the given filters and paginated by the given pagination.
// Dynamic information for subscriptions will also be added.
func (e *API) FilterSubscriptions(
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
			query.Terms("categories", filters.GetCategories()...),
		),
	)
	subscriptions, err := e.getAllSubscriptionsByQuery(ctx, subscriptionQuery)
	if err != nil {
		return nil, "", fmt.Errorf("filter subscriptions: api request failed: %w", err)
	}
	if len(subscriptions) == 0 {
		return nil, "", ErrNotFound
	}
	// Add dynamic info.
	err = e.AddSubscriptionDynamicInfo(ctx, subscriptions)
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

// AddSubscriptionDynamicInfo adds dynamically generated information (e.g., unread count, stats, etc.) to subscriptions.
// At the least, all subscriptions will have an unread count and last updated info generated. Other stats will also be
// generated if the user has set the display option ShowSubscriptionStats in their account settings.
//
//nolint:gocognit,funlen
func (e *API) AddSubscriptionDynamicInfo(ctx context.Context, subscriptions models.Subscriptions) error {
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
		extraSubscriptions, err := e.GetSubscriptionsByIDs(ctx, extraIDs...)
		if err != nil {
			return fmt.Errorf("add subscription dynamic info: get additional subscriptions: %w", err)
		}
		subscriptions = append(subscriptions, extraSubscriptions...)
	}

	fetchJobs, ctx := errgroup.WithContext(ctx)

	// Get unread count per feed.
	var unreadCounts map[models.FeedID]int64
	fetchJobs.Go(func() error {
		var err error
		unreadCounts, err = e.getFeedUnreadCounts(ctx, subscriptions)
		if err != nil {
			return fmt.Errorf("get unread counts: %w", err)
		}
		return nil
	})

	// For search subscriptions, run queries directly to add unread count and last update.
	fetchJobs.Go(func() error {
		for subscription := range slices.Values(subscriptions) {
			if subscription.GetSubscriptionType() != models.SubscriptionTypeSearch {
				continue
			}
			search := subscription.SearchData.Search
			// Build query to get unread count.
			query, err := e.BuildSearchResultsQuery(ctx, user, &search)
			if err != nil {
				return fmt.Errorf(
					"add subscription dynamic info: build search subscription %s query: %w",
					subscription.GetID(),
					err,
				)
			}
			count, err := e.CountItems(ctx, query)
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
			query, err = e.BuildSearchResultsQuery(ctx, user, &search)
			if err != nil {
				return fmt.Errorf(
					"add subscription dynamic info: build search subscription %s query: %w",
					subscription.GetID(),
					err,
				)
			}
			items, _, err := e.SearchItems(ctx, query, 1, &sort, nil)
			if err == nil {
				if len(items) > 0 {
					subscription.Stats.LastUpdate = items[0].GetTimestamp()
				}
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
		lastUpdate, err = e.getFeedLastUpdates(ctx, subscriptions.GetFeedIDs()...)
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
			avgDailyUpdates, err = e.getFeedAverageDailyUpdates(ctx, subscriptions.GetFeedIDs()...)
			if err != nil {
				return fmt.Errorf("get average daily updates: %w", err)
			}
			return nil
		})
	}

	err := fetchJobs.Wait()
	if err != nil {
		return fmt.Errorf("add subscription dynamic info: run jobs: %w", err)
	}

	// For feed subscriptions, add stats.
	for subscription := range slices.Values(subscriptions) {
		if subscription.GetSubscriptionType() == models.SubscriptionTypeFeed {
			// Add stats for feed subscriptions.
			subscription.Stats.UnreadCount = int(unreadCounts[subscription.FeedData.FeedID])
			subscription.Stats.LastUpdate = lastUpdate[subscription.FeedData.FeedID]
			if user.GetSettings().ShowSubscriptionStats {
				subscription.Stats.AvgDailyUpdates = avgDailyUpdates[subscription.FeedData.FeedID]
			}
		}
	}

	// For group subscriptions, calculate stats from other subscriptions.
	for subscription := range slices.Values(subscriptions) {
		if subscription.GetSubscriptionType() == models.SubscriptionTypeGroup {
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
	}

	return nil
}

// GetFeedSubscriptionStats fetches the stats for FeedSubscriptions and returns a map of the SubscriptionID to
// SubscriptionStats that can be used to lookup the stats pertaining to a particular subscription.
func (e *API) getFeedAverageDailyUpdates(ctx context.Context, ids ...models.FeedID) (map[models.FeedID]float64, error) {
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

	results, err := e.ItemsAggregation(ctx, query, len(ids), aggs)
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

	for feed := range slices.Values(feedStatsBuckets) {
		feedID, ok := feed.Key.(string)
		if !ok {
			slogctx.FromCtx(ctx).Debug("Unable to extract feed ID for aggregation", slog.Any("feed_id", feed.Key))
			continue
		}
		updatesResult, ok := feed.Aggregations["avg_daily_updates"].(*types.SimpleValueAggregate)
		if !ok {
			slogctx.FromCtx(ctx).
				Debug("Unable to extract avg_daily_updates agg for subscription", slog.String("feed_id", feedID))
			continue
		}

		stats[feedID] = float64(*updatesResult.Value)
	}

	return stats, nil
}

func (e *API) getFeedUnreadCounts(
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
	results, err := e.ItemsAggregation(ctx, query, 0, aggs)
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

	for feed := range slices.Values(unreadCountsBuckets) {
		feedID, ok := feed.Key.(string)
		if !ok {
			slogctx.FromCtx(ctx).Debug("Unable to extract feed ID for aggregation", slog.Any("feed_id", feed.Key))
			continue
		}
		stats[feedID] = feed.DocCount
	}
	return stats, nil
}

func (e *API) getFeedLastUpdates(ctx context.Context, ids ...models.FeedID) (map[models.FeedID]time.Time, error) {
	items, err := e.GetLastUpdatedItems(ctx, ids...)
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
func (e *API) MarkSubscriptions(
	ctx context.Context,
	mark models.Mark,
	subscriptionIDs ...models.SubscriptionID,
) error {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return fmt.Errorf("mark subscriptions: get user data: %w", models.ErrNoUserCtx)
	}

	subscriptions, err := e.GetSubscriptionsByIDs(ctx, subscriptionIDs...)
	if err != nil {
		return fmt.Errorf("mark subscriptions: %w", err)
	}

	for subscription := range slices.Values(subscriptions) {
		if subscription.GetSubscriptionType() == models.SubscriptionTypeGroup {
			if err = e.MarkSubscriptions(ctx, mark, subscription.GroupData.Subscriptions...); err != nil {
				return fmt.Errorf("mark subscriptions: mark group subscription: %w", err)
			}
		} else {
			subscription.Mark(user, mark)
			if _, err = e.UpdateSubscriptions(ctx, subscriptions...); err != nil {
				return fmt.Errorf("mark subscriptions: update subscription data: %w", err)
			}
		}
	}

	return nil
}

// getAllSubscriptionsByQuery returns all subscriptions that match the given query.
func (e *API) getAllSubscriptionsByQuery(ctx context.Context, query query.Option) (models.Subscriptions, error) {
	index, err := SubscriptionsReadIndexFromCtx(ctx)
	if err != nil {
		return nil, ErrNoIndexInCtx //nolint:wrapcheck
	}

	var subscriptions models.Subscriptions
	subscriptions, err = SearchAll[*models.Subscription](ctx, e.GetAPI(), index, query, 5000)
	if err != nil {
		return nil, toAPIError(err)
	}
	return subscriptions, nil
}

func (e *API) SearchSubscriptions(
	ctx context.Context,
	query query.Option,
	count int,
	sort *models.Sort,
	pagination *models.Pagination,
) (models.Subscriptions, models.Pagination, error) {
	index, err := SubscriptionsReadIndexFromCtx(ctx)
	if err != nil {
		return nil, "", ErrNoIndexInCtx //nolint:wrapcheck
	}

	searchAfter, err := decodePagination(pagination)
	if err != nil {
		return nil, "", models.NewAPIError(
			fmt.Errorf("search subscriptions: decode pagination failed: %w", err),
			http.StatusInternalServerError,
		) //nolint:wrapcheck
	}

	// Perform search.
	subscriptions, newSearchAfter, err := Search[*models.Subscription](ctx, e.GetAPI(), index, query, count,
		WithSortOptions[*search.Search, SearchRequest](newSubscriptionSortOptions(sort)...),
		WithSearchAfter[*search.Search, SearchRequest](searchAfter...),
	)
	if err != nil {
		return nil, "", toAPIError(err)
	}
	// Parse search after into pagination.
	if pagination != nil {
		*pagination, err = encodePagination(newSearchAfter)
		if err != nil {
			return nil, "", models.NewAPIError(
				fmt.Errorf("search subscriptions: encode pagination failed: %w", err),
				http.StatusInternalServerError,
			) //nolint:wrapcheck
		}
		return subscriptions, *pagination, nil
	}

	return subscriptions, "", nil
}

// // GetSubscription retrieves the subscription with the given ID.
// func (a *API) GetSubscriptions(ctx context.Context, ids ...models.SubscriptionID) (models.Subscriptions, error) {
// 	index, err := UserReadIndexFromCtx(ctx)
// 	if err != nil {
// 		return nil, ErrNoIndexInCtx //nolint:wrapcheck
// 	}
// 	subscriptions, err := GetDocs[models.SubscriptionID, *models.Subscription](ctx, a.GetAPI(), index, ids...)
// 	if err != nil {
// 		return nil, toAPIError(err)
// 	}
// 	return subscriptions, nil
// }

// // GetSubscription retrieves the subscription with the given ID.
// func (a *API) GetSubscription(ctx context.Context, id models.SubscriptionID) (*models.Subscription, error) {
// 	index, err := UserReadIndexFromCtx(ctx)
// 	if err != nil {
// 		return nil, ErrNoIndexInCtx //nolint:wrapcheck
// 	}
// 	subscription, err := GetDoc[models.SubscriptionID, *models.Subscription](ctx, a.GetAPI(), index, id)
// 	if err != nil {
// 		return nil, toAPIError(err)
// 	}
// 	return subscription, nil
// }

// UpdateSubscriptions will bulk update the given subscriptions in Elasticsearch.
func (e *API) UpdateSubscriptions(
	ctx context.Context,
	subscriptions ...*models.Subscription,
) (map[models.SubscriptionID]*bulk.OperationResponse, error) {
	index, err := SubscriptionsWriteIndexFromCtx(ctx)
	if err != nil {
		return nil, ErrNoIndexInCtx //nolint:wrapcheck
	}
	return BulkUpdate(ctx, e, index, subscriptions...)
}

// UpdateSubscriptions will bulk update the given subscriptions in Elasticsearch.
func (e *API) RemoveSubscriptions(ctx context.Context, query query.Option) error {
	index, err := SubscriptionsWriteIndexFromCtx(ctx)
	if err != nil {
		return ErrNoIndexInCtx //nolint:wrapcheck
	}
	err = DeleteDocs(ctx, e.GetAPI(), index, query)
	if err != nil {
		return toAPIError(err)
	}
	return nil
}

// BuildSubscriptionQueries generates a slices of queries for the given subscriptions, based on the given filters.
func (e *API) BuildSubscriptionQueries(
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

// FilterArticles returns Articles filtered by the given filters and paginated by the given pagination.
func (e *API) FilterArticles(
	ctx context.Context,
	filters *models.ListDisplayFilters,
	pagination models.Pagination,
) (models.Articles, models.Pagination, error) {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, "", fmt.Errorf("filter articles: get user data: %w", models.ErrNoUserCtx)
	}

	subscriptions, err := e.GetSubscriptionsByIDs(ctx, filters.GetSubscriptions()...)
	if err != nil {
		return nil, "", fmt.Errorf("filter articles: get subscriptions: %w", err)
	}
	// Return early if there the user has no subscriptions (i.e., new user).
	if len(subscriptions) == 0 {
		return nil, "", ErrNotFound
	}
	// Search through items matching any given feeds filters, excluding any read
	// items.
	articleQuery := query.Bool(
		query.Filter(
			// Must match any of the given categories.
			query.Terms("categories.raw", filters.GetCategories()...),
			query.Bool(
				query.Should(e.BuildSubscriptionQueries(user, filters.GetView(), subscriptions)...),
			),
		),
	)

	sort := filters.GetSort()

	// Find items matching filters.
	items, pagination, err := e.SearchItems(ctx, articleQuery, filters.GetCount(), &sort, &pagination)
	if err != nil {
		return nil, "", fmt.Errorf("could not retrieve filtered items: %w", err)
	}
	// Generate articles.
	articles, err := models.GenerateArticles(ctx, e, items)
	if err != nil {
		return nil, "", fmt.Errorf("could not generate articles from items: %w", err)
	}

	return articles, pagination, nil
}

// FindSimilarArticles performs a "more like this" search to find other Articles that are similar to the Items with the
// given IDs.
func (e *API) FindSimilarArticles(ctx context.Context, itemIDs ...models.ItemID) (models.Articles, error) {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("find similar articles: get user data: %w", models.ErrNoUserCtx)
	}
	subscriptions, err := e.GetAllSubscriptions(ctx)
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
				query.Should(e.BuildSubscriptionQueries(user, models.ViewUnread, subscriptions)...),
			),
		),
		query.Must(
			mlt.ToQueryOption(),
		),
	)
	// Query for similar articles.
	sort := models.SortMostRelevant
	items, _, err := e.SearchItems(ctx, similarQuery, 15, &sort, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to find similar articles: %w", err)
	}
	// Generate article data.
	articles, err := models.GenerateArticles(ctx, e, items)
	if err != nil {
		return nil, fmt.Errorf("unable to find similar articles: %w", err)
	}
	return articles, nil
}

// BuildItemsQuery generates a query to fetch the Items that match the given Filters from the given Subscriptions.
func (e *API) BuildItemsQuery(
	ctx context.Context,
	filters models.Filters,
	subscriptionIDs ...models.SubscriptionID,
) (query.Option, error) {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("unable to build items query: %w", models.ErrNoUserCtx)
	}

	subscriptions, err := e.GetSubscriptionsByIDs(ctx, subscriptionIDs...)
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
				query.Should(e.BuildSubscriptionQueries(user, filters.GetView(), subscriptions)...),
			),
		),
	), nil
}

// BuildSearchResultsQuery generates a query that can be used to fetch appropriate results for a given SearchRequest
// criteria.
func (e *API) BuildSearchResultsQuery(
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

	subscriptions, err := e.GetSubscriptionsByIDs(ctx, request.Subscriptions...)
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
				query.Should(e.BuildSubscriptionQueries(user, request.View, subscriptions)...),
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

// GetFeed retrieves a single feed with the given ID.
func (a *API) GetFeed(ctx context.Context, id models.FeedID) (*models.Feed, error) {
	index, err := FeedsReadIndexFromCtx(ctx)
	if err != nil {
		return nil, ErrNoIndexInCtx //nolint:wrapcheck
	}
	feed, err := GetDoc[models.FeedID, *models.Feed](ctx, a.GetAPI(), index, id)
	if err != nil {
		return nil, toAPIError(err)
	}
	return feed, nil
}

// CreateFeed creates a new feed doc in Elasticsearch.
func (a *API) CreateFeed(ctx context.Context, feed *models.Feed) error {
	index, err := FeedsWriteIndexFromCtx(ctx)
	if err != nil {
		return ErrNoIndexInCtx //nolint:wrapcheck
	}
	err = CreateDoc(ctx, a.GetAPI(), index, feed.GetID(), feed)
	if err != nil {
		return toAPIError(err)
	}
	return nil
}

// DeleteFeed deletes a feed doc with the given ID from Elasticsearch.
func (a *API) DeleteFeed(ctx context.Context, id models.FeedID) error {
	index, err := FeedsWriteIndexFromCtx(ctx)
	if err != nil {
		return ErrNoIndexInCtx //nolint:wrapcheck
	}
	// Delete the feed.
	err = DeleteDoc(ctx, a.GetAPI(), index, id)
	if err != nil {
		return toAPIError(err)
	}
	return nil
}

// GetFeeds retrieves the feeds with the given IDs.
func (a *API) GetFeeds(ctx context.Context, ids ...models.FeedID) (models.Feeds, error) {
	index, err := FeedsReadIndexFromCtx(ctx)
	if err != nil {
		return nil, ErrNoIndexInCtx //nolint:wrapcheck
	}

	feeds, err := GetDocs[models.FeedID, *models.Feed](ctx, a.GetAPI(), index, ids...)
	if err != nil {
		return nil, toAPIError(err)
	}
	return feeds, nil
}

// SearchFeeds will search the feeds index for feed matching the given query. Count, sort and pagination values are
// optional.
func (e *API) SearchFeeds(
	ctx context.Context,
	query query.Option,
	count int,
	sort *models.Sort,
	pagination *models.Pagination,
) (models.Feeds, models.Pagination, error) {
	index, err := FeedsReadIndexFromCtx(ctx)
	if err != nil {
		return nil, "", ErrNoIndexInCtx //nolint:wrapcheck
	}

	searchAfter, err := decodePagination(pagination)
	if err != nil {
		return nil, "", models.NewAPIError(
			fmt.Errorf("search feeds: decode pagination failed: %w", err),
			http.StatusInternalServerError,
		) //nolint:wrapcheck
	}

	// Perform search.
	feeds, newSearchAfter, err := Search[*models.Feed](ctx, e.GetAPI(), index, query, count,
		WithSortOptions[*search.Search, SearchRequest](newFeedSortOptions(sort)...),
		WithSearchAfter[*search.Search, SearchRequest](searchAfter...),
	)
	if err != nil {
		return nil, "", toAPIError(err)
	}
	// Parse search after into pagination.
	if pagination != nil {
		*pagination, err = encodePagination(newSearchAfter)
		if err != nil {
			return nil, "", models.NewAPIError(
				fmt.Errorf("search feeds: encode pagination failed: %w", err),
				http.StatusInternalServerError,
			) //nolint:wrapcheck
		}
		return feeds, *pagination, nil
	}

	return feeds, "", nil
}

// func (e *API) MultiSearchFeeds(ctx context.Context, queries ...*models.MultiSearchQuery) (results.MSearchResults, error) {
// 	return MultiSearch(ctx, e.GetAPI(), queries...)
// }

// GetNewFeedsSince will return a slice of all feeds that have been created since the given timestamp.
func (e *API) GetNewFeedsSince(ctx context.Context, since time.Time) (models.Feeds, error) {
	// Get all new feeds created since last checkpoint.
	index, err := FeedsReadIndexFromCtx(ctx)
	if err != nil {
		return nil, ErrNoIndexInCtx //nolint:wrapcheck
	}
	// Generate query. We detect new feeds by those where the last_fetched value equals Unix Epoch, indicating they
	// don't have a job scheduled for updating their items.
	query := query.Term("last_fetched", models.UnixEpoch)
	var feeds models.Feeds
	feeds, err = SearchAll[*models.Feed](ctx, e.GetAPI(), index, query, 1000)
	if err != nil {
		return nil, toAPIError(err)
	}
	return feeds, nil
}

// UpdateFeed will update the feed with the given id, using the new feed information provided.
func (e *API) UpdateFeed(ctx context.Context, id models.FeedID, updated *feeds.Feed) error {
	// Update the feed timestamp.
	index, err := FeedsWriteIndexFromCtx(ctx)
	if err != nil {
		return ErrNoIndexInCtx //nolint:wrapcheck
	}
	updates := map[string]any{
		"last_fetched": time.Now().UTC(),
	}
	err = UpdateDoc(ctx, e.GetAPI(), index, id, updates)
	if err != nil {
		return toAPIError(err)
	}
	return nil
}

// SearchItems will search the items index for items matching the given query. Count, sort and pagination values are
// optional.
func (e *API) SearchItems(
	ctx context.Context,
	query query.Option,
	count int,
	sort *models.Sort,
	pagination *models.Pagination,
) (models.Items, models.Pagination, error) {
	index, err := ItemsReadIndexFromCtx(ctx)
	if err != nil {
		return nil, "", ErrNoIndexInCtx //nolint:wrapcheck
	}

	searchAfter, err := decodePagination(pagination)
	if err != nil {
		return nil, "", models.NewAPIError(
			fmt.Errorf("search items: decode pagination failed: %w", err),
			http.StatusInternalServerError,
		) //nolint:wrapcheck
	}
	// Perform search.
	items, newSearchAfter, err := Search[*models.Item](ctx, e.GetAPI(), index, query, count,
		WithSortOptions[*search.Search, SearchRequest](newItemSortOptions(sort)...),
		WithSearchAfter[*search.Search, SearchRequest](searchAfter...),
	)
	if err != nil {
		return nil, "", toAPIError(err)
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

func (e *API) GetLastUpdatedItems(ctx context.Context, feedIDs ...models.FeedID) (models.Items, error) {
	index, err := ItemsReadIndexFromCtx(ctx)
	if err != nil {
		return nil, ErrNoIndexInCtx //nolint:wrapcheck
	}

	items, _, err := Search[*models.Item](
		ctx,
		e.GetAPI(),
		index,
		query.Terms("feed_id", feedIDs...),
		len(feedIDs),
		WithCollapseField("feed_id"),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to get feed last updates: %w", err)
	}

	return items, nil
}

// ItemsAggregation performs an aggregation-only (i.e., search request with no hits returned) using the given query as
// the set of documents and performing the given aggregations across the documents.
func (e *API) ItemsAggregation(
	ctx context.Context,
	query query.Option,
	size int,
	aggregations aggregations.Aggs,
) (*search.Response, error) {
	index, err := ItemsReadIndexFromCtx(ctx)
	if err != nil {
		return nil, toAPIError(err)
	}

	req := NewSearchRequest(e.GetAPI(),
		WithRequestID[*search.Search, SearchRequest](middleware.GetReqID(ctx)),
		WithIndex[*search.Search, SearchRequest](index),
		WithQueryOptions[*search.Search, SearchRequest](query),
		WithSize[*search.Search, SearchRequest](size),
		WithSortOptions[*search.Search, SearchRequest](&types.SortOptions{Doc_: types.NewScoreSort()}),
		WithAggregations[*search.Search, SearchRequest](aggregations),
	)
	resp, err := req.Do(ctx)
	if err != nil {
		return nil, toAPIError(err)
	}

	return resp, nil
}

// CountItems returns a count of items that match the given query.
func (e *API) CountItems(ctx context.Context, query query.Option) (int64, error) {
	index, err := ItemsReadIndexFromCtx(ctx)
	if err != nil {
		return 0, ErrNoIndexInCtx //nolint:wrapcheck
	}

	count, err := Count(ctx, e.GetAPI(), index, query)
	if err != nil {
		return 0, toAPIError(err)
	}

	return count, nil
}

// AddItems will bulk index the given items.
func (e *API) AddItems(ctx context.Context, items ...*models.Item) (map[models.ItemID]*bulk.OperationResponse, error) {
	index, err := ItemsWriteIndexFromCtx(ctx)
	if err != nil {
		return nil, ErrNoIndexInCtx //nolint:wrapcheck
	}
	return BulkUpdate(ctx, e, index, items...)
}

// ArchiveArticle will index the given article content to the article archive for permanent storage.
func (a *API) ArchiveArticle(ctx context.Context, article *models.ArticleArchive) error {
	index, err := FavoriteItemsWriteIndexFromCtx(ctx)
	if err != nil {
		return ErrNoIndexInCtx //nolint:wrapcheck
	}
	err = CreateDoc(ctx, a.GetAPI(), index, article.ItemID, article)
	if err != nil {
		return toAPIError(err)
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
		return toAPIError(err)
	}
	return nil
}

// GetJobState retrieves the job state doc with the given ID from Elasticsearch.
func (a *API) GetJobState(ctx context.Context, id string) (*models.JobState, error) {
	index, err := SchedulerReadIndexFromCtx(ctx)
	if err != nil {
		return nil, ErrNoIndexInCtx //nolint:wrapcheck
	}
	state, err := GetDoc[string, *models.JobState](ctx, a.GetAPI(), index, id)
	if err != nil {
		return nil, toAPIError(err)
	}
	return state, nil
}

// UpdateJobState updates the job state doc with the given ID in Elasticsearch.
func (a *API) UpdateJobState(ctx context.Context, id string, updates map[string]any) error {
	index, err := SchedulerWriteIndexFromCtx(ctx)
	if err != nil {
		return ErrNoIndexInCtx //nolint:wrapcheck
	}
	updates["updated_at"] = time.Now().UTC()
	err = UpdateDoc(ctx, a.GetAPI(), index, id, updates,
		UpdateDocAsUpsert(),
		WithRefresh("true"),
	)
	if err != nil {
		return toAPIError(err)
	}
	return nil
}

// CountJobs returns a count of the scheduler jobs in the jobs index.
func (e *API) CountJobs(ctx context.Context) (int64, error) {
	index, err := SchedulerReadIndexFromCtx(ctx)
	if err != nil {
		return 0, ErrNoIndexInCtx //nolint:wrapcheck
	}
	count, err := Count(ctx, e.GetAPI(), index, query.Exists("job_type"))
	if err != nil {
		return 0, toAPIError(err)
	}

	return count, nil
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
		return nil, fmt.Errorf("bulk operation failed: %w", bulkOpResponse.Err)
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
		return nil, fmt.Errorf("bulk operation failed: %w", bulkOpResponse.Err)
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
		return false, fmt.Errorf("exists request failed: %w", errNotFound)
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
		return 0, fmt.Errorf("count request failed: %w", err)
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
		return nil, fmt.Errorf("get docs: mget request failed: %w", err)
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
		return doc, fmt.Errorf("get doc: get request failed: %w", err)
	}
	if !resp.Found {
		return doc, ErrNotFound //nolint:wrapcheck
	}
	doc, err = results.ExtractSource[O](resp.Source_)
	if err != nil {
		return doc, fmt.Errorf("get doc: extract doc failed: %w", err)
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
		return fmt.Errorf("create doc: create request failed: %w", err)
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
		return fmt.Errorf("update doc: update doc request failed: %w", err)
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
		return fmt.Errorf("delete doc: delete request failed: %w", err)
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
		return fmt.Errorf("delete docs: delete by query request failed: %w", err)
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
		return nil, nil, fmt.Errorf("search: search request failed: %w", err)
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
			return nil, fmt.Errorf("search all: search request failed: %w", err)
		}
		pagination, err := encodePagination(nextSearchAfter)
		if err != nil {
			return nil, models.NewAPIError(
				fmt.Errorf("search all: encode pagination failed: %w", err),
				http.StatusInternalServerError,
			) //nolint:wrapcheck
		}
		searchAfter, err = decodePagination(&pagination)
		if err != nil {
			return nil, models.NewAPIError(
				fmt.Errorf("search all: decode pagination failed: %w", err),
				http.StatusInternalServerError,
			) //nolint:wrapcheck
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
func toAPIError(err error) error {
	var esErr *types.ElasticsearchError
	if errors.As(err, &esErr) {
		msg := fmt.Errorf("%s: %s", esErr.ErrorCause.Type, *esErr.ErrorCause.Reason)
		return models.NewAPIError(msg, esErr.Status)
	}
	return models.NewAPIError(err, http.StatusInternalServerError)
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
