// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package api

import (
	"errors"

	"github.com/joshuar/go-feed-me/internal/validation"
)

var ErrNewSubscriptionRequest = errors.New("could not create new subscription request")

func (r *APISubscriptionRequest) Valid() bool {
	valid, problems := validation.ValidateStruct(r)
	if valid {
		return true
	}

	if len(r.ValidationErrors) == 0 {
		r.ValidationErrors = make(map[string]string)
	}

	for field, problem := range problems {
		r.ValidationErrors[field] = problem
	}

	return false
}

func (r *APISubscriptionRequest) HasErrors() bool {
	return r.ValidationErrors != nil
}
