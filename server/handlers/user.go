// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"
	"github.com/justinas/alice"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic"
	"github.com/joshuar/go-feed-me/server/forms"
	"github.com/joshuar/go-feed-me/validation"
	"github.com/joshuar/go-feed-me/web/templates/layouts"
	"github.com/joshuar/go-feed-me/web/templates/partials"
	"github.com/joshuar/go-feed-me/web/templates/views"
)

// GetSettings handles retrieving and rendering the user settings page.
func (a *API) GetSettings() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		chain := alice.New(
			RouteLogger,
		)
		resp := models.NewResponse(
			models.WithResponseTemplate(views.NewSettingsPage().Template(req)),
		)
		chain.Then(RenderResponse(resp)).ServeHTTP(res, req)
	}
}

// SubscriptionsSettings shows a table of subscriptions, optionally filtered, with settings controls.
func (a *API) SubscriptionsSettings() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		chain := alice.New(
			RouteLogger,
		)
		// Extract the search request.
		request, valid, err := forms.DecodeForm[*models.SearchRequest](req)
		if err != nil || !valid {
			chain.Then(RenderResponse(RespInvalidInput(err))).ServeHTTP(res, req)
			return
		}
		// Find matching subscriptions.
		var subscriptions models.SubscriptionsSlice
		if request.Text != "" {
			subscriptions, err = a.findSubscriptions(req.Context(), request)
			if err != nil {
				chain.Then(RenderResponse(RespBackendError(err))).ServeHTTP(res, req)
				return
			}
		} else {
			subscriptions, err = a.getSubscriptions(req.Context())
			if err != nil {
				chain.Then(RenderResponse(RespBackendError(err))).ServeHTTP(res, req)
				return
			}
		}
		settings := make([]templ.Component, 0, len(subscriptions))
		for subscription := range slices.Values(subscriptions) {
			settings = append(settings, partials.NewSubscriptionContent(subscription).ShowAsSetting())
		}
		resp := models.NewResponse(
			models.WithResponseTemplate(templ.Join(settings...)),
		)
		RenderResponse(resp).ServeHTTP(res, req)
	}
}

// SetTheme handles setting a theme selected by the user.
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
		// Render the favorites list.
		resp := models.NewResponse(
			models.WithResponseTemplate(layouts.FavoritesList(user.GetFavorites())),
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
		metadata := user.GetSubscriptionMetadata().GetByID(id)
		// Create a new favorite subscription.
		err := user.AddFavoriteSubscription(id, metadata.Customisation.Title)
		if err != nil {
			RenderResponse(RespBackendError(err)).ServeHTTP(res, req)
			return
		}
		err = a.updateUser(req.Context(), map[string]any{
			"favorites": user.Favorites,
		})
		if err != nil {
			RenderResponse(RespBackendError(err)).ServeHTTP(res, req)
			return
		}
		// Update the favorite button.
		var template templ.Component
		currentURL, found := htmx.GetCurrentURL(req)
		if !found {
			RenderResponse(RespBackendError(nil)).ServeHTTP(res, req)
			return
		}
		switch {
		case strings.HasSuffix(currentURL, "/user/settings"):
			s, err := a.getSubscriptions(req.Context(), id)
			if err != nil || len(s) == 0 || len(s) > 1 {
				RenderResponse(RespBackendError(err)).ServeHTTP(res, req)
				return
			}
			template = partials.NewSubscriptionContent(s[0]).ShowAsSetting()
		default:
			template = partials.ToggleFavoriteSubscriptionText(id, true, "#favorite_"+id, "innerHTML")
		}

		resp := models.NewResponse(
			models.WithResponseTemplate(template),
		)
		RenderResponse(resp).ServeHTTP(res, req)
	}).ServeHTTP
}

