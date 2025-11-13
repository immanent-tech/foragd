// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package schema

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/elastic/go-elasticsearch/v9/typedapi/types"

	"github.com/immanent-tech/foragd/config"
)

const (
	// FeedsSchemaPrefix is a prefix used for feed related index/mapping/settings.
	FeedsSchemaPrefix = "feeds"
	// ItemsSchemaPrefix is a prefix used for item related index/mapping/settings.
	ItemsSchemaPrefix = "items"
	// FavoriteItemsSchemaPrefix is a prefix used for item archive related index/mapping/settings.
	FavoriteItemsSchemaPrefix = "favorite-items"
	// UsersSchemaPrefix is a prefix used for user related index/mapping/settings.
	UsersSchemaPrefix = "users"
	// SubscriptionsSchemaPrefix is a prefix used for subscription related index/mapping/settings.
	SubscriptionsSchemaPrefix = "subscriptions"
	// SchedulerSchemaPrefix is a prefix used for scheduler related index/mapping/settings.
	SchedulerSchemaPrefix = "scheduler"
	// SessionsSchemaPrefix is a prefix used for sessions related index/mapping/settings.
	SessionsSchemaPrefix = "sessions"
	// LogsSchemaPrefix is a prefix used for application logs related index/mapping/settings.
	LogsSchemaPrefix = "application_logs"
	// IndexWriteSuffix is the suffix appended to indicies that are used for write (indexing) operations.
	IndexWriteSuffix = "_rw"
	// IndexReadSuffix is the suffix appended to indicies that are used for read (search, get) operations.
	IndexReadSuffix = "_ro"
)

var (
	EnglishExactAnalyzerName = "english_exact"
	// FeedItemCommonMappings are the mappings that are common across both feed and item objects.
	FeedItemCommonMappings = NewProperties()
	// defaultMetadata defines default metadata.
	defaultMetadata = types.Metadata{
		"version":    json.RawMessage(fmt.Sprintf("%q", config.Version)),
		"created_at": json.RawMessage(fmt.Sprintf("%q", time.Now().UTC().String())),
	}
)

// Option is a reusable generic function for applying options to a type.
type Option[T any] func(T)
