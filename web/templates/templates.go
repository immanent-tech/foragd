// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package templates

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"

	"github.com/immanent-tech/foragd/models"
)

const (
	FragmentContent FragmentKey = "content"

	ImgProxyKey  contextKey = "imgproxy_key"
	ImgProxySalt contextKey = "imgproxy_salt"
)

type FragmentKey string

type contextKey string

type partialsCtxKey string

var (
	// ContentID points to the element containing the main content of the page.
	ContentID = models.ElementID("content")
	// ErrorID points to an element that can be used to display error messages to the user.
	ErrorID = models.ElementID("error")
	// NotificationsID points to an element that can be used for displaying notifications to the user.
	NotificationsID = models.ElementID("notifications")
)

type Template struct {
	IsHTMX               bool
	IsHTMXHistoryRestore bool
}

// NewTemplate creates a new Template object from an HTTP Request and templ component.
func NewTemplate(req *http.Request) Template {
	return Template{
		IsHTMX:               htmx.IsHTMX(req),
		IsHTMXHistoryRestore: htmx.IsHistoryRestoreRequest(req),
	}
}

type Component struct {
	Data templ.Component
}

func (c *Component) Render(ctx context.Context, w io.Writer) error {
	if c.Data != nil {
		if err := c.Data.Render(ctx, w); err != nil {
			return fmt.Errorf("render template data component: %w", err)
		}
	}
	return nil
}
