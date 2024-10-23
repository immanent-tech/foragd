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
	"errors"

	"github.com/knadh/koanf/v2"
)

const (
	configPrefix       = "auth0"
	configClientID     = configPrefix + ".clientID"
	configClientSecret = configPrefix + ".clientSecret"
	configDomain       = configPrefix + ".domain"
)

var (
	ErrUnknownDomain   = errors.New("unknown auth0 domain")
	ErrUnknownClientID = errors.New("unknown auth0 client id")
	ErrInvalidConfig   = errors.New("invalid config")
)

type settings struct {
	Audience     string `toml:"audience"`
	ClientID     string `toml:"clientID"`
	ClientSecret string `toml:"clientSecret"`
	Domain       string `toml:"domain"`
	ServerPort   int
}

func getSettings(config *koanf.Koanf) settings {
	return settings{
		ClientID:     config.String(configClientID),
		ClientSecret: config.String(configClientSecret),
		Domain:       config.String(configDomain),
		ServerPort:   config.Int("server.port"),
	}
}
