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

package handlers

import (
	"context"

	"github.com/joshuar/go-feed-me/internal/models"
)

type authAPI interface {
	Create(ctx context.Context, newUser *models.UserSignup) (models.UserDetails, error)
}

type userStore interface {
	AddUser(ctx context.Context, newUser models.UserDetails) error
	GetUser(ctx context.Context) (*models.User, error)
}

type dataStore interface {
	AddSubscription(ctx context.Context, newSubscription *models.SubscriptionRequest) error
	GetSubscriptions(ctx context.Context) ([]models.Subscription, error)
}

type cache interface {
	GetFeedItems(ctx context.Context, filters models.ItemFilters) ([]models.FeedItem, error)
}
