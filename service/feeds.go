/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package service

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/maypok86/otter/v2"
	slogctx "github.com/veqryn/slog-context"
	"github.com/zeebo/xxh3"
	"golang.org/x/net/html"

	feeds "github.com/immanent-tech/go-syndication"
	"github.com/immanent-tech/go-syndication/atom"
	"github.com/immanent-tech/go-syndication/jsonfeed"
	"github.com/immanent-tech/go-syndication/opml"
	"github.com/immanent-tech/go-syndication/rdf"
	"github.com/immanent-tech/go-syndication/rss"
	"github.com/immanent-tech/go-syndication/types"

	"github.com/immanent-tech/go-base/pkg/comparisons"
	"github.com/immanent-tech/go-base/pkg/htmlx"
	"github.com/immanent-tech/go-base/pkg/textx"

	"github.com/immanent-tech/go-base/pkg/zyte"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/google/news"
	"github.com/immanent-tech/foragd/providers/google/youtube"
	"github.com/immanent-tech/foragd/providers/ollama"
	"github.com/immanent-tech/foragd/providers/opensearch"
	"github.com/immanent-tech/foragd/providers/opensearch/bulk"
	"github.com/immanent-tech/foragd/providers/opensearch/doc"
	"github.com/immanent-tech/foragd/providers/opensearch/search"
	"github.com/immanent-tech/foragd/schema"
)

type Classifier interface {
	Classify(ctx context.Context,
		content, contentURL string,
		categories []ollama.CategoryEmbedding,
		domainPriors *ollama.DomainCategoryPrior,
		alpha float64,
	) ([]ollama.Category, error)
	Close() error
}

// feedCacheExpiryCalculator is a custom expiry calculator for the feed cache.
type feedCacheExpiryCalculator struct{}

// ExpireAfterCreate sets expiration time for new entries.
func (ec *feedCacheExpiryCalculator) ExpireAfterCreate(entry otter.Entry[models.FeedID, *models.Feed]) time.Duration {
	return entry.Value.GetUpdateInterval()
}

// ExpireAfterUpdate sets expiration time after updates.
func (ec *feedCacheExpiryCalculator) ExpireAfterUpdate(
	entry otter.Entry[models.FeedID, *models.Feed],
	newVal *models.Feed,
) time.Duration {
	// Use the new feed's update interval for the TTL.
	return newVal.GetUpdateInterval()
}

// ExpireAfterRead returns remaining expiration time for reads.
func (ec *feedCacheExpiryCalculator) ExpireAfterRead(entry otter.Entry[models.FeedID, *models.Feed]) time.Duration {
	return entry.ExpiresAfter()
}

// FeedService holds a cache and backend connection for handling [models.Feed] objects.
type FeedService struct {
	*otter.Cache[models.FeedID, *models.Feed]

	osc     *opensearch.Client
	indexer *bulk.Indexer

	itemSvc    *ItemService
	httpClient *resty.Client
	ollama     Classifier
	logger     *LogIndexer

	loader     otter.LoaderFunc[models.FeedID, *models.Feed]
	bulkLoader otter.BulkLoaderFunc[models.FeedID, *models.Feed]

	closer closeOnce
}

func LoadFeedsService(
	ctx context.Context,
	osc *opensearch.Client,
	indexer *bulk.Indexer,
	httpClient *resty.Client,
	classifier Classifier,
	itemSvc *ItemService,
) (*FeedService, error) {
	if osc == nil {
		return nil, errors.New("no opensearch client")
	}
	if httpClient == nil {
		return nil, errors.New("no http client")
	}
	if itemSvc == nil {
		return nil, errors.New("no item service")
	}
	return &FeedService{
		Cache: otter.Must(&otter.Options[models.FeedID, *models.Feed]{
			MaximumSize:      30_000,
			ExpiryCalculator: &feedCacheExpiryCalculator{},
		}),
		osc:     osc,
		indexer: indexer,
		logger:  NewLogIndexer(indexer),
		loader: otter.LoaderFunc[models.FeedID, *models.Feed](
			func(ctx context.Context, id models.FeedID) (*models.Feed, error) {
				feed, err := doc.Get[models.FeedID, *models.Feed](
					ctx,
					osc.Doc,
					osc.IndexReadAlias(schema.FeedsIndex),
					id,
				)
				switch {
				case err == nil:
					return feed, nil
				case isNotFound(err):
					return nil, fmt.Errorf("get doc: %w: %w", otter.ErrNotFound, err)
				default:
					return nil, fmt.Errorf("get doc: %w", err)
				}
			},
		),
		bulkLoader: otter.BulkLoaderFunc[models.FeedID, *models.Feed](
			func(ctx context.Context, keys []models.FeedID) (map[models.FeedID]*models.Feed, error) {
				fetched, err := doc.GetDocs[models.FeedID, *models.Feed](
					ctx,
					osc.Doc,
					osc.IndexReadAlias(schema.FeedsIndex),
					keys...,
				)
				if err != nil {
					return nil, fmt.Errorf("mget feeds: %w", err)
				}
				feeds := make(map[models.FeedID]*models.Feed, len(fetched))
				for feed := range slices.Values(fetched) {
					feeds[feed.GetID()] = feed
				}
				return feeds, nil
			},
		),
		httpClient: httpClient,
		ollama:     classifier,
		itemSvc:    itemSvc,
	}, nil
}

func (s *FeedService) Close() error {
	// The shutdown context must not derive from the (already cancelled) signal context.
	shutdownCtx, cancel := context.WithTimeoutCause(
		context.Background(),
		20*time.Second,
		errors.New("graceful shutdown timeout"),
	)
	shutdownCtx = slogctx.NewCtx(shutdownCtx, slog.Default())
	defer cancel()

	slogctx.Info(shutdownCtx, "Shutting down feeds service")

	return s.closer.do(func() error {
		s.Cache.CleanUp()
		return nil
	})
}

// GetFeed retrieves a feed with the given FeedID.
func (s *FeedService) GetFeed(ctx context.Context, id models.FeedID) (*models.Feed, error) {
	switch feed, err := s.Get(ctx, id, s.loader); {
	case err == nil:
		return feed, nil
	case errors.Is(err, otter.ErrNotFound):
		return nil, models.ErrNotFound
	default:
		return nil, fmt.Errorf("get feed: %w", err)
	}
}

// GetFeeds retrieves the Feeds matching the given FeedIDs. It will fetch any cached versions before fetching from
// Elasticsearch (and then caching those).
func (s *FeedService) GetFeeds(ctx context.Context, ids ...models.FeedID) (models.Feeds, error) {
	feeds, err := s.BulkGet(ctx, ids, s.bulkLoader)
	if err != nil {
		return nil, fmt.Errorf("bulk get feeds: %w", err)
	}
	if len(feeds) > len(ids) {
		slogctx.Warn(ctx, "Fetched feeds is less than requested feeds.")
	}
	// Otter's bulk loader can return values that are nil. Filter those out.
	cleaned := make(map[models.FeedID]*models.Feed, len(feeds))
	for k, v := range feeds {
		if v != nil {
			cleaned[k] = v
		}
	}
	return slices.Collect(maps.Values(cleaned)), nil
}

// GetAllFeedsExcept retrieves a [models.Feeds] slice containing all feeds except those with the given [models.FeedID].
func (s *FeedService) GetAllFeedsExcept(ctx context.Context, ids ...models.FeedID) (models.Feeds, error) {
	feeds, err := s.searchAllFeeds(ctx, search.BoolQuery(
		search.WithBoolMustNot(
			search.TermsQuery("feed_id", ids),
		),
	))
	if err != nil {
		return nil, fmt.Errorf("search all: %w", err)
	}
	return feeds, nil
}

