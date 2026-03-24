// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package resend

import (
	"context"
	"fmt"
	"slices"

	"github.com/resend/resend-go/v3"
)

func UpdateTemplate(ctx context.Context, alias string, update *resend.UpdateTemplateRequest) error {
	client, err := loadClient()
	if err != nil {
		return fmt.Errorf("load client: %w", err)
	}

	id, err := getTemplateID(ctx, alias)
	if err != nil {
		return fmt.Errorf("get template id: %w", err)
	}

	if id == "" {
		newTemplate := resend.CreateTemplateRequest(*update)
		if _, err := client.Templates.CreateWithContext(ctx, &newTemplate); err != nil {
			return fmt.Errorf("create new template: %w", err)
		}
	} else {
		if _, err := client.Templates.UpdateWithContext(ctx, id, update); err != nil {
			return fmt.Errorf("update template: %w", err)
		}
	}

	if _, err = client.Templates.PublishWithContext(ctx, id); err != nil {
		return fmt.Errorf("publish template: %w", err)
	}

	return nil
}

func getTemplateID(ctx context.Context, alias string) (string, error) {
	client, err := loadClient()
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
