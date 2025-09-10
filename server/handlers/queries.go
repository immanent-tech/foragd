// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"log/slog"
	"strings"

	"github.com/immanent-tech/go-feed-me/models"
	"github.com/immanent-tech/go-feed-me/providers/elastic/query"
)

// BuildSubscriptionQueries generates a slices of queries for the given subscriptions, based on the given filters.
func buildSubscriptionQueries(user *models.User, view models.View, subscriptions ...*models.SubscriptionMetadata) []query.Option {
	queries := make([]query.Option, 0, len(user.Subscriptions))
	// Work out what query to use based on the state filter.
	if len(subscriptions) == 0 {
		return nil
	}
	switch view {
	case models.ViewRead:
		for _, state := range subscriptions {
			queries = append(queries, queryReadItems(user, state))
		}
	case models.ViewAll:
		for _, state := range subscriptions {
			queries = append(queries, queryAllItems(user, state))
		}
	case models.ViewUnread:
		fallthrough
	default:
		for _, state := range subscriptions {
			queries = append(queries, queryUnreadItems(user, state))
		}
	}
	return queries
}

// queryReadItems generates a query for finding read items for the given subscription.
func queryReadItems(user *models.User, subscription *models.SubscriptionMetadata) query.Option {
	slog.Debug("Adding subscription query for read items.",
		slog.String("subscription", subscription.Customisation.Nickname),
		slog.Time("since", subscription.MarkedReadAt),
		slog.String("excluding", strings.Join(subscription.GetUnreadItems(), ",")),
	)
	return query.Bool(
		query.BoolQueryName(subscription.GetFeedID()+"_read_items"),
		query.Filter(
			// Must match this feed.
			query.Term("feed_id", subscription.GetFeedID()),
			// And should be between the user max history and last read time.
			query.Bool(
				query.Should(
					query.Between("published", user.GetMaxHistory(), subscription.MarkedReadAt),
					query.Between("updated", user.GetMaxHistory(), subscription.MarkedReadAt),
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

// QueryUnreadItems generates a query for finding unread items for the given subscription.
func queryUnreadItems(user *models.User, subscription *models.SubscriptionMetadata) query.Option {
	slog.Debug("Adding subscription query for unread items.",
		slog.String("subscription", subscription.Customisation.Nickname),
		slog.Time("since", subscription.MarkedReadAt),
		slog.String("excluding", strings.Join(subscription.GetUnreadItems(), ",")),
	)
	return query.Bool(
		query.BoolQueryName(subscription.GetFeedID()+"_unread_items"),
		query.Filter(
			// Must match this feed.
			query.Term("feed_id", subscription.GetFeedID()),
			query.Bool(
				query.Should(
					query.Since("published", subscription.MarkedReadAt),
					query.Since("updated", subscription.MarkedReadAt),
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
func queryAllItems(user *models.User, subscription *models.SubscriptionMetadata) query.Option {
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
