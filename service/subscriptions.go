/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"net/mail"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	estypes "github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/sortorder"
	"github.com/maypok86/otter/v2"
	slogctx "github.com/veqryn/slog-context"
	"github.com/zeebo/xxh3"
	"go.opentelemetry.io/otel/codes"
	"golang.org/x/sync/errgroup"

	"github.com/immanent-tech/go-base/validation"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/bulk"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/providers/elastic/results"
)

// SubscriptionService holds a cache and backend connection for handling [models.Subscription] objects.
type SubscriptionService struct {
	*otter.Cache[models.UserID, *UserSubscriptions]

	store      *ElasticService
	userLoader otter.LoaderFunc[models.UserID, *UserSubscriptions]
}

func (s *SubscriptionService) get(
	ctx context.Context,
	userID models.UserID,
) (*UserSubscriptions, error) {
	subscriptionsCache, err := s.Get(ctx, userID, s.userLoader)
	switch {
	case err != nil && errors.Is(err, otter.ErrNotFound):
		// span.RecordError(err)
		// span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("get user subscription cache: %w", models.ErrNotFound)
	case err != nil:
		// span.RecordError(err)
		// span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("get user subscription cache: %w", err)
	}
	return subscriptionsCache, nil
}

type UserSubscriptions struct {
	*otter.Cache[models.SubscriptionID, *models.Subscription]

	userID     models.UserID
	loader     otter.LoaderFunc[models.SubscriptionID, *models.Subscription]
	bulkLoader otter.BulkLoaderFunc[models.SubscriptionID, *models.Subscription]
}

func (s *UserSubscriptions) get(
	ctx context.Context,
	subscriptionID models.SubscriptionID,
) (*models.Subscription, error) {
	subscription, err := s.Get(ctx, subscriptionID, s.loader)
	switch {
	case err != nil && errors.Is(err, otter.ErrNotFound):
		// span.RecordError(err)
		// span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("get user subscription cache: %w", models.ErrNotFound)
	case err != nil:
		// span.RecordError(err)
		// span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("get user subscription cache: %w", err)
	}
	return subscription, nil
}

func (s *UserSubscriptions) bulkGet(
	ctx context.Context,
	subscriptionIDs ...models.SubscriptionID,
) (models.Subscriptions, error) {
	results, err := s.BulkGet(ctx, subscriptionIDs, s.bulkLoader)
	switch {
	case err != nil && errors.Is(err, otter.ErrNotFound):
		// span.RecordError(err)
		// span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("get subscription cache: %w", models.ErrNotFound)
	case err != nil:
		// span.RecordError(err)
		// span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("get subscription cache: %w", err)
	}
	return slices.Collect(maps.Values(results)), nil
}

// LoadSubscriptionService loads a service that can manipulate subscription objects in the backend store.
var LoadSubscriptionService = sync.OnceValues(func() (*SubscriptionService, error) {
	svc, err := LoadElasticService()
	if err != nil {
		return nil, fmt.Errorf("load elastic service: %w", err)
	}
	return &SubscriptionService{

		Cache: otter.Must(
			&otter.Options[models.UserID, *UserSubscriptions]{
				MaximumSize:      100,
				ExpiryCalculator: otter.ExpiryAccessing[models.UserID, *UserSubscriptions](time.Hour),
			},
		),
		store: svc,
		userLoader: otter.LoaderFunc[models.UserID, *UserSubscriptions](
			func(
				ctx context.Context,
				userID models.UserID,
			) (*UserSubscriptions, error) {
				userSubscriptionsCache, err := otter.New(&otter.Options[models.SubscriptionID, *models.Subscription]{
					InitialCapacity: 3000,
					MaximumSize:     3000,
				})
				if err != nil {
					return nil, fmt.Errorf("create subscriptions cache: %w", err)
				}
				start := time.Now()
				// Execute query.
				var (
					subscriptions models.Subscriptions
				)
				subscriptions, err = elastic.SearchAll[*models.Subscription](
					ctx,
					svc.GetIndexRO(SubscriptionsIndex),
					query.Term("user_id", userID),
					3000,
				)
				if err != nil {
					return nil, ElasticsearchToAPIError(err)
				}

				if len(subscriptions) == 0 {
					return nil, otter.ErrNotFound
				}

				for subscription := range slices.Values(subscriptions) {
					userSubscriptionsCache.Set(subscription.GetID(), subscription)
				}

				// Load grouped subscriptions into parent.
				for groupSubscription := range slices.Values(subscriptions.FilterByType(models.SubscriptionTypeGroup)) {
					groupSubscription.GroupData.Subscriptions = make(
						[]*models.Subscription,
						0,
						len(groupSubscription.GroupData.Metadata),
					)
					for m := range slices.Values(groupSubscription.GroupData.Metadata) {
						grouped := subscriptions.GetByID(m.SubscriptionID)
						groupSubscription.GroupData.Subscriptions = append(
							groupSubscription.GroupData.Subscriptions,
							grouped,
						)
					}
				}

				slogctx.Debug(ctx, "Created subscriptions cache for user.",
					slog.Duration("took", time.Since(start)))

				return &UserSubscriptions{
					Cache:  userSubscriptionsCache,
					userID: userID,
					loader: otter.LoaderFunc[models.SubscriptionID, *models.Subscription](
						func(ctx context.Context, id models.SubscriptionID) (*models.Subscription, error) {
							subscription, err := elastic.GetDoc[models.SubscriptionID, *models.Subscription](
								ctx,
								svc.GetIndexRO(SubscriptionsIndex),
								id,
							)
							if err != nil && !errors.Is(err, elastic.ErrNotFound) {
								return nil, fmt.Errorf("get subscriptions: %w", ElasticsearchToAPIError(err))
							}
							if errors.Is(err, elastic.ErrNotFound) {
								return nil, otter.ErrNotFound
							}
							// For group subscriptions, load the grouped subscriptions.
							if subscription.Type == models.SubscriptionTypeGroup {
								grouped, err := elastic.GetDocs[models.SubscriptionID, *models.Subscription](
									ctx,
									svc.GetIndexRO(SubscriptionsIndex),
									subscription.GroupData.GetGroupedSubscriptionIDs()...,
								)
								if err != nil {
									return nil, fmt.Errorf("get grouped subscriptions: %w", err)
								}
								subscription.GroupData.Subscriptions = grouped
							}
							return subscription, nil
						}),
					bulkLoader: otter.BulkLoaderFunc[models.SubscriptionID, *models.Subscription](
						func(
							ctx context.Context,
							ids []models.SubscriptionID,
						) (map[models.SubscriptionID]*models.Subscription, error) {
							subscriptions, err := elastic.GetDocs[models.SubscriptionID, *models.Subscription](
								ctx,
								svc.GetIndexRO(SubscriptionsIndex),
								ids...,
							)
							if err != nil {
								return nil, fmt.Errorf("get subscriptions: %w", ElasticsearchToAPIError(err))
							}

							results := make(map[models.SubscriptionID]*models.Subscription, len(subscriptions))

							for subscription := range slices.Values(subscriptions) {
								// For group subscriptions, load the grouped subscriptions.
								if subscription.Type == models.SubscriptionTypeGroup {
									grouped, err := elastic.GetDocs[models.SubscriptionID, *models.Subscription](
										ctx,
										svc.GetIndexRO(SubscriptionsIndex),
										subscription.GroupData.GetGroupedSubscriptionIDs()...,
									)
									if err != nil {
										return nil, fmt.Errorf("get grouped subscriptions: %w", err)
									}
									subscription.GroupData.Subscriptions = grouped
								}
								results[subscription.GetID()] = subscription
							}

							return results, nil
						}),
				}, nil
			}),
	}, nil
})

