// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package schema

import (
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
)

// IndexSettingsOption is a functional option for applying an index setting.
type IndexSettingsOption func(*types.IndexSettings)

// WithIndexLifecycle option will assign the ILM policy with the given name to the index.
func WithIndexLifecycle(name string) IndexSettingsOption {
	return func(settings *types.IndexSettings) {
		settings.Lifecycle = types.NewIndexSettingsLifecycle()
		settings.Lifecycle.Name = &name
	}
}

// WithAnalysis option will apply the provided analysis settings.
func WithAnalysis(analysis types.IndexSettingsAnalysis) IndexSettingsOption {
	return func(settings *types.IndexSettings) {
		settings.Analysis = &analysis
	}
}

// NewIndexSettings creates a new index settings object with the given options.
func NewIndexSettings(options ...IndexSettingsOption) *types.IndexSettings {
	settings := &types.IndexSettings{}
	for _, option := range options {
		option(settings)
	}
	return settings
}

// WithIndexSettings applies the given settings to the index.
func WithIndexSettings(options ...IndexSettingsOption) Option[types.IndexState] {
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
	state := types.IndexState{
		Aliases: make(map[string]types.Alias),
	}

	for _, option := range options {
		state = option(state)
	}

	return state
}
