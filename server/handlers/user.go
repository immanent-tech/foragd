// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"
	"github.com/justinas/alice"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/auth0"
	"github.com/joshuar/go-feed-me/providers/elastic"
	"github.com/joshuar/go-feed-me/server/forms"
	"github.com/joshuar/go-feed-me/server/session"
	"github.com/joshuar/go-feed-me/validation"
	"github.com/joshuar/go-feed-me/web/templates"
	"github.com/joshuar/go-feed-me/web/templates/layouts"
	"github.com/joshuar/go-feed-me/web/templates/pages"
	"github.com/joshuar/go-feed-me/web/templates/partials"
)

// GetSettings handles retrieving and rendering the user settings page.
func (a *API) GetSettings() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		chain := alice.New(
			RouteLogger,
		)
		user, found := models.UserFromCtx(req.Context())
		if !found {
			chain.Then(RenderResponse(RespForbidden())).ServeHTTP(res, req)
			return
		}
		// Render appropriate content.
		var template templ.Component
		page := layouts.NewSettingsPage(user, &models.EditUserRequest{})
		switch {
		case htmx.IsHTMX(req) && !htmx.IsHistoryRestoreRequest(req):
			// Just show content.
			template = page.Content()
		default:
			// Show full page.
			template = templates.Page(
				"Settings - Go Feed Me",
				layouts.Drawer(page.Content()),
			)
		}

		chain.Then(RenderResponse(
			models.NewResponse(models.WithResponseTemplate(template)),
		)).ServeHTTP(res, req)
	}
}

// SubscriptionsSettings shows a table of subscriptions, optionally filtered, with settings controls.
func (a *API) SubscriptionsSettings() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		chain := alice.New(
			RouteLogger,
		)
		var template templ.Component
		// Extract the search request.
		switch req.Method {
		case http.MethodPost:
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
				settings = append(settings, partials.NewSubscriptionContent(subscription).Settings())
			}
			template = templ.Join(settings...)
			// case http.MethodGet:
			// 	template = pages.NewSettingsPage("subscriptions", nil, nil).Template(req)
		}
		resp := models.NewResponse(
			models.WithResponseTemplate(template),
		)
		chain.Then(RenderResponse(resp)).ServeHTTP(res, req)
	}
}

// AccountSettings handles managing user account settings.
func (a *API) AccountSettings() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		chain := alice.New(
			RouteLogger,
		)
		user, found := models.UserFromCtx(req.Context())
		if !found {
			chain.Then(RenderResponse(RespForbidden())).ServeHTTP(res, req)
			return
		}
		var template templ.Component
		// Extract the search request.
		switch req.Method {
		// case http.MethodGet:
		// 	template = pages.NewSettingsPage("account", user, &models.EditUserRequest{}).Template(req)
		case http.MethodPost:
			request, valid, err := forms.DecodeForm[*models.EditUserRequest](req)
			if err != nil || !valid {
				msg := &models.UserMessage{
					Status:  models.UserMessageStatusError,
					Summary: "Could not edit account.",
					Details: "There are problems with the input. Please check and try again.",
				}
				template := templ.Join(layouts.NewSettingsPage(user, request).Content(), partials.Notification(msg))
				resp := models.RespInternalServerError(err, template)
				chain.Then(RenderResponse(resp)).ServeHTTP(res, req)
				return
			}
			// Apply updates.
			err = a.updateUser(req.Context(), map[string]any{
				"nickname": request.Nickname,
			})
			if err != nil {
				msg := &models.UserMessage{
					Status:  models.UserMessageStatusError,
					Summary: "Could not update account settings.",
					Details: "There was a problem editing account settings. Please try again.",
				}
				template := templ.Join(layouts.NewSettingsPage(user, request).Content(), partials.Notification(msg))
				resp := models.RespInternalServerError(err, template)
				chain.Then(RenderResponse(resp)).ServeHTTP(res, req)
				return
			}
			// Report success.
			msg := &models.UserMessage{
				Status:  models.UserMessageStatusSuccess,
				Summary: "Account edits saved.",
			}
			template = templ.Join(layouts.NewSettingsPage(user, request).Content(), layouts.HeaderUserMenu(), partials.Notification(msg))
		}
		// Update the user in the context.
		user, _ = a.DataAPI().GetUser(req.Context(), user.UserID)
		ctx := models.UserToCtx(req.Context(), user)
		// Render the response.
		resp := models.NewResponse(
			models.WithResponseTemplate(template),
		)
		chain.Then(RenderResponse(resp)).ServeHTTP(res, req.WithContext(ctx))
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
		err := a.updateUser(req.Context(), map[string]any{
			"settings":   settings,
			"updated_at": time.Now().UTC(),
		})
		if err != nil {
			RenderResponse(RespBackendError(err)).ServeHTTP(res, req)
			return
		}
		res.WriteHeader(http.StatusOK)
	}).ServeHTTP
}

