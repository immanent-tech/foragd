// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package jobs

import (
	"math/rand/v2"
	"strings"
	"time"

	"github.com/reugn/go-quartz/quartz"
)

const (
	DefaultCronJobTrigger = "0 */5 * * * *"
	DefaultPollInterval   = time.Minute
	DefaultPollJitter     = 5 * time.Second
	pollTriggerID         = "PollTrigger"
	cronTriggerID         = "CronTrigger"
)

// Verify PollTrigger satisfies the Trigger interface.
var _ quartz.Trigger = (*PollTrigger)(nil)

// NewPollTrigger returns a new polling job using the given interval and jitter.
func NewPollTrigger(interval, jitter any) *PollTrigger {
	return &PollTrigger{
		Interval: asDuration(interval, DefaultPollInterval),
		Jitter:   asDuration(jitter, DefaultPollJitter),
	}
}

// NextFireTime returns the next time at which the PollTriggerWithJitter is scheduled to fire.
func (t *PollTrigger) NextFireTime(prev int64) (int64, error) {
	jitter := rand.NormFloat64()*float64(t.Jitter) + float64(t.Interval) // #nosec: G404
	next := prev + int64(jitter)
	return next, nil
}

// Description returns the description of the PollTriggerWithJitter.
func (t *PollTrigger) Description() string {
	return strings.Join([]string{pollTriggerID, t.Interval.String(), t.Jitter.String()}, quartz.Sep)
}

// ParseTrigger will attempt to parse the given trigger interface into its concrete trigger type. If the interface value
// cannot be parsed, a default polling trigger will be returned.
func ParseTrigger(trigger quartz.Trigger) any {
	desc := trigger.Description()
	triggerOpts := strings.Split(desc, quartz.Sep)
	switch {
	case strings.HasPrefix(desc, pollTriggerID):
		if len(triggerOpts) != 3 { //nolint:mnd // this is a very specific check.
			return NewPollTrigger(DefaultPollInterval, DefaultPollJitter)
		}
		return NewPollTrigger(triggerOpts[1], triggerOpts[2])
	case strings.HasPrefix(desc, cronTriggerID):
		return &CronTrigger{Schedule: triggerOpts[1]}
	}
	return NewPollTrigger(DefaultPollInterval, DefaultPollJitter)
}

// asDuration will attempt to parse the given input value as a duration. If the value cannot be parsed, the given
// fallback will be returned instead.
func asDuration(input any, fallback time.Duration) time.Duration {
	switch value := input.(type) {
	case time.Duration:
		return value
	case string:
		dur, err := time.ParseDuration(value)
		if err != nil {
			return fallback
		}
		return dur
	}
	return fallback
}
