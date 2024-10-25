// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package scheduler

import (
	"context"
	"log/slog"

	"github.com/reugn/go-quartz/logger"
	"github.com/reugn/go-quartz/quartz"
)

var sched quartz.Scheduler

func Start(ctx context.Context, logHandler slog.Handler) {
	logger.SetDefault(logger.NewSimpleLogger(slog.NewLogLogger(logHandler, slog.LevelDebug), logger.LevelDebug))
	// create scheduler
	sched = quartz.NewStdScheduler()
	slog.Debug("Starting quartz scheduler...")
	// async start scheduler
	sched.Start(ctx)
}

func Stop() {
	// stop scheduler
	slog.Debug("Stopping quartz scheduler...")
	sched.Stop()
}

func Schedule(jobDetail *quartz.JobDetail, trigger quartz.Trigger) error {
	slog.Debug("Scheduling job.", slog.String("job_key", jobDetail.JobKey().Name()))
	return sched.ScheduleJob(jobDetail, trigger)
}
