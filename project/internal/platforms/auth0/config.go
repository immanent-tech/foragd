// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package auth0

import (
	"errors"
	"sync"

	"github.com/joshuar/go-feed-me/internal/config"
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

// loadConfigOnce loads the auth0 configuration and ensures this is only done
// one time, no matter how many times it is called.
var loadConfigOnce = sync.OnceValue(loadConfig)

func loadConfig() error {
	if err := config.Load(auth0ConfigPrefix, auth0ConfigEnvPrefix, auth0Config); err != nil {
		return errors.Join(config.ErrLoadConfig, err)
	}

	return nil
}
