/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
)

const JobTypeRunImports JobType = "run_imports"

var runImportsRunning atomic.Bool

// NewGetNewFeedsJob creates a job for checking for new feeds.
func NewRunImportsJob() (*SerializedJob, error) {
	job := &SerializedJob{
		CreatedAt:      time.Now().UTC(),
		JobDescription: new("Run any pending import jobs for users."),
		JobKey:         quartz.NewJobKey(string(JobTypeRunImports)).String(),
		JobType:        JobTypeRunImports,
		JobNextRun:     models.UnixEpoch,
		JobTriggerType: TriggerTypePoll,
	}

	if err := job.JobTrigger.FromPollTrigger(*NewPollTrigger(DefaultPollInterval, DefaultPollJitter)); err != nil {
		return nil, fmt.Errorf("create trigger: %w", err)
	}

	return job, nil
}

// ExecuteGetNewFeeds runs a job that will look for newly added feeds and schedule new jobs to fetch item updates for
// them.
func ExecuteRunImportsJob(ctx context.Context, job *SerializedJob) error {
	if wasRunning := runImportsRunning.Swap(true); wasRunning {
		return errors.New("job is running")
	}
	defer runImportsRunning.Store(false)

	importSvc := ImportSvcFromCtx(ctx)
	if importSvc == nil {
		return errors.New("cannot execute: no import service in context")
	}

	httpClient := HTTPClientFromCtx(ctx)
	if httpClient == nil {
		return errors.New("cannot execute: no httpClient in context")
	}

	start := time.Now()

	pendingImports, err := importSvc.GetPendingImports(ctx)
	if err != nil {
		return fmt.Errorf("get pending imports: %w", err)
	}

	maxConcurrentImports := runtime.GOMAXPROCS(0)
	statusCh := make(chan models.ImportStatus, 2*maxConcurrentImports)
	var wg sync.WaitGroup

	for range maxConcurrentImports {
		wg.Go(func() {
			for status := range statusCh {
				if err := importSvc.ProcessRequests(ctx, &status, httpClient); err != nil {
					slogctx.Error(ctx, "Unable to process import.",
						slog.String("job_id", status.GetID()),
						slog.String("user_id", status.UserID),
						slog.Any("error", err),
					)
				}
			}
		})
	}

	for status := range slices.Values(pendingImports) {
		select {
		case statusCh <- *status:
		case <-ctx.Done():
			close(statusCh)
			wg.Wait()
			return ctx.Err()
		}
	}
	close(statusCh)

	wg.Wait()

	slogctx.Debug(ctx, "Finished run imports job.",
		slog.Duration("took", time.Since(start)))

	return nil
}
