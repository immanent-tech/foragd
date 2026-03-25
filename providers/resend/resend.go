// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package resend

import (
	"errors"
	"fmt"
	"sync"

	"github.com/resend/resend-go/v3"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/validation"
)

const (
	// ConfigEnvPrefix is the prefix applied to environment variables for configuring Resend.
	ConfigEnvPrefix = config.ConfigEnvPrefix + "RESEND_"

	// TagUserID is a tag containing a user ID.
	TagUserID string = "user_id"
	// TagCategory is a tag containing a category.
	TagCategory string = "category"
	// TagCategoryPromotional is the promotional category.
	TagCategoryPromotional string = "promotional"
	// TagCategoryAccount is the account category.
	TagCategoryAccount string = "account"
)

var ErrInvalidEmail = errors.New("email is invalid")

var cfg Config

// Config structure.
type Config struct {
	WebHookSecret string `koanf:"webhooksecret" validate:"required"`
	APIKey        string `koanf:"apikey"        validate:"required"`
	CatchAllEmail string `koanf:"catchallemail" validate:"required,email"`
	AdminEmail    string `koanf:"adminemail"    validate:"required,email"`
	Key           string `koanf:"key"           validate:"required"`
	Salt          string `koanf:"salt"          validate:"required"`
}

// loadClient loads the resend API client and ensures this is only done one time, no matter how many times it is called.
var loadClient = sync.OnceValues(func() (*resend.Client, error) {
	if err := loadConfig(); err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	client := resend.NewClient(cfg.APIKey)
	return client, nil
})

// loadConfig loads the Resend configuration and ensures this is only done one time, no matter how many times it is
// called.
var loadConfig = sync.OnceValue(func() error {
	if err := config.Load(ConfigEnvPrefix, &cfg); err != nil {
		return fmt.Errorf("load environment variables: %w", err)
	}

	if err := validation.Validate.Struct(cfg); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}
	return nil
})
