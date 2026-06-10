// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package paddle

import (
	"context"
	"fmt"

	paddle "github.com/PaddleHQ/paddle-go-sdk/v5"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/service"
)

// GetUserByCustomerID retrieves the user associated with the given customer ID. It handles finding and adding customer
// details to an existing user for a new customer.
func GetUserByCustomerID(ctx context.Context, id string) (*models.User, error) {
	// Retrieve the user associated with the customer ID.
	switch resp, err := elastic.Search[*models.User](
		ctx,
		schema.UsersIndexRO(),
		query.Term("subscription.customer_id", id),
		elastic.WithDocSorting(),
		elastic.WithTrackTotalHits(false),
		elastic.WithSize(1),
	); {
	case err != nil:
		return nil, fmt.Errorf("find user by customer id: %w", err)
	case len(resp.Results) == 0:
		user, err := addCustomerDetailsToUser(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("add customer details: %w", err)
		}
		return user, nil
	default:
		return resp.Results[0], nil
	}
}

// addCustomerDetailsToUser fetches a user and adds customer details to their subscription data.
func addCustomerDetailsToUser(ctx context.Context, id string) (*models.User, error) {
	if err := loadClient(); err != nil {
		return nil, fmt.Errorf("load client: %w", err)
	}

	// Fetch customer details.
	customer, err := client.GetCustomer(ctx, &paddle.GetCustomerRequest{
		CustomerID: id,
	})
	if err != nil {
		return nil, fmt.Errorf("get customer: %w", err)
	}

	// Find existing user by email
	user, err := service.GetUserByEmail(ctx, customer.Email)
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}

	// Update the user's subscription.
	var userSubscription models.PaddleSubscription
	if user.HasValidSubscription() {
		userSubscription, err = user.Subscription.AsPaddleSubscription()
		if err != nil {
			return nil, fmt.Errorf("get user paddle subscription: %w", err)
		}
	}
	userSubscription.CustomerID = customer.ID
	if err := user.Subscription.FromPaddleSubscription(userSubscription); err != nil {
		return nil, fmt.Errorf("update user paddle subscription: %w", err)
	}
	if err := service.UpdateUser(ctx, user, map[string]any{
		"subscription_type": models.UserSubscriptionTypePaddle,
		"subscription":      user.Subscription},
	); err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}

	return user, nil
}