// GetUnclassifiedFeeds returns a [models.Feeds] slice containing feeds with any categories (or "Uncategorized").
func (s *FeedService) GetUnclassifiedFeeds(ctx context.Context) (models.Feeds, error) {
	feeds, err := s.searchAllFeeds(ctx,
		search.BoolQuery(
			search.WithBoolFilter(
				search.TermQuery("categories.raw", "Uncategorized"),
			),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("search all: %w", err)
	}

	return feeds, nil
}

// GetNewFeedsSince retrieves a [models.Feeds] slice containing all feeds created since the given timestamp.
func (s *FeedService) GetNewFeedsSince(ctx context.Context, ts time.Time) (models.Feeds, error) {
	feeds, err := s.searchAllFeeds(ctx,
		search.BoolQuery(
			search.WithBoolShould(
				search.BeforeQuery("last_fetched", ts),
				search.BoolQuery(
					search.WithBoolMustNot(
						search.ExistsQuery("last_fetched"),
					),
				),
			),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("search all: %w", err)
	}

	return feeds, nil
}

// GetFeedsByURLs retrieves a [models.Feeds] slice containing all feeds that have a source matching one of the given URLs.
func (s *FeedService) GetFeedsByURLs(ctx context.Context, urls ...string) (models.Feeds, error) {
	var terms []search.QueryOption
	for url := range slices.Values(urls) {
		terms = append(terms, search.TermQuery("source_urls", url))
		// Also match url with trailing slash.
		if !strings.HasSuffix(url, "/") {
			terms = append(terms, search.TermQuery("source_urls", url+"/"))
		}
	}
	// Find any existing feed.
	feeds, err := s.searchAllFeeds(ctx,
		search.BoolQuery(
			search.WithBoolFilter(
				search.BoolQuery(
					search.WithBoolShould(terms...),
				),
			),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("search all: %w", err)
	}
	return feeds, nil
}

// AddFeed adds a new feed to Elasticsearch and the cache.
func (s *FeedService) AddFeed(ctx context.Context, feed *models.Feed) error {
	if err := s.indexer.Add(
		ctx,
		bulk.NewAction(
			bulk.ActionCreate,
			bulk.ToIndex(s.osc.IndexWriteAlias(schema.FeedsIndex)),
			bulk.WithID(feed.GetID()),
			bulk.WithDoc(feed),
		),
	); err != nil {
		return fmt.Errorf("bulk add feed: %w", err)
	}
	if _, ok := s.Set(feed.GetID(), feed); !ok {
		slogctx.FromCtx(ctx).Warn("Unable to cache new feed.",
			slog.String("feed_id", feed.GetID()),
		)
	}
	return nil
}

// UpdateFeed applies the given updates to a Feed. Any cached version of the feed is invalidated.
func (s *FeedService) UpdateFeed(ctx context.Context, feed *models.Feed) error {
	if err := doc.Index(ctx, s.osc.Doc, s.osc.IndexWriteAlias(schema.FeedsIndex), feed); err != nil {
		return fmt.Errorf("update feed: %w", err)
	}
	// Set replaces the entry; Set() respects TTL from ExpireAfterUpdate.
	s.Set(feed.GetID(), feed)

	return nil
}

type Change struct {
	Old any `json:"old"`
	New any `json:"new"`
}

// UpdateFeedItems determines if there are new items in the feed and then adds them to the database, performing item
// enrichment as needed. It returns a timestamp indicating a new lastFetched value for the feed, based off the latest
// new item's timestamp.
func (s *FeedService) UpdateFeedItems(
	ctx context.Context,
	oldData, newData *models.Feed,
) (time.Time, error) {
	// If there are not items, just return.
	if len(newData.GetItems()) == 0 {
		if err := s.logger.Log(
			ctx,
			NewFeedStatusMsg(oldData.GetID(), logWithStatusCode(http.StatusNoContent)),
		); err != nil {
			slogctx.Error(ctx, "Unable to write feed status.",
				slog.Any("error", err))
		}
		return oldData.LastFetched, nil
	}
	// Add any new items since the last feed update.
	if newItems := newData.GetItems().FilterSince(oldData.LastFetched); len(newItems) > 0 {
		// Filter to only items that are new or had significant changes.
		itemStates := s.itemSvc.SortItemsByState(ctx, newItems)
		updates := slices.Concat(itemStates.New, itemStates.Updated)
		if len(updates) > 0 {
			// Enrich new items.
			if _, err := s.itemSvc.EnrichItemBatch(ctx, newData, itemStates.New); err != nil {
				return oldData.LastFetched, fmt.Errorf("enrich items: %w", err)
			}

			// Log item states.
			slogctx.Info(ctx, "Updated feed items.",
				slog.Time("since", oldData.LastFetched),
				slog.Int("new", len(itemStates.New)),
				slog.Int("updated", len(itemStates.Updated)),
				slog.Int("unchanged", len(itemStates.Unchanged)),
			)
			if err := s.logger.Log(ctx, NewFeedStatusMsg(
				oldData.GetID(),
				logWithStatusCode(http.StatusOK),
				logWithItemIDs(updates.GetIDs()...),
			)); err != nil {
				slogctx.Error(ctx, "Unable to write feed status.",
					slog.Any("error", err))
			}
			// Return timestamp of most recent item.
			return updates.SortByTimestamp()[0].GetTimestamp(), nil
		}
	}
	if err := s.logger.Log(
		ctx,
		NewFeedStatusMsg(oldData.GetID(), logWithStatusCode(http.StatusNoContent)),
	); err != nil {
		slogctx.Error(ctx, "Unable to write feed status.",
			slog.Any("error", err))
	}
	return oldData.LastFetched, nil
}

// ApplyFeedUpdates takes an existing feed and new feed data, compares the two, updates any fields as appropriate and
// writes the updated feed back to the database. It will add/update both any new/updated items and any updates to the
// feed metadata.
func (s *FeedService) ApplyFeedUpdates(ctx context.Context,
	oldData, newData *models.Feed) error {
	if err := ctx.Err(); err != nil {
		slogctx.Warn(ctx, "Cannot apply feed updates: context done",
			slog.Any("cause", context.Cause(ctx)),
			slog.Any("error", err),
		)
		return fmt.Errorf("cannot apply feed updates: %w", err)
	}

	// Add any new or update existing items.
	lastFetched, err := s.UpdateFeedItems(ctx, oldData, newData)
	if err != nil {
		slogctx.Warn(ctx, "Unable to add new or update existing items.",
			slog.Any("error", err))
		lastFetched = oldData.LastFetched // don't advance on failure
	}
	// Compare new/old feed data and update only changed fields.
	// Update only the fields that changed in newData, then update timestamps.
	changes := s.updateFeedData(ctx, oldData, newData)
	if len(changes) > 0 {
		var slogAttrs []slog.Attr
		for key, value := range changes {
			slogAttrs = append(slogAttrs,
				slog.Group(key,
					slog.Any("old", fmt.Sprintf("%+v", value.Old)),
					slog.Any("new", fmt.Sprintf("%+v", value.New)),
				),
			)
		}
		slogctx.LogAttrs(ctx, slog.LevelInfo, "Feed data updated.", slogAttrs...)
	}
	// Warn if feed data is not valid.
	if err := newData.Validate(); err != nil {
		slogctx.Warn(ctx, "Feed data is not valid!",
			slog.Any("error", err),
		)
	}
	// Also update LastFetched and Updated
	newData.LastFetched = lastFetched
	newData.Updated = new(time.Now().UTC())
	if err := s.UpdateFeed(ctx, newData); err != nil {
		return fmt.Errorf("update feed: %w", err)
	}

	return nil
}

func (s *FeedService) updateFeedData(ctx context.Context, oldData, newData *models.Feed) map[string]Change {
	changes := make(map[string]Change)
	// Accept any title change.
	if oldData.GetTitle() != newData.GetTitle() {
		changes["title"] = Change{Old: oldData.GetTitle(), New: newData.GetTitle()}
	}
	// Accept any description change.
	if oldData.GetDescription() != newData.GetDescription() {
		changes["description"] = Change{Old: oldData.GetDescription(), New: newData.GetDescription()}
	}
	// Accept any link change.
	if oldData.GetLink() != newData.GetLink() {
		changes["link"] = Change{Old: oldData.GetLink(), New: newData.GetLink()}
	}
	// Only change domain where new domain is not empty.
	if newData.Domain != "" && oldData.Domain != newData.Domain {
		changes["domain"] = Change{Old: oldData.Domain, New: newData.Domain}
	}
	// Accept any source type change.
	if oldData.SourceType != newData.SourceType {
		changes["source_type"] = Change{Old: oldData.SourceType, New: newData.SourceType}
	}
	// Accept any copyright change (case-insensitive match).
	if !strings.EqualFold(oldData.GetRights(), newData.GetRights()) {
		changes["copyright"] = Change{Old: oldData.GetRights(), New: newData.GetRights()}
	}
	// Accept any language change (case-insensitive match).
	if !strings.EqualFold(oldData.GetLanguage(), newData.GetLanguage()) {
		changes["language"] = Change{Old: oldData.GetLanguage(), New: newData.GetLanguage()}
	}
	// Accept any image change.
	switch {
	case newData.Image != nil && oldData.Image == nil:
		changes["image"] = Change{Old: oldData.Image, New: newData.Image}
	case newData.Image != nil && oldData.Image != nil && newData.Image.GetURL() != oldData.Image.GetURL():
		changes["image"] = Change{Old: oldData.Image, New: newData.Image}
	}
	// SourceURLs comparison, ignore query parameters and case differences except path.
	if same, err := comparisons.SameURLs(oldData.GetSourceURLs(), newData.GetSourceURLs()); err == nil && !same {
		// Always append, sort, then compact any source URL changes.
		newData.SourceURLs = append(newData.SourceURLs, oldData.SourceURLs...)
		slices.Sort(newData.SourceURLs)
		newData.SourceURLs = slices.Compact(newData.SourceURLs)
		changes["source_urls"] = Change{Old: oldData.GetSourceURLs(), New: newData.GetSourceURLs()}
	}
	// Always append then compact any author changes.
	if len(newData.GetAuthors()) != len(oldData.GetAuthors()) {
		newData.Authors = append(newData.GetAuthors(), oldData.GetAuthors()...)
		slices.Sort(newData.Authors)
		newData.Authors = slices.Compact(newData.Authors)
		changes["authors"] = Change{Old: oldData.GetAuthors(), New: newData.GetAuthors()}
	}
	// Always append then compact any contributor changes.
	if len(newData.GetContributors()) != len(oldData.GetContributors()) {
		newData.Authors = append(newData.GetContributors(), oldData.GetContributors()...)
		slices.Sort(newData.Contributors)
		newData.Contributors = slices.Compact(newData.Contributors)
		changes["contributors"] = Change{Old: oldData.GetContributors(), New: newData.GetContributors()}
	}
	if len(newData.Categories) > 0 {
		slices.Sort(newData.Categories)
		if !slices.Equal(newData.Categories, oldData.Categories) {
			// Combine the old and new categories.
			newData.Categories = append(newData.Categories, oldData.Categories...)
			newData.Categories = slices.Compact(newData.Categories)
			// Truncate long feed category lists.
			if len(newData.Categories) > 100 {
				newData.Categories = newData.Categories[:100]
			}
			changes["categories"] = Change{Old: oldData.GetCategories(), New: newData.GetCategories()}
		}
	} else {
		newData.Categories = oldData.Categories
	}
	if len(newData.Categories) == 0 {
		slogctx.Debug(ctx, "Feed has no categories. Needs categorizing.")
	}
	// // If the feed does not have categories, use the classifier to generate some.
	// if len(oldData.GetCategories()) == 0 {
	// 	if err := s.ClassifyFeed(ctx, newData); err != nil {
	// 		slogctx.Warn(ctx, "Could not classify feed",
	// 			slog.Any("error", err))
	// 	} else {
	// 		changes["categories"] = Change{Old: oldData.GetCategories(), New: newData.GetCategories()}
	// 	}
	// }
	return changes
}

// SuggestYoutubeFeeds will return a list of youtube feeds that match the given text.
func (s *FeedService) SuggestYoutubeFeeds(ctx context.Context, text string) (*models.SuggestFeedsResults, error) {
	// Get user subscriptions.
	allSubscriptions := models.SubscriptionsFromCtx(ctx)
	if allSubscriptions == nil {
		return nil, fmt.Errorf("get user subscriptions: %w", models.ErrCtxValueNotFound)
	}

	// Perform a search on youtube to find a channel that matches the user's query.
	ytResults, err := youtube.Find(ctx, text, 5)
	if err != nil {
		return nil, fmt.Errorf("search youtube channels: %w", err)
	}
	// Extract the channel RSS feed urls.
	urls := make([]string, 0, len(ytResults))
	for result := range slices.Values(ytResults) {
		urls = append(urls, result.SourceURL())
	}
	var feeds models.Feeds
	// Try to find existing feeds that match the query.
	resp, err := search.Search(ctx, s.osc, s.osc.IndexReadAlias(schema.FeedsIndex),
		search.WithQuery(
			search.BoolQuery(
				search.WithBoolMustNot(
					// User must not already be subscribed.
					search.TermsQuery("feed_id", allSubscriptions.GetFeedIDs()),
				),
				search.WithBoolShould(
					// Match source_urls (preferred) or url.
					search.TermsQuery("source_urls", urls, search.WithQueryBoost(10.0)),
					search.TermsQuery("url", urls),
				),
			),
		),
		search.WithSort(feedSortOptions(new(models.SortMostRelevant))),
		search.WithTrackTotalHits(false),
		search.WithSize(5),
	)
	if err != nil {
		return nil, fmt.Errorf("search feeds: %w", err)
	}

	// If existing feeds are found, return the results
	if len(resp.Hits.Hits) > 0 {
		var (
			feeds models.Feeds
			err   error
		)
		feeds, _, err = resp.ExtractHits[*models.Feed]()
		if err != nil {
			return nil, fmt.Errorf("extract feeds: %w", err)
		}
		// Retrieve the latest 3 articles for each feed.
		latestItems, err := s.getFeedLatestItems(ctx, 3, feeds.GetIDs())
		if err != nil {
			slogctx.FromCtx(ctx).Warn("Unable to get latest items for feeds.",
				slog.Any("error", err),
			)
		}
		return &models.SuggestFeedsResults{
			Text:        text,
			Feeds:       feeds,
			LatestItems: latestItems,
		}, nil
	}

	// Create new Youtube feeds from the results.
	feeds, err = youtube.CreateFeeds(ctx, ytResults...)
	if err != nil {
		return nil, fmt.Errorf("generate feeds: %w", err)
	}
	latestItems := make(map[models.FeedID]models.Items)
	for feed := range slices.Values(feeds) {
		latestItems[feed.GetID()] = feed.GetItems()
	}

	return &models.SuggestFeedsResults{
		Text:        text,
		Feeds:       feeds,
		LatestItems: latestItems,
	}, nil
}

// SuggestGoogleNewsFeeds will return a google news RSS feed for the given search query.
func (s *FeedService) SuggestGoogleNewsFeeds(
	ctx context.Context,
	text string,
) (*models.SuggestFeedsResults, error) {
	newsURL, err := news.GenerateRSSURL(text)
	if err != nil {
		return nil, fmt.Errorf("generate news RSS URL: %w", err)
	}

	// Get user subscriptions.
	allSubscriptions := models.SubscriptionsFromCtx(ctx)
	if allSubscriptions == nil {
		return nil, fmt.Errorf("get user subscriptions: %w", models.ErrCtxValueNotFound)
	}

	// Try to find existing feeds that match the query.
	resp, err := search.Search(ctx, s.osc, s.osc.IndexReadAlias(schema.FeedsIndex),
		search.WithQuery(
			search.BoolQuery(
				search.WithBoolMustNot(
					// User must not already be subscribed.
					search.TermsQuery("feed_id", allSubscriptions.GetFeedIDs()),
				),
				search.WithBoolFilter(
					search.TermQuery("domain.raw", "news.google.com"),
				),
				search.WithBoolShould(
					// Match source_urls (preferred) or url.
					search.TermQuery("source_urls", newsURL.String(), search.WithQueryBoost(10.0)),
					search.TermQuery("url", newsURL.String()),
				),
			),
		),
		search.WithSort(feedSortOptions(new(models.SortMostRelevant))),
		search.WithTrackTotalHits(false),
		search.WithSize(5),
	)
	if err != nil {
		return nil, fmt.Errorf("search feeds: %w", err)
	}

	// If existing feeds are found, return the results
	if len(resp.Hits.Hits) > 0 {
		var (
			feeds models.Feeds
			err   error
		)
		feeds, _, err = resp.ExtractHits[*models.Feed]()
		if err != nil {
			return nil, fmt.Errorf("extract feeds: %w", err)
		}
		// Retrieve the latest 3 articles for each feed.
		latestItems, err := s.getFeedLatestItems(ctx, 3, feeds.GetIDs())
		if err != nil {
			slogctx.FromCtx(ctx).Warn("Unable to get latest items for feeds.",
				slog.Any("error", err),
			)
		}
		return &models.SuggestFeedsResults{
			Text:        text,
			Feeds:       feeds,
			LatestItems: latestItems,
		}, nil
	}

	// Try to create new feeds for the URLs.
	latestItems := make(map[models.FeedID]models.Items)
	slogctx.FromCtx(ctx).Debug("Looking for new feed for URL.",
		slog.String("url", newsURL.String()),
	)
	newFeed, err := s.FetchFeed(ctx, newsURL.String())
	if err != nil {
		return nil, fmt.Errorf("unable to fetch google news RSS feed: %w", err)
	}
	// Retrieve the latest 3 articles for each feed.
	if items := newFeed.GetItems().SortByTimestamp(); len(items) > 0 {
		// Truncate to 3 items.
		if len(items) > 3 {
			items = items[:3]
		}
		latestItems[newFeed.GetID()] = items
	}
	// Always fetch item summaries for Google News feeds.
	newFeed.SetDirectFetchOptions(models.SetDirectFetchWithItemSummaries(true))
	// feeds = append(feeds, newFeed)
	return &models.SuggestFeedsResults{
		Text: text,
		// Feeds:       feeds,
		LatestItems: latestItems,
	}, nil
}

// SuggestFeeds returns a feeds and their latest articles that match the given text. It will search first for existing
// feeds in Elasticsearch. If the given text is a URL, it will fallback to searching the website for a feed.
func (s *FeedService) SuggestFeeds(
	ctx context.Context,
	request *models.SuggestFeedsRequest,
) (*models.SuggestFeedsResults, error) {
	// Ignore if text is empty and no categories specified.
	if request.Text == "" && len(request.Categories) == 0 {
		return &models.SuggestFeedsResults{
			Text: request.Text,
		}, nil
	}

	// Get user subscriptions.
	allSubscriptions := models.SubscriptionsFromCtx(ctx)
	if allSubscriptions == nil {
		return nil, fmt.Errorf("get user subscriptions: %w", models.ErrCtxValueNotFound)
	}

	// Find the top 5 feeds that match the user's query and which they are not already subscribed to.
	var feedSearchQuery search.QueryOption
	switch {
	case strings.HasPrefix(request.Text, "http"):
		parsedURL, err := url.Parse(request.Text)
		if err != nil {
			return nil, fmt.Errorf("malformed URL %s: %w", request.Text, err)
		}
		feedSearchQuery = search.BoolQuery(
			search.WithBoolMustNot(
				// User must not already be subscribed.
				search.TermsQuery("feed_id", allSubscriptions.GetFeedIDs()),
				// Exclude google news custom feeds.
				search.TermQuery("domain.raw", "news.google.com"),
			),
			search.WithBoolShould(
				// For URLs, match with and without a trailing slash. Boost source_URLs over URL.
				search.TermsQuery(
					"source_urls",
					[]string{strings.TrimSuffix(parsedURL.String(), "/"), parsedURL.String()},
					search.WithQueryBoost(10.0),
				),
				search.TermsQuery(
					"url",
					[]string{strings.TrimSuffix(parsedURL.String(), "/"), parsedURL.String()},
					search.WithQueryBoost(5.0),
				),
				// Match the URL domain.
				search.TermQuery(
					"domain.raw",
					parsedURL.Hostname(),
					search.WithQueryBoost(15.0),
				),
			),
		)
	default:
		feedSearchQuery = search.BoolQuery(
			search.WithBoolMustNot(
				// User must not already be subscribed.
				search.TermsQuery("feed_id", allSubscriptions.GetFeedIDs()),
				// Exclude google news custom feeds.
				search.TermQuery("domain.raw", "news.google.com"),
			),
			search.WithBoolMust(
				// Must match any category filters.
				search.TermsQuery("categories.raw", request.Categories),
			),
			search.WithBoolShould(
				search.TermQuery("title.exact", request.Text, search.WithQueryBoost(10.0)),
				// Title or description contains text, with boost for title.
				search.MultiMatchQuery(
					request.Text,
					[]string{"title^5", "customisation.title^4", "description", "customisation.description"},
					search.WithMultiMatchFuzziness("AUTO"),
				),
				// Match an existing subscription category.
				search.MatchQuery("categories", request.Text),
				search.TermQuery("categories.raw", "Uncategorized"),
				// Try to match the domain.
				search.MatchQuery("domain", request.Text),
				search.TermQuery("domain.raw", request.Text, search.WithQueryBoost(15.0)),
			),
		)
	}

	suggestions := &models.SuggestFeedsResults{
		Text:        request.Text,
		Feeds:       make(models.Feeds, 0),
		LatestItems: make(map[models.FeedID]models.Items, 3),
	}

	resp, err := search.Search(ctx, s.osc, s.osc.IndexReadAlias(schema.FeedsIndex),
		search.WithQuery(feedSearchQuery),
		search.WithSort(feedSortOptions(new(models.SortMostRelevant))),
		search.WithSize(request.Count),
		search.WithTrackTotalHits(false),
	)
	if err != nil {
		return nil, fmt.Errorf("search feeds: %w", err)
	}

	if len(resp.Hits.Hits) > 0 {
		// Retrieve the latest 3 articles for each feed.
		var (
			feeds models.Feeds
			err   error
		)
		feeds, _, err = resp.ExtractHits[*models.Feed]()
		if err != nil {
			return nil, fmt.Errorf("extract feeds: %w", err)
		}
		// Retrieve the latest 3 articles for each feed.
		latestItems, err := s.getFeedLatestItems(ctx, 3, feeds.GetIDs())
		if err != nil {
			slogctx.FromCtx(ctx).Warn("Unable to get latest items for feeds.",
				slog.Any("error", err),
			)
		}
		suggestions.Feeds = feeds
		suggestions.LatestItems = latestItems
	}

	// If the request text is a URL, also
	if strings.HasPrefix(request.Text, "http") {
		if newFeedURL, err := models.NormalizeFeedURL(request.Text); err == nil {
			slogctx.FromCtx(ctx).Debug("Looking for new feed for URL.",
				slog.String("url", newFeedURL.String()),
			)
			newFeed, err := s.FetchFeed(ctx, newFeedURL.String())
			if err != nil {
				return nil, fmt.Errorf("new feed from url: %w", err)
			}
			if items := newFeed.GetItems().SortByTimestamp(); len(items) > 0 {
				// Truncate to 3 items.
				if len(items) > 3 {
					items = items[:3]
				}
				suggestions.LatestItems[newFeed.GetID()] = items
			}
			suggestions.Feeds = slices.Concat(models.Feeds{newFeed}, suggestions.Feeds)
		}
	}

	return suggestions, nil
}

// GenerateOPML generates an OPML file of the feeds listed by the given IDs.
func (s *FeedService) GenerateOPML(ctx context.Context, feedIDs ...models.FeedID) ([]byte, error) {
	feeds, err := s.GetFeeds(ctx, feedIDs...)
	if err != nil {
		return nil, fmt.Errorf("get feeds: %w", err)
	}

	// Create outlines for all subscriptions.
	outlines := make([]opml.Outline, 0, len(feeds))
	for feed := range slices.Values(feeds) {
		outlines = append(
			outlines,
			*opml.NewSubscriptionOutline(feed.GetTitle(), feed.GetSourceURLs()[0],
				opml.WithHTMLURL(feed.GetLink()),
				opml.WithOutlineTitle(feed.GetTitle()),
			),
		)
	}
	// Generate the opml file from the outlines.
	title := "Foragd Export (" + time.Now().Format(time.DateTime) + ")"
	opmlExport := opml.NewOPML(
		opml.WithTitle(title),
		opml.WithOutlines(outlines...),
	)
	// Marshal the opml file and convert to a byte reader.
	data, err := xml.Marshal(opmlExport)
	if err != nil {
		return nil, fmt.Errorf("marshal opml: %w", err)
	}
	data = []byte(xml.Header + string(data))

	return data, nil
}

// ClassifyFeed will add appropriate categories to a feed by classifying the item content.
func (s *FeedService) ClassifyFeed(ctx context.Context, feed *models.Feed) error {
	if len(feed.GetItems()) == 0 {
		slogctx.Warn(ctx, "Cannot classify feed, no items.")
		return nil
	}

	ctx = slogctx.With(ctx, "feed_id", feed.GetID())
	ctx = slogctx.With(ctx, "feed_title", feed.GetTitle())

	var classificationText strings.Builder

	// Append together all content for classification.
	if feed.GetDescription() != "" {
		classificationText.WriteString(feed.GetDescription())
		classificationText.WriteRune('\n')
	}
	for item := range slices.Values(feed.GetItems()) {
		switch {
		case item.GetContent() != "":
			classificationText.WriteString(item.GetContent())
			classificationText.WriteRune('\n')
		case item.GetDescription() != "":
			classificationText.WriteString(item.GetDescription())
			classificationText.WriteRune('\n')
		}
	}

	if textx.CountWords(classificationText.String()) < 20 {
		// Ignore feed when there is too little content for good processing.
		slogctx.Warn(ctx, "Not enough content to classify feed. Assigning 'Uncategorized'.")
		feed.Categories = []models.Category{"Uncategorized"}
	} else {
		// Classify from IAB Tier 1 Categories.
		slogctx.Info(ctx, "Assigning categories to feed based on current item content.")

		categories, err := s.ollama.Classify(ctx, classificationText.String(), feed.GetLink(), nil, nil, 0.8)
		if err != nil {
			slogctx.Error(ctx, "Could not classify feed. Leaving uncategorized.",
				slog.Any("error", err))
			feed.Categories = []models.Category{"Uncategorized"}
		}
		if len(categories) == 0 {
			slogctx.Warn(ctx, "No classified categories. Leaving uncategorized.")
			feed.Categories = []models.Category{"Uncategorized"}
		} else {
			feed.Categories = make([]models.Category, 0, len(categories))
			for category := range slices.Values(categories) {
				feed.Categories = append(feed.Categories, category.Label)
			}
		}
	}

	slogctx.Info(ctx, "Feed classified.",
		slog.String("categories", strings.Join(feed.Categories, ",")))

	return nil
}

func (s *FeedService) searchAllFeeds(ctx context.Context, queries ...search.QueryOption) (models.Feeds, error) {
	resp, err := search.SearchAll(ctx, s.osc, s.osc.IndexReadAlias(schema.FeedsIndex), 2500, queries)
	if err != nil {
		return nil, fmt.Errorf("search feeds: %w", err)
	}
	feeds, err := resp.ExtractHits[*models.Feed]()
	if err != nil {
		return nil, fmt.Errorf("extract feeds: %w", err)
	}
	return feeds, nil
}

// getFeedLatestItems fetches the most recent count items for each given feed. An optional query clause can be specified
// that will be added to the bool filter clause of the query to apply additional filtering to the items.
func (s *FeedService) getFeedLatestItems(
	ctx context.Context,
	count int,
	feedIDs []models.FeedID,
) (map[models.FeedID]models.Items, error) {
	resp, err := search.Search(ctx, s.osc, s.osc.IndexReadAlias(schema.ItemsIndex),
		search.WithQuery(
			search.BoolQuery(
				search.WithBoolFilter(
					search.TermsQuery("feed_id", feedIDs),
				),
			),
		),
		search.WithAggregation(
			"feed",
			search.TermsAggregation(
				search.WithTermsAggregationField("feed_id"),
				search.WithTermsAggregationSize(len(feedIDs)),
			),
			search.WithChildAggregation(
				"latest_items",
				search.TopHitsAggregation(
					search.WithTopHitsAggregationSize(count),
					search.WithTopHitsAggregationSort(
						ItemSortCombinations(new(models.SortNewestFirst))),
				),
			),
		),
		search.WithSearchParameters(
			search.WithSortParams("_doc"),
		),
		search.WithTrackTotalHits(false),
		search.WithTrackScores(false),
		search.WithSize(0),
	)
	if err != nil {
		return nil, fmt.Errorf("fetch latest articles: %w", err)
	}
	feedsLatestItems := make(map[models.FeedID]models.Items)
	var wg sync.WaitGroup
	var mu sync.Mutex
	// Extract the feed aggregation. We need to use a custom type as we cannot access nested aggregations from the typed
	// API at the moment.
	type latestItemsAggs struct {
		Aggregations struct {
			Feed struct {
				Buckets []struct {
					Key         string `json:"key"`
					LatestItems struct {
						Hits struct {
							Hits []struct {
								ID     string          `json:"_id"`
								Source json.RawMessage `json:"_source"`
							} `json:"hits"`
						} `json:"hits"`
					} `json:"latest_items"`
				} `json:"buckets"`
			} `json:"feed"`
		} `json:"aggregations"`
	}
	var latestItems latestItemsAggs
	if err := json.NewDecoder(resp.RawBody()).Decode(&latestItems); err != nil {
		return nil, fmt.Errorf("extract aggregation results: %w", err)
	}
	for bucket := range slices.Values(latestItems.Aggregations.Feed.Buckets) {
		feedID := bucket.Key
		if feedID == "" {
			continue
		}
		wg.Go(func() {
			// Get the subscription with this feedID.
			if !slices.Contains(feedIDs, feedID) {
				slogctx.FromCtx(ctx).
					Warn("Could not match feed in aggregation result to a subscription.",
						slog.String("feed_id", feedID),
					)
				return
			}
			var (
				items models.Items
			)
			// Extract the latest items.
			//
			// * Note that the "latest_items" aggregation applies _source filtering,
			// * so only the given fields will be populated in the models.Item object.
			// Loop through the hits, extracting the source into the required type.
			for hit := range slices.Values(bucket.LatestItems.Hits.Hits) {
				source, err := opensearch.ExtractSource[*models.Item](hit.Source)
				if err != nil {
					slogctx.Warn(
						ctx,
						"Could not extract latest item.",
						slog.String("feed_id", feedID),
						slog.Any("error", err),
					)
					continue
				}
				items = append(items, source)
			}
			// Ensure proper sorting.
			items = items.SortByTimestamp()
			mu.Lock()
			feedsLatestItems[feedID] = items
			mu.Unlock()
		})
	}

	wg.Wait()
	return feedsLatestItems, nil
}

func (s *FeedService) getFeedLastUpdates(
	ctx context.Context,
	ids ...models.FeedID,
) (map[models.FeedID]time.Time, error) {
	resp, err := search.Search(ctx, s.osc, s.osc.IndexReadAlias(schema.ItemsIndex),
		search.WithQuery(
			search.TermsQuery("feed_id", ids),
		),
		search.WithSize(len(ids)),
		search.WithCollapse("feed_id"),
		search.WithSort(ItemSortCombinations(new(models.SortNewestFirst))),
		search.WithTrackScores(false),
		search.WithTrackTotalHits(false),
	)
	if err != nil {
		return nil, fmt.Errorf("search items: %w", err)
	}
	items, _, err := resp.ExtractHits[*models.Item]()
	if err != nil {
		return nil, fmt.Errorf("extract items: %w", err)
	}

	updates := make(map[models.FeedID]time.Time)
	for item := range slices.Values(items) {
		updates[item.GetFeedID()] = item.GetTimestamp()
	}

	return updates, nil
}

// GetFeedSubscriptionStats fetches the stats for FeedSubscriptions and returns a map of the SubscriptionID to
// SubscriptionStats that can be used to lookup the stats pertaining to a particular subscription.
func (s *FeedService) getFeedAverageDailyUpdates(
	ctx context.Context,
	ids ...models.FeedID,
) (map[models.FeedID]float64, error) {
	// Round to yesterday.
	since := time.Now().UTC().Add(-24 * 30 * time.Hour).Format(time.DateOnly)
	resp, err := search.Search(ctx, s.osc, s.osc.IndexReadAlias(schema.ItemsIndex),
		search.WithQuery(
			search.BoolQuery(
				search.WithBoolFilter(
					// Must match any of the given feed IDs.
					search.TermsQuery("feed_id", ids),
					search.BoolQuery(
						search.WithBoolShould(
							search.SinceQuery("published", since),
							search.SinceQuery("updated", since),
						),
					),
				),
			),
		),
		search.WithAggregation(
			"feed",
			search.TermsAggregation(
				search.WithTermsAggregationField("feed_id"),
				search.WithTermsAggregationSize(len(ids)),
			),
			search.WithChildAggregation(
				"updates_per_day",
				search.DateHistogramAggregation(
					"@timestamp",
					search.WithDateHistogramAggregationCalendarInterval("day"),
					search.WithDateHistogramAggregationFormat("yyyy-MM-dd"),
				),
			),
			search.WithChildAggregation(
				"avg_daily_updates",
				search.AverageBucketAggregation(
					search.WithPipelineAggregationBucketsPath("updates_per_day._count"),
				),
			),
		),
		search.WithSize(0),
		search.WithTrackTotalHits(false),
		search.WithTrackScores(false),
		search.WithSort(search.NewSortOptions(search.WithSortField("_doc"))),
	)
	if err != nil {
		return nil, fmt.Errorf("aggregate feeds: %w", err)
	}

	// Extract the feed aggregation. We need to use a custom type as we cannot access nested aggregations from the typed
	// API at the moment.
	type avgDailyUpdatesAgg struct {
		Aggregations struct {
			Feed struct {
				Buckets []struct {
					Key           string `json:"key"`
					UpdatesPerDay struct {
						Buckets []struct {
							Key      string `json:"key_as_string"`
							DocCount int    `json:"doc_count"`
						} `json:"buckets"`
					} `json:"updates_per_day"`
					AvgDailyUpdates struct {
						Value float64 `json:"value"`
					} `json:"avg_daily_updates"`
				} `json:"buckets"`
			} `json:"feed"`
		} `json:"aggregations"`
	}
	var dailyUpdates avgDailyUpdatesAgg
	if err := json.NewDecoder(resp.RawBody()).Decode(&dailyUpdates); err != nil {
		return nil, fmt.Errorf("extract aggregation results: %w", err)
	}

	stats := make(map[models.FeedID]float64)
	for feed := range slices.Values(dailyUpdates.Aggregations.Feed.Buckets) {
		if feed.Key == "" {
			continue
		}
		feedID := feed.Key
		stats[feedID] = feed.AvgDailyUpdates.Value
	}

	return stats, nil
}

// FetchOptions defines options that manipulate how a feed will be fetched from a remote URL.
type FetchOptions struct {
	Proxy      bool
	ExistingID models.FeedID
}

// FetchOption is a functional option that controls some aspect of how a feed will be fetched.
type FetchOption func(*FetchOptions)

// FetchWithProxy option will ensure the fetch feed request is proxied.
func FetchWithProxy(value bool) FetchOption {
	return func(fo *FetchOptions) {
		fo.Proxy = value
	}
}

// ApplyExistingID assigns the given FeedID to the feed object.
func ApplyExistingID(id models.FeedID) FetchOption {
	return func(fo *FetchOptions) {
		fo.ExistingID = id
	}
}

// FetchFeed retrieves the feed found at the given URL.
func (s *FeedService) FetchFeed(
	ctx context.Context,
	feedURL string,
	options ...FetchOption,
) (*models.Feed, error) {
	opts := &FetchOptions{}
	for option := range slices.Values(options) {
		option(opts)
	}

	// Parse the URL to ensure its valid.
	sourceURL, err := url.Parse(feedURL)
	if err != nil {
		return nil, models.NewAPIError(
			http.StatusUnprocessableEntity,
			fmt.Errorf("parse url: %w", err),
			models.WithUserMessage(models.NewErrorMessage("Feed URL is invalid", feedURL)),
		)
	}
	if !sourceURL.IsAbs() {
		return nil, models.NewAPIError(
			http.StatusBadRequest,
			fmt.Errorf("%q is not an absolute URL", feedURL),
			models.WithUserMessage(models.NewErrorMessage("Not an absolute URL", feedURL)),
		)
	}
	if sourceURL.Scheme != "https" && sourceURL.Scheme != "http" {
		return nil, models.NewAPIError(
			http.StatusBadRequest,
			fmt.Errorf("%q has an invalid or unknown scheme", sourceURL.String()),
			models.WithUserMessage(models.NewErrorMessage("Unknown URL", feedURL)),
		)
	}

	// Create a buffer for the feed data.
	feedBuf, ok := bufPool.Get().(*bytes.Buffer)
	if !ok {
		return nil, models.NewAPIError(
			http.StatusInternalServerError,
			errors.New("get feed buffer failed"),
			models.WithUserMessage(models.NewErrorMessage("An internal processing error occurred", "")),
		)
	}
	feedBuf.Reset()
	defer bufPool.Put(feedBuf)

	// Fetch the feed data from the source URL.
	switch opts.Proxy {
	case false:
		if err := DirectFetch(ctx, s.httpClient, sourceURL, feedBuf); err != nil {
			return nil, fmt.Errorf("direct fetch: %w", err)
		}
	case true:
		if err := ProxyFetch(ctx, sourceURL, feedBuf); err != nil {
			return nil, fmt.Errorf("proxy fetch: %w", err)
		}
	}

	// Read the fetched data from the buffer.
	data, err := io.ReadAll(feedBuf)
	if err != nil {
		return nil, models.NewAPIError(
			http.StatusUnprocessableEntity,
			fmt.Errorf("read feed data: %w", err),
			models.WithUserMessage(models.NewErrorMessage(
				"Feed URL returned invalid or malformed data",
				sourceURL.String(),
			)),
		)
	}

	// Parse the data into the appropriate source type.
	source, err := parseFeed(sourceURL, data)
	if err != nil && err.Error() == "html source" {
		// Set a context value to let us know we have already attempted fetching from the HTML link. This protects
		// against weird links that do infinite redirection or other strange loops.
		_, found := ctx.Value("html_attempted").(bool)
		if found {
			return nil, models.NewAPIError(
				http.StatusUnprocessableEntity,
				errors.New("cannot fetch from URL"),
				models.WithUserMessage(models.NewWarningMessage("Cannot fetch feed data from URL", sourceURL.String())),
			)
		} else {
			ctx = context.WithValue(ctx, "html_attempted", true)
		}
		// HTML web page. Use "autodiscovery" to find feed.
		if newURL, err := DiscoverFeedURL(
			sourceURL,
			data,
		); err == nil && newURL != "" &&
			newURL != sourceURL.String() {
			slogctx.FromCtx(ctx).Debug("Found feed URL in HTML, re-fetching.")
			return s.FetchFeed(ctx, newURL, options...)
		}
		return nil, models.NewAPIError(
			http.StatusNotFound,
			fmt.Errorf("not found: %s", sourceURL.String()),
			models.WithUserMessage(models.NewErrorMessage("No feed found at URL", sourceURL.String())),
		)
	}

	// Safety-net check that we could actually parse the source.
	if source == nil {
		return nil, models.NewAPIError(
			http.StatusNotFound,
			fmt.Errorf("not found: %s", sourceURL.String()),
			models.WithUserMessage(models.NewErrorMessage("No feed found at URL", sourceURL.String())),
		)
	}

	// Create a new feed from the source.
	feed := models.NewFeed(ctx, sourceURL.String(), opts.ExistingID, source)

	// Set the method used to fetch the feed.
	if opts.Proxy {
		feed.FetchMethod = models.FeedFetchMethodProxied
	} else {
		feed.FetchMethod = models.FeedFetchMethodDirect
	}

	return feed, nil
}

func parseFeed(sourceURL *url.URL, data []byte) (*feeds.Feed, error) {
	// Parse the response as a feed type.
	var feedData *feeds.Feed
	switch feedType, err := feeds.DetectSourceType(bytes.NewReader(data)); {
	case err != nil:
		return nil, models.NewAPIError(
			http.StatusUnprocessableEntity,
			fmt.Errorf("detect source type: %w", err),
			models.WithUserMessage(models.NewWarningMessage("Could not determine source type", sourceURL.String())),
		)
	case feedType == types.SourceUnknown:
		return nil, models.NewAPIError(
			http.StatusUnprocessableEntity, fmt.Errorf("detect source type: %w", err),
			models.WithUserMessage(models.NewWarningMessage("Could not determine source type", sourceURL.String())),
		)
	case feedType == types.SourceAtom:
		// Atom feed.
		feedData, err = feeds.NewDecoder[*atom.Feed](bytes.NewReader(data))
		if err != nil {
			return nil, models.NewAPIError(
				http.StatusUnprocessableEntity,
				fmt.Errorf("parse atom: %w", err),
				models.WithUserMessage(models.NewWarningMessage("Unable to parse Atom feed", sourceURL.String())),
			)
		}
	case feedType == types.SourceRSS:
		// RSS 2.0 feed.
		feedData, err = feeds.NewDecoder[*rss.RSS](bytes.NewReader(data))
		if err != nil {
			return nil, models.NewAPIError(
				http.StatusUnprocessableEntity,
				fmt.Errorf("parse rss: %w", err),
				models.WithUserMessage(models.NewWarningMessage("Unable to parse RSS feed", sourceURL.String())),
			)
		}
	case feedType == types.SourceRDF:
		// RDF/RSS 1.0 feed.
		feedData, err = feeds.NewDecoder[*rdf.RDF](bytes.NewReader(data))
		if err != nil {
			return nil, models.NewAPIError(
				http.StatusUnprocessableEntity,
				fmt.Errorf("parse rss: %w", err),
				models.WithUserMessage(models.NewWarningMessage("Unable to parse RDF feed", sourceURL.String())),
			)
		}
	case feedType == types.SourceJSONFeed:
		// JSONFeed.
		feedData, err = feeds.NewDecoder[*jsonfeed.Feed](bytes.NewReader(data))
		if err != nil {
			return nil, models.NewAPIError(
				http.StatusUnprocessableEntity,
				fmt.Errorf("parse rss: %w", err),
				models.WithUserMessage(models.NewWarningMessage("Unable to parse JSON feed", sourceURL.String())),
			)
		}
	case feedType == types.SourceHTML:
		return nil, errors.New("html source")
	default:
		return nil, models.NewAPIError(
			http.StatusUnsupportedMediaType,
			errors.New("unsupported media type"),
			models.WithUserMessage(models.NewWarningMessage("Unsupported feed type", sourceURL.String())),
		)
	}
	return feedData, nil
}

// FetchFeedUpdates fetches an updated version of the feed, including items. It returns the updated feed, and the source
// URL used to fetch the updates (for disambiguation of feeds with multiple source URLs). A non-nil error is returned
// where there is a critical error fetching the feed details.
func (s *FeedService) FetchFeedUpdates(
	ctx context.Context,
	details models.Feed,
) (*models.Feed, models.URL, error) {
	if err := ctx.Err(); err != nil {
		slogctx.Warn(ctx, "context done",
			slog.Any("cause", context.Cause(ctx)),
			slog.Any("error", err),
		)
		return nil, "", fmt.Errorf("cannot fetch feed updates: %w", err)
	}

	// Set fetch options.
	var (
		proxyRequest bool
	)
	if details.FetchMethod == models.FeedFetchMethodProxied {
		proxyRequest = true
	}

	// Get new items since the last fetch. Try each listed source URL for the feed until one succeeds.
	var errs []error
	tryURLs := details.GetSourceURLs()
	if len(tryURLs) == 0 {
		// If there are no SourceURL values for some reason, add the feed link (which should be a webpage). FetchFeed
		// will automatically try to find a feed URL in the page as needed.
		tryURLs = append(tryURLs, details.GetLink())
	}
	for feedURL := range slices.Values(tryURLs) {
		feed, err := s.FetchFeed(
			ctx,
			feedURL,
			FetchWithProxy(proxyRequest),
			ApplyExistingID(details.GetID()),
		)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", feedURL, err))
			if s.osc != nil {
				logGeneralError(ctx, s.indexer, err, feedURL, details.GetID())
			} else {
				logGeneralError(ctx, nil, err, feedURL, details.GetID())
			}
			continue
		}
		// Set the FeedID to ensure consistency.
		feed.FeedID = details.GetID()
		return feed, feedURL, nil
	}

	// No feed data returned by any url. Log and return error.
	err := models.NewAPIError(
		http.StatusNoContent,
		errors.New("failed to fetch feed details with any source URL: "+errors.Join(errs...).Error()),
	)
	if s.osc != nil {
		logGeneralError(ctx, s.indexer, err, strings.Join(details.GetSourceURLs(), ","), details.GetID())
	} else {
		logGeneralError(ctx, nil, err, strings.Join(details.GetSourceURLs(), ","), details.GetID())
	}
	return nil, "", err
}

// FetchFeedUpdatesAsArticles fetches an updated version of the feed, using Zyte to generate a feed from the source
// website articles list. It returns the updated feed, and the source URL used to fetch the updates (for disambiguation
// of feeds with multiple source URLs). A non-nil error is returned where there is a critical error fetching the feed
// details.
func (s *FeedService) FetchFeedUpdatesAsArticles(
	ctx context.Context,
	details models.Feed,
) (*models.Feed, models.URL, error) {
	if err := ctx.Err(); err != nil {
		slogctx.Warn(ctx, "context done",
			slog.Any("cause", context.Cause(ctx)),
			slog.Any("error", err),
		)
		return nil, "", fmt.Errorf("cannot fetch feed updates: %w", err)
	}

	// Get any extraction options from the feed.
	var fetchOptions models.FetchZyteOptions
	if details.FetchOptions != nil {
		var err error
		fetchOptions, err = details.FetchOptions.AsFetchZyteOptions()
		if err != nil {
			return nil, "", fmt.Errorf("extract fetch options: %w", err)
		}
	}

	// Set from where the list will be extracted.
	extractFrom := zyte.ExtractFromHttpResponseBody
	if fetchOptions.ExtractFrom != nil {
		extractFrom = *fetchOptions.ExtractFrom
	}

	// Get new items since the last fetch. Try each listed source URL for the feed until one succeeds.
	var errs []error
	for feedURL := range slices.Values(details.GetSourceURLs()) {
		// Fetch the feed details using Zyte as an article list.
		resp, err := zyte.Proxy(
			ctx,
			feedURL,
			zyte.WithExtractFrom(extractFrom),
			zyte.AsArticleList(&zyte.ExtractOptions{
				ExtractFrom: &extractFrom,
			}),
			zyte.WithTag("feed_id", details.GetID()),
			zyte.WithTag("action", "update_feed"),
		)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", feedURL, err))
			if s.osc != nil {
				logZyteError(ctx, s.indexer, err, feedURL, details.GetID())
			} else {
				logZyteError(ctx, nil, err, feedURL, details.GetID())
			}
			continue
		}
		// Generate feed details from Zyte response.
		feed, err := NewFeedFromZyteResponse(ctx, resp)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", feedURL, err))
			if s.osc != nil {
				logGeneralError(ctx, s.indexer, err, feedURL, details.GetID())
			} else {
				logGeneralError(ctx, nil, err, feedURL, details.GetID())
			}
			continue
		}

		// Extract and enrich articles from Zyte response.
		items, err := NewItemsFromZyteArticles(ctx, feed, resp.ArticleList)
		if err != nil {
			if s.osc != nil {
				logGeneralError(ctx, s.indexer, err, feedURL, details.GetID())
			} else {
				logGeneralError(ctx, nil, err, feedURL, details.GetID())
			}
			return nil, feedURL, fmt.Errorf("%s: %w", feedURL, err)
		}
		feed.Items = items

		return feed, feedURL, nil
	}

	// No feed data returned by any url. Log and return error.
	err := models.NewAPIError(
		http.StatusNoContent,
		errors.New("failed to fetch feed details with any source URL: "+errors.Join(errs...).Error()),
	)
	if s.osc != nil {
		logGeneralError(ctx, s.indexer, err, strings.Join(details.GetSourceURLs(), ","), details.GetID())
	} else {
		logGeneralError(ctx, nil, err, strings.Join(details.GetSourceURLs(), ","), details.GetID())
	}
	return nil, "", err
}

