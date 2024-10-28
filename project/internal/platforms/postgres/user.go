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

	"gorm.io/gorm"

	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/server/session"
)

var (
	ErrUserNotCreated = errors.New("no user created")
	ErrInvalidToken   = errors.New("session data is invalid")
	ErrUnknownUser    = errors.New("user does not exist")
)

func (c *Client) AddUser(_ context.Context, user models.UserDetails) error {
	newUser := &models.User{
		Preferences: models.NewUserPreferences(),
	}

	newUser.ID = user.UserID()

	tx := c.db.Create(newUser)

	if tx.RowsAffected == 0 {
		return ErrUserNotCreated
	}

	if tx.Error != nil {
		return fmt.Errorf("add user: %w", tx.Error)
	}

	return nil
}

func (c *Client) ValidateUser(ctx context.Context) (bool, error) {
	var user models.User

	tokens, err := session.GetTokens(ctx)
	if err != nil {
		return false, errors.Join(ErrInvalidToken, err)
	}

	tx := c.db.First(&user, "id = ?", tokens.UserID())
	if tx.Error != nil || errors.Is(tx.Error, gorm.ErrRecordNotFound) {
		return false, errors.Join(ErrUnknownUser, err)
	}

	return true, nil
}

func (c *Client) GetUser(ctx context.Context) (*models.User, error) {
	var user models.User

	tokens, err := session.GetTokens(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to get user details from session: %w", err)
	}

	tx := c.db.First(&user, "id = ?", tokens.UserID())
	if tx.Error != nil || errors.Is(tx.Error, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("unable to get user from database: %w", tx.Error)
	}

	user.SessionData = tokens

	return &user, nil
}
