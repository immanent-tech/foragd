// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package auth0

import (
	"context"
	"errors"
	"fmt"

	"github.com/auth0/go-auth0/management"

	"github.com/immanent-tech/go-feed-me/models"

	"github.com/auth0/go-auth0/authentication"
	"github.com/auth0/go-auth0/authentication/database"
)

const userDBConnection = "Username-Password-Authentication"

// UserAPI represents the Auth0 user API backend connection.
type UserAPI struct {
	*authentication.Authentication
}

// ManagementAPI represents the Auth0 management API backend connection.
type ManagementAPI struct {
	*management.Management
}

// NewUserAPI creates a authentication API connection.
func NewUserAPI(ctx context.Context) (*UserAPI, error) {
	// Load config.
	err := loadConfigOnce()
	if err != nil {
		return nil, fmt.Errorf("auth0: load config: %w", err)
	}
	// Set up connection to auth0 backend.
	authAPI, err := authentication.New(
		ctx,
		auth0Config.Domain,
		authentication.WithClientID(auth0Config.ClientID),
		authentication.WithClientSecret(auth0Config.ClientSecret),
	)
	if err != nil {
		return nil, fmt.Errorf("auth0: auth api backend: %w", err)
	}
	return &UserAPI{
		Authentication: authAPI,
	}, nil
}

// NewManagementAPI creates a new management API connection.
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
		return nil, fmt.Errorf("auth0: management api backend: %w", err)
	}
	return &ManagementAPI{Management: api}, nil
}

// Create will create a new user with the given details on the Auth0 backend.
func (u *UserAPI) Create(ctx context.Context, details *models.UserSignupRequest) (string, error) {
	userData := database.SignupRequest{
		Connection: userDBConnection,
		Nickname:   details.Nickname,
		Email:      details.Email,
		Password:   details.Password,
	}

	user, err := u.Database.Signup(ctx, userData)
	if err != nil {
		auth0Err := &authentication.Error{}
		if errors.Is(err, auth0Err) {
			return "", fmt.Errorf("auth0: create user: %w", auth0Err)
		}
		return "", fmt.Errorf("auth0: create user: %w", err)
	}

	return user.ID, nil
}

// Delete will delete the given user from the Auth0 backend.
func Delete(ctx context.Context, user *models.User) error {
	api, err := NewManagementAPI()
	if err != nil {
		return fmt.Errorf("auth0: delete user: %w", err)
	}
	err = api.User.Delete(ctx, user.ExternalUserId)
	if err != nil {
		return fmt.Errorf("auth0: delete user: %w", err)
	}
	return nil
}
