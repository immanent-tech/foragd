// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/goforj/godump"
	"github.com/reugn/go-quartz/quartz"

	"github.com/immanent-tech/foragd/models"
)

func NewTestJob() (*SerializedJob, error) {
	job := &SerializedJob{
		CreatedAt:      time.Now().UTC(),
		JobDescription: new("test job"),
		JobKey:         quartz.NewJobKeyWithGroup("test_oneshot", "test").String(),
		JobType:        JobTypeTest,
		JobNextRun:     models.UnixEpoch,
		JobTriggerType: TriggerTypeOneshot,
	}

	if err := job.JobTrigger.FromOneShotTrigger(OneShotTrigger{Delay: 10 * time.Second}); err != nil {
		return nil, fmt.Errorf("create trigger: %w", err)
	}

	return job, nil
}

func ExecuteTest(ctx context.Context, job *SerializedJob) error {
	godump.Dump(job)
	return nil
}
