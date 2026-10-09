/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package auth0

import (
	"context"
	"encoding/gob"
	"fmt"
	"sync"

	"github.com/auth0/go-auth0/v2/management"
	"github.com/auth0/go-auth0/v2/management/client"
	"github.com/auth0/go-auth0/v2/management/option"

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
	// The user's email address.
	Email string `json:"email" validate:"email"`
	// Indicates whether the user has verified their email address.
	EmailVerified bool `json:"email_verified"`
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
	// LoginsCount is the number of times the user has logged in. If a user is blocked and logs in, the blocked session
	// is still counted. For a new user, this will be 1 as creating the account is counted as the first login.
	LoginsCount int64 `json:"logins_count" validate:"omitempty,gt=1"`
	// Blocked indicates whether the user has been blocked. Importing enables subscribers to ensure that users remain
	// blocked when migrating to Auth0.
	Blocked bool `json:"blocked"`
	// Custom fields that store info about a user that influences the user’s access, such as support plan, security
	// roles (if not using the Authorization Core feature set), or access control groups.
	AppMetadata map[string]any `json:"app_metadata"`
}

// GetID returns a string that represents the ID of the external user.
func (u *UserProfile) GetID() string {
	return u.Subject
}

// GetEmail returns the email address associated with the external user.
func (u *UserProfile) GetEmail() string {
	return u.Email
}

type UserData struct {
	*management.GetUserResponseContent
	*management.UserResponseSchema
}

type UpdateUserData struct {
	*management.UpdateUserRequestContent

	ID string
}

type Manager struct {
	*client.Management
}

// LoadManager loads a connection to the Auth0 management API.
var LoadManager = sync.OnceValues(func() (*Manager, error) {
	cfg, err := loadConfigOnce()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	mgmt, err := client.New(
		cfg.MgmtDomain,
		option.WithClientCredentials(
			context.Background(),
			cfg.ClientID,
			cfg.ClientSecret,
		),
	)
	if err != nil {
		return nil, fmt.Errorf("new management api connection: %w", err)
	}

	return &Manager{Management: mgmt}, nil
})

// DeleteUser will delete the given user from the Auth0 backend.
func (m *Manager) DeleteUser(ctx context.Context, id string) error {
	// ! Only supported with Auth0 enterprise subscription.
	// // Delete the user's active sessions.
	// if err := mgmt.Users.Sessions.Delete(ctx, id); err != nil {
	// 	slogctx.FromCtx(ctx).Warn("Could not remove active sessions for user while deleting account.",
	// 		slog.Any("error", err),
	// 	)
	// }

	if err := m.Users.Delete(ctx, id); err != nil {
		return fmt.Errorf("unable to delete user account on backend: %w", err)
	}
	return nil
}

// GetUser fetches the user with the given ID from Auth0.
func (m *Manager) GetUser(ctx context.Context, id string) (*UserData, error) {
	resp, err := m.Users.Get(ctx, id, &management.GetUserRequestParameters{})
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	return &UserData{GetUserResponseContent: resp}, nil
}

// UpdateUser updates user data in Auth0.
func (m *Manager) UpdateUser(ctx context.Context, update *UpdateUserData) error {
	// Update the user.
	_, err := m.Users.Update(
		ctx,
		update.ID,
		update.UpdateUserRequestContent,
	)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	return nil
}

// GetNewInactiveUsers returns all accounts created on the backend that haven't yet logged in to the app.
func (m *Manager) GetNewInactiveUsers(ctx context.Context) ([]*UserData, error) {
	query := "logins_count:[0 TO 1]"
	resp, err := m.Users.List(ctx, &management.ListUsersRequestParameters{
		Q:       &query,
		PerPage: management.Int(100),
	})
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}

	var users []*UserData

	iterator := resp.Iterator()
	for iterator.Next(ctx) {
		users = append(users, &UserData{UserResponseSchema: iterator.Current()})
	}

	if iterator.Err() != nil {
		return nil, fmt.Errorf("loop over inactive users: %w", iterator.Err())
	}

	return users, nil
}

// UpdateUserCustomisation writes local user customization changes back to the authorization service.
func (m *Manager) UpdateUserCustomisation(ctx context.Context, request *models.EditUserRequest) error {
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
	updates := &UpdateUserData{
		ID: user.GetExternalID(),
		UpdateUserRequestContent: &management.UpdateUserRequestContent{
			Nickname:    &request.Nickname,
			Email:       &request.Email,
			Picture:     request.AvatarURL,
			VerifyEmail: &verifyEmail,
		},
	}

	return m.UpdateUser(ctx, updates)
}

// UpdateUserMetadata updates the metadata associated with a user on the backend service.
func (m *Manager) UpdateUserMetadata(ctx context.Context, id string, key string, value any) error {
	update := &UpdateUserData{
		ID: id,
		UpdateUserRequestContent: &management.UpdateUserRequestContent{
			UserMetadata: &management.UserMetadata{
				key: value,
			},
		},
	}
	return m.UpdateUser(ctx, update)
}

// ChangeUserPassword will perform a password change on behalf of a user.
func (m *Manager) ChangeUserPassword(ctx context.Context, request *models.ChangePasswordRequest) error {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound)
	}
	// Create backend update request.
	updates := &management.UpdateUserRequestContent{
		Password: &request.NewPassword,
	}
	// Update the user.
	_, err := m.Users.Update(
		ctx,
		user.GetExternalID(),
		updates,
	)
	if err != nil {
		return fmt.Errorf("change password: %w", err)
	}
	return nil
}
