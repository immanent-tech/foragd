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

package handlers

import (
	"log/slog"
	"net/http"

	"github.com/angelofallars/htmx-go"
	components "github.com/joshuar/go-templ-daisyui"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/server/handlers/renderers"
)

// Validate will read form input, validate the input and render the form input
// values with updated content, including values and any user-facing validation
// issues.
func Validate[T Validator](res http.ResponseWriter, req *http.Request, updater func(field string, item T, problems models.ValidationErrors) components.Input) {
	trigger, ok := htmx.GetTriggerName(req)
	if !ok {
		logging.FromContext(req.Context()).
			Warn("No trigger found but validation called?")
	}

	item, problems, err := decodeForm[T](req)
	if err != nil && len(problems) == 0 { // Internal validation error.
		logging.FromContext(req.Context()).
			Warn("Internal validation error.", slog.Any("error", err))
	}

	if err := renderers.FormInput(req, res, updater(trigger, item, problems)); err != nil {
		logging.FromContext(req.Context()).
			Warn("Unable to render form with validation results.", slog.Any("error", err))
	}
}
