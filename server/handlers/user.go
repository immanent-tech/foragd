// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//nolint:dupl // potential future refactoring.
package handlers

import (
	"embed"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-syndication/opml"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/auth0"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/github"
	"github.com/immanent-tech/foragd/providers/stripe"
	"github.com/immanent-tech/foragd/server/forms"
	"github.com/immanent-tech/foragd/server/session"
	"github.com/immanent-tech/foragd/validation"
	"github.com/immanent-tech/foragd/web/templates"
)

// ShowSettings handles retrieving and rendering the user settings page.
func (a *API) ShowSettings() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		renderPage(
			wrapContent(req, templates.SettingsPage()),
			templates.GeneratePageTitle("Settings"),
		).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// ShowDisplaySettings handles showing the settings related to the application display.
func ShowDisplaySettings() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		user := models.UserFromCtx(req.Context())
		if user == nil {
			renderPartial(
				templates.ServerErrorNotification(
					models.NewErrorMessage(
						"Unable to show display settings",
						"This might be a temporary error, please try again.",
					),
				),
			).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("unable to retrieve user data: %w", models.ErrNoUserCtx),
				http.StatusInternalServerError,
			)
		}
		renderPartial(templates.DisplaySettings(user)).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// ShowAccountSettings handles showing the settings related to user accounts.
func ShowAccountSettings() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		user := models.UserFromCtx(req.Context())
		if user == nil {
			renderPartial(
				templates.ServerErrorNotification(
					models.NewErrorMessage(
						"Unable to show account settings",
						"This might be a temporary error, please try again.",
					),
				),
			).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("unable to retrieve user data: %w", models.ErrNoUserCtx),
				http.StatusInternalServerError,
			)
		}
		renderPartial(templates.AccountSettings(user)).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// SaveDisplaySettings handles saving user settings after user submitted changes.
func SaveDisplaySettings(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Decode request.
		request, valid, err := forms.DecodeForm[*models.UserSettings](req)
		if err != nil || !valid {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to save account settings", "Data is invalid."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("save display settings: %w: %w", ErrInvalidRequestParams, err),
				http.StatusUnprocessableEntity,
			)
		}
		// Get user object
		user := models.UserFromCtx(req.Context())
		if user == nil {
			renderPartial(
				templates.ServerErrorNotification(
					models.NewErrorMessage(
						"Unable to save account settings",
						"This might be a temporary error, please try again.",
					),
				),
			).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("save display settings: get user data: %w", models.ErrNoUserCtx),
				http.StatusInternalServerError,
			)
		}
		// Update local user object.
		err = api.UpdateUser(req.Context(), user.GetID(), map[string]any{"settings": request})
		if err != nil {
			renderPartial(
				templates.ServerErrorNotification(
					models.NewErrorMessage(
						"Unable to save account settings",
						"This might be a temporary error, please try again.",
					),
				),
			).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("save account settings: update elastic: %w", err),
				http.StatusInternalServerError,
			)
		}
		// Report success.
		msg := models.NewSuccessMessage("Account edits saved!", "")
		template := templates.Notification(msg, 0)
		renderPartial(template).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// SaveAccountSettings handles processing and saving new account settings.
