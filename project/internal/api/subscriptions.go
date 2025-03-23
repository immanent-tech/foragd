// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/validation"
)

// Valid returns a boolean indicating whether the SubscriptionRequest is valid,
// and any validation errors if applicable.
func (r *SubscriptionRequest) Valid() (bool, error) {
	if r.URL == "" {
		return false, errors.New("no URL specified")
	}
	return validation.ValidateStruct(r)
}

func (r *SubscriptionRequest) GetURL() string {
	return r.URL
}

// ToSubscription converts a SubscriptionRequest to a Subscription using the
// request details and the given feed and user IDs.
func (r *SubscriptionRequest) ToSubscription() (*models.Subscription, error) {
	if valid, err := r.Valid(); !valid {
		return nil, err
	}

	var subscription models.Subscription

	data, err := json.Marshal(r)
	if err != nil {
		return nil, fmt.Errorf("could not marshal request: %w", err)
	}

	err = json.Unmarshal(data, &subscription)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal subscription: %w", err)
	}

	subscription.CreatedAt = time.Now().UTC()

	return &subscription, nil
}

// SubscriptionRequests is a list of subscription requests.
type SubscriptionRequests []*SubscriptionRequest

// // Validate will inspect each SubscriptionRequest and validate it. If a request
// // is invalid, its Err field will be set to a non-nil validation.ValidationErrors.
// func (r SubscriptionRequests) Validate() {
// 	for request := range slices.Values(r) {
// 		if valid, problems := request.Valid(); !valid {
// 			request.Err = WrapError(problems, "api", "subscription is invalid")
// 		}
// 	}
// }

// FilterValid returns a slice of SubscriptionRequest containing only those
// requests that are valid (i.e., do not have an error).
func (r SubscriptionRequests) FilterValid() SubscriptionRequests {
	return slices.Collect(filterSlice(r, func(request *SubscriptionRequest) bool {
		return request.Err == nil
	}))
}

// FilterInValid returns a slice of SubscriptionRequest containing only those
// requests that are invalid (i.e., have an error).
func (r SubscriptionRequests) FilterInValid() SubscriptionRequests {
	return slices.Collect(filterSlice(r, func(request *SubscriptionRequest) bool {
		return request.Err != nil
	}))
}

// FilterValid returns a slice of SubscriptionRequest containing only those
// requests that are valid (i.e., do not have an error).
func (r SubscriptionRequests) FilterFeedNeeded() SubscriptionRequests {
	return slices.Collect(filterSlice(r, func(request *SubscriptionRequest) bool {
		return request.Feed != nil
	}))
}

// Feeds extracts and returns a list of Feeds from the requests.
func (r SubscriptionRequests) Feeds() []*models.Feed {
	feeds := make([]*models.Feed, 0, len(r))
	for request := range slices.Values(r) {
		if request.Feed != nil {
			feeds = append(feeds, request.Feed)
		}
	}
	return feeds
}

// URLs extracts and returns a list of URLs from the requests.
func (r SubscriptionRequests) URLs() []models.URL {
	urls := make([]models.URL, 0, len(r))
	for request := range slices.Values(r) {
		if url := request.GetURL(); url != "" {
			urls = append(urls, url)
		}
	}
	return urls
}
