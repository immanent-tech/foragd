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
	"net/http"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v9/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	slogctx "github.com/veqryn/slog-context"
	"github.com/zeebo/xxh3"
	"golang.org/x/sync/errgroup"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/bulk"
	"github.com/immanent-tech/foragd/providers/elastic/query"
)

type ImportService struct {
	store *ElasticService
}

func NewImportService() (*ImportService, error) {
	svc, err := LoadElasticService()
	if err != nil {
		return nil, fmt.Errorf("load elastic service: %w", err)
	}

	return &ImportService{
		store: svc,
	}, nil
}

// StartImport generates a [models.ImportStatus] with a slice of [models.ImportRequest] for all feeds found in the given
// OPML file. An import job run by the scheduler will find the status and process it.
func (i *ImportService) StartImport(
	ctx context.Context,
	file *models.OPMLFile,
) (string, error) {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return "", models.NewAPIError(
			http.StatusInternalServerError,
			fmt.Errorf("get user details: %w", models.ErrCtxValueNotFound))
	}

	// Check if there is an existing import running or pending for this user. Bail if so.
	activeImports, err := i.findActiveImports(ctx, user.GetID())
	if err != nil {
		return "", models.NewAPIError(
			http.StatusInternalServerError,
			fmt.Errorf("count existing imports: %w", err))
	}
	if len(activeImports) > 0 {
		return "", models.NewAPIError(
			http.StatusConflict,
			errors.New("count existing imports: existing import found"),
			models.WithUserMessage(models.NewErrorMessage(
				"Existing import running",
				"An existing import is running. Please wait until that import has finished before starting another.",
			)),
		)
	}

	// Generate requests from OPML file.
	requests, err := i.generateRequests(file)
	if err != nil {
		return "", models.NewAPIError(
			http.StatusBadRequest,
			fmt.Errorf("generate requests from opml: %w", err),
			models.WithUserMessage(models.NewWarningMessage(
				"Invalid OPML data",
				"The import failed as the OPML file had errors. Please check and try again.",
			)),
		)

	}
	if len(requests) == 0 {
		return "", models.NewAPIError(
			http.StatusNoContent,
			errors.New("no requests"),
			models.WithUserMessage(models.NewWarningMessage(
				"No feeds found",
				"The import failed as no feeds could be read from the OPML file. Please check and try again.",
			)),
		)
	}

	// Create the import status.
	status := &models.ImportStatus{
		CreatedAt: time.Now().UTC(),
		JobID:     "import_" + strconv.FormatUint(xxh3.Hash([]byte(user.GetID()+time.Now().String())), 10),
		Requests:  requests,
		Status:    models.ImportStatusStatusPending,
		UserID:    user.GetID(),
		DocType:   "import_status",
	}

	// Add to the store for the import job to find and process.
	if err := elastic.CreateDoc(ctx, i.store.GetIndexRW(ImportIndex), status.GetID(), status); err != nil {
		return "", models.NewAPIError(
			http.StatusInternalServerError,
			fmt.Errorf("get user details: %w", models.ErrCtxValueNotFound))
	}

	return status.GetID(), nil
}

