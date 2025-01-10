// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package forms

import (
	"net/http"

	components "github.com/joshuar/go-templ-daisyui"

	"github.com/joshuar/go-feed-me/internal/models"
)

// Validator is an object that can be validated.
type Validator interface {
	// Valid checks the object and returns any
	// problems. If len(problems) == 0 then
	// the object is valid.
	Valid() (bool, models.ValidationErrors)
}

// Validate will read form input, validate the input and render the form input
// values with updated content, including values and any user-facing validation
// issues.
//
//nolint:lll
func Validate[T Validator](res http.ResponseWriter, req *http.Request, updater func(field string, item T, problems models.ValidationErrors) components.TextInputProps) {
	// trigger, ok := htmx.GetTriggerName(req)
	// if !ok {
	// 	logging.FromContext(req.Context()).
	// 		Warn("No trigger found but validation called?")
	// }

	// item, problems, err := DecodeForm[T](req)
	// if err != nil && len(problems) == 0 { // Internal validation error.
	// 	logging.FromContext(req.Context()).
	// 		Warn("Internal validation error.", slog.Any("error", err))
	// }

	// if err := updateFormInput(req, res, updater(trigger, item, problems)); err != nil {
	// 	logging.FromContext(req.Context()).
	// 		Warn("Unable to render form with validation results.", slog.Any("error", err))
	// }
}
