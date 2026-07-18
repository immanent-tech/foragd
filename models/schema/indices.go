// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package schema

import (
	"github.com/immanent-tech/go-base/config"

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

func FeedsIndexRO() string {
	return feedsIndexPrefix + "_" + config.GetEnvironment().String() + indexReadSuffix
}

func FeedsIndexRW() string {
	return feedsIndexPrefix + "_" + config.GetEnvironment().String() + indexWriteSuffix
}

func ItemsIndexRO() string {
	return itemsSchemaPrefix + "_" + config.GetEnvironment().String() + indexReadSuffix
}

func ItemsIndexRW() string {
	return itemsSchemaPrefix + "_" + config.GetEnvironment().String() + indexWriteSuffix
}

func SubscriptionsIndexRO() string {
	return subscriptionsSchemaPrefix + "_" + config.GetEnvironment().String() + indexReadSuffix
}

func SubscriptionsIndexRW() string {
	return subscriptionsSchemaPrefix + "_" + config.GetEnvironment().String() + indexWriteSuffix
}

func FavoritesIndexRO() string {
	return favoritesSchemaPrefix + "_" + config.GetEnvironment().String() + indexReadSuffix
}

func FavoritesIndexRW() string {
	return favoritesSchemaPrefix + "_" + config.GetEnvironment().String() + indexWriteSuffix
}

func UsersIndexRO() string {
	return usersSchemaPrefix + "_" + config.GetEnvironment().String() + indexReadSuffix
}
func UsersIndexRW() string {
	return usersSchemaPrefix + "_" + config.GetEnvironment().String() + indexWriteSuffix
}

func SessionsIndexRO() string {
	return sessionsSchemaPrefix + "_" + config.GetEnvironment().String() + indexReadSuffix
}

func SessionsIndexRW() string {
	return sessionsSchemaPrefix + "_" + config.GetEnvironment().String() + indexWriteSuffix
}

func SchedulerIndexRO() string {
	return schedulerIndexPrefix + "_" + config.GetEnvironment().String() + indexReadSuffix
}

func SchedulerIndexRW() string {
	return schedulerIndexPrefix + "_" + config.GetEnvironment().String() + indexWriteSuffix
}

var allILMPolicies = map[string]*ilm.Policy{
	"logs": logsILMPolicy,
}

// ILMOptions contains the options for performing ILM schema operations.
type ILMOptions struct {
	Policies []string `arg:"" default:"all" enum:"all,logs"`
}
