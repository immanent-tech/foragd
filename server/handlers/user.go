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
	"github.com/joshuar/go-feed-me/web/templates/layouts"
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

// GetFavorites handles getting the list of user favorites and displaying them in the side drawer.
func (a *API) GetFavorites() http.HandlerFunc {
	return alice.New(
		RouteLogger,
	).ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		user, found := models.UserFromCtx(req.Context())
		if !found {
			RenderResponse(RespForbidden()).ServeHTTP(res, req)
			return
		}
		// Retrieve favorites index.
		index := elastic.FavoritesIndexFromCtx(req.Context())
		if index == "" {
			RenderResponse(RespBackendError(ErrNoCtxData)).ServeHTTP(res, req)
			return
		}
		// Set up the query to retrieve favorite subscriptions for the user.
		query := query.Bool(
			query.Filter(
				query.Term("user_id", user.GetID()),
			),
		)
		// Run the query.
		var favorites models.Favorites
		var err error
		favorites, err = elastic.SearchAll[*models.Favorite](req.Context(), a.DataAPI().GetAPI(), index, query, 0)
		if err != nil {
			RenderResponse(RespBackendError(err)).ServeHTTP(res, req)
			return
		}
		// Render the favorites list.
		resp := models.NewResponse(
			models.WithResponseTemplate(layouts.FavoritesList(favorites)),
		)
		RenderResponse(resp).ServeHTTP(res, req)
	}).ServeHTTP
}

// AddFavoriteSubscription handles adding a new favorite subscription for a user.
func (a *API) AddFavoriteSubscription() http.HandlerFunc {
	return alice.New(
		RouteLogger,
		TriggerEvents("updateFavorites"),
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
		// Get the subscription state.
		states, err := a.getSubscriptionStates(req.Context(), id)
		if err != nil {
			RenderResponse(RespBackendError(err)).ServeHTTP(res, req)
			return
		}
		// Create a new favorite subscription.
		fav, err := models.NewFavoriteSubscription(user.GetID(), id, states[0].Customisation)
		if err != nil {
			RenderResponse(RespBackendError(err)).ServeHTTP(res, req)
			return
		}
		// Add the favorite subscription.
		index := elastic.FavoritesIndexFromCtx(req.Context())
		if index == "" {
			RenderResponse(RespBackendError(ErrNoCtxData)).ServeHTTP(res, req)
			return
		}
		err = elastic.CreateDoc(req.Context(), a.DataAPI().GetAPI(), index, fav.GetID(), fav)
		if err != nil {
			RenderResponse(RespBackendError(fmt.Errorf("could not add favorite subscription: %w", err))).ServeHTTP(res, req)
			return
		}
		res.WriteHeader(http.StatusOK)
	}).ServeHTTP
}

// RemoveFavoriteSubscription handles removing a Favorite subscription for a user.
func (a *API) RemoveFavoriteSubscription() http.HandlerFunc {
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
		// Create the query to remove the Favorite.
		query := query.Bool(
			query.Filter(
				query.Term("object_id", id),
				query.Term("user_id", user.GetID()),
				query.Term("type", models.FavoriteTypeSubscription),
			),
		)
		// Remove the favorite.
		index := elastic.FavoritesIndexFromCtx(req.Context())
		if index == "" {
			RenderResponse(RespBackendError(ErrNoCtxData)).ServeHTTP(res, req)
			return
		}
		err := elastic.DeleteDocs(req.Context(), a.DataAPI().GetAPI(), index, query)
		if err != nil {
			RenderResponse(RespBackendError(fmt.Errorf("could not remove Favorite: %w", err))).ServeHTTP(res, req)
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
