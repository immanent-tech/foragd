// Copyright (C) 2024 Joshua Rich <joshua.rich@gmail.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package middlewares

import (
	"net/http"

	"github.com/angelofallars/htmx-go"
	"github.com/rs/cors"
)

const (
	CORSMaxAge = 300 // Maximum value not ignored by any of major browsers.
)

func CORS(env string) func(next http.Handler) http.Handler {
	options := cors.Options{
		AllowCredentials:    true,
		MaxAge:              CORSMaxAge,
		AllowPrivateNetwork: true,
		OptionsPassthrough:  true,
	}

	if env == "development" {
		// options.Debug = true
		options.AllowedOrigins = []string{"*"}
	}

	options.AllowedHeaders = []string{
		"Accept", "Authorization", "Content-Type", "X-CSRF-Token",
		htmx.HeaderBoosted,
		htmx.HeaderCurrentURL,
		htmx.HeaderHistoryRestoreRequest,
		htmx.HeaderPrompt,
		htmx.HeaderRequest,
		htmx.HeaderTarget,
		htmx.HeaderTriggerName,
		htmx.HeaderTrigger,
	}

	options.ExposedHeaders = []string{
		"Link",
		htmx.HeaderLocation,
		htmx.HeaderPushURL,
		htmx.HeaderRedirect,
		htmx.HeaderRefresh,
		htmx.HeaderReplaceUrl,
		htmx.HeaderReswap,
		htmx.HeaderRetarget,
		htmx.HeaderReselect,
		htmx.HeaderTriggerAfterSettle,
		htmx.HeaderTriggerAfterSwap,
		htmx.HeaderTrigger,
	}

	corsH := cors.New(options)

	return corsH.Handler
}
