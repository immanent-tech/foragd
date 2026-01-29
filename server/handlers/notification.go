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

type Notification struct {
	msg     *models.UserMessage
	timeout time.Duration
}

func (n *Notification) PartialResponse(res http.ResponseWriter, req *http.Request) {
	if n.timeout == 0 {
		n.timeout = templates.DefaultNotificationTimeout
	}
	templ.Handler(templates.Notification(n.msg, n.timeout)).ServeHTTP(res, req)
}