// ProcessRequests handles processing an [models.ImportStatus] to generate subscriptions for a user.
func (i *ImportService) ProcessRequests(
	ctx context.Context,
	status *models.ImportStatus,
) error {
	ctx = slogctx.With(ctx,
		slog.String("import_id", status.GetID()),
		slog.String("user_id", status.UserID),
	)

	slogctx.Info(ctx, "Processing user import")

	// Sort requests.
	slices.SortFunc(status.Requests, func(a, b models.ImportRequest) int {
		return strings.Compare(a.URL, b.URL)
	})
	// Remove duplicates.
	status.Requests = slices.CompactFunc(status.Requests, func(a, b models.ImportRequest) bool {
		return a.URL == b.URL
	})

	feedSvc, err := LoadFeedService()
	if err != nil {
		if err := i.updateStatus(ctx, status, models.ImportStatusStatusFailed, nil); err != nil {
			slogctx.Error(ctx, "Could not update status", slog.Any("error", err))
		}
		return fmt.Errorf("load feed service: %w", err)
	}

	userSvc, err := LoadUserService()
	if err != nil {
		if err := i.updateStatus(ctx, status, models.ImportStatusStatusFailed, nil); err != nil {
			slogctx.Error(ctx, "Could not update status", slog.Any("error", err))
		}
		return fmt.Errorf("load user service: %w", err)
	}

	subSvc, err := LoadSubscriptionService()
	if err != nil {
		if err := i.updateStatus(ctx, status, models.ImportStatusStatusFailed, nil); err != nil {
			slogctx.Error(ctx, "Could not update status", slog.Any("error", err))
		}
		return fmt.Errorf("load subscription service: %w", err)
	}

	user, err := userSvc.GetUser(ctx, status.UserID)
	if err != nil {
		if err := i.updateStatus(ctx, status, models.ImportStatusStatusFailed, nil); err != nil {
			slogctx.Error(ctx, "Could not update status", slog.Any("error", err))
		}
		return fmt.Errorf("get user details: %w", err)
	}
	ctx = models.UserToCtx(ctx, user)

	if err := i.updateStatus(ctx, status, models.ImportStatusStatusRunning, nil); err != nil {
		slogctx.Error(ctx, "Could not update status", slog.Any("error", err))
		return fmt.Errorf("update status to running: %w", err)
	}

	// Process requests.
	type result struct {
		Details      *models.ImportResult
		Subscription *models.Subscription
		AddFeed      bool
		Feed         *models.Feed
	}

	// Set up parallel processing of requests.
	maxWorkers := min(runtime.GOMAXPROCS(0)*4, 100)
	resultsCh := make(chan result, len(status.Requests))
	var g errgroup.Group
	g.SetLimit(maxWorkers)
	start := time.Now()

	// Start parallel processing of requests.
	for request := range slices.Values(status.Requests) {
		g.Go(func() error {
			requestCtx := slogctx.With(ctx, slog.String("source_url", request.URL))

			slogctx.Info(requestCtx, "Processing subscription request")
			start := time.Now()

			// Create result object.
			result := result{
				Details: &models.ImportResult{
					Categories: request.Categories,
					URL:        request.URL,
					JobID:      status.GetID(),
					DocType:    "import_result",
				},
			}

			// Check and create a feed as needed.
			feed, addFeed, err := i.checkFeeds(ctx, feedSvc, request.URL)
			if err != nil {
				slogctx.Error(ctx, "Unable check feeds.", slog.Any("error", err))
				if apiErr, ok := errors.AsType[*models.APIError](err); ok {
					result.Details.Error = apiErr.GetUserMessage()
				} else {
					result.Details.Error = models.NewErrorMessage(
						"Could not fetch feed data from url",
						request.URL,
					)
				}
				resultsCh <- result
				return nil
			}
			result.Feed = feed
			result.AddFeed = addFeed

			// Check and create subscription as needed.
			subscription, err := i.checkSubscriptions(ctx, subSvc, result.Feed, request.URL)
			if err != nil {
				slogctx.Error(ctx, "Unable check subscriptions.", slog.Any("error", err))
				if apiErr, ok := errors.AsType[*models.APIError](err); ok {
					result.Details.Error = apiErr.GetUserMessage()
				} else {
					result.Details.Error = models.NewErrorMessage(
						"Could not check existing subscriptions for feed",
						request.URL,
					)
				}
				resultsCh <- result
				return nil
			}
			// Add any categories from the OPML to the subscription.
			if len(request.Categories) > 0 {
				subscription.Customisation.Categories = request.Categories
			}
			result.Details.SubscriptionID = new(subscription.GetID())
			result.Subscription = subscription

			slogctx.Info(requestCtx, "Subscription request processed",
				slog.Duration("took", time.Since(start)),
			)

			resultsCh <- result
			return nil
		})
	}

	_ = g.Wait()
	close(resultsCh)

	// Process results.
	var errs error
	for result := range resultsCh {
		// Add new feed as needed.
		if result.AddFeed {
			if err := bulk.AddAction(ctx,
				bulk.NewAction(
					result.Feed,
					bulk.AsOperation[models.FeedID](bulk.OpCreate),
					bulk.ToIndex[models.FeedID](i.store.GetIndexRW(FeedsIndex)),
				),
			); err != nil {
				slogctx.Error(ctx, "Failed to process new feed",
					slog.Any("error", err))
				errs = errors.Join(errs, err)
				continue
			}
		}
		// Add new subscription as needed.
		if result.Subscription != nil {
			if err := bulk.AddAction(ctx,
				bulk.NewAction(
					result.Subscription,
					bulk.AsOperation[models.SubscriptionID](bulk.OpCreate),
					bulk.ToIndex[models.SubscriptionID](i.store.GetIndexRW(SubscriptionsIndex)),
				),
			); err != nil {
				slogctx.Error(ctx, "Failed to process new subscription",
					slog.Any("error", err))
				errs = errors.Join(errs, err)
				continue
			}
		}
		// Index the results.
		if err := bulk.AddAction(ctx,
			bulk.NewAction(
				result.Details,
				bulk.AsOperation[string](bulk.OpCreate),
				bulk.ToIndex[string](i.store.GetIndexRW(ImportIndex)),
			),
		); err != nil {
			slogctx.Error(ctx, "Failed to process import result",
				slog.Any("error", err))
			errs = errors.Join(errs, err)
			continue
		}
	}

	if err := bulk.Flush(ctx); err != nil {
		slogctx.Warn(ctx, "Could not flush bulk indexer after adding subscriptions.",
			slog.Any("error", err),
		)
	}

	if err := i.updateStatus(ctx, status, models.ImportStatusStatusDone, nil); err != nil {
		slogctx.Error(ctx, "Could not update status",
			slog.String("status", string(status.Status)),
			slog.Any("error", err),
		)
	}

	slogctx.Info(ctx, "User import processed",
		slog.Duration("took", time.Since(start)),
	)

	return errs
}