// GetAllSubscriptions returns a [models.Subscriptions] slice of all subscriptions for a user.
func (s *SubscriptionService) GetAllSubscriptions(
	ctx context.Context,
) (models.Subscriptions, error) {
	ctx, span := tracer.Start(ctx, "GetAllSubscriptions")
	defer span.End()

	user := models.UserFromCtx(ctx)
	if user == nil {
		span.RecordError(models.ErrCtxValueNotFound)
		span.SetStatus(codes.Error, models.ErrCtxValueNotFound.Error())
		return nil, fmt.Errorf("get user: %w", models.ErrCtxValueNotFound)
	}

	subscriptionsCache, err := s.get(ctx, user.GetID())
	if err != nil && !errors.Is(err, models.ErrNotFound) {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("get subscription cache for user: %w", err)
	}

	if subscriptionsCache == nil {
		return make(models.Subscriptions, 0), nil
	}

	return slices.Collect(subscriptionsCache.Values()), nil
}

// GetSubscription returns a [*models.Subscription] that matches the given [models.SubscriptionID] for the given user.
func (s *SubscriptionService) GetSubscription(
	ctx context.Context,
	id models.SubscriptionID,
) (*models.Subscription, error) {
	ctx, span := tracer.Start(ctx, "GetSubscription")
	defer span.End()

	user := models.UserFromCtx(ctx)
	if user == nil {
		span.RecordError(models.ErrCtxValueNotFound)
		span.SetStatus(codes.Error, models.ErrCtxValueNotFound.Error())
		return nil, fmt.Errorf("get user: %w", models.ErrCtxValueNotFound)
	}

	subscriptionsCache, err := s.get(ctx, user.GetID())
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("get subscription cache for user: %w", err)
	}

	subscription, err := subscriptionsCache.get(ctx, id)
	if err != nil && !errors.Is(err, models.ErrNotFound) {
		return nil, fmt.Errorf("get subscription by id: %w", err)
	}

	return subscription, nil
}

// BulkGetSubscriptions returns a [models.Subscriptions] slice of subscriptions that match the given
// [models.SubscriptionID].
func (s *SubscriptionService) BulkGetSubscriptions(
	ctx context.Context,
	ids ...models.SubscriptionID,
) (models.Subscriptions, error) {
	ctx, span := tracer.Start(ctx, "GetSubscriptionsByID")
	defer span.End()

	user := models.UserFromCtx(ctx)
	if user == nil {
		span.RecordError(models.ErrCtxValueNotFound)
		span.SetStatus(codes.Error, models.ErrCtxValueNotFound.Error())
		return nil, fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound)
	}

	subscriptionsCache, err := s.get(ctx, user.GetID())
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("get subscription cache for user: %w", err)
	}

	subscriptions, err := subscriptionsCache.bulkGet(ctx, ids...)
	if err != nil && !errors.Is(err, models.ErrNotFound) {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("bulk get subscriptions: %w", err)
	}

	return subscriptions, nil
}

// RemoveSubscriptions removes subscriptions with the given [models.SubscriptionID] from a user.
func (s *SubscriptionService) RemoveSubscriptions(
	ctx context.Context,
	ids ...models.SubscriptionID,
) error {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound)
	}
	if err := elastic.DeleteDocs(ctx, s.store.GetIndexRW(SubscriptionsIndex),
		query.Bool(
			query.Filter(
				query.Term("user_id", user.GetID()),
				query.Terms("subscription_id", ids),
			),
		),
	); err != nil {
		return ElasticsearchToAPIError(err)
	}
	// Remove the subscriptions from the cache.
	if subscriptionsCache, ok := s.GetIfPresent(user.GetID()); ok {
		for id := range slices.Values(ids) {
			subscriptionsCache.Invalidate(id)
		}
	}

	return nil
}

// UpdateSubscriptions will bulk update each given [*models.Subscription].
func (s *SubscriptionService) UpdateSubscriptions(
	ctx context.Context,
	subscriptions ...*models.Subscription,
) error {
	ctx, span := tracer.Start(ctx, "UpdateSubscriptions")
	defer span.End()

	if err := bulk.IndexDocuments(ctx, s.store.GetIndexRW(SubscriptionsIndex), subscriptions...); err != nil {
		return ElasticsearchToAPIError(err)
	}
	if err := bulk.Flush(ctx); err != nil {
		slogctx.Warn(ctx, "Failed to flush subscription updates.",
			slog.Any("error", err))
	}

	// Update the subscription dynamic info
	if err := s.UpdateSubscriptionDynamicInfo(ctx, subscriptions); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		slogctx.FromCtx(ctx).Warn("Could not update subscription dynamic info.",
			slog.Any("errro", err),
		)
	}

	// Update the cached subscriptions.
	user := models.UserFromCtx(ctx)
	if user == nil {
		span.RecordError(models.ErrCtxValueNotFound)
		span.SetStatus(codes.Error, models.ErrCtxValueNotFound.Error())
		return fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound)
	}
	if subscriptionsCache, ok := s.GetIfPresent(user.GetID()); ok {
		for subscription := range slices.Values(subscriptions) {
			subscriptionsCache.Invalidate(subscription.GetID())
			subscriptionsCache.Set(subscription.GetID(), subscription)
		}
	}

	return nil
}

