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
	"github.com/joshuar/go-feed-me/web/views"
)

func GetSettings() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		resp := models.NewResponse(
			models.WithResponseTemplate(views.NewSettingsPage().Template(req)),
		)
		alice.New(
			RouteLogger,
		).Then(RenderTemplate(resp)).ServeHTTP(res, req)
	}
}

func (a *API) SetTheme() http.HandlerFunc {
	return alice.New(
		RouteLogger,
	).ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		theme := chi.URLParam(req, "theme")
		user, found := models.UserFromCtx(req.Context())
		if !found {
			RenderTemplate(RespForbidden()).ServeHTTP(res, req)
			return
		}
		settings := user.GetSettings()
		settings.Theme = theme
		if err := a.updateUser(req.Context(), map[string]any{
			"settings":   settings,
			"updated_at": time.Now().UTC(),
		}); err != nil {
			RenderTemplate(RespBackendError(err)).ServeHTTP(res, req)
			return
		}
		res.WriteHeader(http.StatusOK)
	}).ServeHTTP
}

// func (a *API) AddFavouriteSubscription() http.HandlerFunc {
// 	return alice.New(
// 		RouteLogger,
// 	).ThenFunc(func(res http.ResponseWriter, req *http.Request) {
// 		id := chi.URLParam(req, "subscription")
// 		if valid, err := validation.ValidateVariable(id, "required,startswith=sub_"); !valid || err != nil {
// 			RenderError(res, req, models.NewResponse(http.StatusBadRequest, err))
// 			return
// 		}
// 		fav, err := NewFavouriteSubscription(req.Context(), a.DataAPI(), id)
// 		if err != nil {
// 			RenderError(res, req, models.RespErrBackend(err))
// 			return
// 		}
// 		user, found := models.UserFromCtx(req.Context())
// 		if !found {
// 			RenderError(res, req, models.RespErrUnauthorized())
// 			return
// 		}
// 		favourites := user.GetFavourites()
// 		favourites.Add(fav)
// 		spew.Dump(favourites)
// 		if err := a.updateUser(req.Context(), map[string]any{
// 			"favourites": favourites,
// 			"updated_at": time.Now().UTC(),
// 		}); err != nil {
// 			RenderError(res, req, models.RespErrBackend(fmt.Errorf("failed to update favourites: %w", err)))
// 			return
// 		}
// 		res.WriteHeader(http.StatusOK)
// 	}).ServeHTTP
// }

// func NewFavouriteSubscription(ctx context.Context, api models.DocumentsAPI, id models.SubscriptionID) (*models.Favourite, error) {
// 	s, resp := models.GetSubscriptions(ctx, api, id)
// 	if resp != nil {
// 		return nil, resp.InternalError
// 	}
// 	if len(s) == 0 {
// 		return nil, errors.New("invalid subscription ID")
// 	}

// 	filters := models.NewArticleFilters()
// 	filters.Subscriptions = append(filters.Subscriptions, id)

// 	link := link.New("/articles",
// 		link.WithPushURL(),
// 		link.WithValues(filters.Parameters()),
// 	)

// 	data, err := json.Marshal(link)
// 	if err != nil {
// 		return nil, fmt.Errorf("could not marshal favourite data: %w", err)
// 	}

// 	return &models.Favourite{
// 		Name: s[0].GetTitle(),
// 		Type: models.FavouriteTypeSubscription,
// 		Link: data,
// 		ID:   id,
// 	}, nil
// }

// UpdateUser performs a partial update of the user object. On an error, a non-nil response is returned.
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
