// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package auth

import (
	"fmt"
	"sync"

	"github.com/immanent-tech/go-feed-me/config"
)

const (
	auth0ConfigEnvPrefix = "GOFEEDME_AUTH0_"
	auth0ConfigPrefix    = "auth0"
)

// Default auth0Config values.
var auth0Config = &Config{}

// Config structure.
type Config struct {
	Domain       string `toml:"domain"`
	ClientID     string `toml:"client_id"`
	ClientSecret string `toml:"client_secret"`
}

func (c *Config) domainURL() string {
	// serverURI + "/login/auth0/callback"
	return "https://localhost:7000/login/auth0/callback"
}

// loadConfigOnce loads the auth0 configuration and ensures this is only done
// one time, no matter how many times it is called.
var loadConfigOnce = sync.OnceValue(loadConfig)

func loadConfig() error {
	err := config.Load(auth0ConfigPrefix, auth0ConfigEnvPrefix, auth0Config)
	if err != nil {
		return fmt.Errorf("auth0: unable to load config: %w", err)
	}

	return nil
}
