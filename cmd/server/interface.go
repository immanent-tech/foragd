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

package server

import (
	"github.com/joshuar/go-feed-me/internal/auth"
	"github.com/joshuar/go-feed-me/internal/platforms/auth0"
)

func (s Server) AppSecret() string {
	return ServerConfig.Secret
}

// Port returns the port on which the server is listening.
func Port() int {
	return ServerConfig.Port
}

// UserAPI returns the API endpoint for manipulating users.
func (s Server) UserAPI() *auth0.UserAPI {
	return s.API.user
}

// DataAPI returns the API endpoint for the backend data-store which holds
// cache/temp/non-permanent data.
func (s Server) DataAPI() DataAPI {
	return s.API.elastic
}

func (s Server) Authenticator() *auth0.Authenticator {
	return s.API.auth
}

func (s Server) AuthAPI() *auth.Authenticator {
	return s.API.goth
}
