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
	"sync"
	"sync/atomic"
	"time"

	"github.com/reugn/go-quartz/quartz"
	slogctx "github.com/veqryn/slog-context"
	"golang.org/x/sync/errgroup"

	"github.com/immanent-tech/foragd/models"
	gerror "github.com/immanent-tech/foragd/providers/google/error"
)

const (
	JobTypeRunImports JobType = "run_imports"
	// importTimeout is the maximum amount of time a single import can run for.
	importTimeout = time.Hour
)

var runImportsRunning atomic.Bool

// NewRunImportsJob creates a job for checking for new imports.
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

// ExecuteRunImportsJob runs a job that will look for newly imports and process them.
func ExecuteRunImportsJob(ctx context.Context, job *SerializedJob) error {
	// Singleton runner flag
	if wasRunning := runImportsRunning.Swap(true); wasRunning {
		return errors.New("job is running")
	}
	defer runImportsRunning.Store(false)

	services := ServicesFromCtx(ctx)
	if services == nil {
		return errors.New("no services in context")
	}

	start := time.Now()

	pendingImports, err := services.Imports.GetPendingImports(ctx)
	if err != nil {
		return fmt.Errorf("get pending imports: %w", err)
	}

	// Bounded concurrency with errgroup
	const maxWorkers = 5 // Cap to prevent oversubscription
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(maxWorkers)

	var (
		failed    int
		failedMut sync.Mutex
	)

	// Fan-out: send imports to workers
	for _, status := range pendingImports {
		g.Go(func() error {
			importCtx, importCancel := context.WithTimeoutCause(
				ctx,
				importTimeout,
				errors.New("import job exceeded maximum timeout"),
			)
			defer importCancel()

			// Panic recovery at goroutine boundary
			defer func() {
				if rvr := recover(); rvr != nil {
					failedMut.Lock()
					failed++
					failedMut.Unlock()
					slogctx.FromCtx(ctx).Error("Worker panic", slog.Any("recover", rvr))
					// Log to GCP error console.
					switch v := rvr.(type) {
					case error:
						gerror.ReportError(v)
					default:
						gerror.ReportError(fmt.Errorf("panic: %v", v))
					}
				}
			}()

			// Process the import
			if err := services.Imports.ProcessRequests(importCtx, status); err != nil {
				slogctx.FromCtx(ctx).Error("Unable to process import.",
					slog.String("job_id", status.GetID()),
					slog.String("user_id", status.UserID),
					slog.Any("error", err))
				return err // Propagate error
			}
			return nil
		})
	}

	// Wait for all workers
	if err := g.Wait(); err != nil {
		slogctx.FromCtx(ctx).Warn("Some imports failed", slog.Any("error", err))
	}

	slogctx.Debug(ctx, "Finished run imports job.",
		slog.Duration("took", time.Since(start)),
		slog.Int("failed", failed),
	)

	return nil
}
