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
	"context"
	"errors"
	"fmt"
	"log/slog"

	"gorm.io/gorm"

	"github.com/joshuar/go-feed-me/internal/models"
)

var (
	ErrAddUser      = errors.New("error adding user")
	ErrGetUser      = errors.New("error retrieving user")
	ErrInvalidToken = errors.New("session data is invalid")
	ErrUnknownUser  = errors.New("user does not exist")
)

func (c *Client) AddUser(_ context.Context, userID string, newUser *models.APIUser) error {
	user := &models.User{ID: userID}

	result := c.db.Create(&user)
	if result.Error != nil {
		return errors.Join(ErrAddUser, result.Error)
	}

	c.logger.Debug("Added user.",
		slog.String("id", user.ID),
	)

	return nil
}

func (c *Client) GetUserByID(_ context.Context, userID string) (*models.User, error) {
	var user models.User

	if result := c.db.First(&user, "id = ?", userID); result.Error != nil {
		return nil, fmt.Errorf("unable to get user from database: %w", result.Error)
	}

	return &user, nil
}

// IsValidUser checks that the given user ID exists in the database store. It
// returns a boolean for this check and a non-nil error if a problem occurred
// when trying to execute the check.
func (c *Client) IsValidUser(_ context.Context, userID string) (bool, error) {
	var user models.User

	result := c.db.First(&user, "id = ?", userID)

	switch {
	case result.Error != nil && errors.Is(result.Error, gorm.ErrRecordNotFound):
		return false, nil
	case result.Error != nil:
		return false, fmt.Errorf("%w: %w", ErrGetUser, result.Error)
	default:
		return true, nil
	}
}
