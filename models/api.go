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
	"github.com/joshuar/go-feed-me/providers/elastic/results"
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
		subscriptionQueries = append(subscriptionQueries, QueryUnreadItems(user, state))
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

func FilterSubscriptions(ctx context.Context, api DocumentsAPI, filters *SubscriptionFilters) (Subscriptions, Pagination, *Response) {
	subscriptions, resp := GetSubscriptions(ctx, api)
	if resp != nil {
		return nil, "", resp
	}
	sort := filters.GetSort()

	subscriptions = subscriptions.FilterByCategories(filters.Categories...).
		FilterByView(filters.View).
		Sort(&sort)

	var pagination string
	if filters.Pagination != nil {
		pagination = *filters.Pagination
	}
	subscriptions, pagination = subscriptions.Paginate(pagination, filters.GetCount())
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

func GetArticles(ctx context.Context, api DocumentsAPI, itemIDs ...ItemID) (Articles, *Response) {
	user, found := UserFromCtx(ctx)
	if !found {
		return nil, RespErrUnauthorized()
	}

	// Search through items matching any given feeds filters, excluding any read
	// items.
	query := query.Bool(
		query.Filter(
			// Must match any of the given item IDs,
			query.Terms("item_id", itemIDs...),
		),
	)

	items, _, err := api.SearchItems(ctx, query, len(itemIDs), nil, nil)
	if err != nil {
		return nil, RespErrBackend(err)
	}

	// Retrieve subscription customisations for feed subscriptions.
	states := user.FilterSubscriptionStatesByFeed(items.GetFeedIDs()...)
	customisations, err := api.GetSubscriptionCustomisations(ctx, GetIDsFromStates(states)...)
	if err != nil {
		return nil, RespErrBackend(err)
	}
	// Create articles from the items.
	articles := make(Articles, 0, len(items))
	for item := range slices.Values(items) {
		state := states[item.GetFeedID()]
		customisation := customisations.GetCustomisation(state.GetID())
		article, err := GenerateArticle(item, state.GetItemState(item.GetID()), state.GetID(), customisation)
		if err != nil {
			slogctx.FromCtx(ctx).Warn("Could not generate article from data.",
				slog.Any("error", err),
			)
			continue
		}
		articles = append(articles, article)

	}

	return articles, nil
}

func GetSearchSuggestions(ctx context.Context, api DocumentsAPI, searchTerms string) (Subscriptions, Articles, *Response) {
	// Retrieve user object.
	user, found := UserFromCtx(ctx)
	if !found {
		return nil, nil, RespErrUnauthorized()
	}

	states := user.GetAllSubscriptionStatesByFeed()

	msearchResults, err := api.FindSuggestions(ctx, searchTerms)
	if err != nil {
		return nil, nil, RespErrBackend(err)
	}

	customisations, _ := results.GetHits[*SubscriptionCustomisation]("customisations", msearchResults)
	feeds, _ := results.GetHits[*Feed]("feeds", msearchResults)
	items, _ := results.GetHits[*Item]("items", msearchResults)

	// Generate subscriptions from data sources.
	subscriptions := make(Subscriptions, 0, len(feeds))
	maxSubscriptionResults := 10

	// Make subscriptions from customisation results, up to the maxObjectResults.
	for idx, customisation := range customisations {
		var feed *Feed
		if fidx := slices.IndexFunc(feeds, func(f *Feed) bool {
			// Feed already fetched in msearch results, use it.
			return f.GetID() == customisation.GetFeedID()
		}); fidx != -1 {
			feed = feeds[fidx]
		} else {
			// Get feed details.
			f, err := api.GetFeeds(ctx, customisation.GetFeedID())
			if err != nil {
				continue
			}
			feed = f[0]
		}
		subscription, err := GenerateSubscription(
			user.GetID(),
			feed,
			customisation,
			states[customisation.GetFeedID()],
			0,
		)
		if err != nil {
			slogctx.FromCtx(ctx).Warn("Could not generate subscription from data.",
				slog.Any("error", err),
			)
			continue
		}
		if idx == maxSubscriptionResults {
			break
		}
		subscriptions = append(subscriptions, subscription)
	}
	// Make subscriptions from the feed results up to maxObjectResults - customisationResults.
	for idx, feed := range feeds {
		var state *SubscriptionState
		var found bool
		if state, found = states[feed.GetID()]; !found {
			slogctx.FromCtx(ctx).Warn("No subscription state for retrieved feed.",
				slog.String("feed_id", feed.GetID()),
			)
			continue
		}
		subscription, err := GenerateSubscription(
			user.GetID(),
			feed,
			nil,
			state,
			0,
		)
		if err != nil {
			slogctx.FromCtx(ctx).Warn("Could not generate subscription from data.",
				slog.Any("error", err),
			)
			continue
		}
		if idx == (maxSubscriptionResults - len(customisations)) {
			break
		}
		subscriptions = append(subscriptions, subscription)
	}

	articles := make(Articles, 0, len(items))
	for item := range slices.Values(items) {
		var state *SubscriptionState
		var found bool
		if state, found = states[item.GetFeedID()]; !found {
			slogctx.FromCtx(ctx).Warn("No subscription state for retrieved item.",
				slog.String("item_id", item.GetID()),
			)
			continue
		}
		var customisation *SubscriptionCustomisation
		if cidx := slices.IndexFunc(customisations, func(c *SubscriptionCustomisation) bool {
			// Feed already fetched in msearch results, use it.
			return c.GetFeedID() == item.GetFeedID()
		}); cidx != -1 {
			customisation = customisations[cidx]
		} else {
			// Get customisation details.
			c, err := api.GetSubscriptionCustomisations(ctx, state.GetID())
			if err != nil {
				continue
			}
			if len(c) > 0 {
				customisation = c[0]
			}
		}

		article, err := GenerateArticle(item, state.GetItemState(item.GetID()), state.GetID(), customisation)
		if err != nil {
			slogctx.FromCtx(ctx).Warn("Could not generate article from data.",
				slog.Any("error", err),
			)
			continue
		}
		articles = append(articles, article)
	}

	return subscriptions, articles, nil
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
			queries = append(queries, QueryUnreadItems(user, state))
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

// QueryUnreadItems generates a query for finding unread items for the given subscription.
func QueryUnreadItems(user *User, subscription *SubscriptionState) query.Option {
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

func NewRoute(path string, filters Filters) Route {
	route := &Route{
		Path:    path,
		Filters: &Route_Filters{},
	}
	switch value := filters.(type) {
	case *SubscriptionFilters:
		err := route.Filters.FromSubscriptionFilters(*value)
		if err != nil {
			route.Filters.FromSubscriptionFilters(NewSubscriptionFilters())
		}
	case *ArticleFilters:
		err := route.Filters.FromArticleFilters(*value)
		if err != nil {
			route.Filters.FromArticleFilters(NewArticleFilters())
		}
	}
	return *route
}

// Clone creates a new value of the Route. This includes making a copy of the filters. The clone can then be manipulated
// independently from the original.
func (r Route) Clone() Route {
	filtersCopy := *r.Filters
	return Route{
		Path:    r.Path,
		Filters: &filtersCopy,
	}
}

func (r Route) String() string {
	if r.Filters == nil {
		return r.Path
	}
	switch r.Path {
	case "/subscriptions":
		filters, err := r.Filters.AsSubscriptionFilters()
		if err != nil {
			filters = NewSubscriptionFilters()
		}
		return r.Path + "?" + filters.ToQueryParams().Encode()
	case "/articles":
		filters, err := r.Filters.AsArticleFilters()
		if err != nil {
			filters = NewArticleFilters()
		}
		return r.Path + "?" + filters.ToQueryParams().Encode()
	default:
		return r.Path
	}
}

func (r Route) IsSorted(sort Sort) bool {
	if r.Filters == nil {
		return false
	}
	switch r.Path {
	case "/subscriptions":
		filters, err := r.Filters.AsSubscriptionFilters()
		if err != nil {
			filters = NewSubscriptionFilters()
		}
		return filters.IsSorted(sort)
	case "/articles":
		filters, err := r.Filters.AsArticleFilters()
		if err != nil {
			filters = NewArticleFilters()
		}
		return filters.IsSorted(sort)
	default:
		return false
	}
}

func (r Route) SetSort(sort Sort) {
	if r.Filters == nil {
		return
	}
	switch r.Path {
	case "/subscriptions":
		filters, err := r.Filters.AsSubscriptionFilters()
		if err != nil {
			filters = NewSubscriptionFilters()
		}
		f := &filters
		f.SetSort(sort)
		err = r.Filters.FromSubscriptionFilters(*f)
		if err != nil {
			r.Filters.FromSubscriptionFilters(NewSubscriptionFilters())
		}
	case "/articles":
		filters, err := r.Filters.AsArticleFilters()
		if err != nil {
			filters = NewArticleFilters()
		}
		f := &filters
		f.SetSort(sort)
		err = r.Filters.FromArticleFilters(*f)
		if err != nil {
			r.Filters.FromArticleFilters(NewArticleFilters())
		}
	}
}

func (r Route) HasCategory(category Category) bool {
	if r.Filters == nil {
		return false
	}
	switch r.Path {
	case "/subscriptions":
		filters, err := r.Filters.AsSubscriptionFilters()
		if err != nil {
			filters = NewSubscriptionFilters()
		}
		return filters.HasCategory(category)
	case "/articles":
		filters, err := r.Filters.AsArticleFilters()
		if err != nil {
			filters = NewArticleFilters()
		}
		return filters.HasCategory(category)
	default:
		return false
	}
}

// func (r Route) AddCategory(category Category) {
// 	if r.Filters == nil {
// 		return
// 	}
// 	switch r.Path {
// 	case "/subscriptions":
// 		filters, err := r.Filters.AsSubscriptionFilters()
// 		if err != nil {
// 			filters = NewSubscriptionFilters()
// 		}
// 		f := &filters
// 		f.AddCategory(category)
// 		err = r.Filters.FromSubscriptionFilters(*f)
// 		if err != nil {
// 			r.Filters.FromSubscriptionFilters(NewSubscriptionFilters())
// 		}
// 	case "/articles":
// 		filters, err := r.Filters.AsArticleFilters()
// 		if err != nil {
// 			filters = NewArticleFilters()
// 		}
// 		f := &filters
// 		f.AddCategory(category)
// 		err = r.Filters.FromArticleFilters(*f)
// 		if err != nil {
// 			r.Filters.FromArticleFilters(NewArticleFilters())
// 		}
// 	}
// }

// func (r Route) RemoveCategory(category Category) {
// 	if r.Filters == nil {
// 		return
// 	}
// 	switch r.Path {
// 	case "/subscriptions":
// 		filters, err := r.Filters.AsSubscriptionFilters()
// 		if err != nil {
// 			filters = NewSubscriptionFilters()
// 		}
// 		f := &filters
// 		f.RemoveCategory(category)
// 		err = r.Filters.FromSubscriptionFilters(*f)
// 		if err != nil {
// 			r.Filters.FromSubscriptionFilters(NewSubscriptionFilters())
// 		}
// 	case "/articles":
// 		filters, err := r.Filters.AsArticleFilters()
// 		if err != nil {
// 			filters = NewArticleFilters()
// 		}
// 		f := &filters
// 		f.RemoveCategory(category)
// 		err = r.Filters.FromArticleFilters(*f)
// 		if err != nil {
// 			r.Filters.FromArticleFilters(NewArticleFilters())
// 		}
// 	}
// }

func (r Route) IsView(view View) bool {
	if r.Filters == nil {
		return false
	}
	switch r.Path {
	case "/subscriptions":
		filters, err := r.Filters.AsSubscriptionFilters()
		if err != nil {
			filters = NewSubscriptionFilters()
		}
		return filters.IsView(view)
	case "/articles":
		filters, err := r.Filters.AsArticleFilters()
		if err != nil {
			filters = NewArticleFilters()
		}
		return filters.IsView(view)
	default:
		return false
	}
}

func (r Route) SetView(view View) {
	if r.Filters == nil {
		return
	}
	switch r.Path {
	case "/subscriptions":
		filters, err := r.Filters.AsSubscriptionFilters()
		if err != nil {
			filters = NewSubscriptionFilters()
		}
		f := &filters
		f.SetView(view)
		err = r.Filters.FromSubscriptionFilters(*f)
		if err != nil {
			r.Filters.FromSubscriptionFilters(NewSubscriptionFilters())
		}
	case "/articles":
		filters, err := r.Filters.AsArticleFilters()
		if err != nil {
			filters = NewArticleFilters()
		}
		f := &filters
		f.SetView(view)
		err = r.Filters.FromArticleFilters(*f)
		if err != nil {
			r.Filters.FromArticleFilters(NewArticleFilters())
		}
	}
}

func (r Route) SetPagination(pagination Pagination) {
	if r.Filters == nil {
		return
	}
	switch r.Path {
	case "/subscriptions":
		filters, err := r.Filters.AsSubscriptionFilters()
		if err != nil {
			filters = NewSubscriptionFilters()
		}
		f := &filters
		f.SetPagination(pagination)
		err = r.Filters.FromSubscriptionFilters(*f)
		if err != nil {
			r.Filters.FromSubscriptionFilters(NewSubscriptionFilters())
		}
	case "/articles":
		filters, err := r.Filters.AsArticleFilters()
		if err != nil {
			filters = NewArticleFilters()
		}
		f := &filters
		f.SetPagination(pagination)
		err = r.Filters.FromArticleFilters(*f)
		if err != nil {
			r.Filters.FromArticleFilters(NewArticleFilters())
		}
	}
}
