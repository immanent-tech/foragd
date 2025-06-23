// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/providers/elastic/aggregations"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
)

var ErrNoSubscriptionCustomisation = errors.New("no subscription customisation found")

func GetSubscriptionCustomisation(ctx context.Context, api SubscriptionsAPI, id SubscriptionID) (*SubscriptionCustomisation, *Response) {
	// Retrieve user object.
	user, found := UserFromCtx(ctx)
	if !found {
		return nil, RespErrUnauthorized()
	}
	// Make sure the user has a subscription with the given id.
	if !user.HasSubscription(id) {
		return nil, RespErrUnauthorized()
	}
	// Get customisation details.
	existingCustomisations, err := api.GetSubscriptionCustomisations(ctx, id)
	if err != nil {
		return nil, RespErrBackend(err)
	}
	// Return the customisation for the given id.
	for customisation := range slices.Values(existingCustomisations) {
		if customisation.GetID() == id {
			return customisation, nil
		}
	}
	// If no customisation, return a new customisation object.
	state := user.GetSubscriptionState(id)
	return &SubscriptionCustomisation{
		FeedID:         state.GetFeedID(),
		SubscriptionID: state.GetID(),
		UserID:         user.GetID(),
	}, nil
}

func GetAllSubscriptionCustomisations(ctx context.Context, api SubscriptionsAPI) (SubscriptionCustomisations, *Response) {
	user, found := UserFromCtx(ctx)
	if !found {
		return nil, RespErrUnauthorized()
	}
	states := user.GetAllSubscriptionStates()
	existingCustomisations, err := api.GetSubscriptionCustomisations(ctx, slices.Collect(maps.Keys(states))...)
	if err != nil {
		return nil, RespErrBackend(err)
	}

	allCustomistations := make(SubscriptionCustomisations, 0, len(states))
	for id, state := range states {
		if customisation := existingCustomisations.GetCustomisation(id); customisation != nil {
			allCustomistations = append(allCustomistations, customisation)
		} else {
			allCustomistations = append(allCustomistations, &SubscriptionCustomisation{
				FeedID:         state.GetFeedID(),
				SubscriptionID: state.GetID(),
				UserID:         user.GetID(),
			})
		}
	}
	return allCustomistations, nil
}

func GetSubscriptionUnreadCounts(ctx context.Context, api DocumentsAPI, states SubscriptionStates[FeedID]) (*aggregations.TermsAggregationResults, *Response) {
	// Retrieve user object.
	user, found := UserFromCtx(ctx)
	if !found {
		return nil, RespErrUnauthorized()
	}

	subscriptionQueries := make([]query.Option, 0, len(states))
	for _, state := range states {
		subscriptionQueries = append(subscriptionQueries, queryUnreadItems(user, state))
	}
	query := query.Bool(
		query.Filter(
			query.Bool(
				query.Should(subscriptionQueries...),
			),
		),
	)
	aggResults, resp := api.ItemsAggregation(ctx, query, aggregations.NewTermsAggregation("UnreadCounts", "feed_id", len(states)))
	if resp != nil {
		return nil, resp
	}
	var (
		categoryCounts aggregations.TermsAggregationResults
		err            error
	)
	categoryCounts.StringTermsAggregate, err = aggregations.ExtractAggregation[*types.StringTermsAggregate](aggResults.Aggregations, "UnreadCounts")
	if err != nil {
		return nil, NewResponse(http.StatusInternalServerError, fmt.Errorf("could not extract category counts: %w", err))
	}

	return &categoryCounts, nil
}

func GetSubscriptions(ctx context.Context, api DocumentsAPI, ids ...SubscriptionID) (Subscriptions, *Response) {
	// Retrieve user object.
	user, found := UserFromCtx(ctx)
	if !found {
		return nil, RespErrUnauthorized()
	}

	states := user.GetAllSubscriptionStatesByFeed()
	if len(ids) > 0 {
		states = FilterStatesByID(states, ids...)
	}

	if len(states) == 0 {
		return nil, &Response{StatusCode: 404}
	}

	var unreadCounts *aggregations.TermsAggregationResults
	var resp *Response
	// Get unread counts.
	unreadCounts, resp = GetSubscriptionUnreadCounts(ctx, api, states)
	if resp != nil {
		return nil, resp
	}
	// Get feed data for subscriptions.
	feeds, err := api.GetFeeds(ctx, slices.Collect(maps.Keys(states))...)
	if err != nil {
		return nil, &Response{StatusCode: 404}
	}
	// Get customisation data.
	customisations, resp := GetAllSubscriptionCustomisations(ctx, api)
	if resp != nil {
		return nil, resp
	}
	// Generate subscriptions from data sources.
	subscriptions := make(Subscriptions, 0, len(feeds))
	for feed := range slices.Values(feeds) {
		var state *SubscriptionState
		var count int
		var found bool
		if state, found = states[feed.GetID()]; !found {
			slogctx.FromCtx(ctx).Warn("No subscription state for retrieved feed.",
				slog.String("feed_id", feed.GetID()),
			)
			continue
		}
		if unreadCounts != nil {
			count = unreadCounts.GetCount(feed.GetID())
		}

		subscription, err := GenerateSubscription(
			user.GetID(),
			feed,
			customisations.GetCustomisation(state.GetID()),
			state,
			count,
		)
		if err != nil {
			slogctx.FromCtx(ctx).Warn("Could not generate subscription from data.",
				slog.Any("error", err),
			)
			continue
		}
		subscriptions = append(subscriptions, subscription)
	}
	return subscriptions, nil
}

