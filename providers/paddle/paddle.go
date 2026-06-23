// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package paddle

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"

	paddle "github.com/PaddleHQ/paddle-go-sdk/v5"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/validation"
)

const (
	// ConfigEnvPrefix is the prefix applied to environment variables for configuring Paddle.
	ConfigEnvPrefix = "PADDLE_"
)

var ErrNotFound = errors.New("not found")

// Config contains the pubsub configuration options.
type Config struct {
	// WebhookSecret is the secret used to verify webhook requests.
	WebhookSecret string `koanf:"webhooksecret" validate:"required"`
	// APIKey is the api key used to authorize requests with the paddle backend.
	APIKey string `koanf:"apikey" validate:"required"`
	// ClientToken is a token used for setting up paddle in the app.
	ClientToken string            `koanf:"clienttoken" validate:"required"`
	Pricing     map[string]string `koanf:"pricing"     validate:"required"`
	// // MonthlyPriceID is the ID of the monthly subscription object in the paddle backend.
	// MonthlyPriceID string `koanf:"monthlypriceid" validate:"required"`
	// // AnnualPriceID is the ID of the annual subscription object in the paddle backend.
	// AnnualPriceID string `koanf:"annualpriceid" validate:"required"`
	// CustomerPortalURL is the URL that customers can use to manage their subscription.
	CustomerPortalURL string `koanf:"customerportalurl" validate:"required"`
}

var cfg = Config{
	Pricing: make(map[string]string),
}

var loadConfig = sync.OnceValue(func() error {
	if err := config.Load(ConfigEnvPrefix, &cfg); err != nil {
		return fmt.Errorf("load from envrionment: %w", err)
	}

	if err := validation.Validate.Struct(cfg); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	slog.Info("Paddle config loaded.") //nolint:sloglint // we don't pass a context.
	return nil
})

type Client struct {
	*paddle.SDK
}

var client Client

var loadClient = sync.OnceValue(func() error {
	if err := loadConfig(); err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	switch config.GetEnvironment() {
	case config.EnvDevelopment:
		var err error
		client.SDK, err = paddle.NewSandbox(cfg.APIKey)
		if err != nil {
			return fmt.Errorf("load client: %w", err)
		}
		return nil
	case config.EnvProduction:
		var err error
		client.SDK, err = paddle.New(cfg.APIKey)
		if err != nil {
			return fmt.Errorf("load client: %w", err)
		}
		return nil
	}

	return errors.New("unsupported environment")
})

func GetPriceID(frequency string) (string, error) {
	if err := loadConfig(); err != nil {
		return "", fmt.Errorf("load config: %w", err)
	}

	if priceID, ok := cfg.Pricing[frequency]; ok {
		return priceID, nil
	}

	return "", fmt.Errorf("%w: price frequency %s", ErrNotFound, frequency)
}

func GetClientToken() (string, error) {
	if err := loadConfig(); err != nil {
		return "", fmt.Errorf("load config: %w", err)
	}

	return cfg.ClientToken, nil
}

func GetCustomerPortalURL() (string, error) {
	if err := loadConfig(); err != nil {
		return "", fmt.Errorf("load config: %w", err)
	}

	return cfg.CustomerPortalURL, nil
}
