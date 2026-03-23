// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"embed"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strconv"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"
	slogctx "github.com/veqryn/slog-context"
	"github.com/zeebo/xxh3"

	"github.com/immanent-tech/go-syndication/opml"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/auth0"
	"github.com/immanent-tech/foragd/providers/stripe"
	"github.com/immanent-tech/foragd/server/forms"
	"github.com/immanent-tech/foragd/server/session"
	"github.com/immanent-tech/foragd/web/templates"
)

// UserSettings contains the data for rendering the user settings page.
type UserSettings struct{}

// FullResponse renders a full page (headers, footers and content).
func (t *UserSettings) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(
		templates.CreatePage(templates.UserSettings(),
			templates.WithPageTitle("Settings"),
		)).ServeHTTP(res, req)
}

// PartialResponse renders just the content and performs OOB swaps to update the title (if set) and sidebar/dock.
func (t *UserSettings) PartialResponse(res http.ResponseWriter, req *http.Request) {
	res.Header().Set(htmx.HeaderPushURL, req.URL.String())
	templ.Handler(templates.UserSettings(), templ.WithFragments(templates.ContentFragment)).ServeHTTP(res, req)
	templ.Handler(templates.SideBar(templ.Attributes{"hx-swap-oob": "true"})).ServeHTTP(res, req)
	templ.Handler(templates.Dock(templ.Attributes{"hx-swap-oob": "true"})).ServeHTTP(res, req)
	templ.Handler(templates.UpdateTitle("Settings")).ServeHTTP(res, req)
}

// ShowSettings handles retrieving and rendering the user settings page.
func ShowSettings() http.HandlerFunc {
	return userContentHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		RenderInternalPage(&UserSettings{}).ServeHTTP(res, req)
	}).ServeHTTP
}

// HandleShowDisplaySettings handles showing the settings related to the application display.
func HandleShowDisplaySettings() http.HandlerFunc {
	return userContentHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		user := models.UserFromCtx(req.Context())
		if user == nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to show display settings",
					"This might be a temporary error, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}
		RenderPartial(&PartialTemplate{
			template: templates.DisplaySettings(user),
		}).ServeHTTP(res, req)
	}).ServeHTTP
}

// HandleShowAccountSettings handles showing the settings related to user accounts.
func HandleShowAccountSettings() http.HandlerFunc {
	return userContentHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		user := models.UserFromCtx(req.Context())
		if user == nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to show account settings",
					"This might be a temporary error, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}
		RenderPartial(&PartialTemplate{
			template: templates.AccountSettings(user),
		}).ServeHTTP(res, req)
	}).ServeHTTP
}

