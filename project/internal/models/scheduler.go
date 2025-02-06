// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/reugn/go-quartz/quartz"

	"github.com/joshuar/go-feed-me/internal/id"
	"github.com/joshuar/go-feed-me/internal/logging"
)

const (
	// defaultJobTrigger is a cron schedule to run a job every 5 minutes.
	defaultJobTrigger = "0 */5 * * * *"
	// FeedJobGroup is a scheduler group key for jobs to fetch new items for
	// feeds.
	FeedJobGroup = "get_items"
)

var (
	_ quartz.ScheduledJob = (*ScheduledJob)(nil)
	_ quartz.Job          = (*FeedJob)(nil)
)

var (
	ErrExecuteJobFailed = errors.New("could not execute job")
	ErrCreateJobFailed  = errors.New("create job failed")
	ErrMarshalJobFailed = errors.New("could not marshal job")
	ErrInvalidJob       = errors.New("invalid job")
	ErrNoJob            = errors.New("no job found")
)

// ScheduledJob satisfies quartz.ScheduledJob through the following methods.

func (sj *ScheduledJob) JobDetail() *quartz.JobDetail {
	job, err := sj.Data.AsFeedJob()
	if err != nil {
		return nil
	}

	if sj.Options != nil {
		return quartz.NewJobDetailWithOptions(&job, GenerateJobKey(job.ID), sj.Options)
	}

	return quartz.NewJobDetail(&job, GenerateJobKey(job.ID))
}

func (sj *ScheduledJob) Trigger() quartz.Trigger {
	trigger, _ := quartz.NewCronTrigger(sj.Schedule)
	return trigger
}

func (sj *ScheduledJob) NextRunTime() int64 {
	return sj.NextRun.UnixNano()
}

// MarshalJob takes a quartz.ScheduledJob object and marshals it back into a
// ScheduledJob, updating fields as appropriate.
func MarshalJob(job quartz.ScheduledJob) (*ScheduledJob, error) {
	triggerOpts := strings.Split(job.Trigger().Description(), quartz.Sep)
	serialized := &ScheduledJob{
		NextRun:   time.Unix(0, job.NextRunTime()),
		CreatedAt: time.Now().UTC(),
		Options:   job.JobDetail().Options(),
		Schedule:  triggerOpts[1],
	}

	switch job := job.JobDetail().Job().(type) {
	case *FeedJob:
		if err := serialized.Data.FromFeedJob(*job); err != nil {
			return nil, errors.Join(ErrMarshalJobFailed, err)
		}
	default:
		return nil, errors.Join(ErrMarshalJobFailed, ErrInvalidJob)
	}

	return serialized, nil
}

// FeedJob satisfies quartz.Job through the following methods.

// Execute is called by the scheduler when the job is scheduled to run.
func (job *FeedJob) Execute(ctx context.Context) error {
	api := FeedManagementAPIFromCtx(ctx)
	if api == nil {
		return errors.Join(ErrExecuteJobFailed, fmt.Errorf("no feed management api in context"))
	}

	// Get the time the feed items were last fetched.
	state, err := api.GetFeedJobState(ctx, job.ID)
	if err != nil && !errors.Is(err, ErrNoJob) {
		return errors.Join(ErrExecuteJobFailed, err)
	}

	if state.UpdatedAt == nil {
		updated := time.Time{}
		state.UpdatedAt = &updated
	}

	logging.FromContext(ctx).Debug("Checking for feed updates.",
		slog.String("feed_id", job.ID),
		slog.Time("since", *state.UpdatedAt))

	// Get new items since the last fetch.
	items, err := job.getItemsSince(*state.UpdatedAt)
	if err != nil {
		return errors.Join(ErrExecuteJobFailed, err)
	}
	// Cache the new items.
	if err := api.AddItems(ctx, items...); err != nil {
		return errors.Join(ErrExecuteJobFailed, err)
	}

	updated := time.Now().UTC()
	update := &APIFeedState{
		ID:        job.ID,
		UpdatedAt: &updated,
	}

	if err := api.UpdateFeedJobState(ctx, update); err != nil {
		return errors.Join(ErrExecuteJobFailed, err)
	}

	return nil
}

// Description returns the description of the job.
func (job *FeedJob) Description() string {
	return "Fetches new items for a feed."
}

// getItemsSince retrieves the feed items that are newer than the given time.
//
//nolint:prealloc
func (job *FeedJob) getItemsSince(since time.Time) ([]Item, error) {
	var items []Item

	details, err := parser.ParseURL(job.URL)
	if err != nil {
		return nil, errors.Join(ErrParseFeed, err)
	}

	for _, i := range details.Items {
		item, err := NewFeedItem(job.ID, i)
		if err != nil {
			continue
		}

		if !item.isNewer(since) {
			continue
		}

		items = append(items, *item)
	}

	return items, nil
}

// NewFeedJob creates a job that can be scheduled from the given feed data.
func NewFeedJob(feed APIFeed) (*ScheduledJob, error) {
	job := &ScheduledJob{
		CreatedAt: time.Now().UTC(),
		Schedule:  defaultJobTrigger,
	}

	if err := job.Data.FromFeedJob(FeedJob{
		ID:  feed.GetID(),
		URL: feed.GetLink(),
	}); err != nil {
		return nil, errors.Join(ErrCreateJobFailed, err)
	}

	return job, nil
}

// GenerateJobKey generates an appropriate job key based on the type of job.
func GenerateJobKey(jobID string) *quartz.JobKey {
	switch id.IdentifyID(jobID) {
	case id.Feed:
		return quartz.NewJobKeyWithGroup(jobID, FeedJobGroup)
	}

	return nil
}
