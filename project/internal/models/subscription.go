// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"errors"
	"time"

	"github.com/davecgh/go-spew/spew"

	"github.com/joshuar/go-feed-me/internal/validation"
)

var ErrNewSubscriptionRequest = errors.New("could not create new subscription request")

func (s *APISubscriptionRequest) Valid() bool {
	valid, problems := validation.ValidateStruct(s)
	if valid {
		return true
	}

	spew.Dump(s)

	if len(s.ValidationErrors) == 0 {
		s.ValidationErrors = make(map[string]string)
	}

	for field, problem := range problems {
		s.ValidationErrors[field] = problem
	}

	return false
}

func (s *APISubscriptionRequest) HasErrors() bool {
	return s.ValidationErrors != nil
}

func NewSubscriptionState(details *APISubscriptionRequest) SubscriptionState {
	return SubscriptionState{
		CreatedAt:  time.Now().UTC(),
		Name:       details.Name,
		Categories: details.Categories,
	}
}
