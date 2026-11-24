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

	slogctx "github.com/veqryn/slog-context"
	"github.com/zeebo/xxh3"
	"golang.org/x/sync/errgroup"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/opensearch"
	"github.com/immanent-tech/foragd/providers/opensearch/bulk"
	"github.com/immanent-tech/foragd/providers/opensearch/doc"
	"github.com/immanent-tech/foragd/providers/opensearch/search"
	"github.com/immanent-tech/foragd/schema"
)

type ImportService struct {
	osc     *opensearch.Client
	indexer *bulk.Indexer

	feedsSvc         *FeedService
	subscriptionsSvc *SubscriptionService
	usersSvc         *UserService

	closer closeOnce
}

func LoadImportsService(
	ctx context.Context,
	osc *opensearch.Client,
	indexer *bulk.Indexer,
	feedsSvc *FeedService,
	subscriptionsSvc *SubscriptionService,
	usersSvc *UserService,
) (*ImportService, error) {
	if osc == nil {
		return nil, errors.New("no opensearch client")
	}
	if indexer == nil {
		return nil, errors.New("no indexer")
	}
	if feedsSvc == nil {
		return nil, errors.New("no feeds service")
	}
	if subscriptionsSvc == nil {
		return nil, errors.New("no subscriptions service")
	}
	if usersSvc == nil {
		return nil, errors.New("no users service")
	}

	return &ImportService{
		osc:              osc,
		indexer:          indexer,
		feedsSvc:         feedsSvc,
		subscriptionsSvc: subscriptionsSvc,
		usersSvc:         usersSvc,
	}, nil
}

func (i *ImportService) Close() error {
	shutdownCtx, cancel := context.WithTimeoutCause(
		context.Background(),
		20*time.Second,
		errors.New("graceful shutdown timeout"),
	)
	shutdownCtx = slogctx.NewCtx(shutdownCtx, slog.Default())
	defer cancel()

	slogctx.Info(shutdownCtx, "Shutting down imports service")

	return i.closer.do(func() error {
		return nil
	})
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
	if err := doc.Create(ctx, i.osc.Doc, i.osc.IndexWriteAlias(schema.ImportsIndex), status); err != nil {
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

	user, err := i.usersSvc.GetUser(ctx, status.UserID)
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
			feed, addFeed, err := i.checkFeeds(ctx, request.URL)
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
			subscription, err := i.checkSubscriptions(ctx, result.Feed, request.URL)
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
			if err := i.indexer.Add(
				ctx,
				bulk.NewAction(bulk.ActionCreate,
					bulk.ToIndex(i.osc.IndexWriteAlias(schema.FeedsIndex)),
					bulk.WithID(result.Feed.GetID()),
					bulk.WithDoc(result.Feed),
				),
			); err != nil {
				slogctx.Error(ctx, "Failed to process new feed",
					slog.Any("error", err))
				errs = errors.Join(errs, err)
				// continue
			}
		}
		// Add new subscription as needed.
		if result.Subscription != nil {
			if err := i.indexer.Add(
				ctx,
				bulk.NewAction(bulk.ActionCreate,
					bulk.ToIndex(i.osc.IndexWriteAlias(schema.SubscriptionsIndex)),
					bulk.WithID(result.Subscription.GetID()),
					bulk.WithDoc(result.Subscription),
				),
			); err != nil {
				slogctx.Error(ctx, "Failed to process new subscription",
					slog.Any("error", err))
				errs = errors.Join(errs, err)
				// continue
			}
		}
		// Index the results.
		if err := i.indexer.Add(ctx, bulk.NewAction(bulk.ActionCreate,
			bulk.ToIndex(i.osc.IndexWriteAlias(schema.ImportsIndex)),
			bulk.WithID(result.Details.JobID),
			bulk.WithDoc(result.Details),
		)); err != nil {
			slogctx.Error(ctx, "Failed to process import result",
				slog.Any("error", err))
			errs = errors.Join(errs, err)
			// continue
		}
	}

	if err := i.indexer.Flush(ctx); err != nil {
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
	requestURL string,
) (*models.Feed, bool, error) {
	// Fetch feed details from remote URL.
	feed, err := i.feedsSvc.FetchFeed(ctx, requestURL)
	if err != nil {
		return nil, false, fmt.Errorf("fetch feed: %w", err)
	}

	existingFeeds, err := i.feedsSvc.GetFeedsByURLs(ctx, feed.GetSourceURLs()...)
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
		// If the existing feed has fetching disabled, re-enable fetching.
		if feed.IsDisabled() {
			feed.SetDisabled(true)
			if err := i.feedsSvc.UpdateFeed(ctx, feed); err != nil {
				slogctx.Warn(ctx, "Could not re-enable disabled feed.", slog.Any("error", err))
			} else {
				slogctx.Info(ctx, "Re-enabled disabled feed.")
			}
		}
		// There is an existing feed. Return its details but do not set create flag.
		return feed, false, nil
	default:
		return feed, true, nil
	}
}

