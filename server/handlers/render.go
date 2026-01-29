// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//nolint:iface // duplication is more for readability than simplicity.
package handlers

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"

	"github.com/immanent-tech/foragd/web/templates"
)

// PartialResponseHandler is a handler that handles partial responses.
type PartialResponseHandler interface {
	PartialResponse(w http.ResponseWriter, r *http.Request)
}

// FullResponseHandler is a handler that handles full responses.
type FullResponseHandler interface {
	FullResponse(w http.ResponseWriter, r *http.Request)
}

// InternalPage is a handler for internal pages. Internal pages support rendering either full or partial responses.
type InternalPage interface {
	PartialResponseHandler
	FullResponseHandler
}

// RenderInternalPage is a handler that chooses the appropriate rendering for an internal page (full or partial), based
// on the request.
func RenderInternalPage(content InternalPage) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		if content == nil {
			// If there is no response, return 204: No Content.
			res.WriteHeader(http.StatusNoContent)
			return
		}
		switch {
		case !htmx.IsHTMX(req) || htmx.IsHistoryRestoreRequest(req): // Non-HTMX or HistoryRestoreRequests render a full-page.
			if htmx.IsHistoryRestoreRequest(req) {
				res.Header().Set("Cache-Control", "private, max-age=0, must-revalidate")
			}
			content.FullResponse(res, req)
		default: // HTMX request renders partial content.
			content.PartialResponse(res, req)
		}
	}
}

// ExternalPage is a handler for external pages. External pages only support rendering full responses.
type ExternalPage interface {
	FullResponseHandler
}

// RenderExternalPage is a handler that renders a full external page.
func RenderExternalPage(content ExternalPage) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		if content == nil {
			// If there is no response, return 204: No Content.
			res.WriteHeader(http.StatusNoContent)
			return
		}
		content.FullResponse(res, req)
	}
}

// RenderPartial is a handler that renders a partial response.
func RenderPartial(content PartialResponseHandler) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		if content == nil {
			// If there is no response, return 204: No Content.
			res.WriteHeader(http.StatusNoContent)
			return
		}
		if !htmx.IsHTMX(req) {
			// If the request is not a HTMX request, return 406: Not Acceptable.
			res.WriteHeader(http.StatusNotAcceptable)
			return
		}

		content.PartialResponse(res, req)
	}
}

// PartialTemplate is a template that only supports being rendered in a partial response.
type PartialTemplate struct {
	template templ.Component
}

// PartialResponse renders the template.
func (t *PartialTemplate) PartialResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(t.template).ServeHTTP(res, req)
}

// FullInternalTemplate is a template for an internal page that supports being rendered as a full page or partial.
type FullInternalTemplate struct {
	title      string
	template   templ.Component
	partialKey templates.FragmentKey
}

// FullResponse renders a full page (headers, footers and content).
func (t *FullInternalTemplate) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(
		templates.CreatePage(t.template,
			templates.WithPageTitle(t.title),
		)).ServeHTTP(res, req)
}

// PartialResponse renders just the content and performs OOB swaps to update the title (if set) and sidebar/dock.
func (t *FullInternalTemplate) PartialResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(t.template, templ.WithFragments(t.partialKey)).ServeHTTP(res, req)
	templ.Handler(templates.SideBar(templ.Attributes{"hx-swap-oob": "true"})).ServeHTTP(res, req)
	templ.Handler(templates.Dock(templ.Attributes{"hx-swap-oob": "true"})).ServeHTTP(res, req)
	if t.title != "" {
		templ.Handler(templates.UpdateTitle(t.title)).ServeHTTP(res, req)
	}
}
