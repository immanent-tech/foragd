// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"time"

	"github.com/a-h/templ"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/calendarinterval"
	"github.com/go-chi/chi/v5"
	"github.com/immanent-tech/go-syndication/opml"
	"github.com/justinas/alice"
	"github.com/justinas/nosurf"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/aggregations"
	"github.com/immanent-tech/foragd/providers/elastic/bulk"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/providers/github"
	"github.com/immanent-tech/foragd/server/forms"
	"github.com/immanent-tech/foragd/server/session"
	"github.com/immanent-tech/foragd/validation"
	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/layouts"
	"github.com/immanent-tech/foragd/web/templates/partials"
)

// GetSubscriptions handles showing a filtered collection of subscriptions as cards.
func (a *API) GetSubscriptions() http.HandlerFunc {
	return alice.New(
		decodeSubscriptionFilters,
		saveSubscriptionFilters,
	).ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		filters := subscriptionFiltersFromCtx(req.Context())
		// Get subscriptions matching filters.
		subscriptions, pagination, err := a.filterSubscriptions(req.Context(), &filters)
		if err != nil {
			return models.NewAPIError(fmt.Errorf("unable to get subscriptions: %w", err), http.StatusInternalServerError)
		}
		// Get subscription stats.
		stats, err := a.getSubscriptionStats(req.Context(), &filters)
		if err != nil {
			return models.NewAPIError(fmt.Errorf("unable to get subscriptions: %w", err), http.StatusInternalServerError)
		}
		// Render appropriate content.
		template := layouts.SubscriptionsGrid(subscriptions, &filters, pagination, stats)
		renderPage(template, templates.GeneratePageTitle("Subscriptions")).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// GetSubscriptionUpdates handles checking for updates to subscriptions and showing a notification to the user to
// refresh the content.
//
//nolint:gocognit
func (a *API) GetSubscriptionUpdates() http.HandlerFunc {
	return alice.New(
		decodeSubscriptionFilters,
	).ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "text/event-stream")
		res.Header().Set("Cache-Control", "no-cache")
		res.Header().Set("Connection", "keep-alive")
		if f, ok := res.(http.Flusher); ok {
			f.Flush()
		} else {
			slogctx.FromCtx(req.Context()).Warn("Cannot flush update stream!")
			res.WriteHeader(http.StatusNoContent)
		}
		// Get filters and generate query.
		filters := subscriptionFiltersFromCtx(req.Context())
		query, err := generateItemsQuery(req.Context(), &filters)
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Cannot generate query for updates.",
				slog.Any("error", err))
			res.WriteHeader(http.StatusInternalServerError)
			return
		}

		var (
			currentCount int64
			prevCount    int64
		)
		prevCount, err = a.DataAPI().CountItems(req.Context(), query)
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Cannot get updates count.",
				slog.Any("error", err))
			res.WriteHeader(http.StatusInternalServerError)
			return
		}

		for {
			select {
			case <-req.Context().Done():
				res.Header().Set("Connection", "close")
				res.WriteHeader(http.StatusRequestTimeout)
				return
			default:
				currentCount, err = a.DataAPI().CountItems(req.Context(), query)
				if err != nil {
					slogctx.FromCtx(req.Context()).Warn("Cannot get updates count.",
						slog.Any("error", err))
					continue
				}
				// Show updates toast if new items found.
				if currentCount > prevCount {
					slogctx.FromCtx(req.Context()).Debug("Subscription updates found.")
					var b bytes.Buffer //nolint:varnamelen
					template := bufio.NewWriter(&b)
					err := partials.UpdatesToast().Render(req.Context(), template)
					if err != nil {
						slogctx.FromCtx(req.Context()).Warn("Unable to render template.",
							slog.Any("error", err))
						continue
					}
					err = template.Flush()
					if err != nil {
						slogctx.FromCtx(req.Context()).Error("Failed to flush SSE message buffer.",
							slog.Any("error", err))
					}
					_, err = fmt.Fprintf(res, "data: %s\n\n", b.String())
					if err != nil {
						slogctx.FromCtx(req.Context()).Error("Failed to send update SSE message.",
							slog.Any("error", err))
					}
					if f, ok := res.(http.Flusher); ok {
						f.Flush()
					}
				}
				prevCount = currentCount
				time.Sleep(defaultUpdateInterval)
			}
		}
	}).ServeHTTP
}

// PaginateSubscriptions handles showing the next set of subscriptions.
func (a *API) PaginateSubscriptions() http.HandlerFunc {
	return alice.New(
		decodeSubscriptionFilters,
	).ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		filters := subscriptionFiltersFromCtx(req.Context())
		// Get subscriptions matching filters.
		subscriptions, pagination, err := a.filterSubscriptions(req.Context(), &filters)
		if err != nil {
			res.WriteHeader(http.StatusInternalServerError)
			return
		}
		// Get subscription stats.
		stats, err := a.getSubscriptionStats(req.Context(), &filters)
		if err != nil {
			res.WriteHeader(http.StatusInternalServerError)
			return
		}
		// Render appropriate content.
		if len(subscriptions) > 0 {
			renderPartial(layouts.SubscriptionsList(subscriptions, &filters, pagination, stats)).ServeHTTP(res, req)
		} else {
			res.WriteHeader(http.StatusNoContent)
		}
	}).ServeHTTP
}

