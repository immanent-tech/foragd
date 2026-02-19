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
	tags []resend.Tag

	mu sync.Mutex
}

type TemplateEmailOption func(*templateEmail)

func WithTemplateVariable(key string, value any) TemplateEmailOption {
	return func(te *templateEmail) {
		te.mu.Lock()
		defer te.mu.Unlock()
		te.Variables[key] = value
	}
}

func WithTag(key string, value string) TemplateEmailOption {
	return func(te *templateEmail) {
		te.mu.Lock()
		defer te.mu.Unlock()
		te.tags = append(te.tags, resend.Tag{Name: key, Value: value})
	}
}

// SendTemplatedEmail sends the template with the given id to the given address, with any additional template options
// applied.
func SendTemplatedEmail(ctx context.Context, to string, templateID string, options ...TemplateEmailOption) error {
	template := &templateEmail{
		EmailTemplate: &resend.EmailTemplate{
			Id:        templateID,
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
		Tags:     template.tags,
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
