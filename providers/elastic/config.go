// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"fmt"
	"sync"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/validation"
)

const (
	elasticConfigEnvPrefix = "ELASTICSEARCH_"

	// ReqIDHeader is the value that will be used to assign a unique ID to an Elasticsearch API request (that can be
	// used to associate the API request with a web server request).
	ReqIDHeader = "X-Opaque-Id"
)

// Define default server configuration options.
var cfg *esconfig

// config defines the configuration options for connecting to Elasticsearch. Set from the environment.
type esconfig struct {
	CloudID  string   `koanf:"cloudid"  validate:"required"`
	APIKey   string   `koanf:"apikey"   validate:"required_with=CloudID"`
	URLs     []string `koanf:"urls"     validate:"required_without=CloudID"`
	CAFile   string   `koanf:"cafile"   validate:"omitempty,file"`
	Username string   `koanf:"username"`
	Password string   `koanf:"password"`
}

// loadConfigOnce loads the Elasticsearch configuration and ensures this is done
// one-time only, no matter how many times it is called.
var loadConfigOnce = sync.OnceValue(func() error {
	if err := config.Load(elasticConfigEnvPrefix, &cfg); err != nil {
		return fmt.Errorf("unable to load production config: %w", err)
	}
	if err := validation.Validate.Struct(cfg); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}
	return nil
})
