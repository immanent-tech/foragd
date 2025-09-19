// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/justinas/alice"
	"github.com/justinas/nosurf"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/auth0"
	"github.com/immanent-tech/foragd/server/forms"
	"github.com/immanent-tech/foragd/server/session"
	"github.com/immanent-tech/foragd/validation"
	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/layouts"
	"github.com/immanent-tech/foragd/web/templates/pages"
	"github.com/immanent-tech/foragd/web/templates/partials"
)

// GetSettings handles retrieving and rendering the user settings page.
func (a *API) GetSettings() http.HandlerFunc {
	return alice.New(
		routeLogger,
	).ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			return fmt.Errorf("unable to get user settings: %w", err)
		}
		// Render appropriate content.
		template := layouts.NewSettingsPage(user, &models.EditUserRequest{}).Content()
		ctx := models.CSRFTokenToCtx(req.Context(), nosurf.Token(req))
		renderPage(layouts.Drawer(user, template), templates.GeneratePageTitle("Settings")).ServeHTTP(res, req.WithContext(ctx))
		return nil
	})).ServeHTTP
}

// SubscriptionsSettings shows a table of subscriptions, optionally filtered, with settings controls.
func (a *API) SubscriptionsSettings() http.HandlerFunc {
	return alice.New(
		routeLogger,
	).ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		var template templ.Component
		switch req.Method {
		case http.MethodPost:
			request, valid, err := forms.DecodeForm[*models.SearchRequest](req)
			if err != nil || !valid {
				template := partials.Notification(
					models.NewErrorMessage("Unable to filter subscriptions", ""),
				)
				renderPartial(template).ServeHTTP(res, req)
				return models.NewAPIError(err, http.StatusUnprocessableEntity)
			}
			// Find matching subscriptions.
			var subscriptions models.SubscriptionsSlice
			if request.Text != "" {
				subscriptions, err = a.findSubscriptions(req.Context(), request)
			} else {
				subscriptions, err = a.getSubscriptions(req.Context())
			}
			if err != nil {
				template := partials.Notification(
					models.NewErrorMessage("Unable to filter subscriptions", ""),
				)
				renderPartial(template).ServeHTTP(res, req)
				return models.NewAPIError(err, http.StatusInternalServerError)
			}
			settings := make([]templ.Component, 0, len(subscriptions))
			for subscription := range slices.Values(subscriptions) {
				settings = append(settings, partials.NewSubscriptionContent(subscription).Settings())
			}
			template = templ.Join(settings...)
			// case http.MethodGet:
			// 	template = pages.NewSettingsPage("subscriptions", nil, nil).Template(req)
		}
		renderPartial(template).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// AccountSettings handles managing user account settings.
func (a *API) AccountSettings() http.HandlerFunc {
	return alice.New(
		routeLogger,
	).ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		_, err := models.UserFromCtx(req.Context())
		if err != nil {
			return fmt.Errorf("unable to get account settings: %w", err)
		}
		// Extract the search request.
		switch req.Method {
		// case http.MethodGet:
		// 	template = pages.NewSettingsPage("account", user, &models.EditUserRequest{}).Template(req)
		case http.MethodPost:
			request, valid, err := forms.DecodeForm[*models.EditUserRequest](req)
			if err != nil || !valid {
				msg := models.NewErrorMessage(
					"Could not edit account.",
					"There are problems with the input. Please check and try again.",
				)
				template := partials.Notification(msg)
				renderPartial(template).ServeHTTP(res, req)
				return models.NewAPIError(err, http.StatusUnprocessableEntity)
			}
			// Apply updates.
			err = a.DataAPI().UpdateUser(req.Context(), map[string]any{
				"nickname": request.Nickname,
			})
			if err != nil {
				msg := models.NewErrorMessage(
					"Could not update account settings.",
					"There was a problem editing account settings. Please try again.",
				)
				template := partials.Notification(msg)
				renderPartial(template).ServeHTTP(res, req)
				return models.NewAPIError(err, http.StatusInternalServerError)
			}
			// Report success.
			msg := models.NewSuccessMessage("Account edits saved.", "")
			template := partials.Notification(msg)
			renderPartial(template).ServeHTTP(res, req)
		}
		// Update the user in the context.
		// user, _ = a.DataAPI().GetUser(req.Context(), user.UserID)
		// ctx := models.UserToCtx(req.Context(), user)
		return nil
	})).ServeHTTP
}

