// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package schema

import (
	"errors"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

var (
	ErrPutComponentTemplate = errors.New("put component template failed")
	ErrPutIndexTemplate     = errors.New("put index template failed")
)

// WithIndexLifeCycle will assign the ILM policy with the given name to the index.
func WithIndexLifecycle(name string) Option[*types.IndexSettings] {
	return func(settings *types.IndexSettings) *types.IndexSettings {
		settings.Lifecycle = types.NewIndexSettingsLifecycle()
		settings.Lifecycle.Name = &name

		return settings
	}
}

// NewIndexSettings creates a new index settings object with the given options.
func NewIndexSettings(options ...Option[*types.IndexSettings]) *types.IndexSettings {
	settings := &types.IndexSettings{}

	for _, option := range options {
		settings = option(settings)
	}

	return settings
}

// WithIndexSettings applies the given settings to the index.
func WithIndexSettings(options ...Option[*types.IndexSettings]) Option[types.IndexState] {
	return func(state types.IndexState) types.IndexState {
		state.Settings = NewIndexSettings(options...)
		return state
	}
}

// WithAliases adds the given aliases to the index.
func WithAliases(name string, props types.Alias) Option[types.IndexState] {
	return func(state types.IndexState) types.IndexState {
		state.Aliases[name] = props
		return state
	}
}

// WithMappings adds the given mapping properties to the index.
func WithMappings(mappings *types.TypeMapping) Option[types.IndexState] {
	return func(state types.IndexState) types.IndexState {
		state.Mappings = mappings
		return state
	}
}

// WithSettings adds the given index settings to the index.
func WithSettings(settings *types.IndexSettings) Option[types.IndexState] {
	return func(state types.IndexState) types.IndexState {
		state.Settings = settings
		return state
	}
}

// NewIndexState creates a new index state with the given options.
func NewIndexState(options ...Option[types.IndexState]) types.IndexState {
	state := types.IndexState{}

	for _, option := range options {
		state = option(state)
	}

	return state
}
