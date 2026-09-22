// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"time"

	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/conflicts"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-base/logging"

	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/reindex"
	"github.com/immanent-tech/foragd/service"
)

func main() {
	ctx := context.TODO()

	logger := logging.New()
	ctx = slogctx.NewCtx(ctx, logger)

	client, err := elastic.GetAPI()
	if err != nil {
		panic(err)
	}
	api := client.TypedClient

	elasticSvc, err := service.LoadElasticService()
	if err != nil {
		panic(err)
	}

	for index := range slices.Values([]string{"feeds", "items", "favorites", "users", "subscriptions", "scheduler", "sessions"}) {
		var src, dest string
		switch index {
		case "feeds":
			src = elasticSvc.GetIndexRO(service.FeedsIndex)
			dest = elasticSvc.GetIndexRW(service.FeedsIndex)
		case "items":
			src = elasticSvc.GetIndexRO(service.ItemsIndex)
			dest = elasticSvc.GetIndexRW(service.ItemsIndex)
		case "favorites":
			src = elasticSvc.GetIndexRO(service.FavoritesIndex)
			dest = elasticSvc.GetIndexRW(service.FavoritesIndex)
		case "users":
			src = elasticSvc.GetIndexRO(service.UsersIndex)
			dest = elasticSvc.GetIndexRW(service.UsersIndex)
		case "subscriptions":
			src = elasticSvc.GetIndexRO(service.SubscriptionsIndex)
			dest = elasticSvc.GetIndexRW(service.SubscriptionsIndex)
		case "scheduler":
			src = elasticSvc.GetIndexRO(service.ScheduleIndex)
			dest = elasticSvc.GetIndexRW(service.ScheduleIndex)
		case "sessions":
			src = elasticSvc.GetIndexRO(service.SessionsIndex)
			dest = elasticSvc.GetIndexRW(service.SessionsIndex)
		}
		reindexResp, err := reindex.NewReindexOperation(
			api,
			&types.ReindexSource{
				Index: []string{src},
				Remote: &types.RemoteSource{
					Host:   os.Getenv("ELASTICSEARCH_OLD_HOST"),
					ApiKey: new(os.Getenv("ELASTICSEARCH_OLD_APIKEY")),
				},
			},
			reindex.NewDest(dest, "")).
			WaitForCompletion(false).
			Conflicts(conflicts.Proceed).
			// RequestsPerSecond("1000").
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
						panic(fmt.Errorf("reindex: %v", tasksResp.Error))
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
						slog.String("source", src),
						slog.String("destination", dest),
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
	}
}