// MarkSubscriptions will mark as appropriate all the given subscriptions. Marking a subscription includes updating the
// subscription data in the user object and clearing any individual item states for a subscription.
func (s *SubscriptionService) MarkSubscriptions(
	ctx context.Context,
	mark models.Mark,
	subscriptionIDs ...models.SubscriptionID,
) error {
	ctx, span := tracer.Start(ctx, "MarkSubscriptions")
	defer span.End()

	user := models.UserFromCtx(ctx)
	if user == nil {
		span.RecordError(models.ErrCtxValueNotFound)
		span.SetStatus(codes.Error, models.ErrCtxValueNotFound.Error())
		return fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound)
	}

	subscriptions, err := s.BulkGetSubscriptions(ctx, subscriptionIDs...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("get subscription details: %w", err)
	}

	for subscription := range slices.Values(subscriptions) {
		if subscription.GetSubscriptionType() == models.SubscriptionTypeGroup {
			if err = s.MarkSubscriptions(
				ctx,
				mark,
				subscription.GroupData.GetGroupedSubscriptionIDs()...); err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				return fmt.Errorf("mark group subscription: %w", err)
			}
		} else {
			subscription.Mark(user, mark)
			if err = s.UpdateSubscriptions(ctx, subscriptions...); err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				return fmt.Errorf("update subscription data: %w", err)
			}
			slogctx.Debug(ctx, "Marked subscription.",
				slog.String("subscription_id", subscription.GetID()),
				slog.String("mark", string(mark)),
			)
		}
	}

	return nil
}

func (s *SubscriptionService) MarkArticles(
	ctx context.Context,
	mark models.Mark,
	subscriptionID models.SubscriptionID,
	itemIDs ...models.ItemID,
) error {
	subscription, err := s.GetSubscription(ctx, subscriptionID)
	if err != nil {
		return models.NewAPIError(
			http.StatusInternalServerError,
			fmt.Errorf("get subscriptions: %w", err),
			models.WithUserErrorSummary("Backend request failed!"),
			models.WithUserErrorDescription("This might be a temporary error, please try again."),
		)
	}
	subscription.MarkItems(mark, itemIDs...)
	if err = s.UpdateSubscriptions(ctx, subscription); err != nil {
		return models.NewAPIError(
			http.StatusInternalServerError,
			fmt.Errorf("update subscription: %w", err),
			models.WithUserErrorSummary("Backend request failed!"),
			models.WithUserErrorDescription("This might be a temporary error, please try again."),
		)
	}
	return nil
}

// GetSubscriptionSuggestions returns subscriptions that match the given text. A set of ids can be optionally passed to
// ignore those subscriptions.
func (s *SubscriptionService) GetSubscriptionSuggestions(
	ctx context.Context,
	text string,
	count int,
	ignoredSubscriptions []models.SubscriptionID,
) (models.Subscriptions, error) {
	// Get subscriptions by ID.
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound)
	}

	// Perform search.
	resp, err := elastic.Search[*models.Subscription](
		ctx,
		s.store.GetIndexRO(SubscriptionsIndex),
		elastic.WithQueryOptions[*elastic.SearchRequest](
			query.Bool(
				query.Filter(
					query.Term("user_id", user.GetID()),
					// query.Bool(
					// 	query.Should(
					// 		query.Term("type", models.SubscriptionTypeEmail),
					// 		query.Term("type", models.SubscriptionTypeFeed),
					// 	),
					// ),
				),
				query.Must(
					query.Bool(
						query.Should(
							query.SearchAsYouType(text, "customisation.nickname"),
						),
					),
				),
				query.MustNot(
					query.Terms("subscription_id", ignoredSubscriptions),
				),
			),
		),
		elastic.WithSort(newSubscriptionSortOptions(new(models.SortMostRelevant))...),
		elastic.WithSize(count),
	)
	if err != nil {
		return nil, fmt.Errorf("search subscriptions: %w", err)
	}
	if len(resp.Results) == 0 {
		return nil, fmt.Errorf("search subscriptions: %w", models.ErrNotFound)
	}

	subscriptions := resp.Results
	if err = s.UpdateSubscriptionDynamicInfo(ctx, subscriptions); err != nil {
		return nil, fmt.Errorf("add dynamic info: %w", err)
	}

	return subscriptions, nil
}

func (s *SubscriptionService) GetSubscriptionCategorySuggestions(
	ctx context.Context,
	feedIDs []models.FeedID,
	excludedCategories []models.Category,
) []models.Category {
	var suggestions []models.Category

	feedSvc, err := LoadFeedService()
	if err != nil {
		slogctx.Warn(ctx, "Could not load feed service.", slog.Any("error", err))
		return nil
	}

	itemSvc, err := LoadItemService()
	if err != nil {
		slogctx.Warn(ctx, "Could not load feed service.", slog.Any("error", err))
		return nil
	}

	// Get categories from feed sources.
	if feeds, err := feedSvc.GetFeeds(ctx, feedIDs...); err != nil {
		slogctx.FromCtx(ctx).Warn("Unable to get feeds for category suggestions.",
			slog.Any("error", err))
	} else {
		suggestions = feeds.GetCategories()
	}

	// Query items for feeds and append top categories from items.
	topCategoriesQuery := query.Bool(
		query.Filter(
			// Must match any of the given feed IDs.
			query.Terms("feed_id", feedIDs),
		),
		query.MustNot(
			query.Terms(
				"categories.raw",
				slices.Concat(models.CommonCategoryFilters, excludedCategories),
			),
		),
	)
	if topCategories, resp := itemSvc.GetTopCategoriesForItems(ctx, topCategoriesQuery); resp == nil {
		for c := range slices.Values(topCategories) {
			suggestions = append(suggestions, c.Category)
		}
	}

	slices.Sort(suggestions)
	return slices.Compact(suggestions)
}

