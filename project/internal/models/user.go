// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"fmt"
	"slices"
)

var ErrAddUser = errors.New("add subscription failed")

type UserPreferences map[string]any

func NewUserPreferences() UserPreferences {
	return map[string]any{
		"theme": "light",
	}
}

func (u *User) GetSubscribedFeedIDs() []FeedID {
	var feedIDs []FeedID

	for feedID := range u.Subscriptions {
		feedIDs = append(feedIDs, feedID)
	}

	return feedIDs
}

func (u *User) GetReadItemIDs(feedIDs ...FeedID) []ItemID {
	var readItemsIDs []ItemID

	for feedID, items := range u.ReadItems {
		if len(feedIDs) > 0 {
			if !slices.Contains(feedIDs, feedID) {
				continue
			}
		}

		for _, item := range items {
			readItemsIDs = append(readItemsIDs, item.ItemID)
		}
	}

	return readItemsIDs
}

func (u *User) DocumentID() *string {
	return &u.ID
}

func (u *User) DocumentType() DocumentType {
	return TypeUser
}

func (u *User) Valid(_ context.Context) (bool, ValidationErrors) {
	return validateStruct(u)
}

func (s *APINewUser) Valid(_ context.Context) (bool, ValidationErrors) {
	return validateStruct(s)
}

func (t *Tokens) UserID() string {
	return t.IDToken.Subject
}

func (t *Tokens) Nickname() string {
	return t.Claims.UserNickName
}

func (t *Tokens) Email() string {
	return t.Claims.UserName
}

func (t *Tokens) DecodeClaims() error {
	var claims Claims

	if err := t.IDToken.Claims(&claims); err != nil {
		return fmt.Errorf("cannot decode user claims from ID token: %w", err)
	}

	t.Claims = claims

	return nil
}