// MarkSubscription handles marking a subscription as read or unread.
func (a *API) MarkSubscription() http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Construct the request from parameters.
		request := &models.MarkSubscriptionsRequest{
			Mark:          models.Mark(chi.URLParam(req, models.ParamMark)),
			Subscriptions: []models.SubscriptionID{chi.URLParam(req, models.ParamSubscriptionID)},
		}
		view := models.View(req.FormValue("view"))
		// Mark subscription.
		err := a.markSubscriptions(req.Context(), request)
		if err != nil {
			renderPartial(partials.Notification(
				models.NewErrorMessage(
					"Unable to mark subscription",
					"Something went wrong, please try again",
				), 0))
			return models.NewAPIError(
				fmt.Errorf("unable to mark subscription: %w", err),
				http.StatusInternalServerError)
		}
		// If the view is "all" send back the updated subscription card.
		if view == models.ViewAll {
			s, err := a.getSubscriptions(req.Context(), request.Subscriptions...)
			if err != nil || len(s) == 0 || len(s) > 1 {
				renderPartial(partials.Notification(
					models.NewErrorMessage(
						"Unable to refresh subscription",
						"Something went wrong, please try again",
					), 0))
				return models.NewAPIError(
					fmt.Errorf("unable to mark subscription: %w", err),
					http.StatusInternalServerError)
			}
			// Get subscription stats.
			filters := session.SubscriptionFiltersFromSession(req.Context())
			stats, err := a.getSubscriptionStats(req.Context(), &filters)
			if err != nil {
				renderPartial(partials.Notification(
					models.NewErrorMessage(
						"Unable to refresh subscription",
						"Something went wrong, please try again",
					), 0))
				return models.NewAPIError(
					fmt.Errorf("unable to mark subscription: %w", err),
					http.StatusInternalServerError)
			}
			subscriptionStats := stats[s[0].GetID()]
			renderPartial(partials.NewSubscriptionContent(s[0], &subscriptionStats).Card()).ServeHTTP(res, req)
		} else {
			res.WriteHeader(http.StatusOK)
		}
		return nil
	})).ServeHTTP
}

// MarkAllSubscriptions handles marking all subscriptions as read or unread.
func (a *API) MarkAllSubscriptions() http.HandlerFunc {
	return alice.New(
		decodeSubscriptionFilters,
	).ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Extract filters from request.
		filters := subscriptionFiltersFromCtx(req.Context())
		slogctx.FromCtx(req.Context()).Debug("Marking subscriptions.", slog.String("filters", filters.Query()))
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			return fmt.Errorf("unable to mark all subscriptions: %w", err)
		}
		// Set the appropriate mark.
		var mark models.Mark
		switch filters.GetView() {
		case models.ViewUnread:
			mark = models.MarkRead
		default:
			mark = models.MarkUnread
		}
		// Construct the request from parameters.
		request := &models.MarkSubscriptionsRequest{
			Mark: mark,
		}
		if len(filters.Subscriptions) == 0 {
			request.Subscriptions = user.GetSubscriptionMetadata().GetIDs()
		} else {
			request.Subscriptions = filters.Subscriptions
		}
		// Mark subscriptions.
		err = a.markSubscriptions(req.Context(), request)
		if err != nil {
			renderPartial(partials.Notification(
				models.NewErrorMessage(
					"Unable to mark subscriptions",
					"Something went wrong, please try again",
				), 0))
			return models.NewAPIError(
				fmt.Errorf("unable to mark all subscriptions: %w", err),
				http.StatusInternalServerError)
		}
		// Get updated subscriptions.
		subscriptions, pagination, err := a.filterSubscriptions(req.Context(), &filters)
		if err != nil {
			renderPartial(partials.Notification(
				models.NewErrorMessage(
					"Unable to refresh subscriptions",
					"Something went wrong, please try again",
				), 0))
			return models.NewAPIError(
				fmt.Errorf("unable to mark all subscriptions: %w", err),
				http.StatusInternalServerError)
		}
		stats, err := a.getSubscriptionStats(req.Context(), &filters)
		if err != nil {
			renderPartial(partials.Notification(
				models.NewErrorMessage(
					"Unable to refresh subscription",
					"Something went wrong, please try again",
				), 0))
			return models.NewAPIError(
				fmt.Errorf("unable to mark subscription: %w", err),
				http.StatusInternalServerError)
		}
		// Render appropriate content.
		template := layouts.SubscriptionsGrid(subscriptions, &filters, pagination, stats)
		renderPage(template, templates.GeneratePageTitle("Subscriptions")).ServeHTTP(res, req)

		// Redirect depending on the current view.
		switch filters.GetView() {
		case models.ViewRead, models.ViewUnread:
			SetRedirect(req.Context(), "/home", res)
		case models.ViewAll:
			SetRedirect(req.Context(), "/subscriptions", res)
		}

		return nil
	})).ServeHTTP
}

// EditSubscription handles presenting the user with a form for editing a subscription.
func (a *API) EditSubscription() http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Retrieve the subscription ID from the URL parameter.
		id := chi.URLParam(req, models.ParamSubscriptionID)
		// Retrieve user object.
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			return fmt.Errorf("unable to edit subscription: %w", err)
		}
		metadata := user.GetSubscriptionMetadata().GetByID(id)
		// Convert metadata into edit request data.
		request := &models.EditSubscriptionRequest{
			SubscriptionID:         id,
			Nickname:               metadata.Customisation.Nickname,
			Categories:             metadata.Customisation.Categories,
			ShowFullArticleContent: metadata.Settings.ShowFullArticleContent,
			ArticleFilters:         metadata.Customisation.ArticleFilters,
		}
		// Get top categories across items in subscription feed and add as suggested categories for the
		// subscription.
		categories, resp := a.getItemTopCategories(req.Context(), metadata.GetFeedID())
		if resp == nil {
			request.SuggestedCategories = categories
		}
		// Generate page template.
		template := layouts.EditSubscription(request)
		ctx := models.CSRFTokenToCtx(req.Context(), nosurf.Token(req))
		renderPage(template, templates.GeneratePageTitle("Editing "+request.GetNickname())).ServeHTTP(res, req.WithContext(ctx))
		return nil
	})).ServeHTTP
}

