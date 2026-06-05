// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"
	"time"

	"github.com/a-h/templ"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/templates"
)

// Notification contains a message that will be displayed to the user as a notification.
type Notification struct {
	msg     *models.UserMessage
	timeout time.Duration
}

// PartialResponse renders the notification into the notification container on the page as an OOB response.
func (n *Notification) PartialResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(templates.Notification(n.msg, templates.WithNotificationTimeout(n.timeout))).ServeHTTP(res, req)
}

type Modal struct {
	template templ.Component
}

func (m *Modal) PartialResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(m.template).ServeHTTP(res, req)
}
