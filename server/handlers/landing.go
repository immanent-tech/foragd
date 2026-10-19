/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package handlers

import (
	"net/http"

	"github.com/immanent-tech/foragd/web/templates"
)

type Landing struct {
	*Page
}

func (m *Manager) HandleLanding() http.HandlerFunc {
	landing := &Landing{
		Page: m.NewPage("/",
			WithSummary("RSS and Atom Feed Reader"),
			WithDescription("View RSS, Atom and other syndicated content in your browser"),
			WithMetadataDescription("View RSS, Atom and other syndicated content in your browser"),
			WithTemplate(templates.Landing()),
		),
	}
	return landing.Handle()
}
