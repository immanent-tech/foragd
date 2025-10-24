// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"embed"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"slices"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-syndication/opml"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/auth0"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/github"
	"github.com/immanent-tech/foragd/server/forms"
	"github.com/immanent-tech/foragd/server/session"
	"github.com/immanent-tech/foragd/validation"
	"github.com/immanent-tech/foragd/web/templates"
)

// ShowSettings handles retrieving and rendering the user settings page.
func (a *API) ShowSettings() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		renderPage(templates.SettingsPage(), templates.GeneratePageTitle("Settings")).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// ShowDisplaySettings handles showing the settings related to the application display.
func ShowDisplaySettings() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			renderPartial(
				templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to show display settings", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to retrieve user data: %w", err), http.StatusInternalServerError)
		}
		renderPartial(templates.DisplaySettings(user)).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// ShowAccountSettings handles showing the settings related to user accounts.
func ShowAccountSettings() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			renderPartial(
				templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to show account settings", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to retrieve user data: %w", err), http.StatusInternalServerError)
		}
		renderPartial(templates.AccountSettings(user)).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// SaveDisplaySettings handles saving user settings after user submitted changes.
func SaveDisplaySettings(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			renderPartial(
				templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to save display settings", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to retrieve user data: %w", err), http.StatusInternalServerError)
		}
		settings := user.GetSettings()
		// Parse show_unread_counts setting.
		switch req.FormValue("show_unread_counts") {
		case "on":
			settings.ShowUnreadCounts = true
		case "":
			settings.ShowUnreadCounts = false
		}
		// Parse mark_article_read_on_view setting.
		switch req.FormValue("mark_article_read_on_view") {
		case "on":
			settings.MarkArticleReadOnView = true
		case "":
			settings.MarkArticleReadOnView = false
		}
		// Update user object with new settings.
		err = api.UpdateUser(req.Context(), user.GetID(), map[string]any{"settings": settings})
		if err != nil {
			renderPartial(
				templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to save display settings", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to update user data: %w", err), http.StatusInternalServerError)
		}
		// Show success notification.
		template := templates.Notification(
			models.NewSuccessMessage("Settings saved", ""), 0,
		)
		renderPartial(template).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// SaveAccountSettings handles processing and saving new account settings.
func SaveAccountSettings(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		request, valid, err := forms.DecodeForm[*models.EditUserRequest](req)
		if err != nil || !valid {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to save account settings", "Data is invalid."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("%w: %w", ErrInvalidRequestParams, err), http.StatusUnprocessableEntity)
		}
		// Update on backend.
		err = auth0.UpdateUser(req.Context(), request)
		if err != nil || !valid {
			renderPartial(
				templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to save account settings", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to update user on backend: %w", err), http.StatusInternalServerError)
		}
		// Update local copy.
		err = models.UpdateUser(req.Context(), api, request)
		if err != nil || !valid {
			renderPartial(
				templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to save account settings", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to update user data: %w", err), http.StatusInternalServerError)
		}
		// Report success.
		msg := models.NewSuccessMessage("Account edits saved!", "")
		template := templates.Notification(msg, 0)
		renderPartial(template).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// ChangePassword handles a change password request from the user.
func ChangePassword(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		request, valid, err := forms.DecodeForm[*models.ChangePasswordRequest](req)
		if err != nil || !valid {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to change password", "Data is invalid."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("%w: %w", ErrInvalidRequestParams, err), http.StatusUnprocessableEntity)
		}
		// Update on backend.
		err = auth0.ChangeUserPassword(req.Context(), request)
		if err != nil || !valid {
			renderPartial(
				templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to change password", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to update user data: %w", err), http.StatusInternalServerError)
		}
		// Report success.
		msg := models.NewSuccessMessage("Password changed!", "Logout and log back in to use the new password.")
		template := templates.Notification(msg, 0)
		renderPartial(template).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// SetTheme handles setting a theme selected by the user.
func (a *API) SetTheme() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		theme := chi.URLParam(req, "theme")
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to set theme", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to retrieve user data: %w", err), http.StatusInternalServerError)
		}
		settings := user.GetSettings()
		settings.Theme = theme
		err = a.DataAPI().UpdateUser(req.Context(), user.GetID(), map[string]any{
			"settings": settings,
		})
		if err != nil {
			renderPartial(
				templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to set theme", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to update user data: %w", err), http.StatusInternalServerError)
		}
		renderPartial(templates.DisplaySettings(user)).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// AddFavoriteSubscription handles adding a new favorite subscription for a user.
