// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"fmt"

	"github.com/davecgh/go-spew/spew"

	"github.com/joshuar/go-feed-me/internal/id"
	"github.com/joshuar/go-feed-me/internal/validation"
)

func NewSubscriptionRequest() (SubscriptionID, *APISubscriptionRequest, error) {
	request := &APISubscriptionRequest{}

	subscriptionID, err := id.NewID(id.Subscription)
	if err != nil {
		return "", nil, fmt.Errorf("could not generate subscription id: %w", err)
	}

	return subscriptionID, request, nil
}

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
