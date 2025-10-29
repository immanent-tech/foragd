// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/scheduler"
)

// SchedulerCmd defines the `scheduler` command, for running the job scheduler.
type SchedulerCmd struct {
	Run RunSchedulerCmd `cmd:"delete" help:"Run scheduler."`
	// RemoveOrphanFeedJobs RemoveOrphanFeedJobs `cmd:"remove-orphan-feed-jobs" help:"Remove feed jobs that have no corresponding feed.`
}

type RunSchedulerCmd struct{}

// Run contains logic for setup and execution of the scheduler.
func (r *RunSchedulerCmd) Run(opts *Arguments) error {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()
	ctx = slogctx.NewCtx(ctx, opts.Logger)
	// Run scheduler.
	err := scheduler.Run(ctx, opts.Environment)
	if err != nil {
		return fmt.Errorf("could not run scheduler: %w", err)
	}
	return nil
}

// type RemoveOrphanFeedJobs struct{}

// func (c *RemoveOrphanFeedJobs) Run() error {
// 	// Set up context.
// 	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
// 	defer cancelFunc()
// 	ctx = elastic.SetupIndexAliases(ctx)
// 	// Load the Elastic backend
// 	env := os.Getenv("FORAGD_ENVIRONMENT")
// 	client, err := elastic.Connect(ctx, env)
// 	if err != nil {
// 		return fmt.Errorf("failed to connect to backend: %w", err)
// 	}

// 	return nil
// }
