// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/justinas/alice"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
	"github.com/joshuar/go-feed-me/validation"
	"github.com/joshuar/go-feed-me/web/views"
)

func (a *API) GetSettings() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		chain := alice.New(
			RouteLogger,
		)
		subscriptions, err := a.getSubscriptions(req.Context())
		if err != nil {
			chain.Then(RenderResponse(RespBackendError(err))).ServeHTTP(res, req)
			return
		}
		resp := models.NewResponse(
			models.WithResponseTemplate(views.NewSettingsPage(subscriptions).Template(req)),
		)
		chain.Then(RenderResponse(resp)).ServeHTTP(res, req)
	}
}

func (a *API) SetTheme() http.HandlerFunc {
	return alice.New(
		RouteLogger,
	).ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		theme := chi.URLParam(req, "theme")
		user, found := models.UserFromCtx(req.Context())
		if !found {
			RenderResponse(RespForbidden()).ServeHTTP(res, req)
			return
		}
		settings := user.GetSettings()
		settings.Theme = theme
		if err := a.updateUser(req.Context(), map[string]any{
			"settings":   settings,
			"updated_at": time.Now().UTC(),
		}); err != nil {
			RenderResponse(RespBackendError(err)).ServeHTTP(res, req)
			return
		}
		res.WriteHeader(http.StatusOK)
	}).ServeHTTP
}

// AddFavouriteSubscription handles adding a new favourite subscription for a user.
func (a *API) AddFavouriteSubscription() http.HandlerFunc {
	return alice.New(
		RouteLogger,
	).ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		id := chi.URLParam(req, "subscription")
		if valid, err := validation.ValidateVariable(id, "required,startswith=sub_"); !valid || err != nil {
			RenderResponse(RespInvalidInput(err)).ServeHTTP(res, req)
			return
		}
		user, found := models.UserFromCtx(req.Context())
		if !found {
			RenderResponse(RespForbidden()).ServeHTTP(res, req)
			return
		}
		// Create a new favourite subscription.
		fav, err := models.NewFavouriteSubscription(user.GetID(), id)
		if err != nil {
			RenderResponse(RespBackendError(err)).ServeHTTP(res, req)
			return
		}
		err = a.addFavourite(req.Context(), fav)
		if err != nil {
			RenderResponse(RespBackendError(err)).ServeHTTP(res, req)
			return
		}
		res.WriteHeader(http.StatusOK)
	}).ServeHTTP
}

// RemoveFavouriteSubscription handles removing a favourite subscription for a user.
func (a *API) RemoveFavouriteSubscription() http.HandlerFunc {
	return alice.New(
		RouteLogger,
	).ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		id := chi.URLParam(req, "subscription")
		if valid, err := validation.ValidateVariable(id, "required,startswith=sub_"); !valid || err != nil {
			RenderResponse(RespInvalidInput(err)).ServeHTTP(res, req)
			return
		}
		user, found := models.UserFromCtx(req.Context())
		if !found {
			RenderResponse(RespForbidden()).ServeHTTP(res, req)
			return
		}
		// Create the query to remove the favourite.
		query := query.Bool(
			query.Filter(
				query.Term("object_id", id),
				query.Term("user_id", user.GetID()),
				query.Term("type", models.FavouriteTypeSubscription),
			),
		)
		// Remove the favourite.
		err := a.removeFavourite(req.Context(), query)
		if err != nil {
			RenderResponse(RespBackendError(err)).ServeHTTP(res, req)
			return
		}
		res.WriteHeader(http.StatusOK)
	}).ServeHTTP
}

func (a *API) updateUser(ctx context.Context, updates map[string]any) error {
	// Retrieve user object.
	user, found := models.UserFromCtx(ctx)
	if !found {
		return models.ErrUserCtx
	}
	index := elastic.UserIndexFromCtx(ctx)

	if err := elastic.UpdateDoc(ctx, a.DataAPI().GetAPI(), index, user.GetID(), updates); err != nil {
		return fmt.Errorf("could not update user: %w", err)
	}
	return nil
}

func (a *API) addFavourite(ctx context.Context, fav *models.Favourite) error {
	index := elastic.FavouritesIndexFromCtx(ctx)
	if index == "" {
		return ErrNoCtxData
	}
	err := elastic.CreateDoc(ctx, a.DataAPI().GetAPI(), index, fav.GetID(), fav)
	if err != nil {
		return fmt.Errorf("could not add favourite: %w", err)
	}
	return nil
}

func (a *API) removeFavourite(ctx context.Context, query query.Option) error {
	index := elastic.FavouritesIndexFromCtx(ctx)
	if index == "" {
		return ErrNoCtxData
	}
	err := elastic.DeleteDocs(ctx, a.DataAPI().GetAPI(), index, query)
	if err != nil {
		return fmt.Errorf("could not remove favourite: %w", err)
	}
	return nil
}