func (a *API) AddFavoriteSubscription() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		id := chi.URLParam(req, models.ParamSubscriptionID)
		valid, err := validation.ValidateVariable(id, "required,startswith=sub_")
		if !valid || err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to add favorite subscription", "Data is invalid."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("%w: %w", ErrInvalidRequestParams, err), http.StatusUnprocessableEntity)
		}
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to add favorite subscription", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to retrieve user data: %w", err), http.StatusInternalServerError)
		}
		// Get the subscription state.
		metadata := user.GetSubscriptionMetadata().GetByID(id)
		// Create a new favorite subscription.
		err = user.AddFavoriteSubscription(id, metadata.Customisation.Nickname)
		if err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to add favorite subscription", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to add favorite subscription to user: %w", err), http.StatusInternalServerError)
		}
		err = a.DataAPI().UpdateUser(req.Context(), user.GetID(), map[string]any{
			"favorites": user.Favorites,
		})
		if err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to add favorite subscription", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to update user object: %w", err), http.StatusInternalServerError)
		}
		subscriptions, err := models.GetSubscriptions(req.Context(), a.Elastic, id)
		if err != nil || len(subscriptions) == 0 || len(subscriptions) > 1 {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to add favorite subscription", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to retrieve updated subscriptions: %w", err), http.StatusInternalServerError)
		}
		renderPartial(templates.ToggleFavorite(subscriptions[0])).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// RemoveFavoriteSubscription handles removing a favorite subscription for a user.
func (a *API) RemoveFavoriteSubscription() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		id := chi.URLParam(req, models.ParamSubscriptionID)
		valid, err := validation.ValidateVariable(id, "required,startswith=sub_")
		if !valid || err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to remove favorite subscription", "Data is invalid."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("%w: %w", ErrInvalidRequestParams, err), http.StatusUnprocessableEntity)
		}
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to remove favorite subscription", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to retrieve user data: %w", err), http.StatusInternalServerError)
		}
		user.RemoveFavorite(id)
		err = a.DataAPI().UpdateUser(req.Context(), user.GetID(), map[string]any{
			"favorites": user.Favorites,
		})
		if err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to remove favorite subscription", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to update user data: %w", err), http.StatusInternalServerError)
		}
		// Update the favorite button.
		subscriptions, err := models.GetSubscriptions(req.Context(), a.Elastic, id)
		if err != nil || len(subscriptions) == 0 || len(subscriptions) > 1 {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to remove favorite subscription", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to retrieve updated subscriptions: %w", err), http.StatusInternalServerError)
		}
		renderPartial(templates.ToggleFavorite(subscriptions[0])).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// AddFavoriteArticle handles adding a new favorite article for a user.
func (a *API) AddFavoriteArticle() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		id := chi.URLParam(req, models.ParamItemID)
		valid, err := validation.ValidateVariable(id, "required,startswith=item_")
		if !valid || err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to add favorite article", "Data is invalid."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("%w: %w", ErrInvalidRequestParams, err), http.StatusUnprocessableEntity)
		}
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to add favorite article", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to retrieve user data: %w", err), http.StatusInternalServerError)
		}
		// Get the article details.
		articles, err := models.GetArticles(req.Context(), a.Elastic, id)
		if err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to add favorite article", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to get articles: %w", err), http.StatusInternalServerError)
		}
		if len(articles) != 1 {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to add favorite article", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("%w: expected single result, got %d", ErrInvalidAPIResponse, len(articles)), http.StatusInternalServerError)
		}
		article := articles[0]
		// Create a new favorite article.
		err = user.AddFavoriteArticle(article.GetTitle(), article)
		if err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to add favorite article", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to add favorite article to user: %w", err), http.StatusInternalServerError)
		}
		err = a.DataAPI().UpdateUser(req.Context(), user.GetID(), map[string]any{
			"favorites": user.Favorites,
		})
		if err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to add favorite article", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to update user data: %w", err), http.StatusInternalServerError)
		}
		// Archive the article.
		err = a.archiveArticle(req.Context(), article)
		if err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to add favorite article", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to archive article: %w", err), http.StatusInternalServerError)
		}
		article.Favorite = true
		// Get the display type.
		display := req.FormValue("display")
		// Update the content as appropriate.
		var template templ.Component
		switch display {
		case "card":
			template = templates.ToggleFavorite(article)
		case "content":
			template = templates.UpdateViewArticleFavorite(article)
		}
		renderPartial(template).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// RemoveFavoriteArticle handles removing a favorite article for a user.
