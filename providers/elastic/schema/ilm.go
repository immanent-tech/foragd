// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package schema

import (
	"github.com/elastic/go-elasticsearch/v9/typedapi/ilm/putlifecycle"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
)

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

		action.Rollover.MaxSize = types.ByteSize(size)
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

// NewILMPolicy creates a new ILM policy with the given options and encapsulates it in an appropriate request object.
func NewILMPolicy(options ...Option[*types.IlmPolicy]) *putlifecycle.Request {
	policy := &types.IlmPolicy{}

	for _, option := range options {
		option(policy)
	}

	return &putlifecycle.Request{Policy: policy}
}
