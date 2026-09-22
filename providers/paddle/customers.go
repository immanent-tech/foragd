// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package paddle

import (
	"context"
	"fmt"

	paddle "github.com/PaddleHQ/paddle-go-sdk/v5"

	"github.com/immanent-tech/foragd/models"
)

// addCustomerDetailsToUser fetches a user and adds customer details to their subscription data.
func addCustomerDetailsToUser(ctx context.Context, userSvc UserService, id string) (*models.User, error) {
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
	user, err := userSvc.GetUserByEmail(ctx, customer.Email)
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
	} else {
		user.Subscription = &models.User_Subscription{}
	}
	userSubscription.CustomerID = customer.ID
	if err := user.Subscription.FromPaddleSubscription(userSubscription); err != nil {
		return nil, fmt.Errorf("update user paddle subscription: %w", err)
	}
	if err := userSvc.UpdateUser(ctx, user, map[string]any{
		"subscription_type": models.UserSubscriptionTypePaddle,
		"subscription":      user.Subscription},
	); err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}

	return user, nil
}