func (a *API) RemoveFavoriteArticle() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		id := chi.URLParam(req, models.ParamItemID)
		valid, err := validation.ValidateVariable(id, "required,startswith=item_")
		if !valid || err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to remove favorite article", "Data is invalid."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("%w: %w", ErrInvalidRequestParams, err), http.StatusUnprocessableEntity)
		}
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to remove favorite article", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to retrieve user data: %w", err), http.StatusInternalServerError)
		}
		user.RemoveFavorite(id)
		err = a.DataAPI().UpdateUser(req.Context(), user.GetID(), map[string]any{
			"favorites": user.Favorites,
		})
		if err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to remove favorite article", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to update user data: %w", err), http.StatusInternalServerError)
		}
		err = a.unarchiveArticle(req.Context(), id)
		if err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to remove favorite article", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to remove article archive: %w", err), http.StatusInternalServerError)
		}
		articles, err := models.GetArticles(req.Context(), a.Elastic, id)
		if err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to remove favorite article", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to get updated articles: %w", err), http.StatusInternalServerError)
		}
		if len(articles) != 1 {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to remove favorite article", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("%w: expected single result, got %d", ErrInvalidAPIResponse, len(articles)), http.StatusInternalServerError)
		}
		article := articles[0]
		// Get the display type.
		display := req.FormValue("display")
		// Update the content as appropriate.
		var template templ.Component
		switch display {
		case "card":
			template = templates.ToggleFavorite(article)
		case "content":
			template = templates.UpdateViewArticleFavorite(article)
		}
		renderPartial(template).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// AddFavoriteSearch handles adding a new favorite search for a user.
func (a *API) AddFavoriteSearch() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Retrieve the search details.
		request, valid, err := forms.DecodeForm[*models.SearchRequest](req)
		if err != nil || !valid {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to add favorite search", "Data is invalid."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("%w: %w", ErrInvalidRequestParams, err), http.StatusUnprocessableEntity)
		}
		name := req.FormValue("search_name")
		// Add the favorite.
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to add favorite search", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to retrieve user data: %w", err), http.StatusInternalServerError)
		}
		err = user.AddFavoriteSearch(name, request)
		if err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to add favorite search", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to add favorite search to user: %w", err), http.StatusInternalServerError)
		}
		err = a.DataAPI().UpdateUser(req.Context(), user.GetID(), map[string]any{
			"favorites": user.Favorites,
		})
		if err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to add favorite search", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to update user data: %w", err), http.StatusInternalServerError)
		}
		// Update the favorite button.
		id := request.ID()
		if id == "" {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to add favorite search", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("%w: search request ID was empty", ErrInvalidAPIResponse), http.StatusInternalServerError)
		}
		fav := user.GetAllFavorites().Get(id)
		// Update the favorite button and list of favorites.
		renderPartial(templates.RemoveFavoriteSearchButton(fav.GetID())).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// RemoveFavoriteSearch handles removing a favorite article for a user.
func (a *API) RemoveFavoriteSearch() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Retrieve the search details.
		request, valid, err := forms.DecodeForm[*models.SearchRequest](req)
		if err != nil || !valid {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to remove favorite search", "Data is invalid."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("%w: %w", ErrInvalidRequestParams, err), http.StatusUnprocessableEntity)
		}
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to remove favorite search", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to retrieve user data: %w", err), http.StatusInternalServerError)
		}
		// Derive the favorite id.
		id := request.ID()
		if id == "" {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to add favorite search", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("%w: search request ID was empty", ErrInvalidAPIResponse), http.StatusInternalServerError)
		}
		// Remove the favorite.
		user.RemoveFavorite(id)
		err = a.DataAPI().UpdateUser(req.Context(), user.GetID(), map[string]any{
			"favorites": user.Favorites,
		})
		if err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to remove favorite search", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to update user data: %w", err), http.StatusInternalServerError)
		}
		// Update the favorite button and list of favorites.
		renderPartial(templates.AddFavoriteSearchButton()).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// DeleteUser handles removing a user account from the local and backend databases. Once the account is removed, any
// active session is destroyed and the browser is redirected back to the landing page.
func (a *API) DeleteUser() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		switch req.Method {
		case http.MethodGet:
			renderPartial(templates.DeleteAccountModal()).ServeHTTP(res, req)
		case http.MethodPost:
			// Get user account details.
			user, err := models.UserFromCtx(req.Context())
			if err != nil {
				renderPartial(templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to delete account", "This might be a temporary error, please try again.")),
				).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("unable to retrieve user data: %w", err), http.StatusInternalServerError)
			}
			// Delete account on the backend.
			err = auth0.DeleteUser(req.Context(), user)
			if err != nil {
				renderPartial(templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to delete account", "This might be a temporary error, please try again.")),
				).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("unable to delete account in auth backend: %w", err), http.StatusInternalServerError)
			}
			// Delete account locally.
			err = a.DataAPI().DeleteUser(req.Context(), user.GetID())
			if err != nil {
				renderPartial(templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to delete account", "This might be a temporary error, please try again.")),
				).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("unable to delete user: %w", err), http.StatusInternalServerError)
			}
			// Remove session cookie.
			err = session.Manager.Destroy(req.Context())
			if err != nil {
				renderPartial(templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to delete account", "This might be a temporary error, please try again.")),
				).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("unable to delete user sessions: %w", err), http.StatusInternalServerError)
			}
			res.Header().Add(htmx.HeaderRedirect, "/")
		}
		return nil
	})).ServeHTTP
}

