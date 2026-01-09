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
	ContentID = ID("content")
	// ErrorID points to an element that can be used to display error messages to the user.
	ErrorID = ID("error")
	// ModalContainerID points to an element that holds a modal.
	ModalContainerID = ID("modals")
	// ModalID points to an element that can be used to render a modal.
	ModalID = ID("modal")
	// NotificationsID points to an element that can be used for displaying notifications to the user.
	NotificationsID = ID("notifications")
)

// ID represents an id attribute in a HTML element.
type ID string

// Target returns the id attribute as a target (i.e., for htmx requests). This
// is the base id string with a "#" prefix.
func (a ID) Target() string {
	return "#" + string(a)
}

// String returns the id attribute as a string.
func (a ID) String() string {
	return string(a)
}

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