func FilterSubscriptions(ctx context.Context, api DocumentsAPI, filters *Filters) (Subscriptions, Pagination, *Response) {
	subscriptions, resp := GetSubscriptions(ctx, api)
	if resp != nil {
		return nil, "", resp
	}

	subscriptions = subscriptions.FilterByCategories(filters.Categories...).
		FilterByView(filters.View).
		Sort(filters.Sort())

	var pagination string
	if filters.Pagination != nil {
		pagination = *filters.Pagination
	}
	subscriptions, pagination = subscriptions.Paginate(pagination, filters.CountAsInt())
	return subscriptions, pagination, nil
}

func Unsubscribe(ctx context.Context, api DocumentsAPI, ids ...SubscriptionID) *Response {
	if len(ids) == 0 {
		return nil
	}
	user, found := UserFromCtx(ctx)
	if !found {
		return RespErrUnauthorized()
	}

	// Remove any subscription customisations. This is non-critical.
	if err := api.DeleteSubscriptionCustomisations(ctx, ids...); err != nil {
		slogctx.FromCtx(ctx).Warn("Unable to delete user subscription customisations.",
			slog.Any("error", err),
		)
	}
	// Remove states for given subscriptions from user.
	states := user.GetAllSubscriptionStates()
	for id := range states {
		if slices.Contains(ids, id) {
			delete(states, id)
		}
	}
	// Update the user.
	return api.UpdateUser(ctx, map[string]any{
		"subscriptions": slices.Collect(maps.Values(states)),
	})
}

// func UpdateSubscriptionCustomisation(ctx context.Context, api DocumentsAPI, edits *SubscriptionEdit) error {
// 	// Retrieve user object.
// 	user, found := UserFromCtx(ctx)
// 	if !found {
// 		return ErrInvalidID
// 	}
// 	index := elastic.SubscriptionsIndexFromCtx(ctx)
// 	if index == "" {
// 		return ErrFetchCtx
// 	}

// 	found, err := NewDocExistsRequest(e.GetAPI(), index, edits.SubscriptionID).Do(ctx)
// 	if err != nil {
// 		return fmt.Errorf("failed to update subscription: %w", err)
// 	}
// 	if !found {
// 		state := user.GetSubscriptionState(edits.SubscriptionID)
// 		customisation := &SubscriptionCustomisation{
// 			SubscriptionID: edits.SubscriptionID,
// 			FeedID:         state.GetFeedID(),
// 			UserID:         user.GetID(),
// 			Title:          edits.Title,
// 			Categories:     edits.Categories,
// 		}
// 		_, err := NewDocCreateRequest(e.GetAPI(), index, edits.SubscriptionID, customisation, refresh.True).Do(ctx)
// 		if err != nil {
// 			return fmt.Errorf("failed to update subscription: %w", err)
// 		}
// 		return nil
// 	}

// 	updates := map[string]any{
// 		"title":      edits.Title,
// 		"categories": edits.Categories,
// 	}

// 	if err := UpdateDoc(ctx, e.GetAPI(), index, edits.SubscriptionID, updates); err != nil {
// 		return &Response{StatusCode: http.StatusInternalServerError, InternalError: err}
// 	}

// 	return nil
// }

// BuildSubscriptionQueries generates a slices of queries for the given subscriptions, based on the given filters.
func BuildSubscriptionQueries(user *User, view View, states ...*SubscriptionState) []query.Option {
	queries := make([]query.Option, 0, len(user.Subscriptions))
	// Work out what query to use based on the state filter.
	if len(states) == 0 {
		states = slices.Collect(maps.Values(user.GetAllSubscriptionStates()))
	}
	switch view {
	case ViewRead:
		for _, state := range states {
			queries = append(queries, queryReadItems(user, state))
		}
	case ViewAll:
		for _, state := range states {
			queries = append(queries, queryAllItems(user, state))
		}
	case ViewUnread:
		fallthrough
	default:
		for _, state := range states {
			queries = append(queries, queryUnreadItems(user, state))
		}
	}
	return queries
}

