// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

var ErrNoSubscriptionCustomisation = errors.New("no subscription customisation found")

// func GetSubscriptionCustomisation(ctx context.Context, api SubscriptionsAPI, id SubscriptionID) (*SubscriptionCustomisation, *Response) {
// 	// Retrieve user object.
// 	user, found := UserFromCtx(ctx)
// 	if !found {
// 		return nil, RespErrUnauthorized()
// 	}
// 	// Make sure the user has a subscription with the given id.
// 	if !user.HasSubscription(id) {
// 		return nil, RespErrUnauthorized()
// 	}
// 	// Get customisation details.
// 	existingCustomisations, err := api.GetSubscriptionCustomisations(ctx, id)
// 	if err != nil {
// 		return nil, RespErrBackend(err)
// 	}
// 	// Return the customisation for the given id.
// 	for customisation := range slices.Values(existingCustomisations) {
// 		if customisation.GetID() == id {
// 			return customisation, nil
// 		}
// 	}
// 	// If no customisation, return a new customisation object.
// 	state := user.GetSubscriptionState(id)
// 	return &SubscriptionCustomisation{
// 		FeedID:         state.GetFeedID(),
// 		SubscriptionID: state.GetID(),
// 		UserID:         user.GetID(),
// 	}, nil
// }

// func GetAllSubscriptionCustomisations(ctx context.Context, api SubscriptionsAPI) (SubscriptionCustomisations, *Response) {
// 	user, found := UserFromCtx(ctx)
// 	if !found {
// 		return nil, RespErrUnauthorized()
// 	}
// 	states := user.GetAllSubscriptionStates()
// 	existingCustomisations, err := api.GetSubscriptionCustomisations(ctx, slices.Collect(maps.Keys(states))...)
// 	if err != nil {
// 		return nil, RespErrBackend(err)
// 	}

// 	allCustomistations := make(SubscriptionCustomisations, 0, len(states))
// 	for id, state := range states {
// 		if customisation := existingCustomisations.GetCustomisation(id); customisation != nil {
// 			allCustomistations = append(allCustomistations, customisation)
// 		} else {
// 			allCustomistations = append(allCustomistations, &SubscriptionCustomisation{
// 				FeedID:         state.GetFeedID(),
// 				SubscriptionID: state.GetID(),
// 				UserID:         user.GetID(),
// 			})
// 		}
// 	}
// 	return allCustomistations, nil
// }

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

// // BuildSubscriptionQueries generates a slices of queries for the given subscriptions, based on the given filters.
// func BuildSubscriptionQueries(user *User, view View, states ...*SubscriptionState) []query.Option {
// 	queries := make([]query.Option, 0, len(user.Subscriptions))
// 	// Work out what query to use based on the state filter.
// 	if len(states) == 0 {
// 		states = slices.Collect(maps.Values(user.GetAllSubscriptionStates()))
// 	}
// 	switch view {
// 	case ViewRead:
// 		for _, state := range states {
// 			queries = append(queries, queryReadItems(user, state))
// 		}
// 	case ViewAll:
// 		for _, state := range states {
// 			queries = append(queries, queryAllItems(user, state))
// 		}
// 	case ViewUnread:
// 		fallthrough
// 	default:
// 		for _, state := range states {
// 			queries = append(queries, QueryUnreadItems(user, state))
// 		}
// 	}
// 	return queries
// }

// // queryReadItems generates a query for finding read items for the given subscription.
// func queryReadItems(user *User, subscription *SubscriptionState) query.Option {
// 	maxHistory := user.GetMaxHistory()

// 	switch {
// 	case !subscription.IsRead():
// 		return query.Bool(
// 			query.BoolQueryName(subscription.GetFeedID()+"_read_items"),
// 			query.Filter(
// 				// Must match this feed.
// 				query.Term("feed_id", subscription.GetFeedID()),
// 				// And be published/updated since the user max history.
// 				query.Bool(
// 					query.Should(
// 						// query.Since("published", maxHistory),
// 						// query.Since("updated", maxHistory),
// 						query.Terms("item_id", subscription.GetReadItems()...),
// 					),
// 					// Must not match any unread items for the feed
// 					query.MustNot(
// 						query.Terms("item_id", subscription.GetUnreadItems()...),
// 					),
// 				),
// 			),
// 		)
// 	default:
// 		return query.Bool(
// 			query.BoolQueryName(subscription.GetFeedID()+"_read_items"),
// 			query.Filter(
// 				// Must match this feed.
// 				query.Term("feed_id", subscription.GetFeedID()),
// 				// And should be between the user max history and last read time.
// 				query.Bool(
// 					query.Should(
// 						query.Between("published", maxHistory, subscription.GetMarkedRead()),
// 						query.Between("updated", maxHistory, subscription.GetMarkedRead()),
// 						query.Terms("item_id", subscription.GetReadItems()...),
// 					),
// 					// Must not match any unread items for the feed
// 					query.MustNot(
// 						query.Terms("item_id", subscription.GetUnreadItems()...),
// 					),
// 				),
// 			),
// 		)
// 	}
// }

// // QueryUnreadItems generates a query for finding unread items for the given subscription.
// func QueryUnreadItems(user *User, subscription *SubscriptionState) query.Option {
// 	var since time.Time
// 	if subscription.IsRead() {
// 		// Match the item if it is published/updated since last time subscription was marked read.
// 		since = subscription.GetMarkedRead()
// 	} else {
// 		// Match the item if it is published/updated since the max user history window.
// 		since = user.GetMaxHistory()
// 	}

// 	return query.Bool(
// 		query.BoolQueryName(subscription.GetFeedID()+"_unread_items"),
// 		query.Filter(
// 			// Must match this feed.
// 			query.Term("feed_id", subscription.GetFeedID()),
// 			query.Bool(
// 				query.Should(
// 					query.Since("published", since),
// 					query.Since("updated", since),
// 					query.Terms("item_id", subscription.GetUnreadItems()...),
// 				),
// 				// Must not match any read items for the feed
// 				query.MustNot(
// 					query.Terms("item_id", subscription.GetReadItems()...),
// 				),
// 			),
// 		),
// 	)
// }

// // subscriptionQueryReadItems generates a query for finding all items for the given subscription.
// func queryAllItems(user *User, subscription *SubscriptionState) query.Option {
// 	maxHistory := user.GetMaxHistory()
// 	return query.Bool(
// 		query.BoolQueryName(subscription.GetFeedID()+"_all_items"),
// 		query.Filter(
// 			// Must match this feed.
// 			query.Term("feed_id", subscription.GetFeedID()),
// 			// And be published/updated since the user max history.
// 			query.Bool(
// 				query.Should(
// 					query.Since("published", maxHistory),
// 					query.Since("updated", maxHistory),
// 				),
// 			),
// 		),
// 	)
// }

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