func (i *ImportService) checkSubscriptions(
	ctx context.Context,
	feed *models.Feed,
	requestURL string,
) (*models.Subscription, error) {
	// Check for an existing subscription to this feed.
	allSubscriptions, err := i.subscriptionsSvc.GetAllSubscriptions(ctx)
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
		existing, err := doc.Get[string, *models.ImportStatus](
			ctx,
			i.osc.Doc,
			i.osc.IndexReadAlias(schema.ImportsIndex),
			jobID,
		)
		if err != nil {
			if apiErr, ok := errors.AsType[*models.APIError](err); ok && apiErr.HTTPStatus() == http.StatusNotFound {
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
	statuses, err := i.searchAllImports[*models.ImportStatus](ctx,
		search.BoolQuery(
			search.WithBoolFilter(
				search.TermQuery("doc_type", "import_status"),
				search.TermQuery("status", models.ImportStatusStatusPending),
			),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("get all pending imports: %w", err)
	}

	return statuses, nil
}

func (i *ImportService) GetAllImports(ctx context.Context, userID models.UserID) ([]*models.ImportStatus, error) {
	userImports, err := i.searchAllImports[*models.ImportStatus](ctx,
		search.BoolQuery(
			search.WithBoolFilter(
				search.TermQuery("doc_type", "import_status"),
				search.TermQuery("user_id", userID),
			),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("get all pending imports: %w", err)
	}
	if err != nil {
		return nil, fmt.Errorf("find active imports: %w", err)
	}
	return userImports, nil
}

func (i *ImportService) findActiveImports(ctx context.Context, userID models.UserID) ([]*models.ImportStatus, error) {
	activeImports, err := i.searchAllImports[*models.ImportStatus](
		ctx,
		search.BoolQuery(
			search.WithBoolFilter(
				search.TermQuery("doc_type", "import_status"),
				search.TermQuery("user_id", userID),
			),
			search.WithBoolShould(
				search.TermQuery("status", models.ImportStatusStatusPending),
				search.TermQuery("status", models.ImportStatusStatusRunning),
			),
		),
		search.WithSort(
			search.NewSortOptions(
				search.WithSortField("created_at", search.WithFieldSortOrder(search.SortOrderDesc)),
			),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("find active imports: %w", err)
	}
	return activeImports, nil
}

func (i *ImportService) getResultsForImport(ctx context.Context, id string) ([]*models.ImportResult, error) {
	results, err := i.searchAllImports[*models.ImportResult](ctx,
		search.BoolQuery(
			search.WithBoolFilter(
				search.TermQuery("doc_type", "import_result"),
				search.TermQuery("job_id", id),
			),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("get all results: %w", err)
	}
	return results, nil
}

func (i *ImportService) searchAllImports[I any](
	ctx context.Context,
	query search.QueryOption,
	options ...search.SearchOption,
) ([]I, error) {
	resp, err := search.SearchAll(
		ctx,
		i.osc,
		i.osc.IndexReadAlias(schema.ImportsIndex),
		2500,
		[]search.QueryOption{query},
		options...)
	if err != nil {
		return nil, fmt.Errorf("search imports: %w", err)
	}
	results, err := resp.ExtractHits[I]()
	if err != nil {
		return nil, fmt.Errorf("extract imports: %w", err)
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
		status.Error = models.NewErrorMessage(
			"Internal server error",
			"The server encountered a problem that stopped the import process completing",
		)
	}
	if err := doc.Index(ctx, i.osc.Doc, i.osc.IndexWriteAlias(schema.ImportsIndex), status); err != nil {
		return fmt.Errorf("update doc: %w", err)
	}
	return nil
}