func SaveAccountSettings(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Decode request.
		request, valid, err := forms.DecodeForm[*models.EditUserRequest](req)
		if err != nil || !valid {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to save account settings", "Data is invalid."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("save account settings: %w: %w", ErrInvalidRequestParams, err),
				http.StatusUnprocessableEntity,
			)
		}
		// Get user object
		user := models.UserFromCtx(req.Context())
		if user == nil {
			renderPartial(
				templates.ServerErrorNotification(
					models.NewErrorMessage(
						"Unable to save account settings",
						"This might be a temporary error, please try again.",
					),
				),
			).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("save account settings: get user data: %w", models.ErrNoUserCtx),
				http.StatusInternalServerError,
			)
		}
		// Create needed updates by comparing request values to existing user values and adding new values to updates map as appropriate.
		updates := make(map[string]any)
		// Overwrite local avatar with remote avatar if different
		if user.AvatarURL != request.AvatarURL {
			updates["avatar_url"] = request.AvatarURL
		}
		// Overwrite local nickname with remote nickname if different
		if user.Nickname != request.Nickname {
			updates["nickname"] = request.Nickname
		}
		// Overwrite local email with remote email if different
		if user.Email != request.Email {
			updates["email"] = request.Email
		}
		// If no updates are necessary, bail early.
		if len(updates) == 0 {
			res.WriteHeader(http.StatusNoContent)
			return nil
		}
		// Update on backend.
		err = auth0.UpdateUser(req.Context(), request)
		if err != nil || !valid {
			renderPartial(
				templates.ServerErrorNotification(
					models.NewErrorMessage(
						"Unable to save account settings",
						"This might be a temporary error, please try again.",
					),
				),
			).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("save account settings: update auth0: %w", err),
				http.StatusInternalServerError,
			)
		}
		// Update local user object.
		err = api.UpdateUser(req.Context(), user.GetID(), updates)
		if err != nil {
			renderPartial(
				templates.ServerErrorNotification(
					models.NewErrorMessage(
						"Unable to save account settings",
						"This might be a temporary error, please try again.",
					),
				),
			).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("save account settings: update elastic: %w", err),
				http.StatusInternalServerError,
			)
		}
		// Report success.
		msg := models.NewSuccessMessage("Account edits saved!", "")
		template := templates.Notification(msg, 0)
		renderPartial(template).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// ChangePassword handles a change password request from the user.
func ChangePassword() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		request, valid, err := forms.DecodeForm[*models.ChangePasswordRequest](req)
		if err != nil || !valid {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to change password", "Data is invalid."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				http.StatusUnprocessableEntity,
			)
		}
		// Update on backend.
		err = auth0.ChangeUserPassword(req.Context(), request)
		if err != nil || !valid {
			renderPartial(
				templates.ServerErrorNotification(
					models.NewErrorMessage(
						"Unable to change password",
						"This might be a temporary error, please try again.",
					),
				),
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
		user := models.UserFromCtx(req.Context())
		if user == nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to set theme", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("unable to retrieve user data: %w", models.ErrNoUserCtx),
				http.StatusInternalServerError,
			)
		}
		settings := user.GetSettings()
		settings.Theme = theme
		if err := a.DataAPI().UpdateUser(req.Context(), user.GetID(), map[string]any{
			"settings": settings,
		}); err != nil {
			renderPartial(
				templates.ServerErrorNotification(
					models.NewErrorMessage(
						"Unable to set theme",
						"This might be a temporary error, please try again.",
					),
				),
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
		if err := validation.Validate.Var(id, "required,startswith=sub_"); err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to add favorite subscription", "Data is invalid."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				http.StatusUnprocessableEntity,
			)
		}
		// Get the subscription state.
		if err := a.Elastic.UpdateFavoriteSubscription(req.Context(), id, true); err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage(
					"Unable to add favorite subscription",
					"This might be a temporary error, please try again.",
				),
			),
			).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("unable to update user object: %w", err),
				http.StatusInternalServerError,
			)
		}
		// Update the display.
		template := templ.Join(
			templates.ToggleFavorite(id, string(models.ObjectTypeSubscription), true),
			templates.Notification(
				models.NewSuccessMessage("Added Favorite", ""),
				templates.DefaultNotificationTimeout,
			),
		)
		renderPartial(template).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// RemoveFavoriteSubscription handles removing a favorite subscription for a user.
