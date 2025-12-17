// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

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
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-syndication/opml"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/auth0"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/stripe"
	"github.com/immanent-tech/foragd/server/forms"
	"github.com/immanent-tech/foragd/server/session"
	"github.com/immanent-tech/foragd/validation"
	"github.com/immanent-tech/foragd/web/templates"
)

// ShowSettings handles retrieving and rendering the user settings page.
func ShowSettings() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		ctx := templates.PageTitleToCtx(req.Context(), "Settings")
		renderPage(
			wrapContent(req.WithContext(ctx), templates.SettingsPage()),
		).ServeHTTP(res, req.WithContext(ctx))
	}).ServeHTTP
}

// ShowDisplaySettings handles showing the settings related to the application display.
func ShowDisplaySettings() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("get user data: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to show display settings",
					"This might be a temporary error, please try again.",
				),
			}
		}
		renderPartial(templates.DisplaySettings(user)).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// ShowAccountSettings handles showing the settings related to user accounts.
func ShowAccountSettings() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("get user data: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to show display settings",
					"This might be a temporary error, please try again.",
				),
			}
		}
		renderPartial(templates.AccountSettings(user)).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// SaveDisplaySettings handles saving user settings after user submitted changes.
func SaveDisplaySettings(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
		// Decode request.
		request, valid, err := forms.DecodeForm[*models.UserSettings](req)
		if err != nil || !valid {
			return &models.APIError{
				InternalError: fmt.Errorf("decode user settings: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to save settings",
					"This might be a temporary error, please try again.",
				),
			}
		}
		// Get user object
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("get user data: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to save settings",
					"This might be a temporary error, please try again.",
				),
			}
		}
		// Update local user object.
		err = api.UpdateUser(req.Context(), user.GetID(), map[string]any{"settings": request})
		if err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("update data: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to save settings",
					"This might be a temporary error, please try again.",
				),
			}
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
	return defaultHandlerChain.ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
		// Decode request.
		request, valid, err := forms.DecodeForm[*models.EditUserRequest](req)
		if err != nil || !valid {
			return &models.APIError{
				InternalError: fmt.Errorf("decode edit user request: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to save settings",
					"This might be a temporary error, please try again.",
				),
			}
		}
		// Get user object
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("get user data: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to save settings",
					"This might be a temporary error, please try again.",
				),
			}
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
			return &models.APIError{
				InternalError: fmt.Errorf("update user in auth0: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to save settings",
					"This might be a temporary error, please try again.",
				),
			}
		}
		// Update local user object.
		err = api.UpdateUser(req.Context(), user.GetID(), updates)
		if err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("update local user: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to save settings",
					"This might be a temporary error, please try again.",
				),
			}
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
	return defaultHandlerChain.ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
		request, valid, err := forms.DecodeForm[*models.ChangePasswordRequest](req)
		if err != nil || !valid {
			return &models.APIError{
				InternalError: fmt.Errorf("decode change password request: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to change password",
					"This might be a temporary error, please try again.",
				),
			}
		}
		// Update on backend.
		err = auth0.ChangeUserPassword(req.Context(), request)
		if err != nil || !valid {
			return &models.APIError{
				InternalError: fmt.Errorf("change password in auth0: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to change password",
					"This might be a temporary error, please try again.",
				),
			}
		}
		// Report success.
		msg := models.NewSuccessMessage("Password changed!", "Logout and log back in to use the new password.")
		template := templates.Notification(msg, 0)
		renderPartial(template).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// SetTheme handles setting a theme selected by the user.
func SetTheme(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
		theme := chi.URLParam(req, "theme")
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("get user data: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to set theme",
					"This might be a temporary error, please try again.",
				),
			}
		}
		settings := user.GetSettings()
		settings.Theme = theme
		if err := api.UpdateUser(req.Context(), user.GetID(), map[string]any{
			"settings": settings,
		}); err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("update user: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to set theme",
					"This might be a temporary error, please try again.",
				),
			}
		}
		renderPartial(templates.DisplaySettings(user)).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// AddFavoriteSubscription handles adding a new favorite subscription for a user.
