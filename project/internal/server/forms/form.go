// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package forms

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/angelofallars/htmx-go"
	"github.com/go-playground/form/v4"

	components "github.com/joshuar/go-templ-daisyui"

	"github.com/joshuar/go-feed-me/internal/models"
)

var decoder *form.Decoder

func init() {
	decoder = form.NewDecoder()
}

// Validator is an object that can be validated.
type Validator interface {
	// Valid checks the object and returns any
	// problems. If len(problems) == 0 then
	// the object is valid.
	Valid(ctx context.Context) (bool, models.ValidationErrors)
}

func DecodeForm[T Validator](req *http.Request) (T, models.ValidationErrors, error) {
	var obj T
	// Parse form values in request.
	if err := req.ParseForm(); err != nil {
		return obj, nil, fmt.Errorf("parse form: %w", err)
	}

	// Decode into appropriate object.
	err := decoder.Decode(&obj, req.Form)
	if err != nil {
		return obj, nil, fmt.Errorf("decode form: %w", err)
	}

	// Validate the object.
	if ok, problems := obj.Valid(req.Context()); !ok {
		return obj, problems, fmt.Errorf("invalid %T: %d problems", obj, len(problems))
	}

	return obj, nil, nil
}

func DecodeRequest[T Validator](req *http.Request) (T, models.ValidationErrors, error) {
	var obj T
	if err := json.NewDecoder(req.Body).Decode(&obj); err != nil {
		return obj, nil, fmt.Errorf("decode request: %w", err)
	}

	if ok, problems := obj.Valid(req.Context()); !ok {
		return obj, problems, fmt.Errorf("invalid %T: %d problems", obj, len(problems))
	}

	return obj, nil, nil
}

func updateFormInput(req *http.Request, res http.ResponseWriter, input components.Input) error {
	if err := htmx.NewResponse().RenderTempl(req.Context(), res, input.Show()); err != nil {
		return fmt.Errorf("failed to render input: %w", err)
	}

	return nil
}
