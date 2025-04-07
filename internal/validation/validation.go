// SPDX-License-Identifier: 	AGPL-3.0-or-later

package validation

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/go-playground/validator/v10"
)

var validate = validator.New(validator.WithRequiredStructEnabled())

// Error is a map of fields and their validation errors.
type Error struct {
	Details string
	fields  map[string]string
}

func (e *Error) Error() string {
	return fmt.Sprintf("invalid data: %s", e.Details)
}

// FieldError returns the error for the field with the given name. If the field
// has no error, a nil value is returned.
func (e *Error) FieldError(name string) error {
	if field, ok := e.fields[name]; ok {
		return fmt.Errorf("%s: %s", field, e.fields[name])
	}
	return nil
}

// IsValid will check if an object is valid according to the validation tags on
// the object. It does not return any details of validation issues, only a
// boolean for valid (true) or invalid (false).
func IsValid[T any](obj T) bool {
	err := validate.Struct(obj)
	return err != nil
}

func AddStructValidationFunc(fn validator.StructLevelFunc, types ...any) {
	validate.RegisterStructValidation(fn, types...)
}

// ValidateStruct will validate a struct using the validate tags assigned on the
// struct fields. It returns a boolean representing whether the struct is valid.
// If the struct is not valid, the second return value will be a non-nil map of
// struct field names and an array of validation errors for that field.
//
//nolint:errorlint,errcheck
func ValidateStruct[T any](obj T) (bool, error) {
	validationErr := &validator.ValidationErrors{}

	err := validate.Struct(obj)
	if err != nil {
		if !errors.As(err, validationErr) {
			return false, &Error{Details: "internal validation error"}
		}

		problems := parseStructValidationErrors(err.(validator.ValidationErrors))
		spew.Dump(problems)

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
func parseStructValidationErrors(validationErrors validator.ValidationErrors) *Error {
	fields := make(map[string]string)
	// Generate details of fields that failed validation.
	var details strings.Builder
	for err := range slices.Values(validationErrors) {
		details.WriteString(err.Field() + " " + err.Error())
		details.WriteRune('\n')
		fields[err.Field()] = err.Error()
	}
	return &Error{
		Details: details.String(),
		fields:  fields,
	}
}