// GetLatestArticles will fetch and add the latest articles to the given subscriptions.
func (s *SubscriptionService) GetLatestArticles(
	ctx context.Context,
	view models.View,
	subscriptions models.Subscriptions,
) {
	ctx, span := tracer.Start(ctx, "GetLatestArticles")
	defer span.End()

	// NOTE: there is concurrent access to the subscriptions slice, but each element is sequentially accessed within the
	// goroutines. So this is safe access.

	var wg sync.WaitGroup

	wg.Go(func() {
		ctx, span := tracer.Start(ctx, "get-feed-subscription-latest-items")
		defer span.End()

		// For feed/email subscriptions, get the latest 3 items from each.
		feedSubscriptions := subscriptions.FilterByType(models.SubscriptionTypeFeed)
		emailSubscriptions := subscriptions.FilterByType(models.SubscriptionTypeEmail)
		feedsLatestItems, err := s.getFeedSubscriptionLatestItems(
			ctx,
			3,
			slices.Concat(feedSubscriptions, emailSubscriptions),
			view,
		)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			slogctx.FromCtx(ctx).Warn("Unable to retrieve latest items for feed/email subscriptions.",
				slog.Any("error", err),
			)
		}
		for s := range slices.Values(feedSubscriptions) {
			if items, found := feedsLatestItems[s.GetFeedID()]; found {
				articles, err := GenerateArticles(ctx, items)
				if err != nil {
					slogctx.Warn(ctx, "Could not generate articles for feed subscription.",
						slog.String("subscription_id", s.GetID()),
						slog.Any("error", err))
					continue
				}
				s.Articles = articles
			}
		}
		for s := range slices.Values(emailSubscriptions) {
			if items, found := feedsLatestItems[s.GetFeedID()]; found {
				articles, err := GenerateArticles(ctx, items)
				if err != nil {
					slogctx.Warn(ctx, "Could not generate articles for email subscription.",
						slog.String("subscription_id", s.GetID()),
						slog.Any("error", err))
					continue
				}
				s.Articles = articles
			}
		}
	})

	wg.Go(func() {
		ctx, span := tracer.Start(ctx, "get-group-subscription-latest-items")
		defer span.End()

		// For group subscriptions, get the latest 3 items across each group's members.
		groupsLatestItems := s.getGroupSubscriptionLatestItems(
			ctx,
			3,
			subscriptions.FilterByType(models.SubscriptionTypeGroup),
			view,
		)
		groupSubscriptions := subscriptions.FilterByType(models.SubscriptionTypeGroup)
		for s := range slices.Values(groupSubscriptions) {
			if items, found := groupsLatestItems[s.GetID()]; found {
				articles, err := GenerateArticles(ctx, items)
				if err != nil {
					slogctx.Warn(ctx, "Could not generate articles for group subscription.",
						slog.String("subscription_id", s.GetID()),
						slog.Any("error", err))
					continue
				}
				s.Articles = articles
			}
		}
	})

	wg.Go(func() {
		ctx, span := tracer.Start(ctx, "get-search-subscription-latest-items")
		defer span.End()

		// For search subscription, run each search and get the top 3 results.
		searchLatestItems, err := s.getSearchSubscriptionLatestItems(
			ctx,
			3,
			subscriptions.FilterByType(models.SubscriptionTypeSearch),
		)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			slogctx.FromCtx(ctx).Warn("Unable to retrieve top items for search subscriptions.",
				slog.Any("error", err),
			)
		}
		searchSubscriptions := subscriptions.FilterByType(models.SubscriptionTypeSearch)
		for s := range slices.Values(searchSubscriptions) {
			if items, found := searchLatestItems[s.GetID()]; found {
				articles, err := GenerateArticles(ctx, items)
				if err != nil {
					slogctx.Warn(ctx, "Could not generate articles for search subscription.",
						slog.String("subscription_id", s.GetID()),
						slog.Any("error", err))
					continue
				}
				s.Articles = articles
			}
		}
	})

	wg.Wait()
}

