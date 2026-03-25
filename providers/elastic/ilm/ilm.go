// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package ilm contains wrappers that help with creating ILM policies.
package ilm

import (
	"context"
	"fmt"

	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/ilm/putlifecycle"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
)

// Option is a reusable generic function for applying options to a type.
type Option[T any] func(T)

// WithDelete will add a delete action to the phase.
func WithDelete() Option[*types.IlmActions] {
	return func(action *types.IlmActions) {
		action.Delete = types.NewDeleteAction()
	}
}

// WithRolloverMaxSize will apply a rollover action to the phase that will
// rollover indices larger than the given size.
func WithRolloverMaxSize(size string) Option[*types.IlmActions] {
	return func(action *types.IlmActions) {
		if action.Rollover == nil {
			action.Rollover = types.NewRolloverAction()
		}

		action.Rollover.MaxPrimaryShardSize = types.ByteSize(size)
	}
}

// WithRolloverMaxAge will apply a rollover action to the phase that will
// rollover indices older than the given duration.
func WithRolloverMaxAge(duration string) Option[*types.IlmActions] {
	return func(action *types.IlmActions) {
		if action.Rollover == nil {
			action.Rollover = types.NewRolloverAction()
		}

		action.Rollover.MaxAge = types.Duration(duration)
	}
}

// WithForceMergeSegments will apply a force merge action to the phase that will
// force merge indices to the given number of segments.
func WithForceMergeSegments(segments int) Option[*types.IlmActions] {
	return func(action *types.IlmActions) {
		if action.Forcemerge == nil {
			action.Forcemerge = types.NewForceMergeAction()
		}

		action.Forcemerge.MaxNumSegments = segments
	}
}

// WithShrinkToShards will apply a shrink action to the phase that will shrink
// indices to the given number of shards.
func WithShrinkToShards(shards int) Option[*types.IlmActions] {
	return func(action *types.IlmActions) {
		if action.Shrink == nil {
			action.Shrink = types.NewShrinkAction()
		}

		action.Shrink.NumberOfShards = &shards
	}
}

// NewILMAction creates a new ILM action for with the given options.
func NewILMAction(options ...Option[*types.IlmActions]) *types.IlmActions {
	actions := &types.IlmActions{}

	for _, option := range options {
		option(actions)
	}

	return actions
}

// WithActions adds the given actions to the phase.
//
// https://www.elastic.co/docs/reference/elasticsearch/index-lifecycle-actions/
func WithActions(options ...Option[*types.IlmActions]) Option[*types.Phase] {
	return func(phase *types.Phase) {
		phase.Actions = NewILMAction(options...)
	}
}

// WithMinAge defines the minimum age at which this phase applies.
func WithMinAge(age string) Option[*types.Phase] {
	return func(phase *types.Phase) {
		dur := types.Duration(age)
		phase.MinAge = &dur
	}
}

// WithPhase adds a phase to the ILM policy with the given phase options.
func WithPhase(name string, options ...Option[*types.Phase]) Option[*types.IlmPolicy] {
	return func(ilm *types.IlmPolicy) {
		phase := types.NewPhase()
		for _, option := range options {
			option(phase)
		}

		switch name {
		case "hot":
			ilm.Phases.Hot = phase
		case "warm":
			ilm.Phases.Warm = phase
		case "cold":
			ilm.Phases.Cold = phase
		case "frozen":
			ilm.Phases.Frozen = phase
		case "delete":
			ilm.Phases.Delete = phase
		}
	}
}

// Policy represents an index lifecycle management policy.
type Policy struct {
	*putlifecycle.Request

	Name string
}

// NewILMPolicy creates a new ILM policy with the given options and encapsulates it in an appropriate request object.
func NewILMPolicy(name string, options ...Option[*types.IlmPolicy]) *Policy {
	policy := &Policy{
		Name: name,
		Request: &putlifecycle.Request{
			Policy: types.NewIlmPolicy(),
		},
	}
	for _, option := range options {
		option(policy.Policy)
	}
	return policy
}

// Put will send a request to create the index template in the cluster.
func (p *Policy) Put(ctx context.Context, api *elasticsearch.TypedClient) error {
	if _, err := api.Ilm.PutLifecycle(p.Name).Request(p.Request).Do(ctx); err != nil {
		return fmt.Errorf("unable to put ILM policy: %w", err)
	}
	return nil
}
