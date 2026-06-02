// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package resend

import (
	"context"
	"fmt"
	"net/mail"
	"slices"
	"sync"

	"github.com/resend/resend-go/v3"

	"github.com/immanent-tech/foragd/config"
)

type Template struct {
	*resend.UpdateTemplateRequest

	mu    sync.Mutex
	alias string
}

func (t *Template) SetSubject(subject string) {
	t.Subject = subject
}

func (t *Template) SetReplyTo(replyTo any) {
	switch v := replyTo.(type) {
	case string:
		t.ReplyTo = v
	case *mail.Address:
		t.ReplyTo = v.String()
	}
}

func (t *Template) SetFrom(from any) {
	switch v := from.(type) {
	case string:
		t.From = v
	case *mail.Address:
		t.From = v.String()
	}
}

// TemplateOption is a functional option applied to a template.
type TemplateOption func(*Template)

// WithTemplateHTML option sets the HTML format of the template.
func WithTemplateHTML(data []byte) TemplateOption {
	return func(t *Template) {
		t.Html = string(data)
	}
}

// WithTemplateText option sets the text format of the template.
func WithTemplateText(data []byte) TemplateOption {
	return func(t *Template) {
		t.Text = string(data)
	}
}

// WithTemplateName option sets the name of the template.
func WithTemplateName(name string) TemplateOption {
	return func(t *Template) {
		t.Name = name
	}
}

// WithTemplateVariable option sets a variable for use within the template.
func WithTemplateVariable(key, dataType string, fallback any) TemplateOption {
	return func(t *Template) {
		t.mu.Lock()
		defer t.mu.Unlock()
		t.Variables = append(t.Variables, &resend.TemplateVariable{
			Key:           key,
			Type:          resend.VariableType(dataType),
			FallbackValue: fallback,
		})
	}
}

// UpdateTemplate will update the template with the given alias. If a template with the alias does not exist, it will be
// created.
func UpdateTemplate(ctx context.Context, alias string, options ...TemplateOption) error {
	client, err := LoadClient()
	if err != nil {
		return fmt.Errorf("load client: %w", err)
	}

	template := &Template{
		alias: alias,
		UpdateTemplateRequest: &resend.UpdateTemplateRequest{
			Variables: make([]*resend.TemplateVariable, 0),
		},
	}

	// Generate an email address to use as default from/reply-to.
	from := &mail.Address{Name: config.AppName, Address: cfg.ReplyToEmail}
	WithFrom[*Template](from)(template)
	WithReplyTo[*Template](from)(template)

	// Apply any passed in options.
	for option := range slices.Values(options) {
		option(template)
	}

	id, err := templateAliasToID(ctx, alias)
	if err != nil {
		return fmt.Errorf("get template id: %w", err)
	}

	if id == "" {
		newTemplate := resend.CreateTemplateRequest(*template.UpdateTemplateRequest)
		if _, err := client.Templates.CreateWithContext(ctx, &newTemplate); err != nil {
			return fmt.Errorf("create new template: %w", err)
		}
	} else {
		if _, err := client.Templates.UpdateWithContext(ctx, id, template.UpdateTemplateRequest); err != nil {
			return fmt.Errorf("update template: %w", err)
		}
	}

	if _, err = client.Templates.PublishWithContext(ctx, id); err != nil {
		return fmt.Errorf("publish template: %w", err)
	}

	return nil
}

// templateAliasToID will return the ID of the email template with the given alias. If no template exists, it will
// return an empty string. If an error occurs, a non-nil error will also be returned.
func templateAliasToID(ctx context.Context, alias string) (string, error) {
	client, err := LoadClient()
	if err != nil {
		return "", fmt.Errorf("load client: %w", err)
	}

	templates, err := client.Templates.ListWithContext(ctx, &resend.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("get template: %w", err)
	}

	for template := range slices.Values(templates.Data) {
		if template.Alias == alias {
			return template.Id, nil
		}
	}

	return "", nil
}
