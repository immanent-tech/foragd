// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package auth0

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/auth0/go-auth0/v2/management"
	slogctx "github.com/veqryn/slog-context"
	"github.com/zeebo/xxh3"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/service"
)

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

// DeleteUser will delete the given user from the Auth0 backend.
func DeleteUser(ctx context.Context, id string) error {
	mgmt, err := loadManagementAPI()
	if err != nil {
		return fmt.Errorf("unable to connect to auth0 management API: %w", err)
	}

	// ! Only supported with Auth0 enterprise subscription.
	// // Delete the user's active sessions.
	// if err := mgmt.Users.Sessions.Delete(ctx, id); err != nil {
	// 	slogctx.FromCtx(ctx).Warn("Could not remove active sessions for user while deleting account.",
	// 		slog.Any("error", err),
	// 	)
	// }

	if err := mgmt.Users.Delete(ctx, id); err != nil {
		return fmt.Errorf("unable to delete user account on backend: %w", err)
	}
	return nil
}

// CreateUserFromProfileData creates a new user from the external provider details.
func CreateUserFromProfileData(ctx context.Context, profile *UserProfile) (*models.User, error) {
	auth0User, err := GetUser(ctx, profile.GetID())
	if err != nil {
		return nil, fmt.Errorf("get user details: %w", err)
	}

	id := "user_" + strconv.FormatUint(xxh3.Hash([]byte(profile.GetID())), 10)
	ts := time.Now().UTC()
	lastLogin, err := time.Parse(time.RFC3339, auth0User.GetUserResponseContent.GetLastLogin().String)
	if err != nil {
		slogctx.FromCtx(ctx).Warn("Unable to parse last login time from user profile data. Using unix epoch.",
			slog.String("user_id", id),
			slog.String("user_external_id", profile.GetID()),
			slog.Any("error", err),
		)
		lastLogin = models.UnixEpoch
	}
	user := &models.User{
		CreatedAt:      ts,
		UpdatedAt:      &ts,
		ExternalUserID: profile.GetID(),
		Provider:       strings.Split(profile.GetID(), "|")[0],
		Email:          auth0User.GetUserResponseContent.GetEmail(),
		UserID:         id,
		AvatarURL:      new(auth0User.GetUserResponseContent.GetPicture()),
		LoginCount:     *auth0User.GetUserResponseContent.LoginsCount,
		LastLogin:      lastLogin,
		Metadata: models.UserMetadata{
			EmailVerified:    auth0User.GetUserResponseContent.GetEmailVerified(),
			PromotionalEmail: true,
		},
		Settings: models.UserSettings{
			ShowOnboarding:        true,
			ShowSubscriptionStats: false,
			MarkArticleReadOnView: true,
		},
	}
	if accepted, ok := auth0User.GetUserResponseContent.GetAppMetadata()["policies_accepted"].(bool); ok {
		user.Metadata.PoliciesAccepted = accepted
	}

	if err := elastic.CreateDoc(ctx, schema.UsersIndexRW(), user.GetID(), user); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	return user, nil
}

// GetUser fetches the user with the given ID from Auth0.
func GetUser(ctx context.Context, id string) (*UserData, error) {
	mgmt, err := loadManagementAPI()
	if err != nil {
		return nil, fmt.Errorf("load management API: %w", err)
	}
	resp, err := mgmt.Users.Get(ctx, id, &management.GetUserRequestParameters{})
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	return &UserData{GetUserResponseContent: resp}, nil
}

// UpdateUser updates user data in Auth0.
func UpdateUser(ctx context.Context, update *UpdateUserData) error {
	mgmt, err := loadManagementAPI()
	if err != nil {
		return fmt.Errorf("load management API: %w", err)
	}

	// Update the user.
	_, err = mgmt.Users.Update(
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
func GetNewInactiveUsers(ctx context.Context) ([]*UserData, error) {
	mgmt, err := loadManagementAPI()
	if err != nil {
		return nil, fmt.Errorf("load management API: %w", err)
	}
	query := "logins_count:[0 TO 1]"
	resp, err := mgmt.Users.List(ctx, &management.ListUsersRequestParameters{
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

func UpdateUserCustomisation(ctx context.Context, request *models.EditUserRequest) error {
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

	return UpdateUser(ctx, updates)
}

func UpdateUserMetadata(ctx context.Context, id string, key string, value any) error {
	update := &UpdateUserData{
		ID: id,
		UpdateUserRequestContent: &management.UpdateUserRequestContent{
			UserMetadata: &management.UserMetadata{
				key: value,
			},
		},
	}
	return UpdateUser(ctx, update)
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

// SyncUser tries to sync relevant user data from the auth backend to the local data.
func SyncUser(ctx context.Context, localUser *models.User) {
	auth0User, err := GetUser(ctx, localUser.GetExternalID())
	if err != nil {
		slogctx.FromCtx(ctx).Error("Could not sync user data.",
			slog.String("user_id", localUser.GetID()),
			slog.Any("error", err))
		return
	}

	// Create needed updates by comparing request values to existing user values and adding new values to updates map as appropriate.
	updates := make(map[string]any)
	// Overwrite local avatar with remote avatar if different
	if avatarURL := auth0User.GetUserResponseContent.GetPicture(); localUser.GetAvatar() != avatarURL {
		updates["avatar_url"] = avatarURL
		localUser.AvatarURL = &avatarURL
	}
	// Overwrite local nickname with remote nickname if different
	if nickname := auth0User.GetUserResponseContent.GetNickname(); localUser.GetNickname() != nickname {
		updates["nickname"] = nickname
		localUser.Nickname = nickname
	}
	// Overwrite local email with remote email if different
	if email := auth0User.GetUserResponseContent.GetEmail(); localUser.GetEmail() != email {
		updates["email"] = email
		localUser.Email = email
	}
	// Update login count.
	localUser.LoginCount = auth0User.GetUserResponseContent.GetLoginsCount()
	// Update last login timestamp.
	if lastLogin, err := time.Parse(time.RFC3339, auth0User.GetUserResponseContent.GetLastLogin().String); err != nil {
		slogctx.FromCtx(ctx).Warn("Unable to parse last login time from user profile data.",
			slog.String("user_id", localUser.GetID()),
			slog.Any("error", err),
		)
	} else {
		localUser.LastLogin = lastLogin
	}
	// Update user metadata.
	metadata := localUser.Metadata
	if accepted, ok := auth0User.GetUserResponseContent.GetAppMetadata()["policies_accepted"].(bool); ok &&
		metadata.PoliciesAccepted != accepted {
		metadata.PoliciesAccepted = accepted
	}
	if emailVerified := auth0User.GetUserResponseContent.GetEmailVerified(); emailVerified != metadata.EmailVerified {
		metadata.EmailVerified = emailVerified
	}
	localUser.Metadata = metadata
	updates["metadata"] = metadata

	// If no updates are necessary, bail early.
	if len(updates) > 0 {
		if err := service.UpdateUser(ctx, localUser, updates); err != nil {
			slogctx.FromCtx(ctx).Error("Could not sync user data.",
				slog.String("user_id", localUser.GetID()),
				slog.Any("error", err))
			return
		}
	}
}
