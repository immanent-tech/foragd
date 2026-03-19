// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"fmt"

	"github.com/immanent-tech/foragd/validation"
)

func NewGroupSubscriptionRequest(suggestedCategories []Category) *GroupSubscriptionRequest {
	return &GroupSubscriptionRequest{
		Customisation:       &SubscriptionCustomisation{},
		Settings:            newSubscriptionSettings(),
		SuggestedCategories: suggestedCategories,
	}
}

func (r *GroupSubscriptionRequest) Valid() error {
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("group subscription error: %w", err)
	}
	return nil
}

func (r *GroupSubscriptionRequest) Sanitise() error {
	r.Customisation.Sanitise()
	return nil
}

func (r *GroupSubscriptionSuggestionRequest) Valid() error {
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("subscription suggestion request is invalid: %w", err)
	}
	return nil
}

func (r *GroupSubscriptionSuggestionRequest) Sanitise() error {
	r.Text = validation.SanitizeString(r.Text)
	return nil
}

func NewSearchSubscriptionRequest(
	search SearchRequest,
	suggestedCategories []Category,
) *SearchSubscriptionRequest {
	return &SearchSubscriptionRequest{
		Search:              search,
		Customisation:       &SubscriptionCustomisation{},
		SuggestedCategories: suggestedCategories,
	}
}

func (r *SearchSubscriptionRequest) Valid() error {
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("subscription validation error: %w", err)
	}
	return nil
}

func (r *SearchSubscriptionRequest) Sanitise() error {
	if err := r.Search.Sanitise(); err != nil {
		return err
	}
	if r.Customisation != nil {
		r.Customisation.Sanitise()
	}
	return nil
}
