package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/conflicts"
	"github.com/immanent-tech/go-base/logging"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/providers/elastic/reindex"
	"github.com/immanent-tech/foragd/providers/ollama"
	"github.com/immanent-tech/foragd/providers/opensearch"
	osbulk "github.com/immanent-tech/foragd/providers/opensearch/bulk"
	"github.com/immanent-tech/foragd/service"
)

func main() {
	ctx := slogctx.NewCtx(context.Background(), logging.New())

	api, err := elastic.GetAPI()
	if err != nil {
		panic(err)
	}

	elasticSvc, err := service.LoadElasticService()
	if err != nil {
		panic(err)
	}

	osc, err := opensearch.GetClient()
	if err != nil {
		slogctx.Warn(ctx, "Failed to load opensearch client", slog.Any("error", err))
		// return nil, fmt.Errorf("load opensearch client: %w", err)
	}
	indexer, err := osbulk.New()
	if err != nil {
		slogctx.Warn(ctx, "Failed to load bulk indexer", slog.Any("error", err))
	}
	httpClient := service.NewHTTPClient()
	ollamaSvc, err := ollama.NewClient()
	if err != nil {
		panic(fmt.Errorf("load ollama client: %w", err))
	}
	itemsSvc, err := service.LoadItemsService(ctx, osc, indexer, httpClient)
	if err != nil {
		panic(fmt.Errorf("load items service: %w", err))
	}
	feedsSvc, err := service.LoadFeedsService(ctx, osc, indexer, httpClient, ollamaSvc, itemsSvc)
	if err != nil {
		panic(fmt.Errorf("load feeds service: %w", err))
	}
	usersSvc, err := service.LoadUsersService(ctx, osc)
	if err != nil {
		panic(fmt.Errorf("load users service: %w", err))
	}
	subscriptionsSvc, err := service.LoadSubscriptionsService(ctx, osc, indexer, itemsSvc, feedsSvc, usersSvc)
	if err != nil {
		panic(fmt.Errorf("load subscriptions service: %w", err))
	}
	prefix := "subscriptions"

	index := elastic.GenerateIndexName(prefix)
	writeAlias := elasticSvc.GetIndexRW(service.SubscriptionsIndex)
	readAlias := elasticSvc.GetIndexRO(service.SubscriptionsIndex)

	// Create index.
	if _, err := elastic.CreateIndexIfNotExists(ctx, prefix); err != nil {
		panic(fmt.Errorf("could not create index %s: %w", index, err))
	}

	// Update the write alias.
	if err := elastic.UpdateIndexAlias(ctx, writeAlias, index); err != nil {
		panic(fmt.Errorf("migration failed: %w", err))
	}

	// Reindex if requested.
	if found, err := api.Indices.Exists(readAlias).Do(ctx); err != nil || !found {
		panic(fmt.Errorf("could not determine %s index state: %w", readAlias, err))
	}

	slogctx.Info(ctx, "Reindex all subscriptions.")
	reindexResp, err := reindex.NewReindexOperation(api.TypedClient, reindex.NewSource(readAlias), reindex.NewDest(index, "")).
		WaitForCompletion(false).
		Conflicts(conflicts.Proceed).
		RequestsPerSecond("1000").
		Do(ctx)
	if err != nil {
		panic(fmt.Errorf("reindex: %w", err))
	}

	// Wait for the reindex to complete.
	if taskID := reindexResp.Task; taskID != nil {
		for {
			tasksResp, err := api.Tasks.Get(*taskID).Do(ctx)
			if err != nil {
				panic(fmt.Errorf("get tasks: %w", err))
			}

			if tasksResp.Completed {
				if tasksResp.Error != nil {
					panic(fmt.Errorf("reindex: %w", err))
				}
				slogctx.Info(ctx, "Reindex complete!")
				break
			}

			var status struct {
				Total            int `json:"total"`
				Created          int `json:"created"`
				Updated          int `json:"updated"`
				Deleted          int `json:"deleted"`
				Batches          int `json:"batches"`
				VersionConflicts int `json:"version_conflicts"`
			}

			if err := json.Unmarshal(tasksResp.Task.Status, &status); err != nil {
				slogctx.Warn(ctx, "Unable to parse task status.",
					slog.Any("error", err))
			} else {
				slogctx.Info(ctx, "Reindexing...",
					slog.String("task_id", *taskID),
					slog.Int("created", status.Created),
					slog.Int("updated", status.Updated),
					slog.Int("deleted", status.Deleted),
					slog.Int("version_conflicts", status.VersionConflicts),
					slog.Int("total", status.Total),
				)
			}
			time.Sleep(10 * time.Second)
		}
	} else {
		panic(errors.New("no reindex task"))
	}

	// Update the read alias.
	if err = elastic.UpdateIndexAlias(ctx, readAlias, index); err != nil {
		panic(fmt.Errorf("migration failed: %w", err))
	}

	// Update group subscriptions.
	users, err := elastic.SearchAll[*models.User](
		ctx,
		elasticSvc.GetIndexRO(service.UsersIndex),
		query.MatchAll(),
		5000)
	if err != nil {
		panic(err)
	}
	for user := range slices.Values(users) {
		ctx := models.UserToCtx(ctx, user)
		subscriptions, err := subscriptionsSvc.GetAllSubscriptions(ctx)
		if err != nil && !errors.Is(err, models.ErrNotFound) {
			panic(err)
		}
		groupSubscriptions := subscriptions.FilterByType(models.SubscriptionTypeGroup)
		if len(groupSubscriptions) == 0 {
			continue
		}
		slogctx.Info(ctx, "Processing group subscriptions for user.",
			slog.String("user_id", user.GetID()),
			slog.Int("count", len(groupSubscriptions)))
		// for subscription := range slices.Values(groupSubscriptions) {
		// 	grouped, err := service.GetSubscriptionsByID(ctx, subscription.GroupData.Subscriptions...)
		// 	if err != nil {
		// 		panic(fmt.Errorf("get grouped subscription details: %w", err))
		// 	}
		// 	for g := range slices.Values(grouped) {
		// 		subscription.GroupData.Metadata = append(
		// 			subscription.GroupData.Metadata,
		// 			models.GroupedSubscriptionMetadata{
		// 				SubscriptionID: g.GetID(),
		// 				FeedID:         g.GetFeedID(),
		// 			},
		// 		)
		// 	}
		// }
		if err := subscriptionsSvc.UpdateSubscriptions(ctx, groupSubscriptions...); err != nil {
			panic(err)
		}
	}

}