func (i *ImportService) checkFeeds(
	ctx context.Context,
	feedSvc *FeedService,
	requestURL string,
) (*models.Feed, bool, error) {
	// Fetch feed details from remote URL.
	feed, err := feedSvc.FetchFeed(ctx, requestURL)
	if err != nil {
		return nil, false, fmt.Errorf("fetch feed: %w", err)
	}

	existingFeeds, err := feedSvc.GetFeedsByURLs(ctx, feed.GetSourceURLs()...)
	if err != nil {
		return nil, false, fmt.Errorf("match existing feeds: %w", err)
	}
	switch {
	case len(existingFeeds) > 1:
		// Multiple feeds match new feed URLs! This should not happen.
		return nil, false, models.NewAPIError(
			http.StatusConflict,
			fmt.Errorf("%q: multiple matches", requestURL),
			models.WithUserMessage(models.NewErrorMessage(
				"Ambiguous feed match",
				"Could not determine which existing feed this URL is for: "+requestURL,
			)),
		)
	case len(existingFeeds) == 1:
		// There is an existing feed. Return its details but do not set create flag.
		return feed, false, nil
	default:
		return feed, true, nil
	}
}

func (i *ImportService) checkSubscriptions(
	ctx context.Context,
	subSvc *SubscriptionService,
	feed *models.Feed,
	requestURL string,
) (*models.Subscription, error) {
	// Check for an existing subscription to this feed.
	allSubscriptions, err := subSvc.GetAllSubscriptions(ctx)
	if err != nil {
		return nil, models.NewAPIError(http.StatusInternalServerError, fmt.Errorf("get user subscriptions: %w", err))
	}
	existingSubscriptions := allSubscriptions.FilterByFeedIDs(feed.GetID())
	if len(existingSubscriptions) > 0 {
		return nil, models.NewAPIError(
			http.StatusConflict,
			fmt.Errorf("%s: already subscribed", feed.GetID()),
			models.WithUserMessage(models.NewWarningMessage(
				fmt.Sprintf("Already subscribed to feed (%s)", existingSubscriptions[0].GetTitle()),
				requestURL,
			)),
		)
	}

	// Create new subscription.
	newSubscription, err := NewFeedSubscription(ctx, feed, nil)
	if err != nil {
		return nil, models.NewAPIError(
			http.StatusInternalServerError,
			fmt.Errorf("create subscription: %w", err),
			models.WithUserMessage(models.NewErrorMessage(
				"Unable to add subscription",
				"Failed to add subscription for URL: "+requestURL,
			)),
		)
	}
	return newSubscription, nil
}

func (i *ImportService) GetImportStatus(
	ctx context.Context,
	jobID string,
) (*models.ImportStatus, []*models.ImportResult, error) {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, nil, models.NewAPIError(
			http.StatusInternalServerError,
			fmt.Errorf("get user details: %w", models.ErrCtxValueNotFound))
	}

	var status *models.ImportStatus
	if jobID == "" {
		activeImports, err := i.findActiveImports(ctx, user.GetID())
		if err != nil {
			return nil, nil, models.NewAPIError(
				http.StatusInternalServerError,
				fmt.Errorf("find active imports: %w", err),
			)
		}
		if len(activeImports) > 1 {
			activeIDs := make([]string, 0, len(activeImports))
			for activeImport := range slices.Values(activeImports) {
				activeIDs = append(activeIDs, activeImport.GetID())
			}
			slogctx.Warn(ctx, "User has more than 1 active import.",
				slog.String("user_id", user.GetID()),
				slog.String("active_imports", strings.Join(activeIDs, ",")),
			)
		}
		if len(activeImports) == 1 {
			status = activeImports[0]
		} else {
			return nil, nil, models.NewAPIError(
				http.StatusNoContent,
				errors.New("search import status: no recent status"),
				models.WithUserMessage(models.NewWarningMessage(
					"No imports",
					"No recent running/pending imports found",
				)),
			)
		}
	} else {
		// Else, fetch the job status directly.
		existing, err := elastic.GetDoc[string, *models.ImportStatus](ctx, i.store.GetIndexRO(ImportIndex), jobID)
		if err != nil {
			if errors.Is(err, elastic.ErrNotFound) {
				return nil, nil, models.NewAPIError(
					http.StatusNoContent,
					errors.New("search import status: no recent status"),
					models.WithUserMessage(models.NewWarningMessage(
						"No imports",
						"No recent running/pending imports found",
					)),
				)
			}
			return nil, nil, models.NewAPIError(
				http.StatusInternalServerError,
				fmt.Errorf("search import status: %w", err))
		}
		status = existing
	}

	results, err := i.getResultsForImport(ctx, status.GetID())
	if err != nil {
		return nil, nil, models.NewAPIError(
			http.StatusInternalServerError,
			fmt.Errorf("get results: %w", err))
	}
	return status, results, nil
}

