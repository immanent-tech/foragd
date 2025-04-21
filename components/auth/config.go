// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package auth

import (
	"errors"
	"sync"

	"github.com/joshuar/go-feed-me/components/config"
)

const (
	auth0ConfigEnvPrefix = "GOFEEDME_AUTH0_"
	auth0ConfigPrefix    = "auth0"
)

// Default auth0Config values.
var (
	auth0Config   = &Config{}
	ErrLoadConfig = errors.New("error loading config")
)

// Config structure.
type Config struct {
	Domain       string `toml:"domain"`
	ClientID     string `toml:"client_id"`
	ClientSecret string `toml:"client_secret"`
}

func (c *Config) DomainURL() string {
	// serverURI + "/login/auth0/callback"
	return "https://localhost:7000/login/auth0/callback"
}

// loadConfigOnce loads the auth0 configuration and ensures this is only done
// one time, no matter how many times it is called.
var loadConfigOnce = sync.OnceValue(loadConfig)

func loadConfig() error {
	if err := config.Load(auth0ConfigPrefix, auth0ConfigEnvPrefix, auth0Config); err != nil {
		return errors.Join(config.ErrLoadConfig, err)
	}

	return nil
}