// AddFavoriteSubscription handles adding a new favorite subscription for a user.
func (a *API) AddFavoriteSubscription() http.HandlerFunc {
	return alice.New(
		RouteLogger,
	).ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		id := chi.URLParam(req, "subscription")
		valid, err := validation.ValidateVariable(id, "required,startswith=sub_")
		if !valid || err != nil {
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
		err = user.AddFavoriteSubscription(id, metadata.Customisation.Nickname)
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
		// currentURL, found := htmx.GetCurrentURL(req)
		if !found {
			RenderResponse(RespBackendError(nil)).ServeHTTP(res, req)
			return
		}
		subscriptions, err := a.getSubscriptions(req.Context(), id)
		if err != nil || len(subscriptions) == 0 || len(subscriptions) > 1 {
			RenderResponse(RespBackendError(err)).ServeHTTP(res, req)
			return
		}
		template = templ.Join(
			partials.NewSubscriptionContent(subscriptions[0]).ToggleFavorite(),
			partials.FavoritesList(user.GetFavorites()),
		)
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
	).ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		id := chi.URLParam(req, "subscription")
		valid, err := validation.ValidateVariable(id, "required,startswith=sub_")
		if !valid || err != nil {
			RenderResponse(RespInvalidInput(err)).ServeHTTP(res, req)
			return
		}
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
		var template templ.Component
		// currentURL, found := htmx.GetCurrentURL(req)
		if !found {
			RenderResponse(RespBackendError(nil)).ServeHTTP(res, req)
			return
		}
		subscriptions, err := a.getSubscriptions(req.Context(), id)
		if err != nil || len(subscriptions) == 0 || len(subscriptions) > 1 {
			RenderResponse(RespBackendError(err)).ServeHTTP(res, req)
			return
		}
		template = templ.Join(
			partials.NewSubscriptionContent(subscriptions[0]).ToggleFavorite(),
			partials.FavoritesList(user.GetFavorites()),
		)
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
	).ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		id := chi.URLParam(req, "item")
		valid, err := validation.ValidateVariable(id, "required,startswith=item_")
		if !valid || err != nil {
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
		article.Favorite = true
		// Update the content
		template := templ.Join(
			partials.NewArticleContent(article).ToggleFavorite(),
			partials.FavoritesList(user.GetFavorites()),
		)
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
	).ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		id := chi.URLParam(req, "item")
		valid, err := validation.ValidateVariable(id, "required,startswith=item_")
		if !valid || err != nil {
			RenderResponse(RespInvalidInput(err)).ServeHTTP(res, req)
			return
		}
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
		err = a.unarchiveArticle(req.Context(), id)
		if err != nil {
			RenderResponse(RespBackendError(err)).ServeHTTP(res, req)
			return
		}
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
		// Update the content
		template := templ.Join(
			partials.NewArticleContent(article).ToggleFavorite(),
			partials.FavoritesList(user.GetFavorites()),
		)
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
		// Update the favorite button and list of favorites.
		template := templ.Join(
			pages.RemoveFavoriteSearchButton(fav.GetID()),
			partials.FavoritesList(user.GetFavorites()),
		)
		resp := models.NewResponse(
			models.WithResponseTemplate(template),
		)
		RenderResponse(resp).ServeHTTP(res, req)
	}).ServeHTTP
}

// RemoveFavoriteSearch handles removing a favorite article for a user.
func (a *API) RemoveFavoriteSearch() http.HandlerFunc {
	return alice.New(
		RouteLogger,
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
		// Update the favorite button and list of favorites.
		template := templ.Join(
			pages.AddFavoriteSearchButton(),
			partials.FavoritesList(user.GetFavorites()),
		)
		resp := models.NewResponse(
			models.WithResponseTemplate(template),
		)
		RenderResponse(resp).ServeHTTP(res, req)
	}).ServeHTTP
}

// DeleteUser handles removing a user account from the local and backend databases. Once the account is removed, any
// active session is destroyed and the browser is redirected back to the landing page.
func (a *API) DeleteUser() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		chain := alice.New(
			RouteLogger,
		)
		// Get user account details.
		user, found := models.UserFromCtx(req.Context())
		if !found {
			chain.Then(RenderResponse(RespForbidden())).ServeHTTP(res, req)
			return
		}
		// Delete account on the backend.
		err := auth0.Delete(req.Context(), user)
		if err != nil {
			chain.Then(RenderResponse(RespBackendError(err))).ServeHTTP(res, req)
			return
		}
		// Delete account locally.
		err = a.DataAPI().DeleteUser(req.Context(), user.GetID())
		if err != nil {
			chain.Then(RenderResponse(RespBackendError(err))).ServeHTTP(res, req)
			return
		}
		// Remove session cookie.
		err = session.Manager.Destroy(req.Context())
		if err != nil {
			chain.Then(RenderResponse(RespBackendError(err))).ServeHTTP(res, req)
			return
		}

		http.Redirect(res, req, "/", http.StatusSeeOther)
	}
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