func AddFavoriteSubscription(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
		id := chi.URLParam(req, models.ParamSubscriptionID)
		if err := validation.Validate.Var(id, "required,startswith=sub_"); err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("decode subscription: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to add favorite subscription",
					"This might be a temporary error, please try again.",
				),
			}
		}
		// Get the subscription state.
		if err := api.UpdateFavoriteSubscription(req.Context(), id, true); err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("update favorite subscription: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to add favorite subscription",
					"This might be a temporary error, please try again.",
				),
			}
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
func RemoveFavoriteSubscription(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
		id := chi.URLParam(req, models.ParamSubscriptionID)
		if err := validation.Validate.Var(id, "required,startswith=sub_"); err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("decode subscription: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to remove favorite subscription",
					"This might be a temporary error, please try again.",
				),
			}
		}
		if err := api.UpdateFavoriteSubscription(req.Context(), id, false); err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("remove favorite subscription: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to remove favorite subscription",
					"This might be a temporary error, please try again.",
				),
			}
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
func AddFavoriteArticle(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
		id := chi.URLParam(req, models.ParamItemID)
		if err := validation.Validate.Var(id, "required,startswith=item_"); err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("decode article: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to add favorite article",
					"This might be a temporary error, please try again.",
				),
			}
		}
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("get user data: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to add favorite article",
					"This might be a temporary error, please try again.",
				),
			}
		}
		if err := api.UpdateFavoriteArticle(req.Context(), user, id, true); err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("update favorite article: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to add favorite article",
					"This might be a temporary error, please try again.",
				),
			}
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
func RemoveFavoriteArticle(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
		id := chi.URLParam(req, models.ParamItemID)
		if err := validation.Validate.Var(id, "required,startswith=item_"); err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("decode article: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to remove favorite article",
					"This might be a temporary error, please try again.",
				),
			}
		}
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("get user data: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to remove favorite article",
					"This might be a temporary error, please try again.",
				),
			}
		}
		if err := api.UpdateFavoriteArticle(req.Context(), user, id, false); err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("update favorite article: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to remove favorite article",
					"This might be a temporary error, please try again.",
				),
			}
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
	return defaultHandlerChain.ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
		switch req.Method {
		case http.MethodGet:
			renderPartial(templates.DeactivateAccountModal()).ServeHTTP(res, req)
		case http.MethodPost:
			// Get user account details.
			user, err := models.UserFromCtx(req.Context())
			if err != nil {
				return &models.APIError{
					InternalError: fmt.Errorf("get user data: %w", err),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Unable to deactivate account",
						"This might be a temporary error, please try again.",
					),
				}
			}
			// Delete Stripe subscription.
			if err := stripe.CancelSubscription(user); err != nil {
				return &models.APIError{
					InternalError: fmt.Errorf("cancel subscription: %w", err),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Unable to deactivate account",
						"This might be a temporary error, please try again.",
					),
				}
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
	return defaultHandlerChain.ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
		// Get user account details.
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("get user data: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to reactivate account",
					"This might be a temporary error, please try again.",
				),
			}
		}
		// Delete Stripe subscription.
		if err := stripe.StopPendingCancellation(user); err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("cancel pending cancellation: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to reactivate account",
					"This might be a temporary error, please try again.",
				),
			}
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
	return defaultHandlerChain.ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
		// Ignore submission without any feedset selected.
		if req.FormValue("feedset") == "" {
			res.WriteHeader(http.StatusNoContent)
			return nil
		}
		request, valid, err := forms.DecodeForm[*models.AddFeedsetRequest](req)
		if err != nil || !valid {
			return &models.APIError{
				InternalError: fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage:   models.NewErrorMessage("Unable to add feedset", "Data is invalid."),
			}
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
				return &models.APIError{
					InternalError: fmt.Errorf("read feedset: %w", err),
					StatusCode:    http.StatusUnprocessableEntity,
					UserMessage: models.NewErrorMessage(
						"Unable to add feedset",
						"This might be a temporary issue, please try again.",
					),
				}
			}
			opmlImport, err := opml.NewOPMLFromBytes(data)
			if err != nil {
				return &models.APIError{
					InternalError: fmt.Errorf("create opml: %w", err),
					StatusCode:    http.StatusUnprocessableEntity,
					UserMessage: models.NewErrorMessage(
						"Unable to add feedset",
						"This might be a temporary issue, please try again.",
					),
				}
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

// UserChooseSubscriptionPlan handles displaying a page on which the user can choose a subscription plan for purchase.
func UserChooseSubscriptionPlan() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Unable to get user data.",
				slog.Any("error", err),
			)
			renderPage(templates.ExternalError(models.NewErrorMessage(
				"Unable to process checkout",
				"This might be a temporary error, please try again.",
			))).ServeHTTP(res, req)
			return
		}
		// Try to find a selected plan id if it exists, from either the request query params or current session data.
		var planID string
		if req.URL.Query().Get(models.ParamPlanID) != "" {
			planID = req.URL.Query().Get(models.ParamPlanID)
		} else if p, err := session.Restore[string](req.Context(), models.ParamPlanID); err != nil {
			planID = p
		} else {
			slogctx.FromCtx(req.Context()).Error("Unable to process checkout.",
				slog.Any("error", err),
			)
			renderPage(templates.ExternalError(models.NewErrorMessage(
				"Unable to process checkout",
				"This might be a temporary error, please try again.",
			))).ServeHTTP(res, req)
			return
		}
		slogctx.FromCtx(req.Context()).Debug("Presenting user with subscription plan options.")
		ctx := templates.PageTitleToCtx(req.Context(), "Choose a Subscription Plan")
		renderPage(templates.UserChooseSubscriptionPlan(user, planID)).ServeHTTP(res, req.WithContext(ctx))
	}
}

