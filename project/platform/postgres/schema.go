// Copyright (C) 2024 Joshua Rich <joshua.rich@gmail.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package postgres

import (
	"time"

	"gorm.io/gorm"

	"github.com/joshuar/go-feed-me/platform/session"
)

// User represents a user in postgres.
type User struct {
	Preferences   map[string]any `gorm:"serializer:json;"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     gorm.DeletedAt `gorm:"index"`
	ID            string         `gorm:"primary_key;"`
	Topics        []Topic        `gorm:"many2many:user_topics;"`
	Subscriptions []Subscription
	ReadItems     []ReadItems
	SavedItems    []SavedItems
	Notifications []Notification
	SessionData   *session.Tokens `gorm:"-"`
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
	ID            string `gorm:"primarykey"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     gorm.DeletedAt `gorm:"index"`
	URL           string
	Topics        []Topic `gorm:"many2many:feed_topics;"`
	Subscriptions []Subscription
}

type ReadItems struct {
	ID        string `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	ItemID    string
	UserID    string
}

type SavedItems struct {
	ID        string `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	ItemID    string
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
