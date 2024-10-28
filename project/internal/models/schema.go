// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"time"

	"github.com/mmcdole/gofeed"
	"gorm.io/gorm"
)

type MetaFields struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	ID        string         `gorm:"primary_key;"`
}

// User represents a user in postgres.
type User struct {
	MetaFields
	Preferences   map[string]any `gorm:"serializer:json;"`
	SessionData   *Tokens        `gorm:"-"`
	Topics        []Topic        `gorm:"many2many:user_topics;"`
	Subscriptions []Subscription
	ReadItems     []ReadItems
	SavedItems    []SavedItems
	Notifications []Notification
}

type Subscription struct {
	MetaFields
	Name   string
	UserID string
	FeedID string
}

type Topic struct {
	MetaFields
	Name  string `gorm:"size:255;unique;index;"`
	Users []User `gorm:"many2many:user_topics;"`
	Feeds []Feed `gorm:"many2many:feed_topics;"`
}

type Feed struct {
	*gofeed.Feed `gorm:"-"`
	MetaFields
	URL           string
	Topics        []Topic `gorm:"many2many:feed_topics;"`
	Subscriptions []Subscription
}

type FeedItem struct {
	*gofeed.Item `gorm:"-"`
	FeedID       string `json:"feed_id"`
	ItemID       string `gorm:"primarykey" json:"item_id"`
}

type ReadItems struct {
	FeedItem
	MetaFields
	UserID string
}

type SavedItems struct {
	FeedItem
	MetaFields
	UserID string
}

type Notification struct {
	MetaFields
	Content        string
	UserID         string
	NotificationID uint
	Read           bool
}