// SaveSubscription handles saving the edits made by a user to a subscription.
func (a *API) SaveSubscription() http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Retrieve user object.
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			return fmt.Errorf("unable to save subscription: %w", err)
		}
		request, valid, err := forms.DecodeForm[*models.EditSubscriptionRequest](req)
		if err != nil || !valid {
			renderPage(layouts.EditSubscription(request), "").ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		// Update the subscription metadata.
		metadata := user.GetSubscriptionMetadata().GetByID(request.SubscriptionID)
		metadata.Customisation.Nickname = request.GetNickname()
		metadata.Customisation.Categories = request.GetCategories()
		metadata.Settings.ShowFullArticleContent = request.ShowFullArticleContent
		metadata.Customisation.ArticleFilters.Authors = request.ArticleFilters.Authors
		metadata.Customisation.ArticleFilters.Categories = request.ArticleFilters.Categories
		err = user.UpdateSubscription(metadata)
		if err != nil {
			msg := models.NewErrorMessage("An error occurred trying to save the subscription", "Please try again.")
			template := templ.Join(layouts.EditSubscription(request), partials.Notification(msg, 0))
			renderPage(template, "").ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		// Update the user.
		err = a.DataAPI().UpdateUser(req.Context(), map[string]any{
			"subscriptions": user.GetSubscriptionMetadata(),
		})
		if err != nil {
			msg := models.NewErrorMessage("An error occurred trying to save the subscription", "Please try again.")
			template := templ.Join(layouts.EditSubscription(request), partials.Notification(msg, 0))
			renderPage(template, "").ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		template := templ.Join(layouts.EditSubscription(request), layouts.EditSubscriptionSuccessNotification(metadata))
		renderPage(template, "").ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// GetRemoveSubscriptionConfirmation handles showing a confirmation dialog for removing (unsubscribing) from a
// subscription.
func (a *API) GetRemoveSubscriptionConfirmation() http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Show a modal to confirm unsubscribe request.
		id := chi.URLParam(req, models.ParamSubscriptionID)
		subscriptions, err := a.getSubscriptions(req.Context(), id)
		if err != nil || len(subscriptions) == 0 || len(subscriptions) > 1 {
			msg := models.NewErrorMessage("An error occurred processing the request", "Please try again.")
			template := partials.Notification(msg, 0)
			renderPage(template, "").ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		filters := session.SubscriptionFiltersFromSession(req.Context())
		stats, err := a.getSubscriptionStats(req.Context(), &filters)
		if err != nil {
			renderPartial(partials.Notification(
				models.NewErrorMessage(
					"Unable to refresh subscription",
					"Something went wrong, please try again",
				), 0))
			return models.NewAPIError(
				fmt.Errorf("unable to mark subscription: %w", err),
				http.StatusInternalServerError)
		}
		subscriptionStats := stats[subscriptions[0].GetID()]
		renderPartial(partials.NewSubscriptionContent(subscriptions[0], &subscriptionStats).UnsubscribeModal()).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// ProcessRemoveSubscription handles processing a remove (unsubscribe) subscription request.
func (a *API) ProcessRemoveSubscription() http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Perform unsubscribe action.
		id := chi.URLParam(req, models.ParamSubscriptionID)
		// Retrieve user object.
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			return fmt.Errorf("unable to process subscription removal: %w", err)
		}
		// Remove metadata for given subscriptions from user.
		user.RemoveSubscriptions(id)
		// Update the user.
		err = a.DataAPI().UpdateUser(req.Context(), map[string]any{
			"subscriptions": user.GetSubscriptionMetadata(),
		})
		if err != nil {
			msg := models.NewErrorMessage("Unable to remove subscription", "Please try again.")
			template := partials.Notification(msg, 0)
			renderPage(template, "").ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		// Show success notification.
		msg := models.NewSuccessMessage("Unsubscribed!", "")
		renderPartial(partials.Notification(msg, 0)).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// AddSubscription handles adding a new subscription requested by the user.
func (a *API) AddSubscription() http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		switch req.Method {
		case http.MethodGet:
			template := layouts.AddSubscription(&models.SubscriptionRequest{})
			renderPage(template, templates.GeneratePageTitle("Add Subscription")).ServeHTTP(res, req)
		case http.MethodPost:
			request, valid, err := forms.DecodeForm[*models.SubscriptionRequest](req)
			if err != nil || !valid {
				renderPage(layouts.AddSubscription(request), "").ServeHTTP(res, req)
				return models.NewAPIError(err, http.StatusUnprocessableEntity)
			}
			requests := addSubscriptionRequests{
				request: &models.Feed{},
			}
			// Match the request to either and existing or new feed.
			result, err := requests.matchFeedsToSubscriptionRequests(req.Context(), a.Elastic)
			if err != nil {
				msg := models.NewErrorMessage("An error occurred trying to save the subscription", "Please try again.")
				template := templ.Join(layouts.AddSubscription(request), partials.Notification(msg, 0))
				renderPage(template, "").ServeHTTP(res, req)
				return models.NewAPIError(err, http.StatusUnprocessableEntity)
			}
			// If results returned from matching is non-nil, something went wrong.
			if result[request] != nil {
				template := layouts.AddSubscription(request)
				renderPage(template, "").ServeHTTP(res, req)
				res.WriteHeader(http.StatusUnprocessableEntity)
				return nil
			}
			// Create the new subscription.
			createResult, err := requests.createNewSubscriptions(req.Context(), a.Elastic)
			if err != nil {
				msg := models.NewErrorMessage("An error occurred trying to save the subscription", "Please try again.")
				template := templ.Join(layouts.AddSubscription(request), partials.Notification(msg, 0))
				renderPage(template, "").ServeHTTP(res, req)
				return models.NewAPIError(err, http.StatusUnprocessableEntity)
			} else {
				result = createResult
			}
			if result[request].Message.Status != models.UserMessageStatusSuccess {
				msg := models.NewErrorMessage("An error occurred trying to save the subscription", "Please try again.")
				template := templ.Join(layouts.AddSubscription(request), partials.Notification(msg, 0))
				renderPage(template, "").ServeHTTP(res, req)
				return models.NewAPIError(errors.New(result[request].Message.String()), http.StatusUnprocessableEntity)
			}
			template := layouts.AddSubscriptionSuccess(result[request])

			renderPage(template, "").ServeHTTP(res, req)
		}
		return nil
	})).ServeHTTP
}

