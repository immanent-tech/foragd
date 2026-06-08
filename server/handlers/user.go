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
	"slices"
	"strconv"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"
	"github.com/zeebo/xxh3"

	"github.com/immanent-tech/go-syndication/opml"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/auth0"
	"github.com/immanent-tech/foragd/providers/paddle"
	"github.com/immanent-tech/foragd/providers/resend"
	"github.com/immanent-tech/foragd/server/forms"
	"github.com/immanent-tech/foragd/server/otel"
	"github.com/immanent-tech/foragd/service"
	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/element"
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
	templ.Handler(templates.SideBar(element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
	templ.Handler(templates.Dock(element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
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
			HandleInternalError(req.URL.Path,
				&models.APIError{
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
			HandleInternalError(req.URL.Path,
				&models.APIError{
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

// HandleShowSubscriptionsSettings handles showing the user's subscriptions for bulk management.
func HandleShowSubscriptionsSettings() http.HandlerFunc {
	return userContentHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		// Get user data.
		user := models.UserFromCtx(req.Context())
		if user == nil {
			HandleInternalError(req.URL.Path,
				&models.APIError{
					InternalError: fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Unable to show account settings",
						"This might be a temporary error, please try again.",
					),
				}).ServeHTTP(res, req)
			return
		}
		// Get all subscriptions.
		subscriptions, err := service.GetAllSubscriptions(req.Context())
		if err != nil && !errors.Is(err, models.ErrNotFound) {
			HandleInternalError(req.URL.Path,
				&models.APIError{
					InternalError: fmt.Errorf("get subscriptions: %w", err),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Unable to show account settings",
						"This might be a temporary error, please try again.",
					),
				}).ServeHTTP(res, req)
			return
		}
		// Add dynamic info to subscriptions.
		if err := service.UpdateSubscriptionDynamicInfo(req.Context(), subscriptions); err != nil {
			slogctx.FromCtx(req.Context()).Warn("Unable to add subscription dynamic info.",
				slog.Any("error", err),
			)
		}
		// Sort by newest first.
		subscriptions = subscriptions.Sort(models.SortNewestFirst)
		// Render the subscription list.
		RenderPartial(&PartialTemplate{
			template: templates.SubscriptionSettings(&templates.SubscriptionSettingsData{
				User:          user,
				Subscriptions: subscriptions,
			}),
		}).ServeHTTP(res, req)
	}).ServeHTTP
}

// HandleSaveDisplaySettings handles saving user settings after user submitted changes.
func HandleSaveDisplaySettings() http.HandlerFunc {
	return userContentHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		// Decode request.
		request, valid, err := forms.DecodeForm[*models.UserSettings](req)
		if err != nil || !valid {
			HandleInternalError(req.URL.Path,
				&models.APIError{
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
			HandleInternalError(req.URL.Path,
				&models.APIError{
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
		err = service.UpdateUser(req.Context(), user, map[string]any{"settings": request})
		if err != nil {
			HandleInternalError(req.URL.Path,
				&models.APIError{
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
			HandleInternalError(req.URL.Path,
				&models.APIError{
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
			HandleInternalError(req.URL.Path,
				&models.APIError{
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
			HandleInternalError(req.URL.Path,
				&models.APIError{
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
			HandleInternalError(req.URL.Path,
				&models.APIError{
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
				HandleInternalError(req.URL.Path,
					&models.APIError{
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
				HandleInternalError(req.URL.Path,
					&models.APIError{
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
			request.AvatarURL = new(config.GetBaseURL() + "/img/avatar/" + avatarFileID)
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
		// Update promotional email metadata flag.
		if user.Metadata.PromotionalEmail != request.PromotionalEmail {
			user.Metadata.PromotionalEmail = request.PromotionalEmail
			updates["metadata"] = user.Metadata
		}

		// If no updates are necessary, bail early.
		if len(updates) == 0 {
			res.WriteHeader(http.StatusNoContent)
			return
		}
		// Update on backend.
		err = auth0.UpdateUserCustomisation(req.Context(), request)
		if err != nil || !valid {
			HandleInternalError(req.URL.Path,
				&models.APIError{
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
		err = service.UpdateUser(req.Context(), user, updates)
		if err != nil {
			HandleInternalError(req.URL.Path,
				&models.APIError{
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
			HandleInternalError(req.URL.Path,
				&models.APIError{
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
			HandleInternalError(req.URL.Path,
				&models.APIError{
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

// HandleDeactivateAccount handles a user request to deactivate their account. Their subscription in Stripe will be cancelled at
// the end of the current billing period. They can continue to log in and use the service during the current billing
// period, after which a scheduled job will delete their account.
func HandleDeactivateAccount() http.HandlerFunc {
	return userContentHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		switch req.FormValue("confirmed") {
		case "yes":
			// Get user account details.
			user := models.UserFromCtx(req.Context())
			if user == nil {
				HandleInternalError(req.URL.Path,
					&models.APIError{
						InternalError: fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound),
						StatusCode:    http.StatusInternalServerError,
						UserMessage: models.NewErrorMessage(
							"Unable to deactivate account",
							"This might be a temporary error, please try again.",
						),
					}).ServeHTTP(res, req)
				return
			}
			switch {
			case user.InTrial():
				// Delete from Elasticsearch backend.
				if err := service.DeleteUser(req.Context(), user); err != nil {
					HandleInternalError(req.URL.Path,
						&models.APIError{
							InternalError: fmt.Errorf("delete user in elasticsearch: %w", err),
							StatusCode:    http.StatusInternalServerError,
							UserMessage: models.NewErrorMessage(
								"Unable to deactivate account",
								"If this issue persists, please email support@foragd.app.",
							),
						}).ServeHTTP(res, req)
					return
				}

				// Delete from Auth0 backend
				if err := auth0.DeleteUser(req.Context(), user.GetExternalID()); err != nil {
					HandleInternalError(req.URL.Path,
						&models.APIError{
							InternalError: fmt.Errorf("delete user in auth0: %w", err),
							StatusCode:    http.StatusInternalServerError,
							UserMessage: models.NewErrorMessage(
								"Unable to deactivate account",
								"If this issue persists, please email support@foragd.app.",
							),
						}).ServeHTTP(res, req)
					return
				}

				// Create and send deactivation email confirmation.
				email, err := resend.NewTemplatedEmail(
					"user-deactivated",
					resend.WithTo(user.GetEmail()),
					resend.WithTag(resend.TagCategory, resend.TagCategoryAccount),
					resend.WithTag(resend.TagUserID, user.GetID()),
				)
				if err != nil {
					slogctx.FromCtx(req.Context()).Warn("Unable to create deactivation email.",
						slog.String("user_id", user.GetID()),
						slog.Any("error", err),
					)
				}
				if err := resend.SendEmail(req.Context(), resend.WithExistingEmail(email)); err != nil {
					slogctx.FromCtx(req.Context()).Warn("Unable to send deactivation email.",
						slog.String("user_id", user.GetID()),
						slog.Any("error", err),
					)
				}

				slogctx.FromCtx(req.Context()).Info("Deleted trial user.")

				// Pass to logout handler.
				Logout(res, req)
			case user.HasActiveSubscription():
				if err := paddle.CancelSubscription(req.Context(), user); err != nil {
					HandleInternalError(req.URL.Path,
						&models.APIError{
							InternalError: fmt.Errorf("cancel subscription: %w", err),
							StatusCode:    http.StatusInternalServerError,
							UserMessage: models.NewErrorMessage(
								"Unable to deactivate account",
								"If this issue persists, please email support@foragd.app.",
							),
						}).ServeHTTP(res, req)
					return
				}
				res.Header().Set(htmx.HeaderReswap, "innerHTML transition:true")
				res.Header().Set(htmx.HeaderRetarget, templates.ContentID.Target())
				RenderPartial(&PartialTemplate{
					template: templates.DeactivateResult(user),
				}).ServeHTTP(res, req)

				slogctx.FromCtx(req.Context()).Info("Cancelled paid user subscription.")
			}
		default:
			RenderPartial(&Modal{
				template: templates.DeactivateAccountModal(),
			}).ServeHTTP(res, req)
		}
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
			HandleInternalError(req.URL.Path,
				&models.APIError{
					InternalError: fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
					StatusCode:    http.StatusUnprocessableEntity,
					UserMessage:   models.NewErrorMessage("Unable to add feedset", "Data is invalid."),
				}).ServeHTTP(res, req)
			return
		}

		feedsetEnlightened := map[string]models.FeedID{
			"NOEMA Magazine":       "feed_5CqH3PEX5S3FysJ4uU6Pb",
			"waxy.org":             "feed_WOh0ANw9kSg1KI9zaCsCE",
			"webcurios":            "feed_13204732792406402926",
			"EFF's Deeplinks Blog": "feed_JW2_cWxBXhS4rqK8XjHE3",
		}
		feedsetInspired := map[string]models.FeedID{
			"Colossal":             "feed_15392474241830067070",
			"Open Culture":         "feed_XcgMA2rMhX7m7aYhc4NUy",
			"70s Sci-Fi Art":       "feed_0MauaHq9RS42wxhRC4I2y",
			"Public Domain Review": "feed_QHx6MHP8F_SuqvV5hu8KL",
			"500px":                "feed_LoeuTpadhbJam7j5XshVA",
		}
		feedsetInformed := map[string]models.FeedID{
			"Ars Technica": "feed_HOtEGRko3RV0Qy2mSK6zN",
			"WIRED":        "feed_9861496488451910378",
			"Live Science": "feed_xaOVKbhp28Ka7tQfqxEVh",
			"The Guardian": "feed_le7pwn7QyGL5juGdgyQsG",
		}

		// Process requested feedsets and generate subscription requests.
		var subscriptionRequests []models.FeedSubscriptionRequest
		for set := range slices.Values(request.Feedset) {
			var data []byte
			switch set {
			case "enlightened":
				ids := make([]models.FeedID, 0, len(feedsetEnlightened))
				for _, id := range feedsetEnlightened {
					ids = append(ids, id)
				}
				data, err = service.GenerateOPML(req.Context(), ids...)
			case "informed":
				ids := make([]models.FeedID, 0, len(feedsetInformed))
				for _, id := range feedsetInformed {
					ids = append(ids, id)
				}
				data, err = service.GenerateOPML(req.Context(), ids...)
			case "inspired":
				ids := make([]models.FeedID, 0, len(feedsetInspired))
				for _, id := range feedsetInspired {
					ids = append(ids, id)
				}
				data, err = service.GenerateOPML(req.Context(), ids...)
			default:
				slogctx.FromCtx(req.Context()).Warn("Unknown feedset.",
					slog.String("set", set))
				continue
			}
			if err != nil {
				HandleInternalError(req.URL.Path,
					&models.APIError{
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
				HandleInternalError(req.URL.Path,
					&models.APIError{
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
		results := service.BulkImportFeeds(req.Context(), subscriptionRequests...)

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
		// Redirect to payment processor to complete checkout.
		http.Redirect(res, req, "/", http.StatusSeeOther)
	}).ServeHTTP
}

func HandleGenerateSubscriptionEmail() http.HandlerFunc {
	return userContentHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		// Fetch the user details from context.
		user := models.UserFromCtx(req.Context())
		if user == nil {
			HandleInternalError(req.URL.Path,
				&models.APIError{
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

		if err := service.UpdateUser(req.Context(), user, map[string]any{"settings": settings}); err != nil {
			HandleInternalError(req.URL.Path,
				&models.APIError{
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

// Unsubscribe represents an unsubscribe request.
type Unsubscribe struct {
	Token string
}

func (p *Unsubscribe) FullResponse(res http.ResponseWriter, req *http.Request) {
	title := "Unsubscribe Options"
	description := "Unsubscribe from promotional emails from Foragd."
	templ.Handler(
		templates.CreatePage(
			templates.Unsubscribe(p.Token),
			templates.WithPageTitle(title),
			templates.WithPageDescription(description),
		),
	).ServeHTTP(res, req)
}

// UnsubscribeResult represents an unsubscribe result.
type UnsubscribeResult struct {
	Msg *models.UserMessage
}

func (p *UnsubscribeResult) FullResponse(res http.ResponseWriter, req *http.Request) {
	title := "Unsubscribe Result"
	description := "Unsubscribe from promotional emails from Foragd."
	templ.Handler(
		templates.CreatePage(
			templates.UnsubscribeResult(p.Msg),
			templates.WithPageTitle(title),
			templates.WithPageDescription(description),
		),
	).ServeHTTP(res, req)
}

func (p *UnsubscribeResult) PartialResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(templates.UnsubscribeResult(p.Msg), templ.WithFragments(templates.ContentFragment)).
		ServeHTTP(res, req)
}

// HandleUserUnsubscribe handles requests from users to unsubscribe from promotional emails. It handles both interactive
// (user manually goes to page) and non-interactive (as per RFC 8058).
func HandleUserUnsubscribe() http.HandlerFunc {
	return alice.New().ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		token := chi.RouteContext(req.Context()).URLParam("token")
		if token == "" {
			HandleExternalError(&models.APIError{
				InternalError: errors.New("invalid or empty email token"),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Invalid unsubscribe link",
					"The link is invalid. Please check and try again or contact support.",
				),
			}).ServeHTTP(res, req)
			return
		}

		switch req.Method {
		case http.MethodGet:
			RenderExternalPage(&Unsubscribe{
				Token: token,
			}).ServeHTTP(res, req)
		case http.MethodPost:
			// Closure used to display results.
			displayResults := func(err error) {
				var msg *models.UserMessage
				if err != nil {
					res.WriteHeader(http.StatusInternalServerError)
					slogctx.FromCtx(req.Context()).Error("Failed to decode user email.",
						slog.Any("error", err),
					)
					msg = models.NewErrorMessage(
						"Failed to complete unsubscribe request",
						"The server encountered an unexpected issue. Please try again.",
					)
				} else {
					msg = models.NewErrorMessage(
						"Unsubscribe successful",
						"You have been unsubscribed from promotional emails.",
					)
				}
				switch {
				case htmx.IsHTMX(req):
					RenderPartial(&UnsubscribeResult{
						Msg: msg,
					}).ServeHTTP(res, req)
				default:
					RenderExternalPage(&UnsubscribeResult{
						Msg: msg,
					}).ServeHTTP(res, req)
				}
			}

			// Decode the user email address.
			email, err := resend.DecodeEmail(token)
			if err != nil {
				displayResults(err)
				return
			}

			// Retrieve the user details.
			user, err := service.GetUserByEmail(req.Context(), email)
			if err != nil {
				displayResults(err)
				return
			}

			// Mark in the user's metadata that they do not want to receive promotional emails.
			user.Metadata.PromotionalEmail = false
			// Update the user.
			if err := service.UpdateUser(req.Context(), user, map[string]any{
				"metadata": user.Metadata,
			}); err != nil {
				displayResults(err)
				return
			}

			displayResults(nil)
		}
	}).ServeHTTP
}

func ValidateSubscriptionLimits(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		_, span := otel.TracerProvider.Tracer("").Start(req.Context(), "validate-user-limits")
		defer span.End()
		switch err := service.CheckUserLimits(req.Context()); {
		case errors.Is(err, models.ErrForbidden):
			HandleInternalError(req.Referer(), err).ServeHTTP(res, req)
			return
		case errors.Is(err, models.ErrSubscriptionLimitExceeded),
			errors.Is(err, models.ErrEmailNewsletterLimitExceeded):
			slogctx.FromCtx(req.Context()).Warn("User has exceeded account limits.")
		}

		next.ServeHTTP(res, req)
	})
}
