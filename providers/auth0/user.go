// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package auth0

import (
	"context"
	"encoding/gob"
	"fmt"
	"log/slog"

	"github.com/auth0/go-auth0/management"
	slogctx "github.com/veqryn/slog-context"

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

// GetEmail returns the email address associated with the external user.
func (u *UserProfile) GetEmail() string {
	return u.Name
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

// DeleteUser will delete the given user from the Auth0 backend.
func DeleteUser(ctx context.Context, user *models.User) error {
	api, err := NewManagementAPI()
	if err != nil {
		return fmt.Errorf("unable to connect to auth0 management API: %w", err)
	}
	// Delete the user's active sessions.
	err = api.User.DeleteUserSessions(ctx, user.ExternalUserId)
	if err != nil {
		slogctx.FromCtx(ctx).Warn("Could not remove active sessions for user while deleting account.",
			slog.Any("error", err),
		)
	}
	err = api.User.Delete(ctx, user.ExternalUserId)
	if err != nil {
		return fmt.Errorf("unable to delete user account on backend: %w", err)
	}
	return nil
}

func UpdateUser(ctx context.Context, request *models.EditUserRequest) error {
	api, err := NewManagementAPI()
	if err != nil {
		return fmt.Errorf("unable to connect to auth0 management API: %w", err)
	}
	user, err := models.UserFromCtx(ctx)
	if err != nil {
		return fmt.Errorf("could not retrieve current user from context: %w", err)
	}
	// If the user changed their email, end a new verification email.
	var verifyEmail bool
	if user.Email != request.Email {
		verifyEmail = true
	}
	// Create update object.
	updates := &management.User{
		Nickname:    &request.Nickname,
		Email:       &request.Email,
		Picture:     &request.AvatarURL,
		VerifyEmail: &verifyEmail,
	}
	// Update the user.
	err = api.User.Update(ctx, user.ExternalUserId, updates)
	if err != nil {
		return fmt.Errorf("unable to update user in backend: %w", err)
	}
	return nil
}

func ChangeUserPassword(ctx context.Context, request *models.ChangePasswordRequest) error {
	api, err := NewManagementAPI()
	if err != nil {
		return fmt.Errorf("unable to connect to auth0 management API: %w", err)
	}
	user, err := models.UserFromCtx(ctx)
	if err != nil {
		return fmt.Errorf("could not retrieve current user from context: %w", err)
	}
	// Create update object.
	updates := &management.User{
		Password: &request.NewPassword,
	}
	// Update the user.
	err = api.User.Update(ctx, user.ExternalUserId, updates)
	if err != nil {
		return fmt.Errorf("unable to update user in backend: %w", err)
	}
	return nil
}