// HandleSaveDisplaySettings handles saving user settings after user submitted changes.
func HandleSaveDisplaySettings() http.HandlerFunc {
	return userContentHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		// Decode request.
		request, valid, err := forms.DecodeForm[*models.UserSettings](req)
		if err != nil || !valid {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("decode user settings: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to save settings",
					"This might be a temporary error, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}

		// Get user object
		user := models.UserFromCtx(req.Context())
		if user == nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to save settings",
					"This might be a temporary error, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}
		// Update local user object.
		err = models.UpdateUser(req.Context(), user.GetID(), map[string]any{"settings": request})
		if err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("update data: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to save settings",
					"This might be a temporary error, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}
		// Report success.
		RenderPartial(&Notification{
			msg: models.NewSuccessMessage("Account edits saved!", ""),
		}).ServeHTTP(res, req)
	}).ServeHTTP
}

// HandleSaveAccountSettings handles processing and saving new account settings.
//
//nolint:funlen
func HandleSaveAccountSettings() http.HandlerFunc {
	return userContentHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		// Decode request.
		request, valid, err := forms.DecodeMultiPartForm[*models.EditUserRequest](req)
		if err != nil || !valid {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("decode edit user request: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to save settings",
					"One or more of the inputs is invalid. Please check and try again.",
				),
			}).ServeHTTP(res, req)
			return
		}

		avatar, err := forms.DecodeMultipartFile(req, "avatar")
		if err != nil && !errors.Is(err, http.ErrMissingFile) {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("decode avatar: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to save settings",
					"Unable to read uploaded avatar data. Please check the file and try again.",
				),
			}).ServeHTTP(res, req)
			return
		}
		// Maximum size of an avatar image is 1MB.
		const maxAvatarSizeBytes = 1000000
		if avatar.GetSize() > maxAvatarSizeBytes {
			HandleInternalError(&models.APIError{
				InternalError: models.ErrFileTooLarge,
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to save settings",
					"Uploaded avatar image is too large (> 1MB).",
				),
			}).ServeHTTP(res, req)
			return
		}

		// Get user object
		user := models.UserFromCtx(req.Context())
		if user == nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to save settings",
					"This might be a temporary error, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}

		// If the user uploaded a new avatar, process it.
		if avatar != nil {
			if err := loadAvatarCache(); err != nil {
				HandleInternalError(&models.APIError{
					InternalError: fmt.Errorf("load server cache: %w", err),
					StatusCode:    http.StatusUnprocessableEntity,
					UserMessage: models.NewErrorMessage(
						"Unable to save settings",
						"This might be a temporary error, please try again.",
					),
				}).ServeHTTP(res, req)
				return
			}
			// Generate a unique ID for the avatar image in the cache using the user ID.
			avatarFileID := strconv.FormatUint(xxh3.Hash([]byte(user.GetID()+"avatar")), 10)
			// Read the uploaded data and store in the cache.
			avatarData, err := io.ReadAll(avatar.Data)
			if err != nil {
				HandleInternalError(&models.APIError{
					InternalError: fmt.Errorf("read avatar: %w", err),
					StatusCode:    http.StatusUnprocessableEntity,
					UserMessage: models.NewErrorMessage(
						"Unable to save settings",
						"This might be a temporary error, please try again.",
					),
				}).ServeHTTP(res, req)
				return
			}
			avatarCache.Set(req.Context(), avatarFileID, avatarData)
			// Construct a new full URL to the uploaded avatar on the local server.
			baseURL := os.Getenv("FORAGD_BASEURL")
			request.AvatarURL = new(baseURL + "/img/avatar/" + avatarFileID)
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
			return
		}
		// Update on backend.
		err = auth0.UpdateUserCustomisation(req.Context(), request)
		if err != nil || !valid {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("update user in auth0: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to save settings",
					"This might be a temporary error, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}
		// Update local user object.
		err = models.UpdateUser(req.Context(), user.GetID(), updates)
		if err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("update local user: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to save settings",
					"This might be a temporary error, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}
		// Report success.
		RenderPartial(&Notification{
			msg: models.NewSuccessMessage("Account edits saved!", ""),
		}).ServeHTTP(res, req)
		// Update the avatar (in case it changed).
		RenderPartial(&PartialTemplate{
			template: templates.UserAvatar(user, templ.Attributes{"hx-swap-oob": "true"}),
		}).ServeHTTP(res, req)
	}).ServeHTTP
}

// HandleChangePassword handles a change password request from the user.
func HandleChangePassword() http.HandlerFunc {
	return userContentHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		request, valid, err := forms.DecodeForm[*models.ChangePasswordRequest](req)
		if err != nil || !valid {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("decode change password request: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to change password",
					"This might be a temporary error, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}
		// Update on backend.
		err = auth0.ChangeUserPassword(req.Context(), request)
		if err != nil || !valid {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("change password in auth0: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to change password",
					"This might be a temporary error, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}
		// Report success.
		RenderPartial(&Notification{
			msg: models.NewSuccessMessage("Password changed!", "Logout and log back in to use the new password."),
		}).ServeHTTP(res, req)
	}).ServeHTTP
}

// HandleSetTheme handles setting a theme selected by the user.
func HandleSetTheme() http.HandlerFunc {
	return userContentHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		theme := chi.URLParam(req, "theme")
		user := models.UserFromCtx(req.Context())
		if user == nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to set theme",
					"This might be a temporary error, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}
		settings := user.GetSettings()
		settings.Theme = theme
		if err := models.UpdateUser(req.Context(), user.GetID(), map[string]any{
			"settings": settings,
		}); err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("update user: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to set theme",
					"This might be a temporary error, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}
		RenderPartial(&PartialTemplate{
			template: templates.DisplaySettings(user),
		}).ServeHTTP(res, req)
	}).ServeHTTP
}

// HandleDeactivateAccount handles a user request to deactivate their account. Their subscription in Stripe will be cancelled at
// the end of the current billing period. They can continue to log in and use the service during the current billing
// period, after which a scheduled job will delete their account.
func HandleDeactivateAccount() http.HandlerFunc {
	return userContentHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			RenderPartial(&Modal{
				template: templates.DeactivateAccountModal(),
			}).ServeHTTP(res, req)
		case http.MethodPost:
			// Get user account details.
			user := models.UserFromCtx(req.Context())
			if user == nil {
				HandleInternalError(&models.APIError{
					InternalError: fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Unable to deactivate account",
						"This might be a temporary error, please try again.",
					),
				}).ServeHTTP(res, req)
				return
			}
			// Delete Stripe subscription.
			if err := stripe.CancelSubscription(user); err != nil {
				HandleInternalError(&models.APIError{
					InternalError: fmt.Errorf("cancel subscription: %w", err),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Unable to deactivate account",
						"This might be a temporary error, please try again.",
					),
				}).ServeHTTP(res, req)
				return
			}
			// Show notification.
			RenderPartial(&Notification{
				msg: models.NewInfoMessage("Account cancelled", ""),
			}).ServeHTTP(res, req)
			// Refresh the page
			res.Header().Set(htmx.HeaderRefresh, "true")
		}
	}).ServeHTTP
}

