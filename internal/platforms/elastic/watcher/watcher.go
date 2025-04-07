// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package watcher

import (
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/elastic/go-elasticsearch/v8/typedapi"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/conditionop"
	"github.com/elastic/go-elasticsearch/v8/typedapi/watcher/putwatch"
)

type Client interface {
	GetAPI() *typedapi.API
	Log() *slog.Logger
}

// WatchOption is a functional option to apply to a watch request.
type WatchOption func(*putwatch.Request)

// WatchRequest contains data for an Elasticsearch Watch.
type WatchRequest struct {
	mu sync.Mutex
	*putwatch.Request
}

// WithTrigger sets the trigger for the watch.
//
// https://www.elastic.co/guide/en/elasticsearch/reference/current/trigger-schedule.html
func WithTrigger(trigger any) WatchOption {
	return func(watch *putwatch.Request) {
		watchTrigger := types.NewScheduleContainer()

		switch value := trigger.(type) {
		case *string:
			watchTrigger.Cron = value
		case *types.DailySchedule:
			watchTrigger.Daily = value
		case *types.HourlySchedule:
			watchTrigger.Hourly = value
		case *types.Duration:
			watchTrigger.Interval = value
		case []types.TimeOfMonth:
			watchTrigger.Monthly = value
		case []types.TimeOfWeek:
			watchTrigger.Weekly = value
		case []types.TimeOfYear:
			watchTrigger.Yearly = value
		}

		watch.Trigger.Schedule = watchTrigger
	}
}

// WithInput sets the input for the watch. If this option is not specified, an
// empty payload is used.
//
// https://www.elastic.co/guide/en/elasticsearch/reference/current/input.html
func WithInput(input any) WatchOption {
	return func(watch *putwatch.Request) {
		watchInput := types.NewWatcherInput()

		switch value := input.(type) {
		case *types.ChainInput:
			watchInput.Chain = value
		case *types.HttpInput:
			watchInput.Http = value
		case *types.SearchInput:
			watchInput.Search = value
		case map[string]json.RawMessage:
			watchInput.Simple = value
		}

		watch.Input = watchInput
	}
}

// WithCondition sets the condition for executing watch actions. If this option
// is not specified, the always condition is used.
//
// https://www.elastic.co/guide/en/elasticsearch/reference/current/condition.html
func WithCondition(condition any) WatchOption {
	return func(watch *putwatch.Request) {
		watchCondition := types.NewWatcherCondition()

		switch value := condition.(type) {
		case string:
			switch value {
			case "always":
				watchCondition.Always = types.NewAlwaysCondition()
			case "never":
				watchCondition.Never = types.NewNeverCondition()
			}
		case map[string]map[conditionop.ConditionOp]types.FieldValue:
			watchCondition.Compare = value
		}

		watch.Condition = watchCondition
	}
}

func WithAction(name string, action types.WatcherAction) WatchOption {
	return func(watch *putwatch.Request) {
		watch.Actions[name] = action
	}
}

func NewWatch(client Client, name string, options ...WatchOption) *putwatch.PutWatch {
	request := &WatchRequest{
		Request: putwatch.NewRequest(),
	}
	request.mu.Lock()
	defer request.mu.Unlock()

	for _, option := range options {
		option(request.Request)
	}

	return client.GetAPI().Watcher.PutWatch(name).Request(request.Request)
}
