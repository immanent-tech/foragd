// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"log/slog"
	"maps"
	"slices"

	slogctx "github.com/veqryn/slog-context"
)

var ErrNoSubscriptionCustomisation = errors.New("no subscription customisation found")

func GetSubscriptionCustomisation(ctx context.Context, api SubscriptionsAPI, id SubscriptionID) (*SubscriptionCustomisation, *Response) {
	// Retrieve user object.
	user, found := UserFromCtx(ctx)
	if !found {
		return nil, RespErrUnauthorized()
	}
	// Make sure the user has a subscription with the given id.
	if !user.HasSubscription(id) {
		return nil, RespErrUnauthorized()
	}
	// Get customisation details.
	customisations, err := api.GetSubscriptionCustomisations(ctx, id)
	if err != nil {
		return nil, RespErrBackend(err)
	}
	// Return the customisation for the given id.
	for customisation := range slices.Values(customisations) {
		if customisation.GetID() == id {
			return customisation, nil
		}
	}
	// If no customisation, return a new customisation object.
	state := user.GetSubscriptionState(id)
	return &SubscriptionCustomisation{
		FeedID:         state.GetFeedID(),
		SubscriptionID: state.GetID(),
		UserID:         user.GetID(),
	}, nil
}

func Unsubscribe(ctx context.Context, api DocumentsAPI, ids ...SubscriptionID) *Response {
	if len(ids) == 0 {
		return nil
	}
	user, found := UserFromCtx(ctx)
	if !found {
		return RespErrUnauthorized()
	}

	// Remove any subscription customisations. This is non-critical.
	if err := api.DeleteSubscriptionCustomisations(ctx, ids...); err != nil {
		slogctx.FromCtx(ctx).Warn("Unable to delete user subscription customisations.",
			slog.Any("error", err),
		)
	}
	// Remove states for given subscriptions from user.
	states := user.GetAllSubscriptionStates()
	for id := range states {
		if slices.Contains(ids, id) {
			delete(states, id)
		}
	}
	// Update the user.
	return api.UpdateUser(ctx, map[string]any{
		"subscriptions": slices.Collect(maps.Values(states)),
	})
}

// func UpdateSubscriptionCustomisation(ctx context.Context, api DocumentsAPI, edits *SubscriptionEdit) error {
// 	// Retrieve user object.
// 	user, found := UserFromCtx(ctx)
// 	if !found {
// 		return models.ErrInvalidID
// 	}
// 	index := elastic.SubscriptionsIndexFromCtx(ctx)
// 	if index == "" {
// 		return ErrFetchCtx
// 	}

// 	found, err := NewDocExistsRequest(e.GetAPI(), index, edits.SubscriptionID).Do(ctx)
// 	if err != nil {
// 		return fmt.Errorf("failed to update subscription: %w", err)
// 	}
// 	if !found {
// 		state := user.GetSubscriptionState(edits.SubscriptionID)
// 		customisation := &models.SubscriptionCustomisation{
// 			SubscriptionID: edits.SubscriptionID,
// 			FeedID:         state.GetFeedID(),
// 			UserID:         user.GetID(),
// 			Title:          edits.Title,
// 			Categories:     edits.Categories,
// 		}
// 		_, err := NewDocCreateRequest(e.GetAPI(), index, edits.SubscriptionID, customisation, refresh.True).Do(ctx)
// 		if err != nil {
// 			return fmt.Errorf("failed to update subscription: %w", err)
// 		}
// 		return nil
// 	}

// 	updates := map[string]any{
// 		"title":      edits.Title,
// 		"categories": edits.Categories,
// 	}

// 	if err := UpdateDoc(ctx, e.GetAPI(), index, edits.SubscriptionID, updates); err != nil {
// 		return &models.Response{StatusCode: http.StatusInternalServerError, InternalError: err}
// 	}

// 	return nil
// }
