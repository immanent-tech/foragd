// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"fmt"

	"github.com/immanent-tech/go-base/validation"
)

func (r *GetSubscriptionsSuggestionRequest) Valid() error {
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("subscription suggestion is invalid: %w", err)
	}
	return nil
}

func (r *GetSubscriptionsSuggestionRequest) Sanitise() error {
	r.Text = validation.SanitizeString(r.Text)
	return nil
}

func (r *AddSubscriptionToSearchRequest) Valid() error {
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("subscription suggestion is invalid: %w", err)
	}
	return nil
}

func (r *AddSubscriptionToSearchRequest) Sanitise() error {
	r.SelectedSubscription = validation.SanitizeString(r.SelectedSubscription)
	return nil
}

func (r *AddCategoryToSubscriptionRequest) Valid() error {
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("add subscription category request is invalid: %w", err)
	}
	return nil
}

func (r *AddCategoryToSubscriptionRequest) Sanitise() error {
	r.Category = validation.SanitizeString(r.Category)
	for idx := range r.ExistingCategories {
		r.ExistingCategories[idx] = validation.SanitizeString(r.ExistingCategories[idx])
	}
	return nil
}

func (r *FeedSubscriptionRequest) Valid() error {
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("add subscription category request is invalid: %w", err)
	}
	return nil
}

func (r *FeedSubscriptionRequest) Sanitise() error {
	if r.URL != "" {
		r.URL = validation.SanitizeString(r.URL)
	}
	if r.Customisation != nil {
		r.Customisation.Sanitise()
	}
	if r.ArticleFilters != nil {
		r.ArticleFilters.Sanitise()
	}
	return nil
}

func NewGroupSubscriptionRequest(
	suggestedSubscriptions Subscriptions,
	suggestedCategories []Category,
) *GroupSubscriptionRequest {
	return &GroupSubscriptionRequest{
		Customisation:          &SubscriptionCustomisation{},
		Settings:               newSubscriptionSettings(),
		SuggestedSubscriptions: suggestedSubscriptions,
		SuggestedCategories:    suggestedCategories,
	}
}

func (r *GroupSubscriptionRequest) Valid() error {
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("group subscription error: %w", err)
	}
	return nil
}

func (r *GroupSubscriptionRequest) Sanitise() error {
	if r.Customisation != nil {
		r.Customisation.Sanitise()
	}
	if r.ArticleFilters != nil {
		r.ArticleFilters.Sanitise()
	}
	return nil
}

func (r *AddSubscriptionToGroupRequest) Valid() error {
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("add subscription to group error: %w", err)
	}
	return nil
}

func (r *AddSubscriptionToGroupRequest) Sanitise() error {
	r.SuggestionText = validation.SanitizeString(r.SuggestionText)
	return nil
}

func (r *ContactRequest) Valid() error {
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("contact request invalid: %w", err)
	}
	return nil
}

func (r *ContactRequest) Sanitise() error {
	r.ContactEmail = validation.SanitizeString(r.ContactEmail)
	r.Details = validation.SanitizeString(r.Details)
	return nil
}

func (r *DeactivationRequest) Valid() error {
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("contact request invalid: %w", err)
	}
	return nil
}

func (r *DeactivationRequest) Sanitise() error {
	if r.Comments != nil {
		r.Comments = new(validation.SanitizeString(*r.Comments))
	}
	return nil
}

func (r *NextArticleRequest) Valid() error {
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("validate next article request: %w", err)
	}
	return nil
}

func (r *NextArticleRequest) Sanitise() error {
	r.Direction = NextArticleRequestDirection(validation.SanitizeString(string(r.Direction)))
	r.Timestamp = validation.SanitizeString(r.Timestamp)
	return nil
}