// HandleCancelDeactivation handles a user request to stop the pending deactivation of their account. The cancellation
// will be reversed in Stripe and full account functionality restored.
func HandleCancelDeactivation() http.HandlerFunc {
	return userContentHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		// Get user account details.
		user := models.UserFromCtx(req.Context())
		if user == nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to reactivate account",
					"This might be a temporary error, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}
		// Delete Stripe subscription.
		if err := stripe.StopPendingCancellation(user); err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("cancel pending cancellation: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to reactivate account",
					"This might be a temporary error, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}
		RenderPartial(&Notification{
			msg: models.NewSuccessMessage("Stopped account deactivation", ""),
		}).ServeHTTP(res, req)
		// Refresh the page
		res.Header().Set(htmx.HeaderRefresh, "true")
	}).ServeHTTP
}

// HandleAddFeedset handles adding a feedset as subscriptions.
func HandleAddFeedset(static embed.FS) http.HandlerFunc {
	return userContentHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		// Ignore submission without any feedset selected.
		if req.FormValue("feedset") == "" {
			res.WriteHeader(http.StatusNoContent)
			return
		}
		request, valid, err := forms.DecodeForm[*models.AddFeedsetRequest](req)
		if err != nil || !valid {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage:   models.NewErrorMessage("Unable to add feedset", "Data is invalid."),
			}).ServeHTTP(res, req)
			return
		}
		// Process requested feedsets and generate subscription requests.
		var subscriptionRequests []models.FeedSubscriptionRequest
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
				HandleInternalError(&models.APIError{
					InternalError: fmt.Errorf("read feedset: %w", err),
					StatusCode:    http.StatusUnprocessableEntity,
					UserMessage: models.NewErrorMessage(
						"Unable to add feedset",
						"This might be a temporary issue, please try again.",
					),
				}).ServeHTTP(res, req)
				return
			}
			opmlImport, err := opml.NewOPMLFromBytes(data)
			if err != nil {
				HandleInternalError(&models.APIError{
					InternalError: fmt.Errorf("create opml: %w", err),
					StatusCode:    http.StatusUnprocessableEntity,
					UserMessage: models.NewErrorMessage(
						"Unable to add feedset",
						"This might be a temporary issue, please try again.",
					),
				}).ServeHTTP(res, req)
				return
			}
			subscriptionRequests = append(
				subscriptionRequests,
				models.GenerateRequestsFromOutlines(opmlImport.Body...)...)
		}

		// Process requests.
		results := models.BulkImportFeeds(req.Context(), subscriptionRequests...)

		// Process results
		for result := range slices.Values(results) {
			if result.Error != nil {
				switch {
				case result.Error.UserMessage.IsError():
					slogctx.FromCtx(req.Context()).Error("Error occurred during subscription request processing.",
						slog.String("url", result.Request.URL),
						slog.Any("error", result.Error),
					)
				case result.Error.UserMessage.IsWarning():
					fallthrough
				default:
					slogctx.FromCtx(req.Context()).Warn("Warning occurred during subscription request processing.",
						slog.String("url", result.Request.URL),
						slog.Any("error", result.Error),
					)
				}
			}
		}
		// Show notification.
		for feedset := range slices.Values(request.Feedset) {
			RenderPartial(&Notification{
				msg: models.NewSuccessMessage(
					"Added feedset "+feedset, "",
				),
			}).ServeHTTP(res, req)
		}
	}).ServeHTTP
}

// ChooseSubscriptionPlan contains data for rendering a page to present the user with subscription plan options.
type ChooseSubscriptionPlan struct {
	user *models.User
	plan string
}

// FullResponse renders the page for the user to choose a subscription plan.
func (t *ChooseSubscriptionPlan) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(
		templates.CreatePage(
			templates.UserChooseSubscriptionPlan(t.user, t.plan),
			templates.WithPageTitle("Choose Subscription Plan"),
		)).ServeHTTP(res, req)
}

