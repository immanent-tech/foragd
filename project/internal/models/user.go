// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
)

var ErrAddUser = errors.New("add subscription failed")

type UserPreferences map[string]any

func NewUserPreferences() UserPreferences {
	return map[string]any{
		"theme": "light",
	}
}

func (u *User) IsSubscribed(id FeedID) bool {
	_, found := u.Subscriptions[id]
	return found
}

func (u *User) GetSubscribedFeedIDs() []FeedID {
	feedIDs := make([]FeedID, len(u.Subscriptions))
	idx := 0

	for feedID := range u.Subscriptions {
		feedIDs[idx] = feedID
		idx++
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

func (t *Tokens) UserID() string {
	id, _ := strings.CutPrefix(t.IDToken.Subject, "auth0|")
	return id
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
