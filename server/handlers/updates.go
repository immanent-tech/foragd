/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package handlers

import (
	"net/http"

	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/element"
	"github.com/immanent-tech/foragd/web/templates/partials"
)

// HandleUpdatesToast handles rendering a small update toast on the page to signify there are updates.
func (m *Manager) HandleUpdatesToast(res http.ResponseWriter, req *http.Request, path string, values any) {
	toast := partials.UpdatesToast(
		element.WithHXMethod(http.MethodGet, path),
		element.WithHXTarget(templates.ContentID.Target()),
		element.WithHXSwap("innerMorph scroll:top transition:true"),
		element.WithHXValues(values),
	)
	m.NewPartial(toast).Handle().ServeHTTP(res, req)
}