// ImportSubscriptions handles assisting the user with importing subscriptions from an external source.
func (a *API) ImportSubscriptions() http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		switch req.Method {
		// GET: show import modal.
		case http.MethodGet:
			template := layouts.ImportSubscriptions()
			renderPage(template, templates.GeneratePageTitle("Import Subscriptions")).ServeHTTP(res, req)
		// POST: process import.
		case http.MethodPost:
			requests := make(addSubscriptionRequests)
			opmlFile := &models.OPMLFile{}
			opmlFile, valid, err := forms.DecodeMultipartFile(req, "source", opmlFile)
			if err != nil || !valid {
				msg := models.NewErrorMessage(
					"Failed to read OPML file",
					"The OPML could not be read. Is it a valid OPML file? Please check the contents, correct any issues and try again.")
				renderPartial(partials.Notification(msg, 0)).ServeHTTP(res, req)
				return models.NewAPIError(err, http.StatusUnprocessableEntity)
			}
			r, err := opmlFile.GenerateRequests()
			if err != nil {
				msg := models.NewWarningMessage(
					"Failed to extract subscriptions from OPML file.",
					"There was a problem reading the individual feed entries in the OPML file. Please check the contents, correct any issues and try again.",
				)
				renderPartial(partials.Notification(msg, 0)).ServeHTTP(res, req)
				return models.NewAPIError(err, http.StatusUnprocessableEntity)
			}
			for newRequest := range slices.Values(r) {
				requests[newRequest] = &models.Feed{}
			}
			matchResults, err := requests.matchFeedsToSubscriptionRequests(req.Context(), a.Elastic)
			if err != nil {
				msg := models.NewErrorMessage(
					"Error processing OPML file.",
					"The backend had issues processing the OPML file and adding subscriptions, please try again.",
				)
				renderPartial(partials.Notification(msg, 0)).ServeHTTP(res, req)
				return models.NewAPIError(err, http.StatusUnprocessableEntity)
			}
			createResults, err := requests.createNewSubscriptions(req.Context(), a.Elastic)
			if err != nil {
				msg := models.NewErrorMessage(
					"Error processing OPML file.",
					"The backend had issues processing the OPML file and adding subscriptions, please try again.",
				)
				renderPartial(partials.Notification(msg, 0)).ServeHTTP(res, req)
				return models.NewAPIError(err, http.StatusUnprocessableEntity)
			}
			maps.Copy(createResults, matchResults)
			msg := models.NewInfoMessage(
				"OPML import complete.", "Please consult the results and check for any issues.",
			)
			template := templ.Join(layouts.ImportResults(createResults), partials.Notification(msg, 10*time.Second))
			renderPartial(template).ServeHTTP(res, req)
		}
		return nil
	})).ServeHTTP
}