// GetPendingImports retrieves any imports in "pending" status (i.e., ready to be started).
func (i *ImportService) GetPendingImports(ctx context.Context) ([]*models.ImportStatus, error) {
	statuses, err := elastic.SearchAll[*models.ImportStatus](ctx,
		i.store.GetIndexRO(ImportIndex),
		query.Bool(
			query.Filter(
				query.Term("doc_type", "import_status"),
				query.Term("status", models.ImportStatusStatusPending),
			),
		),
		5000,
	)
	if err != nil {
		return nil, fmt.Errorf("get all pending imports: %w", err)
	}

	return statuses, nil
}

func (i *ImportService) GetAllImports(ctx context.Context, userID models.UserID) ([]*models.ImportStatus, error) {
	activeImports, err := elastic.SearchAll[*models.ImportStatus](ctx,
		i.store.GetIndexRO(ImportIndex),
		query.Bool(
			// Is Import Status and owned by User.
			query.Filter(
				query.Term("doc_type", "import_status"),
				query.Term("user_id", userID),
			),
		),
		5000,
		elastic.WithSort[*search.Search](&statusSorting{CreatedAt: "desc"}),
	)
	if err != nil {
		return nil, fmt.Errorf("find active imports: %w", err)
	}
	return activeImports, nil
}

func (i *ImportService) findActiveImports(ctx context.Context, userID models.UserID) ([]*models.ImportStatus, error) {
	activeImports, err := elastic.SearchAll[*models.ImportStatus](ctx,
		i.store.GetIndexRO(ImportIndex),
		query.Bool(
			query.WithBoolShouldMinimumMatch(1),
			// Is Import Status and owned by User.
			query.Filter(
				query.Term("doc_type", "import_status"),
				query.Term("user_id", userID),
			),
			// Status is Pending or Running.
			query.Should(
				query.Term("status", models.ImportStatusStatusPending),
				query.Term("status", models.ImportStatusStatusRunning),
			),
		),
		5000,
		elastic.WithSort[*search.Search](&statusSorting{CreatedAt: "desc"}),
	)
	if err != nil {
		return nil, fmt.Errorf("find active imports: %w", err)
	}
	return activeImports, nil
}

func (i *ImportService) getResultsForImport(ctx context.Context, id string) ([]*models.ImportResult, error) {
	results, err := elastic.SearchAll[*models.ImportResult](ctx,
		i.store.GetIndexRO(ImportIndex),
		query.Bool(
			query.Filter(
				query.Term("doc_type", "import_result"),
				query.Term("job_id", id),
			),
		),
		5000,
	)
	if err != nil {
		return nil, fmt.Errorf("get all results: %w", err)
	}
	return results, nil
}

func (i *ImportService) generateRequests(file *models.OPMLFile) ([]models.ImportRequest, error) {
	requests, err := file.GenerateRequests()
	if err != nil {
		return nil, fmt.Errorf("opml: get requests: %w", err)
	}
	return requests, nil
}

func (i *ImportService) updateStatus(
	ctx context.Context,
	status *models.ImportStatus,
	result models.ImportStatusStatus,
	msg *models.UserMessage,
) error {
	status.Status = result
	if msg != nil {
		status.Error = msg
	} else {
		msg = models.NewErrorMessage(
			"Internal server error",
			"The server encountered a problem that stopped the import process completing",
		)
	}
	if err := elastic.UpdateDoc(ctx, i.store.GetIndexRW(ImportIndex), status.GetID(), status); err != nil {
		return fmt.Errorf("update doc: %w", err)
	}
	return nil
}

type statusSorting struct {
	CreatedAt string `json:"created_at"`
}

// SortCombinationsCaster is required to allow ItemSorting to be used as Elasticsearch sort values.
func (s *statusSorting) SortCombinationsCaster() *types.SortCombinations {
	c := types.SortCombinations(s)
	return &c
}