func NewFeedFromZyteResponse(ctx context.Context, resp *zyte.Response) (*models.Feed, error) {
	// Create a buffer for the feed data.
	feedBuf, ok := bufPool.Get().(*bytes.Buffer)
	if !ok {
		return nil, models.NewAPIError(http.StatusInternalServerError, errors.New("get feed buffer failed"))
	}
	feedBuf.Reset()
	defer bufPool.Put(feedBuf)
	// Extract the response body
	body, err := resp.GetBody()
	if err != nil {
		return nil, models.NewAPIError(http.StatusUnprocessableEntity, fmt.Errorf("get response body: %w", err))
	}
	if _, err := feedBuf.Write(body); err != nil {
		return nil, models.NewAPIError(http.StatusUnprocessableEntity, fmt.Errorf("read response: %w", err))
	}

	sourceURL, err := url.Parse(resp.URL)
	if err != nil {
		return nil, models.NewAPIError(http.StatusUnprocessableEntity, fmt.Errorf("parse response URL: %w", err))
	}

	feedID := "feed_" + strconv.FormatUint(xxh3.Hash([]byte(sourceURL.String())), 10)

	// Extract metadata (opengraph, readability) from HTML.
	ogData, rdData, err := extractMetadataFromHTML(sourceURL, body)
	if err != nil {
		return nil, fmt.Errorf("extract metadata: %w", err)
	}

	feed := &models.Feed{
		FeedID:      feedID,
		CreatedAt:   time.Now().UTC(),
		LastFetched: models.UnixEpoch,
		SourceType:  models.SourceTypeRSS,
		SourceURLs:  []string{sourceURL.String()},
		URL:         sourceURL.String(),
		Domain:      sourceURL.Hostname(),
		Published:   models.UnixEpoch,
	}
	if resp.ArticleList != nil {
		feed.FetchMethod = models.FeedFetchMethodZyteArticles
	}

	// Parse info from opengraph data.
	if ogData != nil {
		if ogData.Title != "" {
			feed.Title = ogData.Title
		}
		if ogData.Description != "" {
			feed.Description = &ogData.Description
		}
		if ogData.Image != "" {
			feed.Image = models.NewRemoteImage(ogData.Image, feed.Title)
		}
	}
	// Parse info from readability data.
	if rdData != nil {
		if rdData.Title() != "" && feed.Title != "" {
			feed.Title = rdData.Title()
		}
		if rdData.Byline() != "" && feed.Description == nil {
			feed.Description = new(rdData.Byline())
		}
		// Parse timestamps (only in readability output).
		if published, err := rdData.PublishedTime(); err == nil {
			feed.Published = published.UTC()
		}
		if updated, _ := rdData.ModifiedTime(); !updated.IsZero() {
			updUTC := updated.UTC()
			feed.Updated = &updUTC
		}
	}

	// Set extract options
	var extractFrom zyte.ExtractFrom
	switch {
	case resp.BrowserHtml != nil:
		extractFrom = zyte.ExtractFromBrowserHtml
	case resp.HttpResponseBody != nil:
		fallthrough
	default:
		extractFrom = zyte.ExtractFromHttpResponseBody
	}
	var fetchOptions models.Feed_FetchOptions
	if err := fetchOptions.FromFetchZyteOptions(models.FetchZyteOptions{ExtractFrom: &extractFrom}); err != nil {
		return feed, models.NewAPIError(http.StatusUnprocessableEntity, fmt.Errorf("set fetch options: %w", err))
	}
	feed.FetchOptions = &fetchOptions

	// Make sure we have generated valid feed data.
	if err := feed.Validate(); err != nil {
		return feed, models.NewAPIError(http.StatusUnprocessableEntity, fmt.Errorf("validate feed: %w", err))
	}

	return feed, nil
}