// ExportSubscriptions handles configuring and performing an export of user subscriptions.
func (a *API) ExportSubscriptions() http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Get the user details.
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			return fmt.Errorf("unable to export subscription: %w", err)
		}
		switch {
		// GET: show import modal.
		case chi.RouteContext(req.Context()).RoutePattern() == "/user/export":
			renderPage(layouts.ExportSubscriptions(), templates.GeneratePageTitle("Export Subscriptions")).ServeHTTP(res, req)
		case chi.RouteContext(req.Context()).RoutePattern() == "/user/export/opml":
			// Get all subscriptions.
			subscriptions, err := a.getSubscriptions(req.Context())
			if err != nil {
				msg := models.NewErrorMessage(
					"Error exporting OPML file.",
					"The backend had issues generating the OPML file, please try again.",
				)
				renderPartial(partials.ServerErrorNotification(msg)).ServeHTTP(res, req)
				return models.NewAPIError(err, http.StatusUnprocessableEntity)
			}
			// Create outlines for all subscriptions.
			outlines := make([]opml.Outline, 0, len(subscriptions))
			for subscription := range slices.Values(subscriptions) {
				outlines = append(outlines, *opml.NewSubscriptionOutline(subscription.GetTitle(), subscription.Feed.GetSourceURLs()[0],
					opml.WithHTMLURL(subscription.GetLink()),
					opml.WithOutlineTitle(subscription.GetTitle()),
					opml.WithDescription(subscription.GetDescription()),
				))
			}
			// Generate the opml file from the outlines.
			title := config.AppName + " subscriptions export for " + user.GetNickname()
			opmlExport := opml.NewOPML(
				opml.WithTitle(title),
				opml.WithOutlines(outlines...),
			)
			// Marshal the opml file and convert to a byte reader.
			data, err := xml.Marshal(opmlExport)
			data = []byte(xml.Header + string(data))
			if err != nil {
				msg := models.NewErrorMessage(
					"Error exporting OPML file.",
					"The backend had issues generating the OPML file, please try again.",
				)
				renderPartial(partials.ServerErrorNotification(msg)).ServeHTTP(res, req)
				return models.NewAPIError(err, http.StatusUnprocessableEntity)
			}
			// Serve the opml content via http.ServeContent.
			res.Header().Set("Content-Type", "text/x-opml+xml; charset=utf-8")
			filename := config.AppName + "-Export.opml"
			res.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
			http.ServeContent(res, req, filename, time.Now(), bytes.NewReader(data))
		}
		return nil
	})).ServeHTTP
}

// AdjustSubscriptionCategories handles adding and removing categories from a subscription, either when editing or
// adding.
func (a *API) AdjustSubscriptionCategories() http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		switch req.Method {
		case http.MethodPost:
			// Add a category.
			currentCategories, _, _ := forms.DecodeForm[*partials.AddSubscriptionCategories](req)
			category := req.FormValue("category")
			if category == "" || (currentCategories != nil && slices.Contains(currentCategories.Categories, category)) {
				res.WriteHeader(http.StatusNoContent)
			} else {
				renderPartial(partials.AddCategory(category)).ServeHTTP(res, req)
			}
		case http.MethodDelete:
			// Remove a category.
			res.WriteHeader(http.StatusOK)
		default:
			// Unsupported, do nothing.
			res.WriteHeader(http.StatusNoContent)
		}
		return nil
	})).ServeHTTP
}