// RemoveFavoriteSubscription handles removing a favorite subscription for a user.
func (a *API) RemoveFavoriteSubscription() http.HandlerFunc {
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
		user.RemoveFavorite(id)
		err := a.updateUser(req.Context(), map[string]any{
			"favorites": user.Favorites,
		})
		if err != nil {
			RenderResponse(RespBackendError(err)).ServeHTTP(res, req)
			return
		}
		// Update the favorite button.
		var template templ.Component
		currentURL, found := htmx.GetCurrentURL(req)
		if !found {
			RenderResponse(RespBackendError(nil)).ServeHTTP(res, req)
			return
		}
		switch {
		case strings.HasSuffix(currentURL, "/user/settings"):
			s, err := a.getSubscriptions(req.Context(), id)
			if err != nil || len(s) == 0 || len(s) > 1 {
				RenderResponse(RespBackendError(err)).ServeHTTP(res, req)
				return
			}
			template = partials.NewSubscriptionContent(s[0]).ShowAsSetting()
		default:
			template = partials.ToggleFavoriteSubscriptionText(id, false, "#favorite_"+id, "innerHTML")
		}

		resp := models.NewResponse(
			models.WithResponseTemplate(template),
		)
		RenderResponse(resp).ServeHTTP(res, req)
	}).ServeHTTP
}

// AddFavoriteArticle handles adding a new favorite article for a user.
func (a *API) AddFavoriteArticle() http.HandlerFunc {
	return alice.New(
		RouteLogger,
		TriggerEvents("updateFavorites"),
	).ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		id := chi.URLParam(req, "item")
		if valid, err := validation.ValidateVariable(id, "required,startswith=item_"); !valid || err != nil {
			RenderResponse(RespInvalidInput(err)).ServeHTTP(res, req)
			return
		}
		// Get the article details.
		articles, err := a.getArticles(req.Context(), id)
		if err != nil {
			RenderResponse(RespBackendError(err)).ServeHTTP(res, req)
			return
		}
		if len(articles) != 1 {
			RenderResponse(RespBackendError(ErrInvalidContent)).ServeHTTP(res, req)
			return
		}
		article := articles[0]
		user, found := models.UserFromCtx(req.Context())
		if !found {
			RenderResponse(RespForbidden()).ServeHTTP(res, req)
			return
		}
		// Create a new favorite article.
		err = user.AddFavoriteArticle(article.GetTitle(), article)
		if err != nil {
			RenderResponse(RespBackendError(err)).ServeHTTP(res, req)
			return
		}
		err = a.updateUser(req.Context(), map[string]any{
			"favorites": user.Favorites,
		})
		if err != nil {
			RenderResponse(RespBackendError(err)).ServeHTTP(res, req)
			return
		}
		// Archive the article.
		err = a.archiveArticle(req.Context(), article)
		if err != nil {
			RenderResponse(RespBackendError(err)).ServeHTTP(res, req)
			return
		}
		// Update the content
		template := partials.ToggleFavoriteArticleText(article.GetID(), true, "#favorite_"+article.GetID(), "innerHTML")
		resp := models.NewResponse(
			models.WithResponseTemplate(template),
		)
		RenderResponse(resp).ServeHTTP(res, req)
	}).ServeHTTP
}

// RemoveFavoriteArticle handles removing a favorite article for a user.
func (a *API) RemoveFavoriteArticle() http.HandlerFunc {
	return alice.New(
		RouteLogger,
		TriggerEvents("updateFavorites"),
	).ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		id := chi.URLParam(req, "item")
		if valid, err := validation.ValidateVariable(id, "required,startswith=item_"); !valid || err != nil {
			RenderResponse(RespInvalidInput(err)).ServeHTTP(res, req)
			return
		}
		user, found := models.UserFromCtx(req.Context())
		if !found {
			RenderResponse(RespForbidden()).ServeHTTP(res, req)
			return
		}
		user.RemoveFavorite(id)
		err := a.updateUser(req.Context(), map[string]any{
			"favorites": user.Favorites,
		})
		if err != nil {
			RenderResponse(RespBackendError(err)).ServeHTTP(res, req)
			return
		}
		err = a.unarchiveArticle(req.Context(), id)
		if err != nil {
			RenderResponse(RespBackendError(err)).ServeHTTP(res, req)
			return
		}
		// Update the content
		template := partials.ToggleFavoriteArticleText(id, false, "#favorite_"+id, "innerHTML")
		resp := models.NewResponse(
			models.WithResponseTemplate(template),
		)
		RenderResponse(resp).ServeHTTP(res, req)
	}).ServeHTTP
}