func feedSortOptions(sort *models.Sort) *search.Sorting {
	if sort == nil {
		return search.NewSortOptions(search.WithSortField("_doc"))
	}
	switch *sort {
	case models.SortNewestFirst:
		return search.NewSortOptions(
			search.WithSortField("published", search.WithFieldSortOrder(search.SortOrderDesc)),
			search.WithSortField("updated", search.WithFieldSortOrder(search.SortOrderDesc)),
			search.WithSortField("feed_id", search.WithFieldSortOrder(search.SortOrderDesc)),
		)
	case models.SortOldestFirst:
		return search.NewSortOptions(
			search.WithSortField("published", search.WithFieldSortOrder(search.SortOrderAsc)),
			search.WithSortField("updated", search.WithFieldSortOrder(search.SortOrderAsc)),
			search.WithSortField("feed_id", search.WithFieldSortOrder(search.SortOrderAsc)),
		)
	case models.SortMostRelevant:
		return search.NewSortOptions(
			search.WithSortField("_score", search.WithFieldSortOrder(search.SortOrderDesc)),
			search.WithSortField("published", search.WithFieldSortOrder(search.SortOrderDesc)),
			search.WithSortField("updated", search.WithFieldSortOrder(search.SortOrderDesc)),
			search.WithSortField("feed_id", search.WithFieldSortOrder(search.SortOrderDesc)),
		)
	default:
		return search.NewSortOptions(
			search.WithSortField("_doc"),
		)
	}
}

