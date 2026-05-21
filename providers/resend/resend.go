// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package resend

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sync"

	"github.com/resend/resend-go/v3"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/validation"
)

const (
	// ConfigEnvPrefix is the prefix applied to environment variables for configuring Resend.
	ConfigEnvPrefix = "RESEND_"

	// TagUserID is a tag containing a user ID.
	TagUserID string = "user_id"
	// TagCategory is a tag containing a category.
	TagCategory string = "category"
	// TagCategoryPromotional is the promotional category.
	TagCategoryPromotional string = "promotional"
	// TagCategoryAccount is the account category.
	TagCategoryAccount string = "account"
	// TagCategorySupport is the support category.
	TagCategorySupport string = "support"
)

var ErrInvalidEmail = errors.New("email is invalid")

var cfg Config

// Config structure.
type Config struct {
	WebHookSecret string `koanf:"webhooksecret" validate:"required"`
	APIKey        string `koanf:"apikey"        validate:"required"`
	AdminEmail    string `koanf:"adminemail"    validate:"required,email"`
	ReplyToEmail  string `koanf:"replyto"       validate:"required,email"`
	Key           string `koanf:"key"           validate:"required"`
	Salt          string `koanf:"salt"          validate:"required"`
}

// LoadClient loads the resend API client and ensures this is only done one time, no matter how many times it is called.
var LoadClient = sync.OnceValues(func() (*resend.Client, error) {
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

func VerifyWebhook(req *http.Request, body []byte) (bool, error) {
	client, err := LoadClient()
	if err != nil {
		return false, fmt.Errorf("load client: %w", err)
	}

	// Extract Svix headers
	headers := resend.WebhookHeaders{
		Id:        req.Header.Get("svix-id"),
		Timestamp: req.Header.Get("svix-timestamp"),
		Signature: req.Header.Get("svix-signature"),
	}

	// Verify the webhook
	if err := client.Webhooks.Verify(&resend.VerifyWebhookOptions{
		Payload:       string(body),
		Headers:       headers,
		WebhookSecret: cfg.WebHookSecret,
	}); err != nil {
		return false, fmt.Errorf("verfication failed: %w", err)
	}

	return true, nil
}

func GetFullEmail(ctx context.Context, id string) (*ReceivedEmail, error) {
	client, err := LoadClient()
	if err != nil {
		return nil, fmt.Errorf("load client: %w", err)
	}

	// Retrieve the full email content and details.
	details, err := client.Emails.Receiving.GetWithContext(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get email details: %w", err)
	}

	return &ReceivedEmail{ReceivedEmail: details}, nil
}

func IsValidReplyTo(to []string) (bool, error) {
	if err := loadConfig(); err != nil {
		return false, fmt.Errorf("load config: %w", err)
	}

	// Ignore emails not explicitly addressed to our admin/catch-all address.
	if !slices.Contains(to, cfg.ReplyToEmail) {
		return false, nil
	}

	return true, nil
}

func ForwardAdminEmail(ctx context.Context, received *EmailRecieved) error {
	// Retrieve the full email content and details.
	email, err := GetFullEmail(ctx, received.EmailId)
	if err != nil {
		return fmt.Errorf("parse email: %w", err)
	}
	if err := email.Valid(); err != nil {
		return fmt.Errorf("validate email: %w", err)
	}

	if err := email.Forward(ctx, cfg.AdminEmail); err != nil {
		return fmt.Errorf("forward non-user email: %w", err)
	}

	return nil
}