// UpdateSubscriptionDynamicInfo adds dynamically generated information (e.g., unread count, stats, etc.) of the subscriptions in the [models.Subscriptions] slice.
// At the least, all subscriptions will have an unread count and last updated info generated. Other stats will also be
// generated if the user has set the display option ShowSubscriptionStats in their account settings.
//
//nolint:gocognit,funlen
func (s *SubscriptionService) UpdateSubscriptionDynamicInfo(
	ctx context.Context,
	subscriptions models.Subscriptions,
) error {
	ctx, span := tracer.Start(ctx, "UpdateSubscriptionDynamicInfo")
	defer span.End()

	// Bail early if given an empty list.
	if len(subscriptions) == 0 {
		return nil
	}

	itemSvc, err := LoadItemService()
	if err != nil {
		return fmt.Errorf("load items service: %w", err)
	}

	user := models.UserFromCtx(ctx)
	if user == nil {
		span.RecordError(models.ErrCtxValueNotFound)
		span.SetStatus(codes.Error, models.ErrCtxValueNotFound.Error())
		return fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound)
	}

	fetchJobs, jobCtx := errgroup.WithContext(ctx)
	defer jobCtx.Done()

	// Get unread count per feed.
	var unreadCounts map[models.FeedID]int64
	fetchJobs.Go(func() error {
		var err error
		unreadCounts, err = s.getSubscriptionUnreadCounts(jobCtx, subscriptions)
		if err != nil {
			return fmt.Errorf("get unread counts: %w", err)
		}
		return nil
	})

	// For search subscriptions, run queries directly to add unread count and last update.
	fetchJobs.Go(func() error {
		searchJobs, jobCtx := errgroup.WithContext(ctx)
		defer jobCtx.Done()
		for subscription := range slices.Values(subscriptions.FilterByType(models.SubscriptionTypeSearch)) {
			searchJobs.Go(func() error {
				request := subscription.SearchData.Search
				request.Sort = models.SortNewestFirst
				request.Count = 1
				items, _, err := itemSvc.RetrieveItems(ctx, &request)
				if err != nil {
					return fmt.Errorf("retrieve items: %w", err)
				}
				subscription.GetStats().LastUpdate = items[0].GetTimestamp()
				count, err := itemSvc.CountSearchResults(jobCtx, &request)
				if err == nil {
					subscription.GetStats().UnreadCount = int(count)
				} else {
					slogctx.FromCtx(jobCtx).
						Warn("Add subscription dynamic info, could not get unread count for search subscription.",
							slog.String("subscription_id", subscription.GetID()),
							slog.Any("error", err),
						)
				}
				return nil
			})
		}
		searchJobs.Wait()
		return nil
	})

	// Get last update (latest item timestamp) per feed.
	var lastUpdate map[models.FeedID]time.Time
	fetchJobs.Go(func() error {
		var err error
		lastUpdate, err = getFeedLastUpdates(jobCtx, s.store, subscriptions.GetFeedIDs()...)
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
			avgDailyUpdates, err = getFeedAverageDailyUpdates(jobCtx, s.store, subscriptions.GetFeedIDs()...)
			if err != nil {
				return fmt.Errorf("get average daily updates: %w", err)
			}
			return nil
		})
	}

	if err := fetchJobs.Wait(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("add subscription dynamic info: run jobs: %w", err)
	}

	// For feed subscriptions, add stats.
	if feedSubscriptions := subscriptions.FilterByType(models.SubscriptionTypeFeed); len(feedSubscriptions) > 0 {
		for subscription := range slices.Values(feedSubscriptions) {
			subscription.GetStats().UnreadCount = int(unreadCounts[subscription.GetFeedID()])
			subscription.GetStats().LastUpdate = lastUpdate[subscription.GetFeedID()]
			if user.GetSettings().ShowSubscriptionStats {
				subscription.GetStats().AvgDailyUpdates = avgDailyUpdates[subscription.GetFeedID()]
			}
		}
	}

	// For email subscriptions, add stats.
	if emailSubscriptions := subscriptions.FilterByType(models.SubscriptionTypeEmail); len(emailSubscriptions) > 0 {
		for subscription := range slices.Values(emailSubscriptions) {
			subscription.GetStats().UnreadCount = int(unreadCounts[subscription.GetFeedID()])
			subscription.GetStats().LastUpdate = lastUpdate[subscription.GetFeedID()]
			if user.GetSettings().ShowSubscriptionStats {
				subscription.GetStats().AvgDailyUpdates = avgDailyUpdates[subscription.GetFeedID()]
			}
		}
	}

	// For group subscriptions, calculate stats from other subscriptions.
	for subscription := range slices.Values(subscriptions.FilterByType(models.SubscriptionTypeGroup)) {
		var avgDailyUpdates []float64
		var unreadCount int
		var lastUpdates []time.Time
		// Get the avg daily updates and last update for each subscription in the group.
		allSubscriptions := models.SubscriptionsFromCtx(ctx)
		if len(allSubscriptions) == 0 {
			return fmt.Errorf("get user subscriptions: %w", models.ErrCtxValueNotFound)
		}
		groupedSubscriptions := allSubscriptions.FilterByIDs(subscription.GroupData.GetGroupedSubscriptionIDs()...)
		for groupedSubscription := range slices.Values(groupedSubscriptions) {
			// Collate statistics from child subscription.
			if user.GetSettings().ShowSubscriptionStats {
				avgDailyUpdates = append(avgDailyUpdates, groupedSubscription.GetStats().AvgDailyUpdates)
			}
			unreadCount += groupedSubscription.GetStats().UnreadCount
			lastUpdates = append(lastUpdates, groupedSubscription.GetStats().LastUpdate)
		}
		if user.GetSettings().ShowSubscriptionStats && len(avgDailyUpdates) > 0 {
			// Use the highest avg daily updates as the avg daily updates of the group.
			slices.Sort(avgDailyUpdates)
			slices.Reverse(avgDailyUpdates)
			subscription.GetStats().AvgDailyUpdates = avgDailyUpdates[0]
		}
		// Unread count is total unread count from all subscriptions in the group.
		subscription.GetStats().UnreadCount = unreadCount
		// LastUpdate is the timestamp of the most recent update across all subscriptions in the group.
		slices.SortFunc(lastUpdates, func(timeA, timeB time.Time) int {
			return timeA.Compare(timeB)
		})
		slices.Reverse(lastUpdates)
		subscription.GetStats().LastUpdate = lastUpdates[0]
	}

	return nil
}

