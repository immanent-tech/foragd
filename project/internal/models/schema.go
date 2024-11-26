// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"database/sql"
	"time"

	"github.com/mmcdole/gofeed"
	"gorm.io/gorm"
)

type MetaFields struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	ID        string         `gorm:"primaryKey;"`
}

// User represents a user in postgres.
type User struct {
	MetaFields
	Preferences   map[string]any `gorm:"serializer:json;"`
	SessionData   *Tokens        `gorm:"-"`
	Subscriptions []Subscription
	// ReadItems     []ReadItems
	// SavedItems    []SavedItems
	Notifications []Notification
}

type Subscription struct {
	MetaFields
	Name   string
	UserID string
	FeedID string
	Topics []Topic `gorm:"many2many:subscription_topics;"`
}

type Topic struct {
	MetaFields
	Name          string         `gorm:"size:255;unique;index;"`
	Subscriptions []Subscription `gorm:"many2many:subscription_topics;"`
}

type Feed struct {
	LastFetched time.Time
	MetaFields
	Title         string
	Description   string
	ImageURL      *string
	ImageTitle    *string
	AuthorName    *string
	AuthorEmail   *string
	Copyright     *string
	Language      *string
	Generator     *string
	Type          string
	Version       string
	URL           string
	Categories    []*string `gorm:"type:text[]"`
	Subscriptions []Subscription
}

type FeedItem struct {
	*gofeed.Item
	FeedID string `json:"feed_id"`
	ItemID string `json:"item_id"`
}

// type ReadItems struct {
// 	FeedItem
// 	MetaFields
// 	UserID string
// }

// type SavedItems struct {
// 	FeedItem
// 	MetaFields
// 	UserID string
// }

type Notification struct {
	MetaFields
	Content        string
	UserID         string
	NotificationID uint
	Read           bool
}

func ToNullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}
