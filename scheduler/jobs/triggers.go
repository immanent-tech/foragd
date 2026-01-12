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
	defaultCronJobTrigger = "0 */5 * * * *"
	defaultPollInterval   = time.Minute
	defaultPollJitter     = 5 * time.Second
	pollTriggerID         = "PollTrigger"
	cronTriggerID         = "CronTrigger"
)

// Verify PollTrigger satisfies the Trigger interface.
var _ quartz.Trigger = (*pollTrigger)(nil)

// newPollTrigger returns a new polling job using the given interval and jitter.
func newPollTrigger(interval, jitter any) *pollTrigger {
	return &pollTrigger{
		Interval: asDuration(interval, defaultPollInterval),
		Jitter:   asDuration(jitter, defaultPollJitter),
	}
}

// NextFireTime returns the next time at which the PollTriggerWithJitter is scheduled to fire.
func (t *pollTrigger) NextFireTime(prev int64) (int64, error) {
	jitter := rand.NormFloat64()*float64(t.Jitter) + float64(t.Interval) // #nosec: G404
	next := prev + int64(jitter)
	return next, nil
}

// Description returns the description of the PollTriggerWithJitter.
func (t *pollTrigger) Description() string {
	return strings.Join([]string{pollTriggerID, t.Interval.String(), t.Jitter.String()}, quartz.Sep)
}

// parseTrigger will attempt to parse the given trigger interface into its concrete trigger type. If the interface value
// cannot be parsed, a default polling trigger will be returned.
func parseTrigger(trigger quartz.Trigger) any {
	desc := trigger.Description()
	switch {
	case strings.HasPrefix(desc, pollTriggerID):
		triggerOpts := strings.Split(desc, quartz.Sep)
		if len(triggerOpts) != 3 { //nolint:mnd // this is a very specific check.
			return newPollTrigger(defaultPollInterval, defaultPollJitter)
		}
		return newPollTrigger(triggerOpts[1], triggerOpts[2])
	case strings.HasPrefix(desc, cronTriggerID):
		return &cronTrigger{Schedule: strings.Split(desc, quartz.Sep)[1]}
	}
	return newPollTrigger(defaultPollInterval, defaultPollJitter)
}

// asDuration will attempt to parse the given input value as a duration. If the value cannot be parsed, the given
// fallback will be returned instead.
func asDuration(input any, fallback time.Duration) time.Duration {
	switch value := input.(type) {
	case time.Duration:
		return value
	case int64:
		return time.Duration(value)
	case string:
		dur, err := time.ParseDuration(value)
		if err != nil {
			return fallback
		}
		return dur
	}
	return fallback
}
