// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"fmt"

	"github.com/immanent-tech/go-syndication/sanitization"

	"github.com/immanent-tech/foragd/validation"
)

func (r *GetSubscriptionsSuggestionRequest) Valid() error {
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("subscription suggestion is invalid: %w", err)
	}
	return nil
}

func (r *GetSubscriptionsSuggestionRequest) Sanitise() error {
	r.Text = sanitization.SanitizeString(r.Text)
	return nil
}

func (r *AddSubscriptionSuggestionRequest) Valid() error {
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("subscription suggestion is invalid: %w", err)
	}
	return nil
}

func (r *AddSubscriptionSuggestionRequest) Sanitise() error {
	r.SelectedSubscription = sanitization.SanitizeString(r.SelectedSubscription)
	return nil
}

func (r *AddSubscriptionCategoryRequest) Valid() error {
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("add subscription category request is invalid: %w", err)
	}
	return nil
}

func (r *AddSubscriptionCategoryRequest) Sanitise() error {
	r.Category = sanitization.SanitizeString(r.Category)
	for idx := range r.ExistingCategories {
		r.ExistingCategories[idx] = sanitization.SanitizeString(r.ExistingCategories[idx])
	}
	return nil
}

func (r *NewFeedSubscriptionRequest) Valid() error {
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("add subscription category request is invalid: %w", err)
	}
	return nil
}

func (r *NewFeedSubscriptionRequest) Sanitise() error {
	if r.Customisation != nil {
		r.Customisation.Sanitise()
	}
	return nil
}

func (s *EditFeedSubscriptionRequest) Valid() error {
	if err := validation.Validate.Struct(s); err != nil {
		return fmt.Errorf("edit feed subscription request is invalid: %w", err)
	}
	return nil
}

func (s *EditFeedSubscriptionRequest) Sanitise() error {
	if s.Customisation != nil {
		s.Customisation.Sanitise()
	}
	return nil
}
