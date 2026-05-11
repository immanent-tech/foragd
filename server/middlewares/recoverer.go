// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/go-chi/chi/v5/middleware"

	gcp "github.com/immanent-tech/foragd/providers/google"
)

// Recoverer is a modified version of the standard chi Recoverer middleware that additional logs to the GCP error
// console.
func Recoverer(next http.Handler) http.Handler {
	fn := func(res http.ResponseWriter, req *http.Request) {
		defer func() {
			if rvr := recover(); rvr != nil {
				if rvr == http.ErrAbortHandler {
					// we don't recover http.ErrAbortHandler so the response
					// to the client is aborted, this should not be logged
					panic(rvr)
				}

				// Log to GCP error console.
				switch v := rvr.(type) {
				case error:
					gcp.ReportError(req.Context(), v)
				default:
					gcp.ReportError(req.Context(), fmt.Errorf("panic: %v", v))
				}

				logEntry := middleware.GetLogEntry(req)
				if logEntry != nil {
					logEntry.Panic(rvr, debug.Stack())
				} else {
					middleware.PrintPrettyStack(rvr)
				}

				if req.Header.Get("Connection") != "Upgrade" {
					res.WriteHeader(http.StatusInternalServerError)
				}
			}
		}()

		next.ServeHTTP(res, req)
	}

	return http.HandlerFunc(fn)
}
