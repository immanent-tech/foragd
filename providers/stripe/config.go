// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package stripe

import (
	"fmt"
	"os"
	"sync"

	"github.com/immanent-tech/go-syndication/validation"
	"github.com/stripe/stripe-go/v83"

	"github.com/immanent-tech/foragd/config"
)

const (
	// ConfigEnvPrefix is the prefix applied to environment variables for configuring Stripe.
	ConfigEnvPrefix = config.ConfigEnvPrefix + "STRIPE_"
	// trialPeriodDays is the number of days a trial of an account lasts.
	trialPeriodDays  = 90
	trialEndBehavior = "pause"
)

var cfg = &Config{}

// Config structure.
type Config struct {
	// APIKey is the Stripe API key for communicating securely with the Stripe API.
	APIKey        string `koanf:"apikey"        validate:"required"`
	BaseURL       string `koanf:"baseurl"       validate:"required,url"`
	WebHookSecret string `koanf:"webhooksecret" validate:"required"`
}

// loadConfigOnce loads the Stripe configuration and ensures this is only done
// one time, no matter how many times it is called.
var loadConfigOnce = sync.OnceValue(func() error {
	err := config.Load(ConfigEnvPrefix, cfg)
	if err != nil {
		return fmt.Errorf("load environment variables: %w", err)
	}

	stripe.Key = cfg.APIKey //nolint:reassign // seems to be a recommended approach in the docs.
	cfg.BaseURL = os.Getenv("FORAGD_BASEURL")

	err = validation.Validate.Struct(cfg)
	if err != nil {
		return fmt.Errorf("validate config: %w", err)
	}
	return nil
})
