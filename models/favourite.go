// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"fmt"
	"slices"
	"time"
)

// NewFavoriteSubscription creates a new favorite subscription for the user.
func NewFavoriteSubscription(userID UserID, id SubscriptionID, customisation ObjectCustomisation) (*Favorite, error) {
	fav := newFavorite(userID, FavoriteTypeSubscription)
	fav.SetObjectID(id)
	err := fav.Data.FromFavoriteSubscription(customisation)
	if err != nil {
		return nil, fmt.Errorf("could not create Favorite subscription: %w", err)
	}
	return fav, nil
}

// NewFavoriteArticle creates a new favorite article for the user.
func NewFavoriteArticle(userID UserID, article *Article) (*Favorite, error) {
	fav := newFavorite(userID, FavoriteTypeArticle)
	fav.SetObjectID(article.GetID())
	err := fav.Data.FromFavoriteArticle(FavoriteArticle{
		Item:                      article.Item,
		SubscriptionCustomisation: article.SubscriptionCustomisation,
		SubscriptionID:            article.GetSubscriptionID(),
	})
	if err != nil {
		return nil, fmt.Errorf("could not create favorite article: %w", err)
	}
	return fav, nil
}

func (f *Favorite) SetObjectID(id string) {
	f.ObjectID = id
}

func (f *Favorite) GetObjectID() string {
	return f.ObjectID
}

func (f *Favorite) GetID() string {
	return f.FavoriteID
}

func (f *Favorite) String() string {
	switch f.Type {
	case FavoriteTypeSubscription:
		data, err := f.Data.AsFavoriteSubscription()
		if err != nil {
			return f.GetID()
		}
		return data.Title
	case FavoriteTypeArticle:
		data, err := f.Data.AsFavoriteArticle()
		if err != nil {
			return f.GetID()
		}
		return data.Item.GetTitle()
	default:
		return f.GetID()
	}
}

func newFavorite(userID UserID, favType FavoriteType) *Favorite {
	return &Favorite{
		CreatedAt:  time.Now().UTC(),
		FavoriteID: NewID(FavoritePFX),
		Type:       favType,
		UserID:     userID,
	}
}

type Favorites []*Favorite

func (f Favorites) GetByID(id string) *Favorite {
	idx := slices.IndexFunc(f, func(f *Favorite) bool {
		return f.GetObjectID() == id
	})
	if idx != -1 {
		return f[idx]
	}
	return nil
}

// FilterByType will return a new slice filtered to the given favorite type.
func (f Favorites) FilterByType(favoriteType FavoriteType) Favorites {
	return slices.Collect(FilterSlice(f, func(f *Favorite) bool {
		return f.Type == favoriteType
	}))
}