// GetSubscriptionIssues handles presenting a form for the user to submit details about subscription issues.
func GetSubscriptionIssues(api *API) http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		id := chi.URLParam(req, models.ParamSubscriptionID)
		s, err := api.getSubscriptions(req.Context(), id)
		if err != nil || len(s) == 0 {
			msg := models.NewErrorMessage(
				"Unable to create report form.",
				"The backend had issues generating the report form. Please try again.",
			)
			renderPartial(partials.ServerErrorNotification(msg)).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		template := layouts.ReportSubscriptionIssue(s[0], &models.SubscriptionIssue{})
		renderPage(template, templates.GeneratePageTitle("Report subscription issue")).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// SubmitSubscriptionIssues handles processing the user submitted subscription issues form.
func SubmitSubscriptionIssues(esapi *API, ghapi *github.Client) http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Validate the subscription issue request.
		request, valid, err := forms.DecodeForm[*models.SubscriptionIssue](req)
		if err != nil || !valid {
			msg := models.NewErrorMessage(
				"Unable to submit issue.",
				"The backend had issues submitting the report. Please try again.",
			)
			renderPartial(partials.ServerErrorNotification(msg)).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		// Get subscription details.
		id := chi.URLParam(req, models.ParamSubscriptionID)
		s, err := esapi.getSubscriptions(req.Context(), id)
		if err != nil || len(s) == 0 {
			msg := models.NewErrorMessage(
				"Unable to get subscription details.",
				"The backend had issues processing the data. Please try again.",
			)
			renderPartial(partials.ServerErrorNotification(msg)).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		// Create the issue in Github.
		err = ghapi.CreateSubscriptionIssue(req.Context(), s[0], request)
		if err != nil {
			msg := models.NewErrorMessage(
				"Unable to submit issue.",
				"The backend had issues submitting the report. Please try again.",
			)
			template := templ.Join(
				layouts.ReportSubscriptionIssue(s[0], request),
				partials.ServerErrorNotification(msg),
			)
			renderPage(template, templates.GeneratePageTitle("Report subscription issue")).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		// Force refresh of page.
		msg := models.NewErrorMessage(
			"Thanks for reporting the issue!",
			"We will look into it and implement fixes as appropriate.",
		)
		renderPage(partials.IssueReportedConfirmation(msg), templates.GeneratePageTitle("Report subscription issue")).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

func (a *API) getSubscriptionUnreadCounts(ctx context.Context, subscriptionMetadata models.SubscriptionMetadataSlice) (*aggregations.TermsAggregationResults, error) {
	// Don't continue if we have no subscriptions to calculate.
	if len(subscriptionMetadata) == 0 {
		return &aggregations.TermsAggregationResults{}, nil
	}
	// Retrieve user object.
	user, err := models.UserFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to get subscription unread counts: %w", err)
	}
	// Generate unread count query.
	subscriptionQueries := make([]query.Option, 0, len(subscriptionMetadata))
	for m := range slices.Values(subscriptionMetadata) {
		subscriptionQueries = append(subscriptionQueries, queryUnreadItems(user, m))
	}
	query := query.Bool(
		query.Filter(
			query.Bool(
				query.Should(subscriptionQueries...),
			),
		),
	)
	// Perform aggregation.
	aggResults, resp := a.DataAPI().ItemsAggregation(ctx, query, 0, aggregations.NewTermsAggregation("UnreadCounts", "feed_id", len(subscriptionMetadata)))
	if resp != nil && !resp.IsNotFound() {
		return nil, resp
	}
	var categoryCounts aggregations.TermsAggregationResults
	if !resp.IsNotFound() {
		categoryCounts.StringTermsAggregate, err = aggregations.ExtractAggregation[*types.StringTermsAggregate](aggResults.Aggregations, "UnreadCounts")
		if err != nil {
			return nil, fmt.Errorf("unable to get subscription unread counts: %w", err)
		}
	}

	return &categoryCounts, nil
}

func (a *API) getSubscriptions(ctx context.Context, ids ...models.SubscriptionID) (models.SubscriptionsSlice, error) {
	user, err := models.UserFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("getSubscriptions: %w", err)
	}
	allFavorites := user.GetFavorites().FilterByType(models.FavoriteTypeSubscription)
	// Get the subscription states.
	var allMetadata models.SubscriptionMetadataSlice
	if len(ids) > 0 {
		allMetadata = user.GetSubscriptionMetadata().FilterByIDs(ids...)
	} else {
		allMetadata = user.GetSubscriptionMetadata()
	}
	// Get unread counts.
	unreadCounts, err := a.getSubscriptionUnreadCounts(ctx, allMetadata)
	if err != nil {
		return nil, fmt.Errorf("getSubscriptions: %w", err)
	}
	// Get feed data for subscriptions.
	feeds, err := a.DataAPI().GetFeeds(ctx, allMetadata.GetFeedIDs()...)
	if err != nil {
		return nil, fmt.Errorf("getSubscriptions: %w", err)
	}
	// Generate subscriptions from data sources.
	subscriptions := make(models.SubscriptionsSlice, 0, len(feeds))
	for feed := range slices.Values(feeds) {
		var metadata *models.SubscriptionMetadata
		var count int
		if metadata = allMetadata.GetByFeedID(feed.GetID()); metadata == nil {
			slogctx.FromCtx(ctx).Warn("No subscription state for retrieved feed.",
				slog.String("feed_id", feed.GetID()),
			)
			continue
		}
		if unreadCounts.HasResults() {
			count = unreadCounts.GetCount(feed.GetID())
		}

		subscription, err := models.GenerateSubscription(metadata, feed, count, allFavorites.HasFavorite(metadata.GetID()))
		if err != nil {
			slogctx.FromCtx(ctx).Warn("Could not generate subscription from data.",
				slog.Any("error", err),
			)
			continue
		}
		subscriptions = append(subscriptions, subscription)
	}

	return subscriptions, nil
}

func (a *API) filterSubscriptions(ctx context.Context, filters *models.SubscriptionFilters) (models.SubscriptionsSlice, models.Pagination, error) {
	// Get subscriptions by ID.
	subscriptions, err := a.getSubscriptions(ctx, filters.Subscriptions...)
	if err != nil {
		return nil, "", fmt.Errorf("filterSubscriptions: %w", err)
	}
	// Filter subscriptions.
	sort := filters.GetSort()
	subscriptions = subscriptions.FilterByCategories(filters.Categories...).
		FilterByView(filters.View).
		Sort(&sort)
	// Set up pagination.
	var pagination string
	if filters.Pagination != "" {
		pagination = filters.Pagination
	}
	subscriptions, pagination = subscriptions.Paginate(pagination, filters.GetCount())
	return subscriptions, pagination, nil
}

func (a *API) getSubscriptionStats(ctx context.Context, filters *models.SubscriptionFilters) (map[models.SubscriptionID]models.SubscriptionStats, error) {
	user, err := models.UserFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("getFeedStats: %w", err)
	}

	// Get the subscription metadata.
	var metadata models.SubscriptionMetadataSlice
	if len(filters.Subscriptions) > 0 {
		metadata = user.GetSubscriptionMetadata().FilterByIDs(filters.Subscriptions...)
	} else {
		metadata = user.GetSubscriptionMetadata()
	}
	// Build query.
	query := query.Bool(
		query.BoolQueryName("feed_stats_query"),
		query.Filter(
			// Must match any of the given feed IDs.
			query.Terms("feed_id", filters.Subscriptions...),
			query.Since("@timestamp", time.Now().UTC().Add(-24*30*time.Hour)),
			query.Bool(
				query.Should(buildSubscriptionQueries(user, filters.GetView(), metadata...)...),
			),
		),
	)
	// Build aggregations.
	termsField := "feed_id"
	termsCount := len(metadata)
	dateHistoField := "@timestamp"
	dateFormat := "yyyy-MM-dd"
	aggs := aggregations.Aggs{
		"feed": types.Aggregations{
			Terms: &types.TermsAggregation{
				Field: &termsField,
				Size:  &termsCount,
			},
			Aggregations: map[string]types.Aggregations{
				"updates_per_day": {
					DateHistogram: &types.DateHistogramAggregation{
						Field:            &dateHistoField,
						CalendarInterval: &calendarinterval.Day,
						Format:           &dateFormat,
					},
				},
				"avg_daily_updates": {
					AvgBucket: &types.AverageBucketAggregation{
						BucketsPath: "updates_per_day._count",
					},
				},
			},
		},
	}

	results, resp := a.DataAPI().ItemsAggregation2(ctx, query, len(metadata), aggs)
	if resp != nil && !resp.IsNotFound() {
		return nil, resp
	}
	feedStats, ok := results.Aggregations["feed"].(*types.StringTermsAggregate)
	if !ok {
		return nil, fmt.Errorf("unable to get feed stats: feed aggregations invalid")
	}
	feedStatsBuckets, ok := feedStats.Buckets.([]types.StringTermsBucket)
	if !ok {
		return nil, fmt.Errorf("unable to get feed stats: feed aggregations invalid")
	}

	stats := make(map[models.FeedID]models.SubscriptionStats)

	for feed := range slices.Values(feedStatsBuckets) {
		feedID, ok := feed.Key.(string)
		if !ok {
			slogctx.FromCtx(ctx).Debug("Unable to extract feed ID for aggregation", slog.Any("feed_id", feed.Key))
			continue
		}
		updatesResult, ok := feed.Aggregations["avg_daily_updates"].(*types.SimpleValueAggregate)
		if !ok {
			slogctx.FromCtx(ctx).Debug("Unable to extract avg_daily_updates agg for subscription", slog.String("subscription", user.GetSubscriptionMetadata().GetByFeedID(feedID).GetID()))
			continue
		}

		stats[user.GetSubscriptionMetadata().GetByFeedID(feedID).GetID()] = models.SubscriptionStats{
			AvgDailyUpdates: float64(*updatesResult.Value),
		}
	}
	return stats, nil
}

func (a *API) markSubscriptions(ctx context.Context, request *models.MarkSubscriptionsRequest) error {
	user, err := models.UserFromCtx(ctx)
	if err != nil {
		return fmt.Errorf("markSubscriptions: %w", err)
	}
	// Validate parameters.
	valid, err := request.Valid()
	if err != nil || !valid {
		return fmt.Errorf("markSubscriptions: %w", err)
	}
	// Mark user subscriptions.
	user.MarkSubscriptions(request.Mark, request.Subscriptions...)
	// Update the user.
	err = a.DataAPI().UpdateUser(ctx, map[string]any{
		"subscriptions": user.GetSubscriptionMetadata(),
	})
	if err != nil {
		return fmt.Errorf("markSubscriptions: %w", err)
	}
	return nil
}

type (
	addSubscriptionRequests map[*models.SubscriptionRequest]*models.Feed
)

// feedURLs retrieves the URLs from the subscription requests.
func (r addSubscriptionRequests) feedURLs() []string {
	urls := make([]string, 0, len(r))
	for req := range maps.Keys(r) {
		urls = append(urls, req.URL)
	}
	return urls
}

// matchFeedsToSubscriptionRequests takes a list of subscription requests, extracts the URLs in each and attempt to
// match them to existing feeds. Where there is no existing feed, it will attempt to generate new feed data. It then
// stores the subscriptions that need new feeds and any with existing feeds in the context for the next handler.
func (r addSubscriptionRequests) matchFeedsToSubscriptionRequests(ctx context.Context, api *elastic.API) (layouts.AddSubscriptionResults, error) {
	// Extract user data.
	user, err := models.UserFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("matchFeedsToSubscriptions: %w", err)
	}

	slogctx.FromCtx(ctx).Debug("Matching existing feeds to subscription requests...")

	// Paginate and gather all feeds matching the request URLs.
	var (
		feedPagination *models.Pagination
		existingFeeds  models.Feeds
	)
	for {
		count := 100
		feeds, nextResults, err := api.SearchFeeds(ctx, query.Terms("source_urls", r.feedURLs()...), count, nil, feedPagination)
		if err != nil {
			return nil, fmt.Errorf("matchFeedsToSubscriptions: %w", err)
		}

		existingFeeds = append(existingFeeds, feeds...)

		if len(feeds) < count {
			break
		}
		feedPagination = &nextResults
	}

	results := make(layouts.AddSubscriptionResults)
	feedsNeeded := make(addSubscriptionRequests)

	// Loop over existing feeds.
	for request := range r {
		existingFeed := existingFeeds.FindByURL(request.GetURL())
		switch {
		case existingFeed == nil:
			// No existing feed, create a new one.
			newFeed, err := models.NewFeedFromURL(ctx, request.GetURL())
			if err != nil {
				msg := fmt.Sprintf("The feed URL %q cannot be parsed as a feed source or is not a valid URL.", request.GetURL())
				request.URLErr = errors.New(msg)
				results[request] = models.NewSubscriptionResult(nil, models.NewErrorMessage(msg, ""))
				continue
			}
			valid, err := validation.ValidateStruct(newFeed)
			if !valid || err != nil {
				msg := fmt.Sprintf("The feed URL %q cannot be parsed as a feed source or is not a valid URL.", request.GetURL())
				request.URLErr = errors.New(msg)
				results[request] = models.NewSubscriptionResult(nil, models.NewErrorMessage(msg, ""))
				continue
			}
			feedsNeeded[request] = newFeed
			slogctx.FromCtx(ctx).Debug("New feed needed for subscription.",
				slog.String("url", request.GetURL()),
				slog.String("feed", newFeed.GetTitle()),
			)
		case user.IsSubscribedToFeed(existingFeed.GetID()):
			// User already subscribed, ignore request.
			msg := "Already subscribed to " + existingFeed.GetTitle()
			request.URLErr = errors.New(msg)
			results[request] = models.NewSubscriptionResult(nil, models.NewWarningMessage(msg, ""))
		default:
			// Existing feed.
			r[request] = existingFeed
			slogctx.FromCtx(ctx).Debug("Existing feed for subscription.",
				slog.String("url", request.GetURL()),
				slog.String("feed", existingFeed.GetTitle()),
			)
		}
	}
	// Add new feeds for requests without an existing feed.
	if len(feedsNeeded) > 0 {
		newFeedsNeededResults, err := feedsNeeded.createNewFeeds(ctx, api)
		if err != nil {
			return nil, fmt.Errorf("matchFeedsToSubscriptions: %w", err)
		}
		maps.Copy(r, feedsNeeded)
		maps.Copy(results, newFeedsNeededResults)
	}

	return results, nil
}

