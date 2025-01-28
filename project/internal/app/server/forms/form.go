// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package forms

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-playground/form/v4"
)

var decoder = form.NewDecoder()

// Validator is an object that can be validated.
type Validator interface {
	Valid() bool
}

// DecodeForm will decode submitted form contents into the passed in type. It
// will perform validation of the type and will return the type and a boolean
// true if it is valid. If decoding the form submission fails, a non-nill error
// is returned.
func DecodeForm[T Validator](req *http.Request) (T, bool, error) {
	var obj T
	// Parse form values in request.
	if err := req.ParseForm(); err != nil {
		return obj, false, fmt.Errorf("parse form: %w", err)
	}

	// Decode into appropriate object.
	err := decoder.Decode(&obj, req.Form)
	if err != nil {
		return obj, false, fmt.Errorf("decode form: %w", err)
	}

	// Validate the object.
	if ok := obj.Valid(); !ok {
		return obj, false, fmt.Errorf("invalid %T", obj)
	}

	return obj, true, nil
}

func DecodeRequest[T any](req *http.Request) (T, error) {
	var obj T
	if err := json.NewDecoder(req.Body).Decode(&obj); err != nil {
		return obj, fmt.Errorf("decode request: %w", err)
	}

	// if ok, problems := obj.Valid(); !ok {
	// 	return obj, problems, fmt.Errorf("invalid %T: %d problems", obj, len(problems))
	// }

	return obj, nil
}

// func updateFormInput(req *http.Request, res http.ResponseWriter, input components.Input) error {
// 	if err := htmx.NewResponse().RenderTempl(req.Context(), res, input.Show()); err != nil {
// 		return fmt.Errorf("failed to render input: %w", err)
// 	}

// 	return nil
// }