// AddFeedset handles adding a feedset as subscriptions.
func AddFeedset(storeAPI *elastic.API, static embed.FS) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		request, valid, err := forms.DecodeForm[*models.AddFeedsetRequest](req)
		if err != nil || !valid {
			res.Header().Add(htmx.HeaderReswap, "none")
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to add feedset", "Data is invalid."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("%w: %w", ErrInvalidRequestParams, err), http.StatusUnprocessableEntity)
		}
		// Process requested feedsets and generate subscription requests.
		var subscriptionRequests models.SubscriptionRequests
		for set := range slices.Values(request.Feedset) {
			var (
				data []byte
				err  error
			)
			switch set {
			case "enlightened":
				data, err = static.ReadFile("content/opml/enlightened.opml")
			case "informed":
				data, err = static.ReadFile("content/opml/informed.opml")
			case "inspired":
				data, err = static.ReadFile("content/opml/inspired.opml")
			default:
				slogctx.FromCtx(req.Context()).Warn("Unknown feedset.",
					slog.String("set", set))
				continue
			}
			if err != nil {
				res.Header().Add(htmx.HeaderReswap, "none")
				renderPartial(templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to add feedset", "This might be a temporary issue, please try again."),
				)).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("unable to read feedset file: %w", err), http.StatusInternalServerError)
			}
			opmlImport, err := opml.NewOPMLFromBytes(data)
			if err != nil {
				res.Header().Add(htmx.HeaderReswap, "none")
				renderPartial(templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to add feedset", "This might be a temporary issue, please try again."),
				)).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("to generate OPML import: %w", err), http.StatusInternalServerError)
			}
			subscriptionRequests = append(subscriptionRequests, models.GenerateRequestsFromOutlines(opmlImport.Body...)...)
		}
		// Process subscription requests.
		subscriptionProcessing := make(addSubscriptionRequests)
		for r := range slices.Values(subscriptionRequests) {
			subscriptionProcessing[r] = &models.Feed{}
		}
		matchResults, err := subscriptionProcessing.matchFeedsToSubscriptionRequests(req.Context(), storeAPI)
		if err != nil {
			res.Header().Add(htmx.HeaderReswap, "none")
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to add feedset", "This might be a temporary issue, please try again."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to match feeds to subscriptions: %w", err), http.StatusInternalServerError)
		}
		createResults, err := subscriptionProcessing.createNewSubscriptions(req.Context(), storeAPI)
		if err != nil {
			res.Header().Add(htmx.HeaderReswap, "none")
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to add feedset", "This might be a temporary issue, please try again."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to create new subscriptions: %w", err), http.StatusInternalServerError)
		}
		maps.Copy(createResults, matchResults)
		msg := models.NewSuccessMessage("Added sets", "Request sets added to your subscriptions.")
		renderPartial(templates.Notification(msg, 0)).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// GetPageIssues handles presenting a form for the user to submit issues about the app.
func GetPageIssues(api *API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Get the current URL on which the issue is being reported.
		currentURL, found := htmx.GetCurrentURL(req)
		if !found {
			slogctx.FromCtx(req.Context()).Warn("No HX-Current-URL header found.")
		}
		// Display the report issue form.
		template := templates.ReportPageIssue(&models.IssueRequest{PageUrl: currentURL})
		renderPage(template, templates.GeneratePageTitle("Report subscription issue")).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// SubmitPageIssues handles processing the user submitted subscription issues form.
func SubmitPageIssues(esapi *API, ghapi *github.Client) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Validate the subscription issue request.
		request, valid, err := forms.DecodeForm[*models.IssueRequest](req)
		if err != nil || !valid {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to submit issue", "Data is invalid."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("%w: %w", ErrInvalidRequestParams, err), http.StatusUnprocessableEntity)
		}
		// Create the issue in Github.
		err = ghapi.CreateIssue(req.Context(), request)
		if err != nil {
			res.Header().Add(htmx.HeaderReswap, "none")
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to submit issue", "This might be a temporary issue, please try again."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("%w: %w", ErrInvalidRequestParams, err), http.StatusUnprocessableEntity)
		}
		// Force refresh of page.
		msg := models.NewErrorMessage(
			"Thanks for reporting the issue!",
			"We will look into it and implement fixes as appropriate.",
		)
		renderPage(templates.IssueReportedConfirmation(msg), templates.GeneratePageTitle("Report subscription issue")).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}
