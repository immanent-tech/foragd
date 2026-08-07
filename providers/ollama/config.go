// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package ollama

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/immanent-tech/go-base/config"
	"github.com/immanent-tech/go-base/validation"
)

const (
	// ConfigEnvPrefix is the prefix applied to environment variables for configuring ollama.
	ConfigEnvPrefix = "OLLAMA_"
)

var cfg = Config{
	BatchSize: 50,
	KeepAlive: config.NewDuration(5 * time.Minute),
}

// Config contains the pubsub configuration options.
type Config struct {
	// URL is the URL to the ollama server.
	URL string `koanf:"url" validate:"required,url"`
	// Model is the model to use.
	Model string `koanf:"model" validate:"required"`
	// BatchSize is the number of input texts to process at once.
	BatchSize int `koanf:"batchsize" validate:"omitempty,gt=0"`
	// KeepAlive is how long to keep a request alive.
	KeepAlive config.Duration `koanf:"keepalive"`
}

// LoadConfig loads the auth0 configuration and ensures this is only done
// one time, no matter how many times it is called.
var LoadConfig = sync.OnceValue(func() error {
	if err := config.Load(ConfigEnvPrefix, &cfg); err != nil {
		return fmt.Errorf("google: unable to load config: %w", err)
	}

	if err := validation.Validate.Struct(cfg); err != nil {
		return fmt.Errorf("google: unable to validate config: %w", err)
	}

	slog.Debug("Ollama config loaded.") //nolint:sloglint // we don't pass a context.
	return nil
})