// getFeedSubscriptionLatestItems fetches the latest items for subscriptions. This is a wrapper around GetFeedLatestItems
// that adds an extra filter clause to the search to return items that match the view status (i.e., read/unread).
func (s *SubscriptionService) getFeedSubscriptionLatestItems(
	ctx context.Context,
	count int,
	subscriptions models.Subscriptions,
	view models.View,
) (map[models.FeedID]models.Items, error) {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get user: %w", models.ErrCtxValueNotFound)
	}

	// Get all Feed IDs.
	feedIDs := subscriptions.GetFeedIDs()

	// Build queries for the filter buckets.
	subscriptionFilters := make(map[string]*estypes.Query)
	for subscription := range slices.Values(subscriptions) {
		switch view {
		case models.ViewAll:
			subscriptionFilters[subscription.GetFeedID()] = query.Build(
				allItemsForSubscriptionClause(subscription, user.GetMaxHistory()),
			)
		case models.ViewRead:
			subscriptionFilters[subscription.GetFeedID()] = query.Build(
				readItemsForSubscriptionClause(subscription, user.GetMaxHistory()),
			)
		case models.ViewUnread:
			fallthrough
		default:
			subscriptionFilters[subscription.GetFeedID()] = query.Build(
				unreadItemsForSubscriptionClause(subscription, user.GetMaxHistory()),
			)
		}
	}

	resp, err := elastic.Search[*models.Item](ctx,
		s.store.GetIndexRO(ItemsIndex),
		elastic.WithQueryOptions[*elastic.SearchRequest](
			query.Bool(
				query.Filter(
					query.Terms("feed_id", feedIDs),
					query.Bool(ArticleFiltersQueryClause(user.GetSettings().GlobalFilters)),
				),
			),
		),
		elastic.WithAggregations(
			elastic.Aggs{
				"feed": estypes.Aggregations{
					Filters: &estypes.FiltersAggregation{
						Filters: subscriptionFilters,
					},
					Aggregations: map[string]estypes.Aggregations{
						"latest_items": {
							TopHits: &estypes.TopHitsAggregation{
								Size: &count,
								Sort: NewItemSortCombinations(new(models.SortNewestFirst)),
							},
						},
					},
				},
			},
		),
		elastic.WithSize(0),
		elastic.WithDocSorting(),
	)
	if err != nil {
		return nil, fmt.Errorf("fetch latest articles: %w", err)
	}
	latestItems := make(map[models.FeedID]models.Items)
	var wg sync.WaitGroup
	var mu sync.Mutex
	// Extract the feed aggregation.
	feedsAgg, hasFeedAgg, err := elastic.ExtractAggregation[*estypes.FiltersAggregate](
		resp.Aggregations,
		"feed",
	)
	if !hasFeedAgg || err != nil {
		return nil, fmt.Errorf("extract feed aggregation: %w", err)
	}
	// Loop over the feed buckets.
	feedBuckets, err := elastic.ExtractBucketsAsMap[estypes.FiltersBucket](feedsAgg.Buckets)
	if err != nil {
		return nil, fmt.Errorf("extract feed aggregation buckets: %w", err)
	}
	for feedID, bucket := range feedBuckets {
		wg.Go(func() {
			if feedID == "" {
				return
			}
			// Get the subscription with this feedID.
			if !slices.Contains(feedIDs, feedID) {
				slogctx.FromCtx(ctx).
					Warn("Could not match feed in aggregation result to a subscription.",
						slog.String("feed_id", feedID),
					)
				return
			}
			// Extract the latest articles aggregation.
			latestItemsAggs, hasLatestItemsAgg, err := elastic.ExtractAggregation[*estypes.TopHitsAggregate](
				bucket.Aggregations,
				"latest_items",
			)
			if !hasLatestItemsAgg || err != nil {
				slogctx.FromCtx(ctx).Warn("Could not extract aggregation.",
					slog.String("aggregation", "latest_items"),
					slog.Any("error", err),
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
			items, _, err = results.ExtractSourceFromHits[*models.Item](latestItemsAggs.Hits.Hits)
			if err != nil {
				slogctx.FromCtx(ctx).
					Warn("Unable to extract latest articles from elastic.",
						slog.Any("error", err),
					)
				return
			}
			// Ensure proper sorting.
			items = items.SortByTimestamp()
			mu.Lock()
			latestItems[feedID] = items
			mu.Unlock()
		})
	}

	wg.Wait()
	return latestItems, nil
}

// getGroupSubscriptionLatestItems will return a map of latest items per subscription for the given group subscriptions.
func (s *SubscriptionService) getGroupSubscriptionLatestItems(
	ctx context.Context,
	count int,
	subscriptions models.Subscriptions,
	view models.View,
) map[models.SubscriptionID]models.Items {
	groupLatestItems := make(map[models.SubscriptionID]models.Items)
	var (
		wg sync.WaitGroup
		mu sync.Mutex
	)
	for subscription := range slices.Values(subscriptions) {
		wg.Go(func() {
			// Get details of all subscriptions that comprise the group.
			allSubscriptions := models.SubscriptionsFromCtx(ctx)
			if len(allSubscriptions) == 0 {
				slogctx.Warn(ctx, "Could not retrieve user subscriptions from context.")
				return
			}
			childSubscriptions := allSubscriptions.FilterByIDs(subscription.GroupData.GetGroupedSubscriptionIDs()...)
			if len(childSubscriptions) == 0 {
				slogctx.Warn(ctx, "Could not retrieve grouped subscriptions.")
				return
			}
			// Get latest items for these subscriptions.
			latestItems, err := s.getFeedSubscriptionLatestItems(ctx, count, childSubscriptions, view)
			// latestItems, err := getFeedSubscriptionLatestItems(ctx, childSubscriptions, filters)
			if err != nil {
				slogctx.FromCtx(ctx).Warn("Unable to get latest items for group subscription.",
					slog.Any("error", err),
				)
				return
			}
			// Concat all items from all subscriptions into the group subscription items list.
			for _, items := range latestItems {
				mu.Lock()
				groupLatestItems[subscription.GetID()] = slices.Concat(groupLatestItems[subscription.GetID()], items)
				// Sort the combined items list.
				groupLatestItems[subscription.GetID()].SortByTimestamp()
				// Truncate the list to the first 3 items if greater than 3.
				if len(groupLatestItems[subscription.GetID()]) > 3 {
					groupLatestItems[subscription.GetID()] = groupLatestItems[subscription.GetID()][:3]
				}
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	return groupLatestItems
}

// getSearchSubscriptionLatestItems will return a map of latest items per subscription for the given search
// subscriptions.
func (s *SubscriptionService) getSearchSubscriptionLatestItems(
	ctx context.Context,
	count int,
	subscriptions models.Subscriptions,
) (map[models.SubscriptionID]models.Items, error) {
	itemSvc, err := LoadItemService()
	if err != nil {
		return nil, fmt.Errorf("load items service: %w", err)
	}

	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("%w: could not find user", models.ErrCtxValueNotFound)
	}

	searchTopItems := make(map[models.SubscriptionID]models.Items)
	var (
		wg sync.WaitGroup
		mu sync.Mutex
	)

	for subscription := range slices.Values(subscriptions) {
		wg.Go(func() {
			request := subscription.SearchData.Search
			request.Count = count
			request.Sort = models.SortNewestFirst
			items, _, err := itemSvc.RetrieveItems(ctx, &request)
			if err != nil && !errors.Is(err, models.ErrNotFound) {
				slogctx.FromCtx(ctx).Warn("Get search results for search subscription failed.",
					slog.String("subscription_id", subscription.GetID()),
					slog.Any("error", err),
				)
				return
			}
			// Add to the subscription top items.
			mu.Lock()
			searchTopItems[subscription.GetID()] = items
			mu.Unlock()
		})
	}
	wg.Wait()
	return searchTopItems, nil
}

func (s *SubscriptionService) getSubscriptionUnreadCounts(
	ctx context.Context,
	subscriptions models.Subscriptions,
) (map[models.FeedID]int64, error) {
	// Retrieve user object.
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound)
	}

	// Generate clauses for aggregation filter buckets.
	subscriptionFilters := make(map[string]*estypes.Query)
	for subscription := range slices.Values(subscriptions) {
		subscriptionFilters[subscription.GetFeedID()] = query.Build(
			unreadItemsForSubscriptionClause(subscription, user.GetMaxHistory()),
		)
	}

	feedIDs := subscriptions.GetFeedIDs()

	// Perform aggregation.
	resp, err := elastic.Search[*models.Item](ctx,
		s.store.GetIndexRO(ItemsIndex),
		elastic.WithQueryOptions[*elastic.SearchRequest](
			query.Bool(
				query.Filter(
					query.Terms(
						"feed_id",
						feedIDs,
						query.WithQueryName[*query.TermsQuery]("match-feed-id"),
					),
					query.Bool(
						ArticleFiltersQueryClause(user.GetSettings().GlobalFilters),
					),
				),
			),
		),
		elastic.WithAggregations(
			elastic.Aggs{
				"UnreadCounts": estypes.Aggregations{
					Filters: &estypes.FiltersAggregation{
						Filters: subscriptionFilters,
					},
				},
			},
		),
		elastic.WithSize(0),
		elastic.WithDocSorting(),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to get subscription unread counts: %w", err)
	}

	// Extract the feed aggregation.
	unreadCountsAggs, aggFound, err := elastic.ExtractAggregation[*estypes.FiltersAggregate](
		resp.Aggregations,
		"UnreadCounts",
	)
	if !aggFound || err != nil {
		return nil, fmt.Errorf("extract feed aggregation: %w", err)
	}
	// Loop over the feed buckets.
	stats := make(map[models.SubscriptionID]int64)
	feedBuckets, err := elastic.ExtractBucketsAsMap[estypes.FiltersBucket](unreadCountsAggs.Buckets)
	if err != nil {
		return nil, fmt.Errorf("extract feed aggregation buckets: %w", err)
	}
	for feedID, bucket := range feedBuckets {
		if feedID == "" {
			continue
		}
		// Get the subscription with this feedID.
		if !slices.Contains(feedIDs, feedID) {
			slogctx.FromCtx(ctx).
				Warn("Could not match feed in aggregation result to a subscription.",
					slog.String("feed_id", feedID),
					slog.Int64("doc_count", bucket.DocCount),
				)
			continue
		}
		stats[feedID] = bucket.DocCount
	}

	return stats, nil
}

// NewFeedSubscription creates a new subscription for a feed with any user customisations given.
func NewFeedSubscription(
	ctx context.Context,
	feed *models.Feed,
	customisation *models.SubscriptionCustomisation,
) (*models.Subscription, error) {
	// Create state based on feed and user data.
	feedSubscription := &models.FeedSubscription{
		FeedID:        feed.GetID(),
		ArticleStates: make(map[models.ItemID]models.ArticleState),
	}

	// Set up subscription customisation.
	if customisation == nil {
		customisation = &models.SubscriptionCustomisation{}
	}
	// Make sure nickname is not empty.
	if customisation.GetNickname() == "" {
		customisation.Nickname = new(feed.GetTitle())
	}
	// Add the feed image if the user has not specified one.
	if customisation.ImageURL == nil && feed.GetImage() != nil {
		customisation.ImageURL = new(feed.GetImage().GetURL())
	}

	// Create the subscription with the feed data and customisations.
	subscription, err := newBaseSubscription(ctx, *customisation, &models.SubscriptionSettings{}, feedSubscription)
	if err != nil {
		return nil, fmt.Errorf("new feed subscription: %w", err)
	}

	// Validate.
	if err := subscription.Validate(); err != nil {
		return nil, fmt.Errorf("new feed subscription: %w", err)
	}

	return subscription, nil
}

func EditFeedSubscription(
	ctx context.Context,
	subscription *models.Subscription,
	edits *models.FeedSubscriptionRequest,
) error {
	// Update customisation.
	subscription.Customisation = edits.Customisation
	// Update settings.
	if edits.Settings != nil {
		subscription.Settings = *edits.Settings
	}
	// Update article filters.
	if edits.ArticleFilters != nil {
		if subscription.FeedData.ArticleFilters == nil {
			subscription.FeedData.ArticleFilters = &models.ArticleFilters{}
		}
		if edits.ArticleFilters.Text != nil {
			subscription.FeedData.ArticleFilters.Text = edits.ArticleFilters.Text
		}
		if edits.ArticleFilters.Authors != nil {
			subscription.FeedData.ArticleFilters.Authors = edits.ArticleFilters.Authors
		}
		if edits.ArticleFilters.Categories != nil {
			subscription.FeedData.ArticleFilters.Categories = edits.ArticleFilters.Categories
		}
	}

	return nil
}

// NewGroupSubscription creates a GroupSubscription. A GroupSubscription is a kind of meta-subscription that aggregates
// all articles from multiple individual subscriptions into a single custom subscription.
func NewGroupSubscription(ctx context.Context, request *models.GroupSubscriptionRequest) (*models.Subscription, error) {
	// Get user subscriptions. This should always return at least one subscription, otherwise this request is invalid.
	allSubscriptions := models.SubscriptionsFromCtx(ctx)
	if len(allSubscriptions) == 0 {
		return nil, fmt.Errorf("get user subscriptions: %w", models.ErrCtxValueNotFound)
	}
	grouped := allSubscriptions.FilterByIDs(slices.Collect(maps.Keys(request.Subscriptions))...)
	if len(grouped) == 0 {
		return nil, fmt.Errorf("get grouped subscriptions: %w", models.ErrCtxValueNotFound)
	}
	groupSubscription := &models.GroupSubscription{
		Metadata: make([]models.GroupedSubscriptionMetadata, 0, len(grouped)),
	}
	for subscription := range slices.Values(grouped) {
		groupSubscription.Metadata = append(groupSubscription.Metadata, models.GroupedSubscriptionMetadata{
			SubscriptionID: subscription.GetID(),
			FeedID:         subscription.GetFeedID(),
		})
	}

	subscription, err := newBaseSubscription(ctx, *request.Customisation, request.Settings, groupSubscription)
	if err != nil {
		return nil, fmt.Errorf("create subscription: %w", err)
	}
	subscription.Favorite = true
	return subscription, nil
}

func EditGroupSubscription(
	ctx context.Context,
	subscription *models.Subscription,
	edits *models.GroupSubscriptionRequest,
) error {
	// Update customisation.
	subscription.Customisation = edits.Customisation
	// Update settings.
	if edits.Settings != nil {
		subscription.Settings = *edits.Settings
	}

	// Get user subscriptions. This should always return at least one subscription, otherwise this request is invalid.
	allSubscriptions := models.SubscriptionsFromCtx(ctx)
	if len(allSubscriptions) == 0 {
		return fmt.Errorf("get user subscriptions: %w", models.ErrCtxValueNotFound)
	}
	grouped := allSubscriptions.FilterByIDs(slices.Collect(maps.Keys(edits.Subscriptions))...)
	if len(grouped) == 0 {
		return fmt.Errorf("get grouped subscriptions: %w", models.ErrCtxValueNotFound)
	}

	subscription.GroupData.Subscriptions = grouped
	subscription.GroupData.Metadata = make([]models.GroupedSubscriptionMetadata, 0, len(grouped))
	for groupedSubscription := range slices.Values(grouped) {
		subscription.GroupData.Metadata = append(subscription.GroupData.Metadata, models.GroupedSubscriptionMetadata{
			SubscriptionID: groupedSubscription.GetID(),
			FeedID:         groupedSubscription.GetFeedID(),
		})
	}
	// Update article filters.
	if edits.ArticleFilters != nil {
		if subscription.GroupData.ArticleFilters == nil {
			subscription.GroupData.ArticleFilters = &models.ArticleFilters{}
		}
		if edits.ArticleFilters.Text != nil {
			subscription.GroupData.ArticleFilters.Text = edits.ArticleFilters.Text
		}
		if edits.ArticleFilters.Authors != nil {
			subscription.GroupData.ArticleFilters.Authors = edits.ArticleFilters.Authors
		}
		if edits.ArticleFilters.Categories != nil {
			subscription.GroupData.ArticleFilters.Categories = edits.ArticleFilters.Categories
		}
	}
	return nil
}

// NewSearchSubscription creates a new SearchSubscription. A SearchSubscription collates articles that match a search
// into a single custom subscription.
func NewSearchSubscription(
	ctx context.Context,
	request *models.SearchSubscriptionRequest,
) (*models.Subscription, error) {
	searchSubscription := &models.SearchSubscription{
		Search: request.Search,
	}
	subscription, err := newBaseSubscription(ctx, *request.Customisation, request.Settings, searchSubscription)
	if err != nil {
		return nil, fmt.Errorf("new search subscription: %w", err)
	}
	subscription.Favorite = true
	return subscription, nil
}

func EditSearchSubscription(
	ctx context.Context,
	subscription *models.Subscription,
	edits *models.SearchSubscriptionRequest,
) error {
	// Update customisation.
	subscription.Customisation = edits.Customisation
	// Update settings.
	if edits.Settings != nil {
		subscription.Settings = *edits.Settings
	}
	// Update search.
	subscription.SearchData.Search = edits.Search
	return nil
}

func NewEmailSubscription(
	ctx context.Context,
	userID models.UserID,
	from *mail.Address,
) (*models.Subscription, error) {
	// Validate sender address.
	if from.Address == "" {
		return nil, fmt.Errorf("%w: blank sender address", validation.ErrInvalid)
	}
	if err := validation.Validate.Var(from.Address, "required,email"); err != nil {
		return nil, fmt.Errorf("%w: sender address: %w", validation.ErrInvalid, err)
	}

	emailSubscription := &models.EmailSubscription{
		EmailSenderID: from.Address,
	}
	customisation := &models.SubscriptionCustomisation{
		Nickname: new(from.String()),
	}
	settings := &models.SubscriptionSettings{}

	subscription, err := newBaseSubscription(ctx, *customisation, settings, emailSubscription)
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

func EditEmailSubscription(
	ctx context.Context,
	subscription *models.Subscription,
	edits *models.EditEmailSubscriptionRequest,
) error {
	// Update customisation.
	subscription.Customisation = edits.Customisation
	// Update settings.
	if edits.Settings != nil {
		subscription.Settings = *edits.Settings
	}
	return nil
}

func newBaseSubscription(
	ctx context.Context,
	customisation models.SubscriptionCustomisation,
	settings *models.SubscriptionSettings,
	data any,
) (*models.Subscription, error) {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound)
	}
	ts := time.Now().UTC()
	maxHistory := user.GetMaxHistory()
	subscription := &models.Subscription{
		SubscriptionID: "sub_" + strconv.FormatUint(xxh3.Hash([]byte(user.GetID()+customisation.GetNickname())), 10),
		UserID:         user.GetID(),
		UpdatedAt:      &ts,
		CreatedAt:      ts,
		MarkedReadAt:   &maxHistory,
		Customisation:  &customisation,
		Settings:       models.SubscriptionSettings{},
		Favorite:       false,
	}
	if settings != nil {
		subscription.Settings = *settings
	}

	switch typeData := data.(type) {
	case *models.FeedSubscription:
		subscription.Type = models.SubscriptionTypeFeed
		subscription.FeedData = typeData
	case *models.SearchSubscription:
		subscription.Type = models.SubscriptionTypeSearch
		subscription.SearchData = typeData
		subscription.Favorite = true
	case *models.GroupSubscription:
		subscription.Type = models.SubscriptionTypeGroup
		subscription.GroupData = typeData
		subscription.Favorite = true
	case *models.EmailSubscription:
		subscription.Type = models.SubscriptionTypeEmail
		subscription.EmailData = typeData
	default:
		return nil, fmt.Errorf("new subscription: %w", models.ErrInvalidAPIResult)
	}

	return subscription, nil
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

func newSubscriptionSortOptions(sort *models.Sort) []estypes.SortCombinationsVariant {
	if sort == nil {
		return []estypes.SortCombinationsVariant{&estypes.SortOptions{Doc_: estypes.NewScoreSort()}}
	}
	var opts []estypes.SortCombinationsVariant
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

func mergeMaps[K comparable, V any](sources ...map[K]V) map[K]V {
	size := 0
	for src := range slices.Values(sources) {
		size += len(src)
	}
	result := make(map[K]V, size)
	for src := range slices.Values(sources) {
		maps.Copy(result, src)
	}
	return result
}
