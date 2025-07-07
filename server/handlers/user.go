// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/angelofallars/htmx-go"
	"github.com/davecgh/go-spew/spew"
	"github.com/go-chi/chi/v5"
	"github.com/justinas/alice"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/validation"
	"github.com/joshuar/go-feed-me/web/link"
	"github.com/joshuar/go-feed-me/web/views"
)

func GetSettings() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Set layout.
		page := views.SettingsPage{}
		ctx := templateToCtx(req.Context(), page.Show())

		// Set up handler chain.
		chain := alice.New(
			RouteLogger,
		)
		// Display content based on request.
		switch {
		case htmx.IsHTMX(req) && !htmx.IsHistoryRestoreRequest(req):
			// Partial update. Only render fragments.
			chain.Then(RenderTemplateFragments("content")).ServeHTTP(res, req.WithContext(ctx))
		default:
			// Full page render.
			chain.Then(RenderTemplate()).ServeHTTP(res, req.WithContext(ctx))
		}
	}
}

func (a *API) SetTheme() http.HandlerFunc {
	return alice.New(
		RouteLogger,
	).ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		theme := chi.URLParam(req, "theme")
		user, found := models.UserFromCtx(req.Context())
		if !found {
			ProcessResponse(res, req, models.RespErrUnauthorized())
			return
		}
		settings := user.GetSettings()
		settings.Theme = theme
		if err := a.DataAPI().UpdateUser(req.Context(), map[string]any{
			"settings":   settings,
			"updated_at": time.Now().UTC(),
		}); err != nil {
			RenderError(res, req, models.RespErrBackend(fmt.Errorf("failed to update theme: %w", err)))
			return
		}
		res.WriteHeader(http.StatusOK)
	}).ServeHTTP
}

func (a *API) AddFavouriteSubscription() http.HandlerFunc {
	return alice.New(
		RouteLogger,
	).ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		id := chi.URLParam(req, "subscription")
		if valid, err := validation.ValidateVariable(id, "required,startswith=sub_"); !valid || err != nil {
			RenderError(res, req, models.NewResponse(http.StatusBadRequest, err))
			return
		}
		fav, err := NewFavouriteSubscription(req.Context(), a.DataAPI(), id)
		if err != nil {
			RenderError(res, req, models.RespErrBackend(err))
			return
		}
		user, found := models.UserFromCtx(req.Context())
		if !found {
			RenderError(res, req, models.RespErrUnauthorized())
			return
		}
		favourites := user.GetFavourites()
		favourites.Add(fav)
		spew.Dump(favourites)
		if err := a.DataAPI().UpdateUser(req.Context(), map[string]any{
			"favourites": favourites,
			"updated_at": time.Now().UTC(),
		}); err != nil {
			RenderError(res, req, models.RespErrBackend(fmt.Errorf("failed to update favourites: %w", err)))
			return
		}
		res.WriteHeader(http.StatusOK)
	}).ServeHTTP
}

func NewFavouriteSubscription(ctx context.Context, api models.DocumentsAPI, id models.SubscriptionID) (*models.Favourite, error) {
	s, resp := models.GetSubscriptions(ctx, api, id)
	if resp != nil {
		return nil, resp.InternalError
	}
	if len(s) == 0 {
		return nil, errors.New("invalid subscription ID")
	}

	filters := models.NewArticleFilters()
	filters.Subscriptions = append(filters.Subscriptions, id)

	link := link.New("/articles",
		link.WithPushURL(),
		link.WithValues(filters.Parameters()),
	)

	data, err := json.Marshal(link)
	if err != nil {
		return nil, fmt.Errorf("could not marshal favourite data: %w", err)
	}

	return &models.Favourite{
		Name: s[0].GetTitle(),
		Type: models.FavouriteTypeSubscription,
		Link: data,
		ID:   id,
	}, nil
}
