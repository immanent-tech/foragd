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
	"time"

	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/conflicts"
	"github.com/goforj/godump"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/logging"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/reindex"
)

func main() {
	ctx := context.TODO()

	logger := logging.New(logging.Options{LogLevel: "debug"})
	ctx = slogctx.NewCtx(ctx, logger)

	client, err := elastic.NewConnection()
	if err != nil {
		panic(err)
	}
	api := client.TypedClient

	reindexResp, err := reindex.NewReindexOperation(
		api,
		&types.ReindexSource{
			Index: []string{"items_production_ro"},
			Remote: &types.RemoteSource{
				Host:   os.Getenv("ELASTICSEARCH_OLD_HOST"),
				ApiKey: new(os.Getenv("ELASTICSEARCH_OLD_APIKEY")),
			},
		},
		reindex.NewDest("items_production_rw", "")).
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
					godump.Dump(tasksResp)
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
