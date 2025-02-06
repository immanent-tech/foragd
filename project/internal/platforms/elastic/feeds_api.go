// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/elastic/go-elasticsearch/v8/typedapi/core/mget"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/joshuar/go-feed-me/internal/models"
)

var defaultFeedFields = []string{
	"publishedParsed",
	"updatedParsed",
	"feed_id",
	"title",
	"description",
	"feedLink",
	"image",
	"categories",
	"authors",
}

var defaultItemFields = []string{
	"publishedParsed",
	"updatedParsed",
	"title",
	"description",
	"item_id",
	"image",
}

var (
	ErrNoFeedID      = errors.New("no feed ID provided")
	ErrExtractSource = errors.New("could not extract document _source")
	ErrAddFailed     = errors.New("adding items failed")
)

// GetNewFeedsSince retrieves a list of feeds that have been updated since the
// given time.
func (c *Client) GetNewFeedsSince(ctx context.Context, since time.Time) ([]models.APIFeed, error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrSearchFailed, ErrNoIndexInCtx)
	}

	resp, err := c.NewSearchRequest(
		WithIndex[*search.Search](index),
		WithSearchQueryOptions(QuerySince("@timestamp", since)),
	).Do(ctx)
	if err != nil {
		return nil, errors.Join(ErrSearchFailed, err)
	}

	feeds, warnings := ExtractSourceFromHits[models.APIFeed](resp.Hits.Hits)
	if warnings != nil {
		c.Logger.Warn("Problems occurred while extracting source from docs.",
			slog.Any("warnings", err))
	}

	return feeds, nil
}

func (c *Client) GetFeedByURL(ctx context.Context, url string) (*models.APIFeed, error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrSearchFailed, ErrNoIndexInCtx)
	}

	resp, err := c.NewSearchRequest(
		WithIndex[*search.Search](index),
		WithFields(defaultFeedFields...),
		WithSearchQueryOptions(QueryByTerm("feedLink", url)),
	).Do(ctx)
	if err != nil {
		return nil, errors.Join(ErrSearchFailed, err)
	}

	// If there are no hits, just return an empty APIFeed object.
	if resp.Hits.Total.Value == 0 {
		return nil, models.ErrNoFeed
	}

	feed, err := ExtractSource[*models.APIFeed](resp.Hits.Hits[0].Source_)
	if err != nil {
		return nil, errors.Join(ErrSearchFailed, err)
	}

	return feed, nil
}

// GetFeedsByURL retrieves a list of APIFeeds based on the given URLs.
func (c *Client) GetFeedsByURL(ctx context.Context, urls ...models.URL) ([]models.APIFeed, error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrSearchFailed, ErrNoIndexInCtx)
	}

	feeds := make([]models.APIFeed, 0, len(urls))

	resp, err := c.NewSearchRequest(
		WithIndex[*search.Search](index),
		WithFields("feed_id", "feedLink"),
		WithSearchQueryOptions(QueryByURLs("feedLink", urls...)),
		WithSearchSize(len(urls)),
	).Do(ctx)
	if err != nil {
		return nil, errors.Join(ErrSearchFailed, err)
	}
	// Stop if there are no hits
	if len(resp.Hits.Hits) == 0 {
		return nil, nil
	}
	// Loop through this set of results.
	sources, warnings := ExtractSourceFromHits[models.APIFeed](resp.Hits.Hits)
	if warnings != nil {
		c.Logger.Warn("Problems occurred while extracting source from docs.",
			slog.Any("warnings", err))
	}

	feeds = append(feeds, sources...)

	return feeds, nil
}

// GetFeedsByID retrieves a list of APIFeeds by their FeedID.
func (c *Client) GetFeedsByID(ctx context.Context, feedIDs ...models.FeedID) ([]*models.APIFeed, error) {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrSearchFailed, ErrNoIndexInCtx)
	}

	// Get the feed details.
	req := c.NewMGetRequest(
		WithIndex[*mget.Mget](index),
		WithIDs[*mget.Mget](feedIDs...),
	)

	res, err := req.Do(ctx)
	if err != nil {
		return nil, errors.Join(ErrSearchFailed, err)
	}

	var feeds []*models.APIFeed

	for _, doc := range res.Docs {
		switch obj := doc.(type) {
		case types.MultiGetError:
			c.Logger.Warn("Problem getting document", slog.Any("error", obj))
		case *types.GetResult:
			feed, err := ExtractSource[*models.APIFeed](obj.Source_)
			if err != nil {
				c.Logger.Warn("Could not unmarshal item source.", slog.Any("error", err))
				continue
			}

			feeds = append(feeds, feed)
		}
	}

	return feeds, nil
}