func (r addSubscriptionRequests) createNewFeeds(ctx context.Context, api *elastic.API) (layouts.AddSubscriptionResults, error) {
	slogctx.FromCtx(ctx).Debug("Adding new feeds for subscriptions.")
	results := make(layouts.AddSubscriptionResults)

	// Testing no-op.
	// return results, nil

	// Add the new feeds.
	index, err := elastic.FeedsWriteIndexFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("createNewFeeds: %w", err)
	}
	addFeedsResults, err := elastic.BulkAdd(ctx, api, index, slices.Collect(maps.Values(r))...)
	if err != nil && !errors.Is(err, bulk.ErrBulkHasErrors) {
		return nil, fmt.Errorf("createNewFeeds: %w", err)
	}

	// Process the add feed results.
	for request, feed := range r {
		resp, found := addFeedsResults[feed.GetID()]
		if found {
			if resp.Created() {
				// Success, add request to map of subscription needed.
				r[request] = feed
			} else {
				results[request] = models.NewSubscriptionResult(nil, &models.UserMessage{
					Status:  models.UserMessageStatusError,
					Summary: "Internal Error. ",
					Details: "An internal, irrecoverable backend error occurred trying to add a subscription for the URL " + request.GetURL(),
				})
			}
		}
	}
	return results, nil
}

