// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package auth0

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/auth0/go-auth0/v2/management"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
)

// UserProfile represents the data returned from the auth0 backend that represents an authorised user.
//
//	https://auth0.com/docs/manage-users/user-accounts/user-profiles/user-profile-structure
//
// https://pkg.go.dev/github.com/coreos/go-oidc/v3@v3.15.0/oidc#IDToken
type UserProfile struct {
	// URL of the server which issued this token.
	Issuer string `json:"iss"            validate:"required,url"`
	// The client ID, or set of client IDs, that this token is issued for.
	Audience string `json:"aud"            validate:"required"`
	// When the token was issued by the provider.
	IssuedAt int64 `json:"iat"            validate:"required"`
	// Expiry of the token.
	Expiry int64 `json:"exp"            validate:"required"`
	// A unique string which identifies the end user.
	Subject string `json:"sub"            validate:"required"`
	// ID of the current session.
	SessionID string `json:"sid"            validate:"required"`
	// The user's email address.
	Email string `json:"email"          validate:"email"`
	// Indicates whether the user has verified their email address.
	EmailVerified bool `json:"email_verified"`
	// URL pointing to the user's profile picture.
	Picture string `json:"picture"        validate:"omitempty,url"`
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
	// LoginsCount is the number of times the user has logged in. If a user is blocked and logs in, the blocked session
	// is still counted. For a new user, this will be 1 as creating the account is counted as the first login.
	LoginsCount int64 `json:"logins_count"   validate:"omitempty,gt=1"`
	// Blocked indicates whether the user has been blocked. Importing enables subscribers to ensure that users remain
	// blocked when migrating to Auth0.
	Blocked bool `json:"blocked"`
}

// GetID returns a string that represents the ID of the external user.
func (u *UserProfile) GetID() string {
	return u.Subject
}

// GetEmail returns the email address associated with the external user.
func (u *UserProfile) GetEmail() string {
	return u.Email
}

// DeleteUser will delete the given user from the Auth0 backend.
func DeleteUser(ctx context.Context, id string) error {
	mgmt, err := loadManagementAPI()
	if err != nil {
		return fmt.Errorf("unable to connect to auth0 management API: %w", err)
	}

	// Delete the user's active sessions.
	if err := mgmt.Users.Sessions.Delete(ctx, id); err != nil {
		slogctx.FromCtx(ctx).Warn("Could not remove active sessions for user while deleting account.",
			slog.Any("error", err),
		)
	}

	if err := mgmt.Users.Delete(ctx, id); err != nil {
		return fmt.Errorf("unable to delete user account on backend: %w", err)
	}
	return nil
}

func UpdateUser(ctx context.Context, request *models.EditUserRequest) error {
	mgmt, err := loadManagementAPI()
	if err != nil {
		return fmt.Errorf("load management API: %w", err)
	}
	user := models.UserFromCtx(ctx)
	if user == nil {
		return fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound)
	}
	// If the user changed their email, end a new verification email.
	var verifyEmail bool
	if user.Email != request.Email {
		verifyEmail = true
	}
	// Create update object.
	updates := &management.UpdateUserRequestContent{
		Nickname:    &request.Nickname,
		Email:       &request.Email,
		Picture:     &request.AvatarURL,
		VerifyEmail: &verifyEmail,
	}
	// Update the user.
	_, err = mgmt.Users.Update(
		ctx,
		user.GetExternalID(),
		updates,
	)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	return nil
}

// ChangeUserPassword will perform a password change on behalf of a user.
func ChangeUserPassword(ctx context.Context, request *models.ChangePasswordRequest) error {
	mgmt, err := loadManagementAPI()
	if err != nil {
		return fmt.Errorf("load management API: %w", err)
	}
	user := models.UserFromCtx(ctx)
	if user == nil {
		return fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound)
	}
	// Create update object.
	updates := &management.UpdateUserRequestContent{
		Password: &request.NewPassword,
	}
	// Update the user.
	_, err = mgmt.Users.Update(
		ctx,
		user.GetExternalID(),
		updates,
	)
	if err != nil {
		return fmt.Errorf("change password: %w", err)
	}
	return nil
}