func (a *API) RemoveFavoriteSubscription() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		id := chi.URLParam(req, models.ParamSubscriptionID)
		if err := validation.Validate.Var(id, "required,startswith=sub_"); err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to remove favorite subscription", "Data is invalid."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				http.StatusUnprocessableEntity,
			)
		}
		if err := a.Elastic.UpdateFavoriteSubscription(req.Context(), id, false); err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage(
					"Unable to remove favorite subscription",
					"This might be a temporary error, please try again.",
				),
			),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to update user data: %w", err), http.StatusInternalServerError)
		}
		// Update the display as appropriate.
		if currentURL, found := htmx.GetCurrentURL(req); found && strings.Contains(currentURL, "/favorites") {
			// On the favorites page, remove the subscription card when removing it as a favorite.
			res.Header().Add(htmx.HeaderReswap, "delete transition:true")
			res.Header().Set(htmx.HeaderRetarget, "#"+id)
			res.WriteHeader(http.StatusOK)
		} else {
			// Update the favorite button.
			template := templ.Join(
				templates.ToggleFavorite(id, string(models.ObjectTypeSubscription), false),
				templates.Notification(
					models.NewSuccessMessage("Removed Favorite", ""),
					templates.DefaultNotificationTimeout,
				),
			)
			renderPartial(template).ServeHTTP(res, req)
		}
		return nil
	})).ServeHTTP
}

// AddFavoriteArticle handles adding a new favorite article for a user.
func (a *API) AddFavoriteArticle() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		id := chi.URLParam(req, models.ParamItemID)
		if err := validation.Validate.Var(id, "required,startswith=item_"); err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to add favorite article", "Data is invalid."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				http.StatusUnprocessableEntity,
			)
		}
		user := models.UserFromCtx(req.Context())
		if user == nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage(
					"Unable to add favorite article",
					"This might be a temporary error, please try again.",
				),
			),
			).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("unable to retrieve user data: %w", models.ErrNoUserCtx),
				http.StatusInternalServerError,
			)
		}
		if err := a.Elastic.UpdateFavoriteArticle(req.Context(), user, id, true); err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage(
					"Unable to add favorite article",
					"This might be a temporary error, please try again.",
				),
			),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to archive article: %w", err), http.StatusInternalServerError)
		}
		// Get the display type.
		display := req.FormValue("display")
		// Update the content as appropriate.
		var template templ.Component
		switch display {
		case "card":
			template = templates.ToggleFavorite(id, string(models.ObjectTypeArticle), true)
		case "content":
			template = templates.UpdateViewArticleFavorite(id, true)
		}
		template = templ.Join(
			template,
			templates.Notification(
				models.NewSuccessMessage("Added Favorite", ""),
				templates.DefaultNotificationTimeout,
			),
		)
		renderPartial(template).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// RemoveFavoriteArticle handles removing a favorite article for a user.
func (a *API) RemoveFavoriteArticle() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		id := chi.URLParam(req, models.ParamItemID)
		if err := validation.Validate.Var(id, "required,startswith=item_"); err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to remove favorite article", "Data is invalid."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				http.StatusUnprocessableEntity,
			)
		}
		user := models.UserFromCtx(req.Context())
		if user == nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage(
					"Unable to remove favorite article",
					"This might be a temporary error, please try again.",
				),
			),
			).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("unable to retrieve user data: %w", models.ErrNoUserCtx),
				http.StatusInternalServerError,
			)
		}
		if err := a.Elastic.UpdateFavoriteArticle(req.Context(), user, id, false); err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage(
					"Unable to remove favorite article",
					"This might be a temporary error, please try again.",
				),
			),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to archive article: %w", err), http.StatusInternalServerError)
		}

		// Update the content as appropriate.
		var template templ.Component
		switch req.FormValue("display") {
		case "card":
			template = templates.ToggleFavorite(id, string(models.ObjectTypeArticle), false)
		case "content":
			template = templates.UpdateViewArticleFavorite(id, false)
		}
		template = templ.Join(
			template,
			templates.Notification(
				models.NewSuccessMessage("Removed Favorite", ""),
				templates.DefaultNotificationTimeout,
			),
		)
		renderPartial(template).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// UserDeactivateAccount handles a user request to deactivate their account. Their subscription in Stripe will be cancelled at
