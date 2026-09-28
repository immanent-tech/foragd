// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//go:generate go tool oapi-codegen -config zyte-cfg.yaml zyte.yaml
package zyte

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/go-resty/resty/v2"
	"github.com/immanent-tech/go-base/client"
	"github.com/immanent-tech/go-base/config"
	"github.com/immanent-tech/go-base/validation"
)

const (
	// ConfigEnvPrefix is the prefix applied to environment variables for configuring Zyte.
	ConfigEnvPrefix = "ZYTE_"

	extractEndpoint = "https://api.zyte.com/v1/extract"
)

var ErrNotFound = errors.New("not found")

// Config is the configuration for Zyte.
type Config struct {
	// APIKey is the api key used to authorize requests with the zyte API.
	APIKey string `koanf:"apikey" validate:"required"`
}

var cfg Config

var loadConfig = sync.OnceValue(func() error {
	if err := config.Load(ConfigEnvPrefix, &cfg); err != nil {
		return fmt.Errorf("load from envrionment: %w", err)
	}

	if err := validation.Validate.Struct(cfg); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	slog.Info("Zyte config loaded.") //nolint:sloglint // we don't pass a context.
	return nil
})

var loadHTTPClient = sync.OnceValue(func() *resty.Client {
	var userAgent string
	if appCfg, err := config.LoadAppConfig(); err != nil {
		userAgent = "Foragd/Unknown (+https://foragd.app/policies/bot)"
	} else {
		userAgent = appCfg.GetAppName() + "/" + appCfg.GetAppVersion() + " (+https://foragd.app/policies/bot)"

	}
	client := client.New().
		SetRetryCount(3).
		AddRetryCondition(
			// Add retry logic for 429 and 520 responses as per Zyte API guidelines.
			func(r *resty.Response, _ error) bool {
				return r.StatusCode() == http.StatusTooManyRequests || r.StatusCode() == 520
			},
		).
		SetHeader("User-Agent", userAgent).
		SetHeader("Content-Type", "application/json")
	return client
})

var bufPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}