type FeedStatusMsg struct {
	*models.FeedStatus

	Labels map[string]string `json:"labels"`
}

func NewFeedStatusMsg(id models.FeedID, options ...func(*FeedStatusMsg)) *FeedStatusMsg {
	msg := &FeedStatusMsg{
		FeedStatus: &models.FeedStatus{
			Timestamp: time.Now().UTC(),
			FeedID:    id,
		},
		Labels: map[string]string{
			"type": "feed-status",
		},
	}
	for option := range slices.Values(options) {
		option(msg)
	}
	return msg
}

func logWithItemIDs(itemIDs ...models.ItemID) func(*FeedStatusMsg) {
	return func(fslm *FeedStatusMsg) {
		fslm.Items = itemIDs
	}
}

func logWithStatusCode(code int) func(*FeedStatusMsg) {
	return func(fslm *FeedStatusMsg) {
		fslm.StatusCode = code
	}
}

func logZyteError(ctx context.Context, indexer *bulk.Indexer, err error, feedURL string, feedID models.FeedID) {
	logMsg := NewFeedStatusMsg(feedID)
	logMsg.FeedStatus.URL = feedURL
	if apiErr, ok := errors.AsType[*models.APIError](err); ok {
		logMsg.StatusCode = apiErr.StatusCode
		logMsg.StatusMessage = new(apiErr.Error())
	} else {
		logMsg.StatusCode = http.StatusInternalServerError
		logMsg.StatusMessage = new(err.Error())
	}
	id := "log_" + strconv.FormatUint(xxh3.Hash([]byte(time.Now().String()+feedID+*logMsg.StatusMessage)), 10)
	if err := indexer.Add(
		ctx,
		bulk.NewAction(bulk.ActionCreate, bulk.ToIndex("logs"), bulk.WithID(id), bulk.WithDoc(logMsg)),
	); err != nil {
		slogctx.FromCtx(ctx).Warn("Unable to record feed status.",
			slog.Any("error", err),
		)
	}
}

