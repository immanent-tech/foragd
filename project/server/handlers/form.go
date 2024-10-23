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
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-playground/form/v4"

	"github.com/joshuar/go-feed-me/model"
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
	Valid(ctx context.Context) (bool, model.ValidationErrors)
}

func decodeForm[T Validator](req *http.Request) (T, model.ValidationErrors, error) {
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

func decodeRequest[T Validator](req *http.Request) (T, model.ValidationErrors, error) {
	var obj T
	if err := json.NewDecoder(req.Body).Decode(&obj); err != nil {
		return obj, nil, fmt.Errorf("decode json: %w", err)
	}

	if ok, problems := obj.Valid(req.Context()); !ok {
		return obj, problems, fmt.Errorf("invalid %T: %d problems", obj, len(problems))
	}

	return obj, nil, nil
}
