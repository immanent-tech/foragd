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
	"strings"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"
	"github.com/justinas/alice"
	"github.com/justinas/nosurf"
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

// GetSettings handles retrieving and rendering the user settings page.
func (a *API) GetSettings() http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			return fmt.Errorf("unable to get user settings: %w", err)
		}
		// Render appropriate content.
		template := templates.NewSettingsPage(user, &models.EditUserRequest{}).Content()
		ctx := models.CSRFTokenToCtx(req.Context(), nosurf.Token(req))
		renderPage(template, templates.GeneratePageTitle("Settings")).ServeHTTP(res, req.WithContext(ctx))
		return nil
	})).ServeHTTP
}

// SaveSettings handles saving user settings after user submitted changes.
func SaveSettings(api *elastic.API) http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			return fmt.Errorf("unable to get user settings: %w", err)
		}
		settings := user.GetSettings()
		// Parse show_unread_counts setting.
		show_unread_counts := req.FormValue("show_unread_counts")
		switch show_unread_counts {
		case "on":
			settings.ShowUnreadCounts = true
		case "":
			settings.ShowUnreadCounts = false
		}
		// Update user object with new settings.
		err = api.UpdateUser(req.Context(), user.GetID(), map[string]any{"settings": settings})
		if err != nil {
			template := templates.Notification(
				models.NewErrorMessage("Unable save settings", "This might be a temporary issue, please try again"), 0,
			)
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		// Show success notification.
		template := templates.Notification(
			models.NewSuccessMessage("Settings saved", ""), 0,
		)
		renderPartial(template).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// GetSubscriptionsSettings shows a table of subscriptions, optionally filtered, with settings controls.
func (a *API) GetSubscriptionsSettings() http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		text := req.FormValue("text")
		request := models.NewSearchRequest()
		request.Text = text
		// Find matching subscriptions.
		var (
			subscriptions models.Subscriptions
			err           error
		)
		if request.Text != "" {
			subscriptions, err = a.findSubscriptions(req.Context(), request)
		} else {
			subscriptions, err = models.GetSubscriptions(req.Context(), a.Elastic)
		}
		if err != nil {
			template := templates.Notification(
				models.NewErrorMessage("Unable to filter subscriptions", ""), 0,
			)
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		settings := make([]templ.Component, 0, len(subscriptions))
		for subscription := range slices.Values(subscriptions) {
			settings = append(settings, templates.SubscriptionSettings(subscription))
		}
		template := templ.Join(settings...)
		renderPartial(template).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// AccountSettings handles managing user account settings.
func (a *API) AccountSettings() http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		user, err := models.UserFromCtx(req.Context())
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
				template := templates.Notification(msg, 0)
				renderPartial(template).ServeHTTP(res, req)
				return models.NewAPIError(err, http.StatusUnprocessableEntity)
			}
			// Apply updates.
			err = a.DataAPI().UpdateUser(req.Context(), user.GetID(), map[string]any{
				"nickname": request.Nickname,
			})
			if err != nil {
				msg := models.NewErrorMessage(
					"Could not update account settings.",
					"There was a problem editing account settings. Please try again.",
				)
				template := templates.Notification(msg, 0)
				renderPartial(template).ServeHTTP(res, req)
				return models.NewAPIError(err, http.StatusInternalServerError)
			}
			// Report success.
			msg := models.NewSuccessMessage("Account edits saved.", "")
			template := templates.Notification(msg, 0)
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
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		theme := chi.URLParam(req, "theme")
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			return fmt.Errorf("unable to set theme: %w", err)
		}
		settings := user.GetSettings()
		settings.Theme = theme
		err = a.DataAPI().UpdateUser(req.Context(), user.GetID(), map[string]any{
			"settings": settings,
		})
		if err != nil {
			msg := models.NewErrorMessage(
				"Unable to set theme.",
				"There was a problem editing account settings. Please try again.",
			)
			template := templates.Notification(msg, 0)
			renderPartial(template).ServeHTTP(res, req)
			return fmt.Errorf("unable to set theme: %w", err)
		}
		renderPartial(templates.AppSettingsTab(user)).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// AddFavoriteSubscription handles adding a new favorite subscription for a user.
