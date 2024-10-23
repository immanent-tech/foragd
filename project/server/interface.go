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
	"log/slog"

	"github.com/joshuar/go-feed-me/platform/auth0"
	"github.com/joshuar/go-feed-me/platform/elastic"
	"github.com/joshuar/go-feed-me/platform/postgres"
)

func (s Server) GetPort() int {
	if port := s.Config.Int("server.port"); port != 0 {
		return port
	}

	return defaultServerPort
}

func (s Server) GetEnvironment() string {
	if environment := s.Config.String("server.environment"); environment != "" {
		return environment
	}

	return EnvDevelopment.String()
}

func (s Server) GetLogLevel() string {
	if loglevel := s.Config.String("server.loglevel"); loglevel != "" {
		return loglevel
	}

	slog.Debug("Log level not found, using default debug.")

	return "debug"
}

func (s Server) CSP() []string {
	csp := s.Config.Strings("server.csp")
	if len(csp) > 0 {
		return csp
	}

	slog.Debug("CSP policy not found, using a default.")

	return []string{"default-src 'self';"}
}

// UserAPI returns the API endpoint for manipulating users.
func (s Server) UserAPI() *auth0.UserAPI {
	return s.API.user
}

// DataAPI returns the API endpoint for the backend data-store which holds
// cache/temp/non-permanent data.
func (s Server) DataAPI() *elastic.Client {
	return s.API.elastic
}

// StoreAPI returns the API endpoint for the backend data-store which holds
// permanent data.
func (s Server) StoreAPI() *postgres.Client {
	return s.API.pg
}

func (s Server) Authenticator() *auth0.Authenticator {
	return s.API.auth
}
