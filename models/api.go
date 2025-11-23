// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/providers/elastic/aggregations"
	"github.com/immanent-tech/foragd/providers/elastic/bulk"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/validation"
)

var (
	// ErrNoUserCtx indicates the user object was not found in the context.
	ErrNoUserCtx = errors.New("no valid user in context")
	// ErrInvalidAPIResult indicates that the backend API returned unexpected, invalid or an otherwise incorrect response.
	ErrInvalidAPIResult = errors.New("invalid backend API result")
)

// FeedsAPI contains API methods for Feeds.
type FeedsAPI interface {
	GetFeeds(ctx context.Context, feedIDs ...FeedID) (Feeds, error)

	SearchFeeds(
		ctx context.Context,
		query query.Option,
		count int,
		sort *Sort,
		pagination *Pagination,
	) (Feeds, Pagination, error)
	// MultiSearchFeeds(ctx context.Context, queries ...*MultiSearchQuery) (results.MSearchResults, error)
	CreateFeed(ctx context.Context, feed *Feed) error
}

// SubscriptionsAPI contains API methods for Subscriptions.
type SubscriptionsAPI interface {
	GetSubscriptionByFeedID(ctx context.Context, id FeedID) (*Subscription, error)
	UpdateSubscriptions(
		ctx context.Context,
		subscriptions ...*Subscription,
	) (map[SubscriptionID]*bulk.OperationResponse, error)
	// UpdateSubscription(ctx context.Context, subscriptions *Subscription) error
	RemoveSubscriptions(ctx context.Context, query query.Option) error
}

// ItemsAPI contains API methods for Items.
type ItemsAPI interface {
	SearchItems(
		ctx context.Context,
		query query.Option,
		count int,
		sort *Sort,
		pagination *Pagination,
	) (Items, Pagination, error)
	CountItems(ctx context.Context, query query.Option) (int64, error)
	GetLastUpdatedItems(ctx context.Context, feedIDs ...FeedID) (Items, error)
	ItemsAggregation(
		ctx context.Context,
		query query.Option,
		count int,
		agg aggregations.Aggs,
	) (*search.Response, error)
	ArchiveArticle(ctx context.Context, article *ArticleArchive) error
	UnarchiveArticle(ctx context.Context, userID UserID, itemID ItemID) error
}

// UserAPI contains API methods for Users.
type UserAPI interface {
	CreateUser(ctx context.Context, user *User) error
	UpdateUser(ctx context.Context, id UserID, updates map[string]any) error
}

// DataAPI contains all methods for data API access.
type DataAPI interface {
	FeedsAPI
	ItemsAPI
	UserAPI
	SubscriptionsAPI
	GetAPI() *elasticsearch.TypedClient
}

// AddSubscriptions adds the given subscriptions to a user.
func AddSubscriptions(ctx context.Context, dataAPI DataAPI, subscriptions ...*Subscription) error {
	user := UserFromCtx(ctx)
	if user == nil {
		return fmt.Errorf("add subscriptions: get user data: %w", ErrNoUserCtx)
	}
	_, err := dataAPI.UpdateSubscriptions(ctx, subscriptions...)
	if err != nil {
		return fmt.Errorf("add subscriptions: %w", err)
	}
	// Disable onboarding once a subscription has been added.
	settings := user.GetSettings()
	if settings.ShowOnboarding {
		settings.ShowOnboarding = false
		// Update the user object.
		err = dataAPI.UpdateUser(ctx, user.GetID(), map[string]any{
			"settings": settings,
		})
		if err != nil {
			return fmt.Errorf("add subscriptions: update user: %w", err)
		}
	}
	return nil
}

// RemoveSubscriptions removes subscriptions with the given ID from a user.
func RemoveSubscriptions(ctx context.Context, dataAPI DataAPI, ids ...SubscriptionID) error {
	user := UserFromCtx(ctx)
	if user == nil {
		return fmt.Errorf("remove subscriptions: get user data: %w", ErrNoUserCtx)
	}
	query := query.Bool(
		query.Filter(
			query.Term("user_id", user.GetID()),
			query.Terms("subscription_id", ids...),
		),
	)
	err := dataAPI.RemoveSubscriptions(ctx, query)
	if err != nil {
		return fmt.Errorf("remove subscriptions: %w", err)
	}
	return nil
}

// CreateFeedSubscriptions will create new FeedSubscriptions for the user from the given requests.
func CreateFeedSubscriptions(ctx context.Context, dataAPI DataAPI, results ...*AddFeedSubscriptionResult) error {
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
		subscription, err := NewFeedSubscription(ctx, &result.Feed, &result.Request)
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
		result.Message = *NewSuccessMessage("Subscription Created: "+result.Feed.GetTitle(), "Articles will be fetched shortly...")
	}
	// Add subscriptions
	err := AddSubscriptions(ctx, dataAPI, subscriptions...)
	if err != nil {
		return fmt.Errorf("unable to create subscriptions: %w", err)
	}
	return nil
}

// CreateSearchSubscriptions will create new SearchSubscriptions for the user from the given requests.
func CreateSearchSubscriptions(ctx context.Context, dataAPI DataAPI, requests ...*SearchSubscriptionRequest) error {
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
	err := AddSubscriptions(ctx, dataAPI, subscriptions...)
	if err != nil {
		return fmt.Errorf("create search subscription: add subscriptions failed: %w", err)
	}
	return nil
}