// SetTheme handles setting a theme selected by the user.
func (a *API) SetTheme() http.HandlerFunc {
	return alice.New(
		routeLogger,
	).ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		theme := chi.URLParam(req, "theme")
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			return fmt.Errorf("unable to set theme: %w", err)
		}
		settings := user.GetSettings()
		settings.Theme = theme
		err = a.DataAPI().UpdateUser(req.Context(), map[string]any{
			"settings":   settings,
			"updated_at": time.Now().UTC(),
		})
		if err != nil {
			msg := models.NewErrorMessage(
				"Unable to set theme.",
				"There was a problem editing account settings. Please try again.",
			)
			template := partials.Notification(msg)
			renderPartial(template).ServeHTTP(res, req)
			return fmt.Errorf("unable to set theme: %w", err)
		}
		renderPartial(layouts.AppSettingsTab(user)).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// AddFavoriteSubscription handles adding a new favorite subscription for a user.
func (a *API) AddFavoriteSubscription() http.HandlerFunc {
	return alice.New(
		routeLogger,
	).ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		id := chi.URLParam(req, "subscription")
		valid, err := validation.ValidateVariable(id, "required,startswith=sub_")
		if !valid || err != nil {
			template := partials.Notification(models.NewErrorMessage("Unable to add favorite.", "Data is invalid."))
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			template := partials.Notification(models.NewErrorMessage("Unable to add favorite.", "User data not found."))
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		// Get the subscription state.
		metadata := user.GetSubscriptionMetadata().GetByID(id)
		// Create a new favorite subscription.
		err = user.AddFavoriteSubscription(id, metadata.Customisation.Nickname)
		if err != nil {
			template := partials.Notification(
				models.NewErrorMessage("Unable to add favorite.", "Temporary backend issue, please try again."))
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		err = a.DataAPI().UpdateUser(req.Context(), map[string]any{
			"favorites": user.Favorites,
		})
		if err != nil {
			template := partials.Notification(
				models.NewErrorMessage("Unable to add favorite.", "Temporary backend issue, please try again."))
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		// Update the favorite button.
		var template templ.Component
		// currentURL, found := htmx.GetCurrentURL(req)
		// if !found {
		// 	render(RespBackendError(nil)).ServeHTTP(res, req)
		// 	return
		// }
		subscriptions, err := a.getSubscriptions(req.Context(), id)
		if err != nil || len(subscriptions) == 0 || len(subscriptions) > 1 {
			template := partials.Notification(
				models.NewErrorMessage("Unable to add favorite.", "Temporary backend issue, please try again."))
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		template = templ.Join(
			partials.NewSubscriptionContent(subscriptions[0]).ToggleFavorite(),
			partials.FavoritesList(user.GetFavorites(), models.OOBSwapTrue),
		)
		renderPartial(template).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// RemoveFavoriteSubscription handles removing a favorite subscription for a user.
func (a *API) RemoveFavoriteSubscription() http.HandlerFunc {
	return alice.New(
		routeLogger,
	).ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		id := chi.URLParam(req, "subscription")
		valid, err := validation.ValidateVariable(id, "required,startswith=sub_")
		if !valid || err != nil {
			template := partials.Notification(models.NewErrorMessage("Unable to add favorite.", "Data is invalid."))
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			template := partials.Notification(models.NewErrorMessage("Unable to add favorite.", "User data not found."))
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		user.RemoveFavorite(id)
		err = a.DataAPI().UpdateUser(req.Context(), map[string]any{
			"favorites": user.Favorites,
		})
		if err != nil {
			template := partials.Notification(
				models.NewErrorMessage("Unable to add favorite.", "Temporary backend issue, please try again."))
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		// Update the favorite button.
		var template templ.Component
		// currentURL, found := htmx.GetCurrentURL(req)
		// if !found {
		// 	template := partials.Notification(
		// 		models.NewErrorMessage("Unable to add favorite.", "Temporary backend issue, please try again."))
		// 	renderPartial(template, "").ServeHTTP(res, req)
		// 	return models.NewAPIError(err, http.StatusInternalServerError)
		// }
		subscriptions, err := a.getSubscriptions(req.Context(), id)
		if err != nil || len(subscriptions) == 0 || len(subscriptions) > 1 {
			template := partials.Notification(
				models.NewErrorMessage("Unable to add favorite.", "Temporary backend issue, please try again."))
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		template = templ.Join(
			partials.NewSubscriptionContent(subscriptions[0]).ToggleFavorite(),
			partials.FavoritesList(user.GetFavorites(), models.OOBSwapTrue),
		)
		renderPartial(template).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// AddFavoriteArticle handles adding a new favorite article for a user.
func (a *API) AddFavoriteArticle() http.HandlerFunc {
	return alice.New(
		routeLogger,
	).ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		id := chi.URLParam(req, "item")
		valid, err := validation.ValidateVariable(id, "required,startswith=item_")
		if !valid || err != nil {
			template := partials.Notification(models.NewErrorMessage("Unable to add favorite.", "Data is invalid."))
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			template := partials.Notification(models.NewErrorMessage("Unable to add favorite.", "User data not found."))
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		// Get the article details.
		articles, err := a.getArticles(req.Context(), id)
		if err != nil {
			template := partials.Notification(
				models.NewErrorMessage("Unable to add favorite.", "Temporary backend issue, please try again."))
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		if len(articles) != 1 {
			template := partials.Notification(
				models.NewErrorMessage("Unable to add favorite.", "Temporary backend issue, please try again."))
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		article := articles[0]
		// Create a new favorite article.
		err = user.AddFavoriteArticle(article.GetTitle(), article)
		if err != nil {
			template := partials.Notification(
				models.NewErrorMessage("Unable to add favorite.", "Temporary backend issue, please try again."))
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		err = a.DataAPI().UpdateUser(req.Context(), map[string]any{
			"favorites": user.Favorites,
		})
		if err != nil {
			template := partials.Notification(
				models.NewErrorMessage("Unable to add favorite.", "Temporary backend issue, please try again."))
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		// Archive the article.
		err = a.archiveArticle(req.Context(), article)
		if err != nil {
			template := partials.Notification(
				models.NewErrorMessage("Unable to add favorite.", "Temporary backend issue, please try again."))
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		article.Favorite = true
		// Get the display type.
		display := req.FormValue("display")
		// Update the content as appropriate.
		var template templ.Component
		switch display {
		case "card":
			template = templ.Join(
				partials.NewArticleContent(article).ToggleFavorite(),
				partials.FavoritesList(user.GetFavorites(), models.OOBSwapTrue),
			)
		case "content":
			template = templ.Join(
				partials.UpdateViewArticleFavorite(article),
				partials.FavoritesList(user.GetFavorites(), models.OOBSwapTrue),
			)
		}
		renderPartial(template).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// RemoveFavoriteArticle handles removing a favorite article for a user.
func (a *API) RemoveFavoriteArticle() http.HandlerFunc {
	return alice.New(
		routeLogger,
	).ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		id := chi.URLParam(req, "item")
		valid, err := validation.ValidateVariable(id, "required,startswith=item_")
		if !valid || err != nil {
			template := partials.Notification(models.NewErrorMessage("Unable to process favorite.", "Data is invalid."))
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			template := partials.Notification(models.NewErrorMessage("Unable to process favorite.", "User data not found."))
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		user.RemoveFavorite(id)
		err = a.DataAPI().UpdateUser(req.Context(), map[string]any{
			"favorites": user.Favorites,
		})
		if err != nil {
			template := partials.Notification(
				models.NewErrorMessage("Unable to process favorite.", "Temporary backend issue, please try again."))
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		err = a.unarchiveArticle(req.Context(), id)
		if err != nil {
			template := partials.Notification(
				models.NewErrorMessage("Unable to process favorite.", "Temporary backend issue, please try again."))
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		articles, err := a.getArticles(req.Context(), id)
		if err != nil {
			template := partials.Notification(
				models.NewErrorMessage("Unable to process favorite.", "Temporary backend issue, please try again."))
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		if len(articles) != 1 {
			template := partials.Notification(
				models.NewErrorMessage("Unable to process favorite.", "Temporary backend issue, please try again."))
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		article := articles[0]
		// Get the display type.
		display := req.FormValue("display")
		// Update the content as appropriate.
		var template templ.Component
		switch display {
		case "card":
			template = templ.Join(
				partials.NewArticleContent(article).ToggleFavorite(),
				partials.FavoritesList(user.GetFavorites(), models.OOBSwapTrue),
			)
		case "content":
			template = templ.Join(
				partials.UpdateViewArticleFavorite(article),
				partials.FavoritesList(user.GetFavorites(), models.OOBSwapTrue),
			)
		}
		renderPartial(template).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// AddFavoriteSearch handles adding a new favorite search for a user.
func (a *API) AddFavoriteSearch() http.HandlerFunc {
	return alice.New(
		routeLogger,
	).ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Retrieve the search details.
		request, valid, err := forms.DecodeForm[*models.SearchRequest](req)
		if err != nil || !valid {
			template := partials.Notification(models.NewErrorMessage("Unable to process favorite.", "Data is invalid."))
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		// Add the favorite.
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			template := partials.Notification(models.NewErrorMessage("Unable to process favorite.", "User data not found."))
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		err = user.AddFavoriteSearch("Search: "+request.Text, request)
		if err != nil {
			template := partials.Notification(
				models.NewErrorMessage("Unable to process favorite.", "Temporary backend issue, please try again."))
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		err = a.DataAPI().UpdateUser(req.Context(), map[string]any{
			"favorites": user.Favorites,
		})
		if err != nil {
			template := partials.Notification(
				models.NewErrorMessage("Unable to process favorite.", "Temporary backend issue, please try again."))
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		// Update the favorite button.
		id := request.ID()
		if id == "" {
			template := partials.Notification(
				models.NewErrorMessage("Unable to process favorite.", "Temporary backend issue, please try again."))
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		fav := user.GetFavorites().Get(id)
		// Update the favorite button and list of favorites.
		template := templ.Join(
			pages.RemoveFavoriteSearchButton(fav.GetID()),
			partials.FavoritesList(user.GetFavorites(), models.OOBSwapTrue),
		)
		renderPartial(template).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// RemoveFavoriteSearch handles removing a favorite article for a user.
func (a *API) RemoveFavoriteSearch() http.HandlerFunc {
	return alice.New(
		routeLogger,
	).ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Retrieve the search details.
		request, valid, err := forms.DecodeForm[*models.SearchRequest](req)
		if err != nil || !valid {
			template := partials.Notification(models.NewErrorMessage("Unable to process favorite.", "Data is invalid."))
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			template := partials.Notification(models.NewErrorMessage("Unable to process favorite.", "User data not found."))
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		// Derive the favorite id.
		id := request.ID()
		if id == "" {
			template := partials.Notification(
				models.NewErrorMessage("Unable to process favorite.", "Temporary backend issue, please try again."))
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		// Remove the favorite.
		user.RemoveFavorite(id)
		err = a.DataAPI().UpdateUser(req.Context(), map[string]any{
			"favorites": user.Favorites,
		})
		if err != nil {
			template := partials.Notification(
				models.NewErrorMessage("Unable to process favorite.", "Temporary backend issue, please try again."))
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		// Update the favorite button and list of favorites.
		template := templ.Join(
			pages.AddFavoriteSearchButton(),
			partials.FavoritesList(user.GetFavorites(), models.OOBSwapTrue),
		)
		renderPartial(template).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// DeleteUser handles removing a user account from the local and backend databases. Once the account is removed, any
// active session is destroyed and the browser is redirected back to the landing page.
func (a *API) DeleteUser() http.HandlerFunc {
	return alice.New(
		routeLogger,
	).ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Get user account details.
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			return fmt.Errorf("could not delete user: %w", err)
		}
		// Delete account on the backend.
		err = auth0.Delete(req.Context(), user)
		if err != nil {
			return fmt.Errorf("could not delete user: %w", err)
		}
		// Delete account locally.
		err = a.DataAPI().DeleteUser(req.Context(), user.GetID())
		if err != nil {
			return fmt.Errorf("could not delete user: %w", err)
		}
		// Remove session cookie.
		err = session.Manager.Destroy(req.Context())
		if err != nil {
			return fmt.Errorf("could not delete user: %w", err)
		}
		http.Redirect(res, req, "/", http.StatusSeeOther)
		return nil
	})).ServeHTTP
}
