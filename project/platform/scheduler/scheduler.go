// Copyright (C) 2024 Joshua Rich <joshua.rich@gmail.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

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
