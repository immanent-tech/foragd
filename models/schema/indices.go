// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package schema

import (
	"github.com/immanent-tech/foragd/providers/elastic/ilm"
)

// Index constants.

const (
	feedsIndexPrefix          = "feeds"
	itemsSchemaPrefix         = "items"
	favoritesSchemaPrefix     = "favorites"
	usersSchemaPrefix         = "users"
	subscriptionsSchemaPrefix = "subscriptions"
	schedulerIndexPrefix      = "scheduler"
	sessionsSchemaPrefix      = "sessions"
	// indexWriteSuffix is the suffix appended to indices that are used for write (indexing) operations.
	indexWriteSuffix = "_rw"
	// indexReadSuffix is the suffix appended to indices that are used for read (search, get) operations.
	indexReadSuffix = "_ro"
)

var allIndices = []string{
	feedsIndexPrefix,
	itemsSchemaPrefix,
	favoritesSchemaPrefix,
	usersSchemaPrefix,
	subscriptionsSchemaPrefix,
	schedulerIndexPrefix,
	sessionsSchemaPrefix,
}

// IndicesOptions contains the options for performing index schema operations.
type IndicesOptions struct {
	Indices []string `arg:"" default:"all" enum:"all,feeds,items,favorites,users,subscriptions,scheduler,sessions" help:"List of indicies to perform command on."`
}

var allILMPolicies = map[string]*ilm.Policy{
	"logs": logsILMPolicy,
}

// ILMOptions contains the options for performing ILM schema operations.
type ILMOptions struct {
	Policies []string `arg:"" default:"all" enum:"all,logs"`
}