func (c *Client) AddFeeds(ctx context.Context, feeds ...models.Feed) error {
	index := FeedsIndexFromCtx(ctx)
	if index == "" {
		return errors.Join(ErrAddFailed, ErrNoIndexInCtx)
	}

	docs := make([]BulkOperation, len(feeds))

	for iter, feed := range feeds {
		feed.Items = nil // don't index items in feed.
		c.Logger.Debug("Adding feed",
			slog.String("name", feed.Title),
			slog.String("item_id", feed.ID),
		)

		docs[iter] = NewBulkOperation(&feed,
			WithDocID[BulkOperation](feed.ID),
			ToIndex(index),
		)
	}

	c.bulkStream <- docs

	return nil
}

// AddItems will bulk index the given items to the Elasticsearch cache.
func (c *Client) AddItems(ctx context.Context, items ...models.Item) error {
	index := ItemsIndexFromCtx(ctx)
	if index == "" {
		return errors.Join(ErrAddFailed, ErrNoIndexInCtx)
	}

	docs := make([]BulkOperation, len(items))

	for iter, item := range items {
		c.Logger.Debug("Adding item",
			slog.String("name", item.Title),
			slog.String("item_id", item.ID),
			slog.String("feed_id", item.FeedID),
		)

		docs[iter] = NewBulkOperation(&item,
			WithDocID[BulkOperation](item.ID),
			ToIndex(index),
		)
	}

	c.bulkStream <- docs

	return nil
}

// GetItemCounts runs an aggregation to count the items totals for the given
// state for the given feeds.
//
//nolint:lll
func (c *Client) GetFeedItemCounts(ctx context.Context, user *models.User, view models.View, feedIDs []models.FeedID) (map[string]int64, error) {
	index := ItemsIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrSearchFailed, ErrNoIndexInCtx)
	}

	var query Option[*types.Query]

	switch view {
	case models.ViewRead:
		query = readFeedItemsQuery(user, feedIDs...)
	case models.ViewUnread:
		fallthrough
	default:
		query = unreadFeedItemsQuery(user, feedIDs...)
	}

	// Search through items matching any given feeds filters, excluding any read
	// items.
	req := c.NewSearchRequest(
		WithIndex[*search.Search](index),
		WithSearchQueryOptions(
			query,
		),
		WithSortOptions(SortTimestampDesc()),
		WithSearchSize(0),
		WithAggregations(TermsAggregation("UnreadCounts", "feed_id")),
	)

	resp, err := req.Do(ctx)
	if err != nil {
		return nil, errors.Join(ErrUserActionFailed, err)
	}

	var (
		results TermsAggregationResults
		ok      bool
	)

	results.StringTermsAggregate, ok = resp.Aggregations["UnreadCounts"].(*types.StringTermsAggregate)
	if !ok {
		return nil, errors.Join(ErrUserActionFailed, fmt.Errorf("not TermsAggregationResults"))
	}

	unreadCounts := make(map[string]int64)
	for _, feedID := range feedIDs {
		unreadCounts[feedID] = results.GetCount(feedID)
	}

	return unreadCounts, nil
}

// unreadFeedItemsQuery generates a query for matching unread items for the
// given feeds.
func unreadFeedItemsQuery(user *models.User, feedIDs ...models.FeedID) Option[*types.Query] {
	clauses := make([]Option[*types.Query], 0, len(feedIDs))
	for _, id := range feedIDs {
		clauses = append(clauses,
			QueryBool(
				BoolFilter(
					// Must match this feed.
					QueryByTerm("feed_id", id),
					// And should be newer than last read or explicitly marked unread.
					QueryBool(
						BoolShould(
							QuerySince("publishedParsed", user.GetFeedLastRead(id)),
							QuerySince("updatedParsed", user.GetFeedLastRead(id)),
							QueryByItemIDs(user.GetItemIDsWithState(models.StateUnread, id)...),
						),
					),
				),
			),
		)
	}

	return QueryBool(
		BoolFilter(
			// Must match any of the given feed IDs.
			QueryByFeedIDs(feedIDs...),
			// And should match one feed clause.
			QueryBool(
				BoolShould(clauses...),
			),
		),
		BoolMustNot(
			// Must not match any read item IDs.
			QueryByItemIDs(user.GetItemIDsWithState(models.StateRead, feedIDs...)...),
		),
	)
}

// readFeedItemsQuery generates a query for matching read items for the given feeds.
func readFeedItemsQuery(user *models.User, feedIDs ...models.FeedID) Option[*types.Query] {
	clauses := make([]Option[*types.Query], 0, len(feedIDs))
	for _, id := range feedIDs {
		clauses = append(clauses,
			QueryBool(
				BoolFilter(
					// Must match this feed.
					QueryByTerm("feed_id", id),
					// And should be newer than the max history for the user.
					QueryBool(
						BoolShould(
							QuerySince("publishedParsed", user.GetMaxHistory()),
							QuerySince("updatedParsed", user.GetMaxHistory()),
						),
					),
				),
			),
		)
	}

	return QueryBool(
		BoolFilter(
			// Must match any of the given feed IDs
			QueryByFeedIDs(feedIDs...),
			// And should match one feed clause.
			QueryBool(
				BoolShould(clauses...),
			),
		),
	)
}
