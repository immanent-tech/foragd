// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"slices"
	"time"
)

// NewFavouriteSubscription creates a new favourite subscription for the user.
func NewFavouriteSubscription(userID UserID, id SubscriptionID) (*Favourite, error) {
	fav := newFavourite(userID, FavouriteTypeSubscription)
	fav.SetObjectID(id)
	return fav, nil
}

func (f *Favourite) SetObjectID(id string) {
	f.ObjectID = id
}

func (f *Favourite) GetObjectID() string {
	return f.ObjectID
}

func (f *Favourite) GetID() string {
	return f.FavouriteID
}

func newFavourite(userID UserID, favType FavouriteType) *Favourite {
	return &Favourite{
		CreatedAt:   time.Now().UTC(),
		FavouriteID: NewID(FavouritePFX),
		Type:        favType,
		UserID:      userID,
	}
}

type Favourites []*Favourite

func (f Favourites) GetByID(id string) *Favourite {
	idx := slices.IndexFunc(f, func(f *Favourite) bool {
		return f.GetObjectID() == id
	})
	if idx != -1 {
		return f[idx]
	}
	return nil
}
