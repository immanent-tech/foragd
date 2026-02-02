// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package auth0

import (
	"context"
	"fmt"
	"sync"

	"github.com/auth0/go-auth0/v2/management/client"
	"github.com/auth0/go-auth0/v2/management/option"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/validation"
)

const (
	// ConfigEnvPrefix is the prefix applied to environment variables for configuring Auth0.
	ConfigEnvPrefix = config.ConfigEnvPrefix + "AUTH0_"
)

var cfg Config

// Config structure.
type Config struct {
	Domain       string `koanf:"domain"       validate:"required"`
	MgmtDomain   string `koanf:"mgmtdomain"   validate:"required"`
	ClientID     string `koanf:"clientid"     validate:"required"`
	ClientSecret string `koanf:"clientsecret" validate:"required"`
	CallbackURL  string `koanf:"callbackurl"  validate:"required,url"`
}

// loadConfigOnce loads the auth0 configuration and ensures this is only done
// one time, no matter how many times it is called.
var loadConfigOnce = sync.OnceValue(func() error {
	var err error
	cfg, err = config.Load[Config](ConfigEnvPrefix)
	if err != nil {
		return fmt.Errorf("auth0: unable to load config: %w", err)
	}
	err = validation.Validate.Struct(cfg)
	if err != nil {
		return fmt.Errorf("auth0: unable to validate config: %w", err)
	}
	return nil
})

// loadManagementAPI loads a connection to the Auth0 management API.
var loadManagementAPI = sync.OnceValues(func() (*client.Management, error) {
	var err error
	err = loadConfigOnce()
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

	return mgmt, nil
})
