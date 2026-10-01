/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package templates_test

import (
	"testing"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/templates"
)

func TestNewMarkAllAction(t *testing.T) {
	got := templates.NewMarkAction("/subscriptions", "Mark all", models.ViewUnread)
	if got.Path != "/subscriptions/"+string(models.MarkRead) || !got.Read {
		t.Fatalf("unexpected action: %+v", got)
	}
}