// the end of the current billing period. They can continue to log in and use the service during the current billing
// period, after which a scheduled job will delete their account.
func UserDeactivateAccount() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		switch req.Method {
		case http.MethodGet:
			renderPartial(templates.DeactivateAccountModal()).ServeHTTP(res, req)
		case http.MethodPost:
			// Get user account details.
			user := models.UserFromCtx(req.Context())
			if user == nil {
				renderPartial(templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to delete account", ""),
				),
				).ServeHTTP(res, req)
				return models.NewAPIError(
					fmt.Errorf("unable to retrieve user data: %w", models.ErrNoUserCtx),
					http.StatusInternalServerError,
				)
			}
			// Delete Stripe subscription.
			if err := stripe.CancelSubscription(user); err != nil {
				renderPartial(templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to delete account", ""),
				),
				).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("unable to delete user: %w", err), http.StatusInternalServerError)
			}
			// Refresh the page
			res.Header().Set(htmx.HeaderRefresh, "true")
			renderPartial(
				templates.Notification(
					models.NewInfoMessage("Account cancelled", ""),
					templates.DefaultNotificationTimeout,
				),
			).ServeHTTP(res, req)
		}
		return nil
	})).ServeHTTP
}

// UserCancelDeactivation handles a user request to stop the pending deactivation of their account. The cancellation
// will be reversed in Stripe and full account functionality restored.
func UserCancelDeactivation() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Get user account details.
		user := models.UserFromCtx(req.Context())
		if user == nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to stop account deactivation", ""),
			),
			).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("unable to retrieve user data: %w", models.ErrNoUserCtx),
				http.StatusInternalServerError,
			)
		}
		// Delete Stripe subscription.
		if err := stripe.StopPendingCancellation(user); err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to stop account deactivation", ""),
			),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to delete user: %w", err), http.StatusInternalServerError)
		}
		// Refresh the page
		res.Header().Set(htmx.HeaderRefresh, "true")
		renderPartial(
			templates.Notification(
				models.NewSuccessMessage("Stopped account deactivation", ""),
				templates.DefaultNotificationTimeout,
			),
		).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// AddFeedset handles adding a feedset as subscriptions.
