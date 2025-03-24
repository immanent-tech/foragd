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
	"log/slog"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"

	"github.com/auth0/go-auth0/authentication"
	"github.com/auth0/go-auth0/authentication/database"
)

var ErrConnectAPIFail = errors.New("could not connect to Auth0 API")

const UserDBConnection = "Username-Password-Authentication"

type UserAPI struct {
	api    *authentication.Authentication
	logger *slog.Logger
}

type UserSignup struct {
	details *database.SignupResponse
}

func (u *UserSignup) UserID() string {
	return "auth0|" + u.details.ID
}

func (u *UserSignup) Nickname() string {
	return u.details.Nickname
}

func (u *UserSignup) Email() string {
	return u.details.Email
}

func (u *UserSignup) Verified() bool {
	return u.details.EmailVerified
}

func NewUserAPI(ctx context.Context) (*UserAPI, error) {
	if err := loadConfigOnce(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConnectAPIFail, err)
	}

	authAPI, err := authentication.New(
		ctx,
		auth0Config.Domain,
		authentication.WithClientID(auth0Config.ClientID),
		authentication.WithClientSecret(auth0Config.ClientSecret),
	)
	if err != nil {
		return nil, fmt.Errorf("could not connect to Auth0 API: %w", err)
	}

	api := &UserAPI{
		api:    authAPI,
		logger: logging.FromContext(ctx).With(slog.Group("auth0")),
	}

	return api, nil
}

func (u *UserAPI) Create(ctx context.Context, details *models.UserSignupRequest) (string, error) {
	userData := database.SignupRequest{
		Connection: UserDBConnection,
		Nickname:   *details.Nickname,
		Email:      details.Email,
		Password:   details.Password,
	}

	user, err := u.api.Database.Signup(ctx, userData)
	if err != nil {
		return "", fmt.Errorf("user creation failed: %w", err)
	}

	return user.ID, nil
}
