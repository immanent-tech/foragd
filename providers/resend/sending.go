// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package resend

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sync"

	"github.com/resend/resend-go/v3"
	slogctx "github.com/veqryn/slog-context"
)

type templateEmail struct {
	*resend.EmailTemplate

	mu sync.Mutex
}

type templateEmailOption func(*templateEmail)

func WithTemplateVariable(key string, value any) templateEmailOption {
	return func(te *templateEmail) {
		te.mu.Lock()
		defer te.mu.Unlock()
		te.Variables[key] = value
	}
}

// SendTemplateEmail sends the template with the given id to the given address, with any additional template options
// applied.
func SendTemplatedEmail(ctx context.Context, to string, templateID string, options ...templateEmailOption) error {
	template := &templateEmail{
		EmailTemplate: &resend.EmailTemplate{
			Variables: make(map[string]interface{}),
		},
	}

	for option := range slices.Values(options) {
		option(template)
	}

	client, err := loadClient()
	if err != nil {
		return fmt.Errorf("load client: %w", err)
	}

	params := &resend.SendEmailRequest{
		To:       []string{to},
		Template: template.EmailTemplate,
	}

	slogctx.FromCtx(ctx).Debug("Sending templated email.",
		slog.String("template", templateID),
	)

	_, err = client.Emails.Send(params)
	if err != nil {
		return fmt.Errorf("send template email: %w", err)
	}

	return nil
}
