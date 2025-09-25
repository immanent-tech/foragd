// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package auth0

import (
	"context"
	"encoding/gob"
	"fmt"

	"github.com/auth0/go-auth0/management"

	"github.com/immanent-tech/foragd/models"
)

func init() {
	gob.Register(UserProfile{})
}

// UserProfile represents the data returned from the auth0 backend that represents an authorised user.
//
//	https://auth0.com/docs/manage-users/user-accounts/user-profiles/user-profile-structure
//
// https://pkg.go.dev/github.com/coreos/go-oidc/v3@v3.15.0/oidc#IDToken
type UserProfile struct {
	// URL of the server which issued this token.
	Issuer string `json:"iss" validate:"required,url"`
	// The client ID, or set of client IDs, that this token is issued for.
	Audience string `json:"aud" validate:"required"`
	// When the token was issued by the provider.
	IssuedAt int64 `json:"iat" validate:"required"`
	// Expiry of the token.
	Expiry int64 `json:"exp" validate:"required"`
	// A unique string which identifies the end user.
	Subject string `json:"sub" validate:"required"`
	// ID of the current session.
	SessionID string `json:"sid" validate:"required"`

	// URL pointing to the user's profile picture.
	Picture string `json:"picture" validate:"omitempty,url"`
	// The user's family name.
	FamilyName string `json:"family_name"`
	// The user's family name.
	GivenName string `json:"given_name"`
	// The user's full name.
	Name string `json:"name"`
	// The user's nickname.
	Nickname string `json:"nickname"`
	// Timestamp indicating when the user's profile was last updated/modified.
	UpdatedAt string `json:"updated_at"`
}

// GetID returns a string that represents the ID of the external user.
func (u *UserProfile) GetID() string {
	return u.Subject
}

// ManagementAPI represents the Auth0 management API backend connection.
type ManagementAPI struct {
	*management.Management
}

// NewManagementAPI creates a new management API connection.
func NewManagementAPI() (*ManagementAPI, error) {
	api, err := management.New(
		cfg.Domain,
		management.WithClientCredentials(
			context.Background(),
			cfg.ClientID,
			cfg.ClientSecret,
		),
	)
	if err != nil {
		return nil, fmt.Errorf("auth0: management api backend: %w", err)
	}
	return &ManagementAPI{Management: api}, nil
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
