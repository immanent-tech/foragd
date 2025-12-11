// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"net/http"
	"strings"
)

// Security middleware enhances security of requests.
func Security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// Do not allow embedding.
		//
		// https://cheatsheetseries.owasp.org/cheatsheets/HTTP_Headers_Cheat_Sheet.html#x-frame-options
		res.Header().Set("X-Frame-Options", "DENY")

		// Do not allow browsers to guess mime-types.
		//
		// https://cheatsheetseries.owasp.org/cheatsheets/HTTP_Headers_Cheat_Sheet.html#x-content-type-options
		res.Header().Set("X-Content-Type-Options", "nosniff")

		// Enforce referrer origin.
		//
		// https://cheatsheetseries.owasp.org/cheatsheets/HTTP_Headers_Cheat_Sheet.html#referrer-policy
		res.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Do not share browsing context.
		//
		// https://cheatsheetseries.owasp.org/cheatsheets/HTTP_Headers_Cheat_Sheet.html#cross-origin-opener-policy-coop
		res.Header().Set("Cross-Origin-Opener-Policy", "same-origin")

		if !strings.Contains(req.URL.Path, "view/article") {
			// Prevent loading of cross-origin resources not explicitly granted.
			//
			// https://cheatsheetseries.owasp.org/cheatsheets/HTTP_Headers_Cheat_Sheet.html#cross-origin-embedder-policy-coep
			res.Header().Set("Cross-Origin-Embedder-Policy", "require-corp")

			// Restrict resource loading to site and sub-domains.
			//
			// https://cheatsheetseries.owasp.org/cheatsheets/HTTP_Headers_Cheat_Sheet.html#cross-origin-resource-policy-corp
			res.Header().Set("Cross-Origin-Resource-Policy", "same-site")
		}

		next.ServeHTTP(res, req)
	})
}
