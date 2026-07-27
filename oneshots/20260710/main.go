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

	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/reindex"
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

	for index := range slices.Values([]string{"feeds", "items", "favorites", "users", "subscriptions", "scheduler", "sessions"}) {
		var src, dest string
		switch index {
		case "feeds":
			src = schema.FeedsIndexRO()
			dest = schema.FeedsIndexRW()
		case "items":
			src = schema.ItemsIndexRO()
			dest = schema.ItemsIndexRW()
		case "favorites":
			src = schema.FavoritesIndexRO()
			dest = schema.FavoritesIndexRW()
		case "users":
			src = schema.UsersIndexRO()
			dest = schema.UsersIndexRW()
		case "subscriptions":
			src = schema.SubscriptionsIndexRO()
			dest = schema.SubscriptionsIndexRW()
		case "scheduler":
			src = schema.SchedulerIndexRO()
			dest = schema.SchedulerIndexRW()
		case "sessions":
			src = schema.SessionsIndexRO()
			dest = schema.SessionsIndexRW()
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
