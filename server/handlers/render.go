// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"

	"github.com/angelofallars/htmx-go"
)

type PartialResponseHandler interface {
	PartialResponse(w http.ResponseWriter, r *http.Request)
}

type FullResponseHandler interface {
	PartialResponseHandler
	FullResponse(w http.ResponseWriter, r *http.Request)
}

func RenderPage(content FullResponseHandler) http.HandlerFunc {
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
