// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"time"

	"github.com/mmcdole/gofeed"
	"gorm.io/gorm"
)

// User represents a user in postgres.
type User struct {
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Preferences   map[string]any `gorm:"serializer:json;"`
	SessionData   *Tokens        `gorm:"-"`
	DeletedAt     gorm.DeletedAt `gorm:"index"`
	ID            string         `gorm:"primary_key;"`
	Topics        []Topic        `gorm:"many2many:user_topics;"`
	Subscriptions []Subscription
	ReadItems     []ReadItems
	SavedItems    []SavedItems
	Notifications []Notification
}

type Subscription struct {
	ID        string `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	Name      string
	UserID    string
	FeedID    string
}

type Topic struct {
	ID        string `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	Name      string         `gorm:"size:255;unique;index;"`
	Users     []User         `gorm:"many2many:user_topics;"`
	Feeds     []Feed         `gorm:"many2many:feed_topics;"`
}

type Feed struct {
	*gofeed.Feed  `gorm:"-"`
	ID            string `gorm:"primarykey"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     gorm.DeletedAt `gorm:"index"`
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
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	UserID    string
}

type SavedItems struct {
	FeedItem
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	UserID    string
}

type Notification struct {
	ID             string `gorm:"primarykey"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      gorm.DeletedAt `gorm:"index"`
	Content        string
	UserID         string
	NotificationID uint
	Read           bool
}
