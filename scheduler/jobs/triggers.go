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
	DefaultPollInterval = time.Minute
	DefaultPollJitter   = 5 * time.Second
	DefaultRunOnceDelay = time.Hour
)

var _ quartz.Trigger = (*PollTrigger)(nil)

// NewPollTrigger returns a new polling job using the given interval and jitter.
func NewPollTrigger(interval, jitter any) *PollTrigger {
	return &PollTrigger{
		Interval: asDuration(interval, DefaultPollInterval),
		Jitter:   asDuration(jitter, DefaultPollJitter),
	}
}

func (t *PollTrigger) Description() string {
	return strings.Join([]string{"poll", t.Interval.String(), t.Jitter.String()}, quartz.Sep)
}

func (t *PollTrigger) NextFireTime(prev int64) (int64, error) {
	jitter := rand.NormFloat64()*float64(t.Jitter) + float64(t.Interval) // #nosec: G404
	next := prev + int64(jitter)
	return next, nil
}

var _ quartz.Trigger = (*OneShotTrigger)(nil)

// NewOneShotTrigger returns a new RunOnceTrigger with the given delay time.
func NewOneShotTrigger(delay time.Duration) *OneShotTrigger {
	return &OneShotTrigger{
		Delay: delay,
	}
}

func (t *OneShotTrigger) Description() string {
	return strings.Join([]string{"oneshot", t.Delay.String()}, quartz.Sep)
}

func (t *OneShotTrigger) NextFireTime(prev int64) (int64, error) {
	if !t.Expired {
		next := prev + t.Delay.Nanoseconds()
		return next, nil
	}
	return 0, quartz.ErrTriggerExpired
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
