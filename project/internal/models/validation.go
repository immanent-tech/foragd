// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"errors"

	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

var ErrValidationFailed = errors.New("internal validation error")

// ValidationErrors is a map of fields and their validation errors.
type ValidationErrors map[string]string

func (v ValidationErrors) GetErrors(field string) string {
	return v[field]
}

func init() {
	validate = validator.New(validator.WithRequiredStructEnabled())
}

func validateStruct[T any](obj T) (bool, ValidationErrors) {
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

func parseValidationErrors(validationErrors validator.ValidationErrors) ValidationErrors {
	problems := make(ValidationErrors)

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