// AddFavoriteSearch handles adding a new favorite search for a user.
func (a *API) AddFavoriteSearch() http.HandlerFunc {
	return alice.New(
		RouteLogger,
		TriggerEvents("updateFavorites"),
	).ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		// Retrieve the search details.
		request, valid, err := forms.DecodeForm[*models.SearchRequest](req)
		if err != nil || !valid {
			RenderResponse(RespInvalidInput(err)).ServeHTTP(res, req)
			return
		}
		// Add the favorite.
		user, found := models.UserFromCtx(req.Context())
		if !found {
			RenderResponse(RespForbidden()).ServeHTTP(res, req)
			return
		}
		err = user.AddFavoriteSearch("Search: "+request.Text, request)
		if err != nil {
			RenderResponse(RespBackendError(err)).ServeHTTP(res, req)
			return
		}
		err = a.updateUser(req.Context(), map[string]any{
			"favorites": user.Favorites,
		})
		if err != nil {
			RenderResponse(RespBackendError(err)).ServeHTTP(res, req)
			return
		}
		// Update the favorite button.
		id := request.ID()
		if id == "" {
			RenderResponse(RespBackendError(err)).ServeHTTP(res, req)
			return
		}
		fav := user.GetFavorites().Get(id)
		// Update the favorite button.
		resp := models.NewResponse(
			models.WithResponseTemplate(views.RemoveFavoriteSearchButton(fav.GetID())),
		)
		RenderResponse(resp).ServeHTTP(res, req)
	}).ServeHTTP
}

// RemoveFavoriteSearch handles removing a favorite article for a user.
func (a *API) RemoveFavoriteSearch() http.HandlerFunc {
	return alice.New(
		RouteLogger,
		TriggerEvents("updateFavorites"),
	).ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		// Retrieve the search details.
		request, valid, err := forms.DecodeForm[*models.SearchRequest](req)
		if err != nil || !valid {
			RenderResponse(RespInvalidInput(err)).ServeHTTP(res, req)
			return
		}
		// Derive the favorite id.
		id := request.ID()
		if id == "" {
			RenderResponse(RespBackendError(err)).ServeHTTP(res, req)
			return
		}
		// Remove the favorite.
		user, found := models.UserFromCtx(req.Context())
		if !found {
			RenderResponse(RespForbidden()).ServeHTTP(res, req)
			return
		}
		user.RemoveFavorite(id)
		err = a.updateUser(req.Context(), map[string]any{
			"favorites": user.Favorites,
		})
		if err != nil {
			RenderResponse(RespBackendError(err)).ServeHTTP(res, req)
			return
		}
		// Update the favorite button.
		resp := models.NewResponse(
			models.WithResponseTemplate(views.AddFavoriteSearchButton()),
		)
		RenderResponse(resp).ServeHTTP(res, req)
	}).ServeHTTP
}

func (a *API) updateUser(ctx context.Context, updates map[string]any) error {
	// Retrieve user object.
	user, found := models.UserFromCtx(ctx)
	if !found {
		return models.ErrUserCtx
	}
	index := elastic.UserIndexFromCtx(ctx)
	err := elastic.UpdateDoc(ctx, a.DataAPI().GetAPI(), index, user.GetID(), updates,
		elastic.WithRefresh("true"),
		elastic.WithRetryOnConflict(5),
	)
	if err != nil {
		return fmt.Errorf("could not update user: %w", err)
	}
	return nil
}
