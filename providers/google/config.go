// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package gcp

import (
	"fmt"
	"log/slog"
	"os"
	"sync"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/validation"
)

const (
	// ConfigEnvPrefix is the prefix applied to environment variables for configuring Auth0.
	ConfigEnvPrefix = "GOOGLE_CLOUD_"
)

var cfg = Config{
	Service:  os.Getenv("K_SERVICE"),
	Revision: os.Getenv("K_REVISION"),
}

// Config contains the pubsub configuration options.
type Config struct {
	// ProjectID is the project ID. Sourced from the environment, otherwise the internal metadata server.
	ProjectID string `koanf:"project" validate:"required"`
	// InstanceID is the ID of the instances. Sourced from the internal metadata server. Will be an empty string if not
	// running in GCP.
	InstanceID string
	// Service is the service name. Sourced from the instance environment, otherwise an empty string if not running in
	// GCP.
	Service string
	// Revision is the service revision. Sourced from the instance environment, otherwise an empty string if not running
	// in GCP.
	Revision string
	// BillingAccountID is the ID of the billing account.
	BillingAccountID string `koanf:"billingaccountid"`
	// APIKey is an API key to use with various Google Cloud APIs
	APIKey string `koanf:"apikey"`
}

// LoadConfig loads the auth0 configuration and ensures this is only done
// one time, no matter how many times it is called.
var LoadConfig = sync.OnceValues(func() (*Config, error) {
	if err := config.Load(ConfigEnvPrefix, &cfg); err != nil {
		return nil, fmt.Errorf("google: unable to load config: %w", err)
	}
	if cfg.ProjectID == "" {
		// Try to fetch details from metadata endpoint.
		cfg.ProjectID, _ = queryMetadataServer("/computeMetadata/v1/project/project-id")
	}

	cfg.InstanceID, _ = queryMetadataServer("/computeMetadata/v1/instance/id")

	if err := validation.Validate.Struct(cfg); err != nil {
		return nil, fmt.Errorf("google: unable to validate config: %w", err)
	}

	slog.Info("GCP config loaded.") //nolint:sloglint // we don't pass a context.
	return &cfg, nil
})

// GetBillingAccountName gets the billing account name in the canonical format used by the Google APIs.
func (c *Config) GetBillingAccountName() string {
	return "billingAccounts/" + c.BillingAccountID
}
