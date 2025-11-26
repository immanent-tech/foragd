// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"context"
	"net/http"

	"github.com/immanent-tech/foragd/web/templates"
)

func SetupImgProxy(key, salt string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			ctx := context.WithValue(req.Context(), templates.ImgProxyKey, key)
			ctx = context.WithValue(ctx, templates.ImgProxySalt, salt)
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}
