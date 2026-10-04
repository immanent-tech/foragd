/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package templates_test

import (
	"context"
	"testing"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/templates"
)

func TestNewListControls_Render(t *testing.T) {
	ctx := context.Background()
	// Test basic rendering
	c := templates.NewListControls("/test/path", models.ListFilters{}, nil)
	doc := render(t, ctx, c)

	// Verify basic structure exists
	if doc.Find("el-dialog").Length() == 0 {
		t.Error("Expected el-dialog for mobile controls, but not found")
	}
	if doc.Find("desktop-sort-filters").Length() == 0 {
		t.Error("Expected desktop-sort-filters, but not found")
	}

	// Verify specific labels
	if doc.Find("View").Length() == 0 {
		t.Error("Expected 'View' label in mobile filters")
	}
	if doc.Find("Sort").Length() == 0 {
		t.Error("Expected 'Sort' label in mobile filters")
	}
}
