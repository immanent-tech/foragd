// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package resend

import (
	"fmt"
	"sync"

	"github.com/immanent-tech/go-syndication/validation"

	"github.com/immanent-tech/foragd/config"
)

const (
	// ConfigEnvPrefix is the prefix applied to environment variables for configuring Resend.
	ConfigEnvPrefix = config.ConfigEnvPrefix + "RESEND_"
)

var cfg Config

// Config structure.
type Config struct {
	WebHookSecret string `koanf:"webhooksecret" validate:"required"`
	APIKey        string `koanf:"apikey"        validate:"required"`
}

// loadConfig loads the Resend configuration and ensures this is only done
// one time, no matter how many times it is called.
var loadConfig = sync.OnceValue(func() error {
	var err error
	cfg, err = config.Load[Config](ConfigEnvPrefix)
	if err != nil {
		return fmt.Errorf("load environment variables: %w", err)
	}

	err = validation.Validate.Struct(cfg)
	if err != nil {
		return fmt.Errorf("validate config: %w", err)
	}
	return nil
})
