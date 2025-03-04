// SPDX-License-Identifier: 	AGPL-3.0-or-later

package validation

import (
	"errors"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New(validator.WithRequiredStructEnabled())

var ErrValidationFailed = errors.New("internal validation error")

// Problems is a map of fields and their validation errors.
type Problems map[string]string

func (v Problems) GetErrors(field string) string {
	return v[field]
}

// IsValid will check if an object is valid according to the validation tags on
// the object. It does not return any details of validation issues, only a
// boolean for valid (true) or invalid (false).
func IsValid[T any](obj T) bool {
	err := validate.Struct(obj)
	return err != nil
}

//nolint:errorlint,errcheck
func ValidateStruct[T any](obj T) (bool, Problems) {
	validationErr := &validator.ValidationErrors{}

	err := validate.Struct(obj)
	if err != nil {
		if !errors.As(err, validationErr) {
			return false, map[string]string{"Internal": ErrValidationFailed.Error()}
		}

		problems := parseValidationErrors(err.(validator.ValidationErrors))

		return false, problems
	}

	return true, nil
}

func parseValidationErrors(validationErrors validator.ValidationErrors) Problems {
	problems := make(Problems)

	for _, err := range validationErrors {
		field := err.Field()

		switch err.Tag() {
		case "required":
			problems[field] = "This field is required"
		case "url":
			problems[field] = "Please enter a valid URL"
		case "email":
			problems[field] = "Please enter a valid email"
		default:
			problems[field] = "Please recheck the input and try again"
		}
	}

	return problems
}