func AddFeedset(api *elastic.API, static embed.FS) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Ignore submission without any feedset selected.
		if req.FormValue("feedset") == "" {
			res.WriteHeader(http.StatusNoContent)
			return nil
		}
		request, valid, err := forms.DecodeForm[*models.AddFeedsetRequest](req)
		if err != nil || !valid {
			res.Header().Add(htmx.HeaderReswap, "none")
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to add feedset", "Data is invalid."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				http.StatusUnprocessableEntity,
			)
		}
		// Process requested feedsets and generate subscription requests.
		var subscriptionRequests []*models.AddFeedSubscriptionRequest
		for set := range slices.Values(request.Feedset) {
			var data []byte
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
					models.NewErrorMessage(
						"Unable to add feedset",
						"This might be a temporary issue, please try again.",
					),
				)).ServeHTTP(res, req)
				return models.NewAPIError(
					fmt.Errorf("unable to read feedset file: %w", err),
					http.StatusInternalServerError,
				)
			}
			opmlImport, err := opml.NewOPMLFromBytes(data)
			if err != nil {
				res.Header().Add(htmx.HeaderReswap, "none")
				renderPartial(templates.ServerErrorNotification(
					models.NewErrorMessage(
						"Unable to add feedset",
						"This might be a temporary issue, please try again.",
					),
				)).ServeHTTP(res, req)
				return models.NewAPIError(
					fmt.Errorf("to generate OPML import: %w", err),
					http.StatusInternalServerError,
				)
			}
			subscriptionRequests = append(
				subscriptionRequests,
				models.GenerateRequestsFromOutlines(opmlImport.Body...)...)
		}
		// Process requests.
		resultsCh := make(chan models.AddFeedSubscriptionResult)
		var wg sync.WaitGroup
		for request := range slices.Values(subscriptionRequests) {
			wg.Go(func() {
				api.ProcessSubscriptionRequest(req.Context(), request, resultsCh)
			})
		}
		// Wait for all request processing to complete.
		go func() {
			defer close(resultsCh)
			wg.Wait()
		}()
		// Process results
		for result := range resultsCh {
			if result.Error != nil {
				switch result.Message.Status {
				case models.UserMessageStatusError:
					slogctx.FromCtx(req.Context()).Error("Error occurred during subscription request processing.",
						slog.String("url", result.Request.GetURL()),
						slog.Any("error", result.Error),
					)
				case models.UserMessageStatusWarning:
					fallthrough
				default:
					slogctx.FromCtx(req.Context()).Warn("Warning occurred during subscription request processing.",
						slog.String("url", result.Request.GetURL()),
						slog.Any("error", result.Error),
					)
				}
			} else {
				err = api.CreateFeedSubscriptions(req.Context(), &result)
				if err != nil {
					msg := models.NewErrorMessage("Failed to create subscription.", "The backend produced an error. This might be temporary, please try again.")
					renderPartial(templates.ServerErrorNotification(msg)).ServeHTTP(res, req)
					return models.NewAPIError(fmt.Errorf("unable process import request: %w", err), http.StatusInternalServerError)
				}
			}
		}
		renderPartial(templates.AddFeedsetsSuccessNotification(request.Feedset)).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// GetPageIssues handles presenting a form for the user to submit issues about the app.
func GetPageIssues() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Get the current URL on which the issue is being reported.
		currentURL, found := htmx.GetCurrentURL(req)
		if !found {
			slogctx.FromCtx(req.Context()).Warn("No HX-Current-URL header found.")
		}
		// Display the report issue form.
		template := templates.ReportPageIssue(&models.IssueRequest{PageUrl: currentURL})
		renderPage(
			wrapContent(req, template),
			templates.GeneratePageTitle("Report subscription issue"),
		).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// SubmitPageIssues handles processing the user submitted subscription issues form.
func SubmitPageIssues() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Validate the subscription issue request.
		request, valid, err := forms.DecodeForm[*models.IssueRequest](req)
		if err != nil || !valid {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to submit issue", "Data is invalid."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				http.StatusUnprocessableEntity,
			)
		}
		// Create the issue in Github.
		err = github.Connect()
		if err != nil {
			res.Header().Add(htmx.HeaderReswap, "none")
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to submit issue", "This might be a temporary issue, please try again."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("unable to connect to github: %w", err),
				http.StatusInternalServerError,
			)
		}
		err = github.CreateIssue(req.Context(), request)
		if err != nil {
			res.Header().Add(htmx.HeaderReswap, "none")
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to submit issue", "This might be a temporary issue, please try again."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				http.StatusUnprocessableEntity,
			)
		}
		// Force refresh of page.
		msg := models.NewErrorMessage(
			"Thanks for reporting the issue!",
			"We will look into it and implement fixes as appropriate.",
		)
		renderPage(
			wrapContent(req, templates.IssueReportedConfirmation(msg)),
			templates.GeneratePageTitle("Report subscription issue"),
		).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// UserChooseSubscriptionPlan handles displaying a page on which the user can choose a subscription plan for purchase.
