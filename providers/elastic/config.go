// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"fmt"
	"sync"

	"github.com/immanent-tech/go-base/config"
	"github.com/immanent-tech/go-base/validation"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("github.com/immanent-tech/foragd/providers/elastic")

const (
	elasticConfigEnvPrefix = "ELASTICSEARCH_"

	// ReqIDHeader is the value that will be used to assign a unique ID to an Elasticsearch API request (that can be
	// used to associate the API request with a web server request).
	ReqIDHeader = "X-Opaque-Id"
)

// Config defines the configuration options for connecting to Elasticsearch. Set from the environment.
type Config struct {
	CloudID  string   `koanf:"cloudid"  validate:"required_without=URLs"`
	APIKey   string   `koanf:"apikey"   validate:"required_without_all=Username Password"`
	URLs     []string `koanf:"urls"     validate:"required_without=CloudID"`
	CAFile   string   `koanf:"cafile"   validate:"omitempty,file"`
	Username string   `koanf:"username" validate:"required_without=APIKey"`
	Password string   `koanf:"password" validate:"required_with=Username"`
}

// loadConfig loads the Elasticsearch configuration and ensures this is done one-time only, no matter how many times it
// is called.
var loadConfig = sync.OnceValues(func() (*Config, error) {
	cfg := &Config{}
	if err := config.Load(elasticConfigEnvPrefix, &cfg); err != nil {
		return nil, fmt.Errorf("unable to load production config: %w", err)
	}
	if err := validation.Validate.Struct(cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}
	return cfg, nil
})