// AddSubscriptions handles adding new subscription via either the add or import user functionality. It
// handles: matching and filtering out requests against existing subscriptions, matching requests to existing feeds,
// creating new feeds as necessary and finally creating user subscriptions.
func (r addSubscriptionRequests) createNewSubscriptions(ctx context.Context, api *elastic.API) (layouts.AddSubscriptionResults, error) {
	// Extract user data.
	user, err := models.UserFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("createNewSubscriptions: %w", err)
	}

	slogctx.FromCtx(ctx).Debug("Adding new subscriptions.")

	// Loop through the subscriptions adding their state to the existing subscription states slice. For any
	// subscriptions that have customisation data, collect the customisation data for adding later.
	results := make(layouts.AddSubscriptionResults)
	allMetadata := make(models.SubscriptionMetadataSlice, 0, len(r))
	for request, feed := range r {
		// // Ignore requests that have already got a message response, indicating some kind of failure or warning.
		// if r[request] != nil {
		// 	continue
		// }
		// Generate metadata and add to metadata slice.
		metadata := models.NewSubscriptionMetadata(user, feed, request)
		valid, err := metadata.Valid()
		if err != nil || !valid {
			slogctx.FromCtx(ctx).Debug("Invalid subscription metadata.",
				slog.Any("error", err),
				slog.String("feed_id", feed.GetID()),
				slog.String("feed", feed.GetTitle()),
				slog.String("url", request.GetURL()),
			)
			results[request] = models.NewSubscriptionResult(nil, &models.UserMessage{
				Status:  models.UserMessageStatusError,
				Summary: "Subscription creation failed",
				Details: request.GetURL(),
			})
			continue
		}
		allMetadata = append(allMetadata, metadata)
		// Generate subscription and add to results map.
		subscription, err := models.GenerateSubscription(metadata, feed, 0, false)
		if err != nil {
			results[request] = models.NewSubscriptionResult(nil, &models.UserMessage{
				Status:  models.UserMessageStatusError,
				Summary: "Subscription creation failed",
				Details: request.GetURL(),
			})
			continue
		}
		results[request] = models.NewSubscriptionResult(subscription, &models.UserMessage{
			Status:  models.UserMessageStatusSuccess,
			Summary: "Subscription Created: " + feed.GetTitle(),
			Details: "Articles will be fetched shortly...",
		})
	}

	// Testing no-op.
	// return results, nil

	// Add the subscription states.
	if len(allMetadata) > 0 {
		user.AddSubscriptions(allMetadata...)
		// Disable onboarding once a subscription has been added.
		settings := user.GetSettings()
		if settings.ShowOnboarding {
			settings.ShowOnboarding = false
		}
		// Update the user object.
		err = api.UpdateUser(ctx, map[string]any{
			"subscriptions": user.Subscriptions,
			"settings":      settings,
		})
		if err != nil {
			return nil, fmt.Errorf("createNewSubscriptions: %w", err)
		}
	}
	return results, nil
}

func decodeSubscriptionFilters(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		filters, valid, err := forms.DecodeForm[*models.SubscriptionFilters](req)
		if err != nil || !valid {
			slogctx.FromCtx(req.Context()).Warn("Invalid subscription filters. Using defaults.",
				slog.Any("error", err),
			)
			ctx = subscriptionFiltersToCtx(ctx, models.NewSubscriptionFilters())
		} else {
			ctx = subscriptionFiltersToCtx(ctx, *filters)
		}
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

// savePageState saves the current page state in the session.
func saveSubscriptionFilters(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		session.FiltersToSession(req.Context(), subscriptionFiltersFromCtx(req.Context()))
		next.ServeHTTP(res, req)
	})
}