func logGeneralError(ctx context.Context, indexer *bulk.Indexer, err error, feedURL string, feedID models.FeedID) {
	logMsg := NewFeedStatusMsg(feedID)
	logMsg.FeedStatus.URL = feedURL
	if apiErr, ok := errors.AsType[*models.APIError](err); ok {
		logMsg.StatusCode = apiErr.StatusCode
		logMsg.StatusMessage = new(apiErr.Error())
	} else {
		logMsg.StatusCode = http.StatusInternalServerError
		logMsg.StatusMessage = new(err.Error())
	}
	id := "log_" + strconv.FormatUint(xxh3.Hash([]byte(time.Now().String()+feedID+*logMsg.StatusMessage)), 10)
	if err := indexer.Add(
		ctx,
		bulk.NewAction(bulk.ActionCreate, bulk.ToIndex("logs"), bulk.WithID(id), bulk.WithDoc(logMsg)),
	); err != nil {
		slogctx.FromCtx(ctx).Warn("Unable to record feed status.",
			slog.Any("error", err),
		)
	}
}

// DiscoverFeedURL attempts to find a feed URL within a HTML page.
//
// There are a couple of "canonical" places the feed URL is located. Firstly, as per the RSS spec, look for a link
// element with rel="alternate" and type="application/rss+xml". Secondly, check for a link element with a URL that ends
// with feed, rss or atom, which would indicate a feed URL.
//
//nolint:gocognit
func DiscoverFeedURL(sourceURL *url.URL, content []byte) (string, error) {
	page, err := html.Parse(bytes.NewReader(content))
	if err != nil {
		return "", fmt.Errorf("parse html: %w", err)
	}

	// Check if its a Medium blog with a custom domain. If so, just return the original URL with the path replaced with
	// `/feed`.
	//
	// https://help.medium.com/hc/en-us/articles/214874118-Using-RSS-feeds-of-profiles-publications-and-topics
	if htmlx.CheckMediumSignals(page) > 0 {
		sourceURL.Path = "/feed"
		return sourceURL.String(), nil
	}

	// RSS feed auto-discovery.
	//
	// https://www.rssboard.org/rss-autodiscovery
	findCanonicalLinkAttribute := func(elem *html.Node) string {
		// Needs to have attributes rel="alternate".
		if !slices.ContainsFunc(
			elem.Attr,
			func(a html.Attribute) bool { return a.Key == "rel" && a.Val == "alternate" },
		) {
			return ""
		}
		// Needs to have type="{feedMimeType}".
		if !slices.ContainsFunc(
			elem.Attr,
			func(a html.Attribute) bool { return a.Key == "type" && slices.Contains(feeds.MimeTypesFeed, a.Val) },
		) {
			return ""
		}
		// Return the href attribute value.
		for a := range slices.Values(elem.Attr) {
			if a.Key == "href" {
				return a.Val
			}
		}
		return ""
	}

	// Commonly used links based on path.
	findCommonFeedURL := func(elem *html.Node) string {
		for a := range slices.Values(elem.Attr) {
			// href attribute should contain a well-known substring.
			if a.Key == "href" &&
				(strings.HasSuffix(a.Val, "feed") || strings.Contains(a.Val, "rss") || strings.Contains(a.Val, "atom")) {
				return a.Val
			}
		}
		return ""
	}
	var findURL func(*html.Node) string
	findURL = func(n *html.Node) string {
		if n.Type == html.ElementNode && (n.Data == "a" || n.Data == "link") {
			if foundURL := findCanonicalLinkAttribute(n); foundURL != "" {
				return foundURL
			}
			if foundURL := findCommonFeedURL(n); foundURL != "" {
				return foundURL
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if foundURL := findURL(c); foundURL != "" {
				return foundURL
			}
		}
		return ""
	}

	foundURL := findURL(page)
	if foundURL == "" {
		return "", fmt.Errorf("%w: no url found", htmlx.ErrParseURL)
	}

	// Parse the discovered URL.
	feedURL, err := url.Parse(foundURL)
	if err != nil {
		return foundURL, fmt.Errorf("discover feed url: %w", err)
	}
	// Check whether the URL is absolute.
	if !feedURL.IsAbs() {
		// Try to create an absolute URL for the feed.
		feedURL, err = url.Parse(sourceURL.String())
		if err != nil {
			return "", fmt.Errorf("discover feed url: %w", err)
		}
		feedURL.Path, err = url.JoinPath("/", foundURL)
		if err != nil {
			return "", fmt.Errorf("discover feed url: %w", err)
		}
	}
	return feedURL.String(), nil
}