// HandleChooseSubscriptionPlan handles displaying a page on which the user can choose a subscription plan for purchase.
func HandleChooseSubscriptionPlan() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		user := models.UserFromCtx(req.Context())
		if user == nil {
			HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("get user: %w", models.ErrCtxValueNotFound),
				StatusCode:    http.StatusInternalServerError,
			}).ServeHTTP(res, req)
			return
		}
		// Try to find a selected plan id if it exists, from either the request query params or current session data.
		var planID string
		if req.URL.Query().Get(models.ParamPlanID) != "" {
			planID = req.URL.Query().Get(models.ParamPlanID)
		} else if p, err := session.Restore[string](req.Context(), models.ParamPlanID); err != nil {
			planID = p
		} else {
			HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("process checkout: %w", err),
				StatusCode:    http.StatusInternalServerError,
			}).ServeHTTP(res, req)
			return
		}
		slogctx.FromCtx(req.Context()).Debug("Presenting user with subscription plan options.")
		RenderExternalPage(&ChooseSubscriptionPlan{
			user: user,
			plan: planID,
		}).ServeHTTP(res, req)
	}
}

// HandleSubscriptionPlanCheckout handles processing the user's choice of subscription plan and redirecting to the payment
// processor.
func HandleSubscriptionPlanCheckout() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Fetch the user details from context.
		user := models.UserFromCtx(req.Context())
		if user == nil {
			HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("get user: %w", models.ErrCtxValueNotFound),
				StatusCode:    http.StatusInternalServerError,
			}).ServeHTTP(res, req)
			return
		}

		// Retrieve the plan id from the session data.
		planID := req.FormValue(models.ParamPlanID)
		if planID == "" {
			HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("no plan"),
				StatusCode:    http.StatusInternalServerError,
			}).ServeHTTP(res, req)
			return
		}

		// Create a new strip checkout session.
		var session *stripe.Checkout
		var err error
		session, err = stripe.NewCheckoutSession(user, planID)
		if err != nil {
			HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("create checkout session: %w", err),
				StatusCode:    http.StatusInternalServerError,
			}).ServeHTTP(res, req)
			return
		}

		// Redirect to strip processor to complete checkout session.
		slogctx.FromCtx(req.Context()).Debug("Redirecting user to Stripe for payment.")
		http.Redirect(res, req, session.URL, http.StatusSeeOther)
	}
}

func HandleAccountSuccess() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// stripeSessionID := req.FormValue("session_id")
		http.Redirect(res, req, "/home", http.StatusTemporaryRedirect)
	}
}

func HandleAccountCancel() http.HandlerFunc {
	return HandleLanding()
}

// AccountIssue contains data for rendering a page to present the user when there is an issue with their account.
type AccountIssue struct{}

// FullResponse renders the page for the user to choose a subscription plan.
func (t *AccountIssue) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(
		templates.CreatePage(
			templates.UserAccountIssue(),
			templates.WithPageTitle("Account Issue"),
		)).ServeHTTP(res, req)
}

// HandleAccountIssue handles showing a page with a message indicating the user needs to contact support, as there is a
// critical issue with their account blocking access to the service.
func HandleAccountIssue() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// stripeSessionID := req.FormValue("session_id")
		RenderExternalPage(&AccountIssue{}).ServeHTTP(res, req)
	}
}

func HandleManageAccountSubscription() http.HandlerFunc {
	return userContentHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		sessionID := req.FormValue("session_id")
		if sessionID == "" {
			HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("no session id"),
				StatusCode:    http.StatusInternalServerError,
			}).ServeHTTP(res, req)
			return
		}

		portalSession, err := stripe.NewPortalSession(sessionID)
		if err != nil {
			HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("new portal session: %w", err),
				StatusCode:    http.StatusInternalServerError,
			}).ServeHTTP(res, req)
			return
		}

		// Redirect to payment processor to complete checkout.
		http.Redirect(res, req, portalSession.URL, http.StatusSeeOther)
	}).ServeHTTP
}

func HandleGenerateSubscriptionEmail() http.HandlerFunc {
	return userContentHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		// Fetch the user details from context.
		user := models.UserFromCtx(req.Context())
		if user == nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("unable to get user data: %w", models.ErrCtxValueNotFound),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to generate email",
					"This might be a temporary problem, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}

		settings := user.GetSettings()
		settings.SubscriptionEmail = new("foragd_user_" + strconv.FormatUint(
			xxh3.Hash([]byte(user.GetID()+user.GetNickname())),
			10,
		) + "@foragd.app")

		if err := models.UpdateUser(req.Context(), user.GetID(), map[string]any{"settings": settings}); err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("update user: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to generate email",
					"This might be a temporary problem, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}

		RenderPartial(&PartialTemplate{
			template: templates.ShowSubscriptionEmail(*settings.SubscriptionEmail),
		}).ServeHTTP(res, req)
	}).ServeHTTP
}
