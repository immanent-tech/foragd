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

package auth0

import (
	"context"
	"errors"
	"fmt"

	"github.com/auth0/go-auth0/management"

	"github.com/joshuar/go-feed-me/models"

	"github.com/auth0/go-auth0/authentication"
	"github.com/auth0/go-auth0/authentication/database"
)

var ErrAuth0Backend = errors.New("auth0 backend error")

const userDBConnection = "Username-Password-Authentication"

type UserAPI struct {
	api *authentication.Authentication
}

type ManagementAPI struct {
	*management.Management
}

// NewUserAPI creates a new user API backend object for user account management.
func NewUserAPI(ctx context.Context) (*UserAPI, error) {
	if err := loadConfigOnce(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrAuth0Backend, err)
	}

	authAPI, err := authentication.New(
		ctx,
		auth0Config.Domain,
		authentication.WithClientID(auth0Config.ClientID),
		authentication.WithClientSecret(auth0Config.ClientSecret),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrAuth0Backend, err)
	}

	api := &UserAPI{
		api: authAPI,
	}

	return api, nil
}

func NewManagementAPI() (*ManagementAPI, error) {
	api, err := management.New(
		auth0Config.Domain,
		management.WithClientCredentials(
			context.Background(),
			auth0Config.ClientID,
			auth0Config.ClientSecret,
		),
	)
	if err != nil {
		return nil, fmt.Errorf("auth0 management api connection failed: %w", err)
	}
	return &ManagementAPI{Management: api}, nil
}

// Create will create a new user account with the given details on the backend.
func (u *UserAPI) Create(ctx context.Context, details *models.UserSignupRequest) (string, error) {
	userData := database.SignupRequest{
		Connection: userDBConnection,
		Nickname:   details.Nickname,
		Email:      details.Email,
		Password:   details.Password,
	}

	user, err := u.api.Database.Signup(ctx, userData)
	if err != nil {
		if authErr, ok := err.(*authentication.Error); ok {
			return "", fmt.Errorf("auth0 backend error: %w", authErr)
		}
		return "", fmt.Errorf("auth0 backend error: %w", err)
	}

	return user.ID, nil
}

func Delete(ctx context.Context, user *models.User) error {
	api, err := NewManagementAPI()
	if err != nil {
		return fmt.Errorf("failed to delete user account from backend: %w", err)
	}
	err = api.User.Delete(ctx, "auth0|"+user.ExternalUserId)
	if err != nil {
		return fmt.Errorf("failed to delete user account from backend: %w", err)
	}
	return nil
}
