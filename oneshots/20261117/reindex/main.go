/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/immanent-tech/go-base/logging"
	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/providers/opensearch"
	"github.com/immanent-tech/foragd/providers/opensearch/reindex"
	"github.com/immanent-tech/foragd/schema"
)

func main() {
	ctx := slogctx.NewCtx(context.Background(), logging.New())

	index := schema.ItemsIndex

	osc, err := opensearch.GetClient()
	if err != nil {
		panic(fmt.Errorf("load opensearch client: %w", err))
	}

	slogctx.Info(ctx, "Reindex from remote.", slog.String("index", index.String()))
	resp, err := reindex.Reindex(ctx,
		osc,
		reindex.NewSource(
			reindex.FromRemote(reindex.RemoteSource{
				Host: os.Getenv("REINDEX_REMOTE"),
				Headers: map[string]string{
					"Authorization": "ApiKey " + os.Getenv("ELASTICSEARCH_APIKEY"),
				},
			}),
			reindex.From(osc.IndexReadAlias(index)),
		),
		reindex.NewDestination(reindex.To(osc.IndexWriteAlias(index))),
		reindex.WaitForCompletion(false),
		reindex.WithProceedOnConflict(),
	)
	if err != nil {
		panic(fmt.Errorf("reindex: %w", err))
	}

	var task opensearchapi.ReindexRespBodyTask
	if err := json.Unmarshal(resp.Inspect().Response.RawBody(), &task); err != nil {
		panic(fmt.Errorf("unmarshal response: %w", err))
	}

	// Wait for the reindex to complete.
	if task.Task != nil {
		for {
			resp, err := osc.Tasks.Get(ctx, opensearchapi.TasksGetReq{
				TaskID: *task.Task,
			})
			if err != nil {
				slogctx.Warn(ctx, "Fetch task status failed", slog.Any("error", err))
			} else {
				if resp.Completed {
					if resp.Error != nil {
						panic(fmt.Errorf("reindex: %w", err))
					}
					slogctx.Info(ctx, "Reindex complete!")
					break
				}
				slogctx.Info(ctx, "Reindexing...",
					slog.String("task_id", *task.Task),
					slog.Time("created", time.Now().Add(time.Duration(-1*resp.Task.RunningTimeInNanos))),
					slog.Duration("runing", time.Duration(resp.Task.RunningTimeInNanos)),
				)
			}
			time.Sleep(10 * time.Second)
		}
	} else {
		panic(errors.New("no reindex task"))
	}
}