func ProcessSubscriptionRequest(
	ctx context.Context,
	dataAPI DataAPI,
	request *AddFeedSubscriptionRequest,
	resultsCh chan AddFeedSubscriptionResult,
) {
	result := AddFeedSubscriptionResult{
		Request: *request,
	}
	// Try to match request URL to an existing feed
	var feed *Feed
	feeds, _, err := dataAPI.SearchFeeds(ctx, query.Term("source_urls", request.GetURL()), 1, nil, nil)
	if err != nil {
		result.Error = err
		result.Message = *NewErrorMessage("Unable to determine existing subscription status", "The backend produced an error. This might be temporary, please try again.")
		resultsCh <- result
		return
	}
	if len(feeds) == 1 {
		feed = feeds[0]
	}

	// If no existing feed, create a new one.
	if feed == nil {
		slogctx.FromCtx(ctx).Debug("Parsing url", slog.String("url", request.GetURL()))
		newFeed, err := NewFeedFromURL(ctx, request.GetURL())
		if err != nil {
			result.Error = err
			result.Message = *NewErrorMessage("Unable to create subscription", fmt.Sprintf("The feed URL %q cannot be parsed as a feed source or is not a valid URL.", request.GetURL()))
			resultsCh <- result
			return
		}
		err = validation.Validate.Struct(newFeed)
		if err != nil {
			result.Error = err
			result.Message = *NewErrorMessage("Unable to create subscription", fmt.Sprintf("The feed URL %q cannot be parsed as a feed source or is not a valid URL.", request.GetURL()))
			resultsCh <- result
			return
		}
		err = CreateFeed(ctx, dataAPI, newFeed)
		if err != nil {
			result.Error = err
			result.Message = *NewErrorMessage("Unable to create new feed for subscription", "The backend produced an error. This might be temporary, please try again.")
			resultsCh <- result
			return
		}
		slogctx.FromCtx(ctx).Debug("Created new feed for request.",
			slog.String("name", newFeed.GetTitle()),
			slog.String("urls", strings.Join(newFeed.GetSourceURLs(), ",")),
		)
		feed = newFeed
	}

	user := UserFromCtx(ctx)
	if user == nil {
		result.Error = ErrNoUserCtx
		result.Message = *NewErrorMessage("Unable to check for existing subscription", "The backend produced an error. This might be temporary, please try again.")
		resultsCh <- result
		return
	}
	subscription, err := dataAPI.GetSubscriptionByFeedID(ctx, feed.GetID())
	if err != nil {
		result.Error = err
		result.Message = *NewErrorMessage("Unable to check for existing subscription", "The backend produced an error. This might be temporary, please try again.")
		resultsCh <- result
		return
	}
	if subscription != nil {
		result.Error = fmt.Errorf("already subscribed")
		result.Message = *NewWarningMessage("Already subscribed to feed", feed.GetTitle()+" ("+request.URL+")")
		resultsCh <- result
		return
	}

	// Add the feed details to the result.
	result.Feed = *feed
	// Send the result back through the channel.
	resultsCh <- result
}

// CreateFeed stores a new Feed.
func CreateFeed(ctx context.Context, dataAPI DataAPI, feed *Feed) error {
	err := dataAPI.CreateFeed(ctx, feed)
	if err != nil {
		return fmt.Errorf("unable to create feed: %w", err)
	}
	return nil
}

// GetArticleTopCategories performs an aggregation to return the top Item categories across the given Feeds.
func GetArticleTopCategories(ctx context.Context, dataAPI DataAPI, feeds ...FeedID) ([]Category, error) {
	// Build query.
	query := query.Bool(
		query.Filter(
			// Must match any of the given feed IDs.
			query.Terms("feed_id", feeds...),
		),
	)
	// Build aggregations.
	termsField := "categories.raw"
	termsCount := 10
	aggs := aggregations.Aggs{
		"TopCategories": types.Aggregations{
			Terms: &types.TermsAggregation{
				Field: &termsField,
				Size:  &termsCount,
			},
		},
	}
	// Perform aggregation.
	results, err := dataAPI.ItemsAggregation(ctx, query, 0, aggs)
	if err != nil {
		return nil, fmt.Errorf("unable to get top categories: %w", err)
	}

	topCategoriesAgg, ok := results.Aggregations["TopCategories"].(*types.StringTermsAggregate)
	if !ok {
		return nil, fmt.Errorf("unable to get top categories: aggregations invalid: %w", ErrInvalidAPIResult)
	}
	topCategoriesBuckets, ok := topCategoriesAgg.Buckets.([]types.StringTermsBucket)
	if !ok {
		return nil, fmt.Errorf("unable to get top categories: aggregations invalid: %w", ErrInvalidAPIResult)
	}

	topCategories := make([]Category, 0)

	for bucket := range slices.Values(topCategoriesBuckets) {
		category, ok := bucket.Key.(Category)
		if ok {
			topCategories = append(topCategories, category)
		}
	}

	return topCategories, nil
}

// type MultiSearchQuery struct {
// 	Name       string
// 	Index      string
// 	Query      query.Option
// 	Sort       *Sort
// 	Pagination *Pagination
// 	Size       int
// }