// UserSubscriptionPlanCheckout handles processing the user's choice of subscription plan and redirecting to the payment
// processor.
func UserSubscriptionPlanCheckout() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		ctx := templates.PageTitleToCtx(req.Context(), "Checkout Subscription Plan")
		// Fetch the user details from context.
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Unable to get user data.",
				slog.Any("error", err),
			)
			renderPage(templates.ExternalError(models.NewErrorMessage(
				"Unable to process checkout",
				"This might be a temporary error, please try again.",
			))).ServeHTTP(res, req)
			return
		}

		// Retrieve the plan id from the session data.
		planID := req.FormValue(models.ParamPlanID)
		if planID == "" {
			slogctx.FromCtx(req.Context()).Error("User checkout session: unable to retrieve plan id from session.",
				slog.Any("error", ErrInvalidRequestParams),
			)
			renderPage(templates.ExternalError(models.NewErrorMessage(
				"Unable to process checkout",
				"This might be a temporary error, please try again.",
			))).ServeHTTP(res, req)
			return
		}

		// Create a new strip checkout session.
		var session *stripe.Checkout
		session, err = stripe.NewCheckoutSession(user, planID)
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Unable to create new checkout session.",
				slog.Any("error", err),
			)
			renderPage(templates.ExternalError(models.NewErrorMessage(
				"Unable to process checkout",
				"This might be a temporary error, please try again.",
			))).ServeHTTP(res, req)
			return
		}

		// Redirect to strip processor to complete checkout session.
		slogctx.FromCtx(ctx).Debug("Redirecting user to Stripe for payment.")
		http.Redirect(res, req.WithContext(ctx), session.URL, http.StatusSeeOther)
	}
}

func UserAccountSuccess() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// stripeSessionID := req.FormValue("session_id")
		http.Redirect(res, req, "/home", http.StatusTemporaryRedirect)
	}
}

func UserAccountCancel() http.HandlerFunc {
	return Landing()
}

// UserAccountIssue handles showing a page with a message indicating the user needs to contact support, as there is a
// critical issue with their account blocking access to the service.
func UserAccountIssue() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		ctx := templates.PageTitleToCtx(req.Context(), "Account Issue")
		// stripeSessionID := req.FormValue("session_id")
		renderPage(templates.UserAccountIssue()).ServeHTTP(res, req.WithContext(ctx))
	}
}

func UserManageAccountSubscription() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		ctx := templates.PageTitleToCtx(req.Context(), "Subscription Plan Checkout")

		sessionID := req.FormValue("session_id")
		if sessionID == "" {
			slogctx.FromCtx(req.Context()).Error("Unable to manage subscription",
				slog.Any("error", stripe.ErrInvalidSubscription),
			)
			renderPage(wrapContent(req.WithContext(ctx), templates.ExternalError(models.NewErrorMessage(
				"Unable to process checkout",
				"This might be a temporary error, please try again.",
			)))).ServeHTTP(res, req.WithContext(ctx))
			return
		}

		portalSession, err := stripe.NewPortalSession(sessionID)
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Unable to create new portal session.",
				slog.Any("error", err),
			)
			renderPage(wrapContent(req.WithContext(ctx), templates.ExternalError(models.NewErrorMessage(
				"Unable to process checkout",
				"This might be a temporary error, please try again.",
			)))).ServeHTTP(res, req.WithContext(ctx))
			return
		}

		// Redirect to payment processor to complete checkout.
		http.Redirect(res, req, portalSession.URL, http.StatusSeeOther)
	}).ServeHTTP
}