func (a *API) AddFavoriteSubscription() http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		id := chi.URLParam(req, models.ParamSubscriptionID)
		valid, err := validation.ValidateVariable(id, "required,startswith=sub_")
		if !valid || err != nil {
			template := templates.Notification(models.NewErrorMessage("Unable to add favorite.", "Data is invalid."), 0)
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			template := templates.Notification(models.NewErrorMessage("Unable to add favorite.", "User data not found."), 0)
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		// Get the subscription state.
		metadata := user.GetSubscriptionMetadata().GetByID(id)
		// Create a new favorite subscription.
		err = user.AddFavoriteSubscription(id, metadata.Customisation.Nickname)
		if err != nil {
			template := templates.Notification(
				models.NewErrorMessage("Unable to add favorite.", "Temporary backend issue, please try again."), 0)
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		err = a.DataAPI().UpdateUser(req.Context(), user.GetID(), map[string]any{
			"favorites": user.Favorites,
		})
		if err != nil {
			template := templates.Notification(
				models.NewErrorMessage("Unable to add favorite.", "Temporary backend issue, please try again."), 0)
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
		subscriptions, err := models.GetSubscriptions(req.Context(), a.Elastic, id)
		if err != nil || len(subscriptions) == 0 || len(subscriptions) > 1 {
			template := templates.Notification(
				models.NewErrorMessage("Unable to add favorite.", "Temporary backend issue, please try again."), 0)
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		template = templ.Join(
			templates.ToggleFavorite(subscriptions[0]),
			templates.FavoritesList(user.GetFavorites(), models.OOBSwapTrue),
		)
		renderPartial(template).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// RemoveFavoriteSubscription handles removing a favorite subscription for a user.
func (a *API) RemoveFavoriteSubscription() http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		id := chi.URLParam(req, models.ParamSubscriptionID)
		valid, err := validation.ValidateVariable(id, "required,startswith=sub_")
		if !valid || err != nil {
			template := templates.Notification(models.NewErrorMessage("Unable to add favorite.", "Data is invalid."), 0)
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			template := templates.Notification(models.NewErrorMessage("Unable to add favorite.", "User data not found."), 0)
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		user.RemoveFavorite(id)
		err = a.DataAPI().UpdateUser(req.Context(), user.GetID(), map[string]any{
			"favorites": user.Favorites,
		})
		if err != nil {
			template := templates.Notification(
				models.NewErrorMessage("Unable to add favorite.", "Temporary backend issue, please try again."), 0)
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		// Update the favorite button.
		var template templ.Component
		// currentURL, found := htmx.GetCurrentURL(req)
		// if !found {
		// 	template := templates.Notification(
		// 		models.NewErrorMessage("Unable to add favorite.", "Temporary backend issue, please try again."))
		// 	renderPartial(template, "").ServeHTTP(res, req)
		// 	return models.NewAPIError(err, http.StatusInternalServerError)
		// }
		subscriptions, err := models.GetSubscriptions(req.Context(), a.Elastic, id)
		if err != nil || len(subscriptions) == 0 || len(subscriptions) > 1 {
			template := templates.Notification(
				models.NewErrorMessage("Unable to add favorite.", "Temporary backend issue, please try again."), 0)
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		template = templ.Join(
			templates.ToggleFavorite(subscriptions[0]),
			templates.FavoritesList(user.GetFavorites(), models.OOBSwapTrue),
		)
		renderPartial(template).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// AddFavoriteArticle handles adding a new favorite article for a user.
func (a *API) AddFavoriteArticle() http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		id := chi.URLParam(req, models.ParamItemID)
		valid, err := validation.ValidateVariable(id, "required,startswith=item_")
		if !valid || err != nil {
			template := templates.Notification(models.NewErrorMessage("Unable to add favorite.", "Data is invalid."), 0)
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			template := templates.Notification(models.NewErrorMessage("Unable to add favorite.", "User data not found."), 0)
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		// Get the article details.
		articles, err := models.GetArticles(req.Context(), a.Elastic, id)
		if err != nil {
			template := templates.Notification(
				models.NewErrorMessage("Unable to add favorite.", "Temporary backend issue, please try again."), 0)
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		if len(articles) != 1 {
			template := templates.Notification(
				models.NewErrorMessage("Unable to add favorite.", "Temporary backend issue, please try again."), 0)
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		article := articles[0]
		// Create a new favorite article.
		err = user.AddFavoriteArticle(article.GetTitle(), article)
		if err != nil {
			template := templates.Notification(
				models.NewErrorMessage("Unable to add favorite.", "Temporary backend issue, please try again."), 0)
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		err = a.DataAPI().UpdateUser(req.Context(), user.GetID(), map[string]any{
			"favorites": user.Favorites,
		})
		if err != nil {
			template := templates.Notification(
				models.NewErrorMessage("Unable to add favorite.", "Temporary backend issue, please try again."), 0)
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		// Archive the article.
		err = a.archiveArticle(req.Context(), article)
		if err != nil {
			template := templates.Notification(
				models.NewErrorMessage("Unable to add favorite.", "Temporary backend issue, please try again."), 0)
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
				templates.ToggleFavorite(article),
				templates.FavoritesList(user.GetFavorites(), models.OOBSwapTrue),
			)
		case "content":
			template = templ.Join(
				templates.UpdateViewArticleFavorite(article),
				templates.FavoritesList(user.GetFavorites(), models.OOBSwapTrue),
			)
		}
		renderPartial(template).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// RemoveFavoriteArticle handles removing a favorite article for a user.
func (a *API) RemoveFavoriteArticle() http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		id := chi.URLParam(req, models.ParamItemID)
		valid, err := validation.ValidateVariable(id, "required,startswith=item_")
		if !valid || err != nil {
			template := templates.Notification(models.NewErrorMessage("Unable to process favorite.", "Data is invalid."), 0)
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			template := templates.Notification(models.NewErrorMessage("Unable to process favorite.", "User data not found."), 0)
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		user.RemoveFavorite(id)
		err = a.DataAPI().UpdateUser(req.Context(), user.GetID(), map[string]any{
			"favorites": user.Favorites,
		})
		if err != nil {
			template := templates.Notification(
				models.NewErrorMessage("Unable to process favorite.", "Temporary backend issue, please try again."), 0)
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		err = a.unarchiveArticle(req.Context(), id)
		if err != nil {
			template := templates.Notification(
				models.NewErrorMessage("Unable to process favorite.", "Temporary backend issue, please try again."), 0)
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		articles, err := models.GetArticles(req.Context(), a.Elastic, id)
		if err != nil {
			template := templates.Notification(
				models.NewErrorMessage("Unable to process favorite.", "Temporary backend issue, please try again."), 0)
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		if len(articles) != 1 {
			template := templates.Notification(
				models.NewErrorMessage("Unable to process favorite.", "Temporary backend issue, please try again."), 0)
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
				templates.ToggleFavorite(article),
				templates.FavoritesList(user.GetFavorites(), models.OOBSwapTrue),
			)
		case "content":
			template = templ.Join(
				templates.UpdateViewArticleFavorite(article),
				templates.FavoritesList(user.GetFavorites(), models.OOBSwapTrue),
			)
		}
		renderPartial(template).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// AddFavoriteSearch handles adding a new favorite search for a user.
func (a *API) AddFavoriteSearch() http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Retrieve the search details.
		request, valid, err := forms.DecodeForm[*models.SearchRequest](req)
		if err != nil || !valid {
			template := templates.Notification(models.NewErrorMessage("Unable to process favorite.", "Data is invalid."), 0)
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		name := req.FormValue("search_name")
		// Add the favorite.
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			template := templates.Notification(models.NewErrorMessage("Unable to process favorite.", "User data not found."), 0)
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		err = user.AddFavoriteSearch(name, request)
		if err != nil {
			template := templates.Notification(
				models.NewErrorMessage("Unable to process favorite.", "Temporary backend issue, please try again."), 0)
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		err = a.DataAPI().UpdateUser(req.Context(), user.GetID(), map[string]any{
			"favorites": user.Favorites,
		})
		if err != nil {
			template := templates.Notification(
				models.NewErrorMessage("Unable to process favorite.", "Temporary backend issue, please try again."), 0)
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		// Update the favorite button.
		id := request.ID()
		if id == "" {
			template := templates.Notification(
				models.NewErrorMessage("Unable to process favorite.", "Temporary backend issue, please try again."), 0)
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		fav := user.GetFavorites().Get(id)
		// Update the favorite button and list of favorites.
		template := templ.Join(
			templates.RemoveFavoriteSearchButton(fav.GetID()),
			templates.FavoritesList(user.GetFavorites(), models.OOBSwapTrue),
		)
		renderPartial(template).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// RemoveFavoriteSearch handles removing a favorite article for a user.
func (a *API) RemoveFavoriteSearch() http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Retrieve the search details.
		request, valid, err := forms.DecodeForm[*models.SearchRequest](req)
		if err != nil || !valid {
			template := templates.Notification(models.NewErrorMessage("Unable to process favorite.", "Data is invalid."), 0)
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			template := templates.Notification(models.NewErrorMessage("Unable to process favorite.", "User data not found."), 0)
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		// Derive the favorite id.
		id := request.ID()
		if id == "" {
			template := templates.Notification(
				models.NewErrorMessage("Unable to process favorite.", "Temporary backend issue, please try again."), 0)
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		// Remove the favorite.
		user.RemoveFavorite(id)
		err = a.DataAPI().UpdateUser(req.Context(), user.GetID(), map[string]any{
			"favorites": user.Favorites,
		})
		if err != nil {
			template := templates.Notification(
				models.NewErrorMessage("Unable to process favorite.", "Temporary backend issue, please try again."), 0)
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		// Update the favorite button and list of favorites.
		template := templ.Join(
			templates.AddFavoriteSearchButton(),
			templates.FavoritesList(user.GetFavorites(), models.OOBSwapTrue),
		)
		renderPartial(template).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// DeleteUser handles removing a user account from the local and backend databases. Once the account is removed, any
// active session is destroyed and the browser is redirected back to the landing page.
func (a *API) DeleteUser() http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
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

func AddFeedset(storeAPI *elastic.API, static embed.FS) http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		request, valid, err := forms.DecodeForm[*models.AddFeedsetRequest](req)
		if err != nil || !valid {
			res.Header().Add(htmx.HeaderReswap, "none")
			msg := models.NewErrorMessage("An error occurred reading feed sets.", "Please try again.")
			renderPartial(templates.Notification(msg, 0))
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		slogctx.FromCtx(req.Context()).Debug("Processing feedsets.",
			slog.String("feedsets", strings.Join(request.Feedset, ",")))
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
				msg := models.NewErrorMessage("An error occurred reading feed sets.", "Please try again.")
				renderPartial(templates.Notification(msg, 0)).ServeHTTP(res, req)
				return models.NewAPIError(err, http.StatusUnprocessableEntity)
			}
			opmlImport, err := opml.NewOPMLFromBytes(data)
			if err != nil {
				res.Header().Add(htmx.HeaderReswap, "none")
				msg := models.NewErrorMessage("An error occurred reading feed sets.", "Please try again.")
				renderPartial(templates.Notification(msg, 0)).ServeHTTP(res, req)
				return models.NewAPIError(err, http.StatusUnprocessableEntity)
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
			msg := models.NewErrorMessage(
				"Error processing feed sets.",
				"The backend had issues processing the request and adding subscriptions, please try again.",
			)
			renderPartial(templates.Notification(msg, 0)).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		createResults, err := subscriptionProcessing.createNewSubscriptions(req.Context(), storeAPI)
		if err != nil {
			res.Header().Add(htmx.HeaderReswap, "none")
			msg := models.NewErrorMessage(
				"Error processing feed sets.",
				"The backend had issues processing the request and adding subscriptions, please try again.",
			)
			renderPartial(templates.Notification(msg, 0)).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		maps.Copy(createResults, matchResults)
		msg := models.NewSuccessMessage("Added sets", "Request sets added to your subscriptions.")
		renderPartial(templates.Notification(msg, 0)).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// GetPageIssues handles presenting a form for the user to submit issues about the app.
func GetPageIssues(api *API) http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
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
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Validate the subscription issue request.
		request, valid, err := forms.DecodeForm[*models.IssueRequest](req)
		if err != nil || !valid {
			msg := models.NewErrorMessage(
				"Unable to submit issue.",
				"The backend had issues submitting the report. Please try again.",
			)
			renderPartial(templates.ServerErrorNotification(msg)).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		// Create the issue in Github.
		err = ghapi.CreateIssue(req.Context(), request)
		if err != nil {
			msg := models.NewErrorMessage(
				"Unable to submit issue.",
				"The backend had issues submitting the report. Please try again.",
			)
			template := templ.Join(
				templates.ReportPageIssue(request),
				templates.ServerErrorNotification(msg),
			)
			renderPage(template, templates.GeneratePageTitle("Report subscription issue")).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
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
