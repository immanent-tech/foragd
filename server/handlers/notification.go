/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package handlers

import (
	"net/http"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/templates/partials"
)

// HandleNotification handles rendering a notification to the page with the given [models.UserMessage] and an optional
// timeout.
func (m *Manager) HandleNotification(
	res http.ResponseWriter,
	req *http.Request,
	msg *models.UserMessage,
	options ...partials.NotificationOption,
) {
	m.NewPartial(partials.Notification(msg, options...)).Render().ServeHTTP(res, req)
}
