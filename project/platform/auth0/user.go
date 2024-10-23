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
	"fmt"
	"log/slog"

	"github.com/knadh/koanf/v2"

	"github.com/joshuar/go-feed-me/logging"
	"github.com/joshuar/go-feed-me/model"

	"github.com/auth0/go-auth0/authentication"
	"github.com/auth0/go-auth0/authentication/database"
)

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

func NewUserAPI(ctx context.Context, config *koanf.Koanf) (*UserAPI, error) {
	settings := getSettings(config)

	authAPI, err := authentication.New(
		ctx,
		settings.Domain,
		authentication.WithClientID(settings.ClientID),
		authentication.WithClientSecret(settings.ClientSecret),
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

func (u *UserAPI) Create(ctx context.Context, details *model.UserSignup) (model.UserDetails, error) {
	userData := database.SignupRequest{
		Connection: UserDBConnection,
		Nickname:   details.Nickname,
		Email:      details.Email,
		Password:   details.Password,
	}

	createdUser, err := u.api.Database.Signup(ctx, userData)
	if err != nil {
		return nil, fmt.Errorf("user creation failed: %w", err)
	}

	return &UserSignup{details: createdUser}, nil
}