func UserChooseSubscriptionPlan() http.HandlerFunc {
	return alice.New().ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		// Try to find a selected plan id if it exists, from either the request query params or current session data.
		var planID string
		switch {
		case req.URL.Query().Get(models.ParamPlanID) != "":
			planID = req.URL.Query().Get(models.ParamPlanID)
		case session.RestoreFromSession(req.Context(), models.ParamPlanID, func() string { return "" }) != "":
			planID = session.RestoreFromSession(req.Context(), models.ParamPlanID, func() string { return "" })
		}
		slogctx.FromCtx(req.Context()).Debug("Presenting user with subscription plan options.")
		renderPage(templates.UserChooseSubscriptionPlan(planID), "Choose a Subscription Plan").ServeHTTP(res, req)
	}).ServeHTTP
}

// UserSubscriptionPlanCheckout handles processing the user's choice of subscription plan and redirecting to the payment
// processor.
func UserSubscriptionPlanCheckout() http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Fetch the user details from context.
		user := models.UserFromCtx(req.Context())
		if user == nil {
			externalPage("Checkout Subscription Plan - "+config.AppName,
				templates.ExternalError(models.NewErrorMessage(
					"Unable to process checkout",
					"This might be a temporary error, please try again.",
				)),
			).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf(
					"user checkout session: unable to retrieve user: %w",
					models.ErrNoUserCtx,
				),
				http.StatusInternalServerError,
			)
		}

		// Retrieve the plan id from the session data.
		planID := req.FormValue(models.ParamPlanID)
		if planID == "" {
			externalPage("Checkout Subscription Plan - "+config.AppName,
				templates.ExternalError(models.NewErrorMessage(
					"Unable to process checkout",
					"This might be a temporary error, please try again.",
				)),
			).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf(
					"user checkout session: unable to retrieve plan id from session: %w",
					ErrInvalidRequestParams,
				),
				http.StatusInternalServerError,
			)
		}

		// Create a new strip checkout session.
		var session *stripe.Checkout
		session, err := stripe.NewCheckoutSession(user, planID)
		if err != nil {
			externalPage("Checkout Subscription Plan - "+config.AppName,
				templates.ExternalError(models.NewErrorMessage(
					"Unable to process checkout",
					"This might be a temporary error, please try again.",
				)),
			).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("user account checkout: %w", err),
				http.StatusInternalServerError,
			)
		}

		// Redirect to strip processor to complete checkout session.
		slogctx.FromCtx(req.Context()).Debug("Redirecting user to Stripe for payment.")
		http.Redirect(res, req, session.URL, http.StatusSeeOther)
		return nil
	})).ServeHTTP
}

func UserAccountSuccess() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// stripeSessionID := req.FormValue("session_id")
		http.Redirect(res, req, "/home", http.StatusTemporaryRedirect)
		return nil
	})).ServeHTTP
}

func UserAccountCancel() http.HandlerFunc {
	return Landing()
}

// UserAccountIssue handles showing a page with a message indicating the user needs to contact support, as there is a
// critical issue with their account blocking access to the service.
func UserAccountIssue() http.HandlerFunc {
	return renderPage(templates.UserAccountIssue(), "Account Issue").ServeHTTP
}

func UserManageAccountSubscription() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {

		sessionID := req.FormValue("session_id")
		if sessionID == "" {
			renderPage(wrapContent(req, templates.ExternalError(models.NewErrorMessage(
				"Unable to process checkout",
				"This might be a temporary error, please try again.",
			))), "Subscription Plan Checkout").ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("user manage subscription: %w: no stripe session ID", stripe.ErrInvalidSubscription),
				http.StatusInternalServerError,
			)
		}

		portalSession, err := stripe.NewPortalSession(sessionID)
		if err != nil {
			renderPage(wrapContent(req, templates.ExternalError(models.NewErrorMessage(
				"Unable to process checkout",
				"This might be a temporary error, please try again.",
			))), "Subscription Plan Checkout").ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("user account checkout: %w", err),
				http.StatusInternalServerError,
			)
		}

		// Redirect to payment processor to complete checkout.
		http.Redirect(res, req, portalSession.URL, http.StatusSeeOther)
		return nil
	})).ServeHTTP
}
