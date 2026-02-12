// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"fmt"
	"slices"

	"github.com/immanent-tech/foragd/validation"
)

func NewGroupSubscriptionRequest() *GroupSubscriptionRequest {
	return &GroupSubscriptionRequest{
		Customisation: &SubscriptionCustomisation{},
		Settings:      newSubscriptionSettings(),
	}
}

func (r *GroupSubscriptionRequest) Valid() error {
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("group subscription error: %w", err)
	}
	return nil
}

func (r *GroupSubscriptionRequest) Sanitise() error {
	if r.Customisation.Nickname != "" {
		r.Customisation.Nickname = validation.SanitizeString(r.Customisation.Nickname)
	}
	categories := make([]Category, 0, len(r.Customisation.Categories))
	for category := range slices.Values(r.Customisation.Categories) {
		category = validation.SanitizeString(category)
		categories = append(categories, category)
	}
	r.Customisation.Categories = slices.Compact(categories)
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

func (s *EditFeedSubscriptionRequest) Valid() error {
	if err := validation.Validate.Struct(s); err != nil {
		return fmt.Errorf("edit feed subscription request is invalid: %w", err)
	}
	return nil
}

func (s *EditFeedSubscriptionRequest) Sanitise() error {
	if s.Customisation != nil {
		s.Customisation.Nickname = validation.SanitizeString(s.Customisation.Nickname)
		categories := make([]Category, 0, len(s.Customisation.Categories))
		for category := range slices.Values(s.Customisation.Categories) {
			category = validation.SanitizeString(category)
			categories = append(categories, category)
		}
		s.Customisation.Categories = slices.Compact(categories)
	}
	if s.ArticleFilters != nil {
		if s.ArticleFilters.Authors != nil {
			cleanAuthorFilters := validation.SanitizeString(*s.ArticleFilters.Authors)
			s.ArticleFilters.Authors = &cleanAuthorFilters
		}
		if s.ArticleFilters.Categories != nil {
			cleanCategoryFilters := validation.SanitizeString(*s.ArticleFilters.Categories)
			s.ArticleFilters.Categories = &cleanCategoryFilters
		}
		if s.ArticleFilters.Text != nil {
			cleanTextFilters := validation.SanitizeString(*s.ArticleFilters.Text)
			s.ArticleFilters.Text = &cleanTextFilters
		}
	}
	return nil
}

func NewSearchSubscriptionRequest(search SearchRequest) *SearchSubscriptionRequest {
	return &SearchSubscriptionRequest{
		Search:        search,
		Customisation: &SubscriptionCustomisation{},
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
	if r.Customisation.Nickname != "" {
		r.Customisation.Nickname = validation.SanitizeString(r.Customisation.Nickname)
	}
	categories := make([]Category, 0, len(r.Customisation.Categories))
	for category := range slices.Values(r.Customisation.Categories) {
		category = validation.SanitizeString(category)
		categories = append(categories, category)
	}
	r.Customisation.Categories = slices.Compact(categories)
	return nil
}
