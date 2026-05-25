// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//go:generate go tool oapi-codegen -config zyte-cfg.yaml zyte.yaml
package zyte

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/validation"
)

const (
	// ConfigEnvPrefix is the prefix applied to environment variables for configuring Zyte.
	ConfigEnvPrefix = "ZYTE_"
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

func (e *ResponseError) Error() string { return fmt.Sprintf("%s: %s", e.Title, e.Detail) }
func (e *ResponseError) Unwrap() error { return fmt.Errorf("%s: %s", e.Title, e.Detail) }

// HTTPStatus returns the status code of the API error.
func (e *ResponseError) HTTPStatus() int { return e.Status }

// WriteLog writes the ResponseError to the log at the appropriate level.
func (e *ResponseError) WriteLog(ctx context.Context) {
	switch {
	case e.HTTPStatus() < 400: //nolint:mnd // easier to read as a number.
		slogctx.FromCtx(ctx).DebugContext(ctx, e.Error())
	case e.HTTPStatus() < 500: //nolint:mnd // easier to read as a number.
		slogctx.FromCtx(ctx).WarnContext(ctx, e.Error())
	default:
		slogctx.FromCtx(ctx).ErrorContext(ctx, e.Error())
	}
}
