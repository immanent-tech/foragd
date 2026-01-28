// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"net/http"
)

// SetCacheControl sets an appropriate Cache-Control header for user content based on the user's update frequency
// setting.
func SetCacheControl(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// user, err := models.UserFromCtx(req.Context())
		// if err != nil {
		// 	return
		// }
		// updateFreq := strconv.FormatFloat(user.GetUpdatesFrequency().Seconds(), 'f', 0, 64)
		// res.Header().Set("Cache-Control", "private, max-age="+updateFreq+", must-revalidate")
		res.Header().Set("Cache-Control", "private, max-age=0, must-revalidate")
		next.ServeHTTP(res, req)
	})
}
