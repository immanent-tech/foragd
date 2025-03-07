// SPDX-License-Identifier: 	AGPL-3.0-or-later

package validation

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New(validator.WithRequiredStructEnabled())

var ErrValidationFailed = errors.New("internal validation error")

// ValidationErrors is a map of fields and their validation errors.
//
//nolint:errname
//revive:disable:exported
type ValidationErrors map[string][]string

// Error allows ValidationErrors to be treated as an error.
func (p ValidationErrors) Error() string {
	var message strings.Builder
	for field, problems := range p {
		// Write the field name.
		message.WriteString(fmt.Sprintf("field %s: ", field))
		// Write each problem with the field.
		for _, problem := range problems {
			message.WriteString(problem)
		}

		message.WriteRune(' ')
	}

	return message.String()
}

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

// ValidateStruct will validate a struct using the validate tags assigned on the
// struct fields. It returns a boolean representing whether the struct is valid.
// If the struct is not valid, the second return value will be a non-nil map of
// struct field names and an array of validation errors for that field.
//
//nolint:errorlint,errcheck
func ValidateStruct[T any](obj T) (bool, ValidationErrors) {
	validationErr := &validator.ValidationErrors{}

	err := validate.Struct(obj)
	if err != nil {
		if !errors.As(err, validationErr) {
			return false, map[string][]string{
				"internal": {ErrValidationFailed.Error()},
			}
		}

		problems := parseStructValidationErrors(err.(validator.ValidationErrors))

		return false, problems
	}

	return true, nil
}

// ValidateVariable takes a single variable of any type and checks whether it is
// valid according to the given validation rule. It returns a boolean
// representing whether the struct is valid. If an error occurred with
// validation, a non-nil error will also be returned.
func ValidateVariable(variable any, rule string) (bool, error) {
	if err := validate.Var(variable, rule); err != nil {
		return false, fmt.Errorf("invalid: %w", err)
	}

	return true, nil
}

// parseStructValidationErrors takes the underlying validation errors and
// formats them so that each struct field has an array of all validation errors
// associated with it.
func parseStructValidationErrors(validationErrors validator.ValidationErrors) ValidationErrors {
	problems := make(ValidationErrors)

	for _, err := range validationErrors {
		field := err.Field()

		problems[field] = append(problems[field], fmt.Sprintf("%s: %s", err.Tag(), err.Error()))
	}

	return problems
}