// queryReadItems generates a query for finding read items for the given subscription.
func queryReadItems(user *User, subscription *SubscriptionState) query.Option {
	maxHistory := user.GetMaxHistory()

	switch {
	case !subscription.IsRead():
		return query.Bool(
			query.BoolQueryName(subscription.GetFeedID()+"_read_items"),
			query.Filter(
				// Must match this feed.
				query.Term("feed_id", subscription.GetFeedID()),
				// And be published/updated since the user max history.
				query.Bool(
					query.Should(
						// query.Since("published", maxHistory),
						// query.Since("updated", maxHistory),
						query.Terms("item_id", subscription.GetReadItems()...),
					),
					// Must not match any unread items for the feed
					query.MustNot(
						query.Terms("item_id", subscription.GetUnreadItems()...),
					),
				),
			),
		)
	default:
		return query.Bool(
			query.BoolQueryName(subscription.GetFeedID()+"_read_items"),
			query.Filter(
				// Must match this feed.
				query.Term("feed_id", subscription.GetFeedID()),
				// And should be between the user max history and last read time.
				query.Bool(
					query.Should(
						query.Between("published", maxHistory, subscription.GetMarkedRead()),
						query.Between("updated", maxHistory, subscription.GetMarkedRead()),
						query.Terms("item_id", subscription.GetReadItems()...),
					),
					// Must not match any unread items for the feed
					query.MustNot(
						query.Terms("item_id", subscription.GetUnreadItems()...),
					),
				),
			),
		)
	}
}

// queryUnreadItems generates a query for finding unread items for the given subscription.
func queryUnreadItems(user *User, subscription *SubscriptionState) query.Option {
	var since time.Time
	if subscription.IsRead() {
		// Match the item if it is published/updated since last time subscription was marked read.
		since = subscription.GetMarkedRead()
	} else {
		// Match the item if it is published/updated since the max user history window.
		since = user.GetMaxHistory()
	}

	return query.Bool(
		query.BoolQueryName(subscription.GetFeedID()+"_unread_items"),
		query.Filter(
			// Must match this feed.
			query.Term("feed_id", subscription.GetFeedID()),
			query.Bool(
				query.Should(
					query.Since("published", since),
					query.Since("updated", since),
					query.Terms("item_id", subscription.GetUnreadItems()...),
				),
				// Must not match any read items for the feed
				query.MustNot(
					query.Terms("item_id", subscription.GetReadItems()...),
				),
			),
		),
	)
}

// subscriptionQueryReadItems generates a query for finding all items for the given subscription.
func queryAllItems(user *User, subscription *SubscriptionState) query.Option {
	maxHistory := user.GetMaxHistory()
	return query.Bool(
		query.BoolQueryName(subscription.GetFeedID()+"_all_items"),
		query.Filter(
			// Must match this feed.
			query.Term("feed_id", subscription.GetFeedID()),
			// And be published/updated since the user max history.
			query.Bool(
				query.Should(
					query.Since("published", maxHistory),
					query.Since("updated", maxHistory),
				),
			),
		),
	)
}

func querySuggestSubscriptions(text string) *types.Query {
	return query.Build(
		query.Bool(
			query.Should(
				query.SearchAsYouType(text, "title"),
				query.SearchAsYouType(text, "description"),
			),
		),
	)
	// subscriptionSearch := &query.MSearchOptions{
	// 	Query: query.Build(
	// 		query.Bool(
	// 			query.Filter(
	// 				query.Term("user_id", user.GetID()),
	// 			),
	// 			query.Must(
	// 				query.Match("title", searchTerms),
	// 				query.Match("description", searchTerms),
	// 				query.Match("categories", searchTerms),
	// 			),
	// 		),
	// 	),
	// 	Sort: []types.SortCombinationsVariant{elastic.SortByScore(), elastic.NewFieldSort("published", models.SortOrderDesc)},
	// }
}

// EncodePagination will take sort values returned from a query, marshal them to
// JSON, then HTML-escape the string into a models.Pagination object, which is
// safe for use in API query parameters.
func EncodePagination(sortValues []types.FieldValue) (Pagination, error) {
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

// DecodePagination will take a models.Pagination object, HTML-unescape the
// string then unmarshal it back into sort values.
func DecodePagination(pagination Pagination) ([]types.FieldValue, error) {
	if pagination == "" {
		return nil, nil
	}
	// Unescape HTML encoded data.
	data, err := url.QueryUnescape(pagination)
	if err != nil {
		return nil, fmt.Errorf("could not decode pagination values: %w", err)
	}
	// Unmarshal sort values.
	var sortValues []types.FieldValue
	err = json.Unmarshal([]byte(data), &sortValues)
	if err != nil {
		return nil, fmt.Errorf("could not decode pagination values: %w", err)
	}
	// Return sort values.
	return sortValues, nil
}
