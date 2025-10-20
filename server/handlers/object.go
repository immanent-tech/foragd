// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"
	"github.com/justinas/alice"
	"github.com/justinas/nosurf"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/github"
	"github.com/immanent-tech/foragd/server/forms"
	"github.com/immanent-tech/foragd/web/templates"
)

// ViewObject handles showing an object's content (e.g. viewing article content).
func ViewObject(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Extract request parameters.
		params := &models.ObjectParams{
			ObjectID: chi.URLParam(req, models.ParamObjectID),
			Object:   models.ObjectType(chi.URLParam(req, models.ParamObjectType)),
		}
		valid, err := params.Valid()
		if err != nil || !valid {
			renderPage(templates.NotFound(), templates.GeneratePageTitle("Unknown article")).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				http.StatusNotFound,
			)
		}
		switch params.Object {
		case models.ObjectTypeArticle:
			articles, err := models.GetArticles(req.Context(), api, params.ObjectID)
			if err != nil {
				renderPartial(templates.Error(
					models.NewErrorMessage("Unable to fetch article content", ""),
				)).ServeHTTP(res, req)
				return models.NewAPIError(err, http.StatusInternalServerError)
			}
			article := articles[0]
			// For POST method, get the "show_full_content" value and override the article value.
			fullContent, err := strconv.ParseBool(req.FormValue(models.ParamFullArticleContent))
			if err != nil || !fullContent {
				article.ShowFullContent = false
			} else if fullContent {
				article.ShowFullContent = fullContent
			}
			// Fetch and set remote content if required.
			if article.ShowFullContent {
				slogctx.FromCtx(req.Context()).Debug("Fetching article remote content.")
				content, err := fetchArticleRemoteContent(article.GetLink())
				if err != nil {
					renderPartial(templates.Notification(
						models.NewErrorMessage("Unable to fetch article remote content", ""), 0,
					)).ServeHTTP(res, req)
					article.ShowFullContent = false
				} else {
					if content == article.Content {
						renderPartial(templates.Notification(
							models.NewWarningMessage("Could not fetch full article content", "Page returned existing content."), 10*time.Second,
						)).ServeHTTP(res, req)
					}
					article.Content = content
				}
			}
			// Render appropriate content.
			template := templates.ArticleContent(article)
			ctx := models.CSRFTokenToCtx(req.Context(), nosurf.Token(req))
			renderPage(template, templates.GeneratePageTitle("Articles")).ServeHTTP(res, req.WithContext(ctx))
		default:
			res.WriteHeader(http.StatusNotImplemented)
		}
		return nil
	})).ServeHTTP
}

// MarkObject handles marking an object as read or unread and updating the UI appropriately.
func MarkObject(api *elastic.API) http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Extract request parameters.
		params := &models.MarkObjectParams{
			ObjectID: chi.URLParam(req, models.ParamObjectID),
			Object:   models.ObjectType(chi.URLParam(req, models.ParamObjectType)),
			Mark:     models.Mark(chi.URLParam(req, models.ParamMark)),
		}
		valid, err := params.Valid()
		if err != nil || !valid {
			msg := models.NewErrorMessage("An error occurred processing the request", "Please try again.")
			template := templates.Notification(msg, 0)
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		// Extract user data.
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			return models.NewAPIError(
				err,
				http.StatusBadRequest,
			)
		}
		switch params.Object {
		case models.ObjectTypeSubscription:
			user.MarkSubscriptions(params.Mark, params.ObjectID)
			// Update the user object.
			err = api.UpdateUser(req.Context(), user.GetID(), map[string]any{
				"subscriptions": user.Subscriptions,
			})
			if err != nil {
				renderPartial(templates.Notification(
					models.NewErrorMessage(
						"Unable to mark article",
						"Could not update user data.",
					), 0))
				return models.NewAPIError(
					fmt.Errorf("unable to mark article: %w", err),
					http.StatusUnprocessableEntity)
			}
			// Client side refresh of page.
			SetRedirect(req.Context(), "/list/subscriptions", models.PageFiltersFromCtx(req.Context(), "/list/subscriptions"), res)
			res.WriteHeader(http.StatusOK)
		case models.ObjectTypeArticle:
			subscriptionID := req.FormValue(models.ParamSubscriptionID)
			if subscriptionID == "" {
				return models.NewAPIError(
					fmt.Errorf("%w: unknown subscription ID", ErrInvalidRequestParams),
					http.StatusBadRequest,
				)
			}
			user.MarkItems(params.Mark, subscriptionID, params.ObjectID)
			// Update the user object.
			err = api.UpdateUser(req.Context(), user.GetID(), map[string]any{
				"subscriptions": user.Subscriptions,
			})
			if err != nil {
				renderPartial(templates.Notification(
					models.NewErrorMessage(
						"Unable to mark article",
						"Could not update user data.",
					), 0))
				return models.NewAPIError(
					fmt.Errorf("unable to mark article: %w", err),
					http.StatusUnprocessableEntity)
			}
			// Get updated articles.
			s, err := models.GetArticles(req.Context(), api, params.ObjectID)
			if err != nil || len(s) == 0 || len(s) > 1 {
				renderPartial(templates.Notification(
					models.NewErrorMessage(
						"Unable to mark object",
						"Could not refresh object.",
					), 0))
				return models.NewAPIError(
					fmt.Errorf("unable to mark object: %w", err),
					http.StatusUnprocessableEntity)
			}
			// Generate appropriate swap content based on target header.
			switch req.Header.Get(htmx.HeaderTarget) {
			case params.ObjectID:
				// Swap target is card.
				filters := models.PageFiltersFromCtx(req.Context(), req.URL.Path)
				renderPartial(templates.ArticleCard(s[0], filters.GetView())).ServeHTTP(res, req)
			case "mark_" + params.ObjectID:
				// Swap target is link.
				renderPartial(templates.UpdateViewArticleMark(s[0])).ServeHTTP(res, req)
			}
		default:
			res.WriteHeader(http.StatusNotImplemented)
		}
		return nil
	})).ServeHTTP
}

// FindSimilar handles finding objects similar to the given objects and showing the results.
func FindSimilar(api *elastic.API) http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Extract request parameters.
		params := &models.ObjectParams{
			ObjectID: chi.URLParam(req, models.ParamObjectID),
			Object:   models.ObjectType(chi.URLParam(req, models.ParamObjectType)),
		}
		valid, err := params.Valid()
		if err != nil || !valid {
			renderPage(templates.NotFound(), templates.GeneratePageTitle("Unknown article")).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				http.StatusNotFound,
			)
		}
		switch params.Object {
		case models.ObjectTypeArticle:
			articles, err := models.FindSimilarArticles(req.Context(), api, params.ObjectID)
			if err != nil {
				renderPartial(templates.Notification(
					models.NewErrorMessage("Unable to find similar articles", ""), 0))
				return models.NewAPIError(err, http.StatusInternalServerError)
			}
			// Show results.
			template := templates.SimilarArticles(articles)
			renderPage(template, templates.GeneratePageTitle("Similar Articles")).ServeHTTP(res, req)
		default:
			res.WriteHeader(http.StatusNotImplemented)
		}
		return nil
	})).ServeHTTP
}

// ConfirmRemoveObject handles showing a confirmation dialog for removing (unsubscribing) from a
// subscription.
func ConfirmRemoveObject(api *elastic.API) http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Extract request parameters.
		params := &models.ObjectParams{
			ObjectID: chi.URLParam(req, models.ParamObjectID),
			Object:   models.ObjectType(chi.URLParam(req, models.ParamObjectType)),
		}
		valid, err := params.Valid()
		if err != nil || !valid {
			msg := models.NewErrorMessage("An error occurred processing the request", "Please try again.")
			template := templates.Notification(msg, 0)
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		switch params.Object {
		case models.ObjectTypeSubscription:
			subscriptions, err := models.GetSubscriptions(req.Context(), api, params.ObjectID)
			if err != nil || len(subscriptions) == 0 || len(subscriptions) > 1 {
				msg := models.NewErrorMessage("An error occurred processing the request", "Please try again.")
				template := templates.Notification(msg, 0)
				renderPartial(template).ServeHTTP(res, req)
				return models.NewAPIError(err, http.StatusInternalServerError)
			}
			// filters := models.ListFiltersFromCtx(req.Context())
			// stats, err := models.GetSubscriptionStats(req.Context(), api, filters)
			// if err != nil {
			// 	renderPartial(templates.Notification(
			// 		models.NewErrorMessage(
			// 			"Unable to refresh subscription",
			// 			"Something went wrong, please try again",
			// 		), 0))
			// 	return models.NewAPIError(
			// 		fmt.Errorf("unable to mark subscription: %w", err),
			// 		http.StatusInternalServerError)
			// }
			// subscriptionStats := stats[subscriptions[0].GetID()]
			renderPartial(templates.UnsubscribeModal(subscriptions[0])).ServeHTTP(res, req)
		default:
			res.WriteHeader(http.StatusNotImplemented)
		}
		return nil
	})).ServeHTTP
}

func RemoveObject(api *elastic.API) http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Extract request parameters.
		params := &models.ObjectParams{
			ObjectID: chi.URLParam(req, models.ParamObjectID),
			Object:   models.ObjectType(chi.URLParam(req, models.ParamObjectType)),
		}
		valid, err := params.Valid()
		if err != nil || !valid {
			msg := models.NewErrorMessage("An error occurred processing the request", "Please try again.")
			template := templates.Notification(msg, 0)
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		// Retrieve user object.
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			return fmt.Errorf("unable to process subscription removal: %w", err)
		}
		switch params.Object {
		case models.ObjectTypeSubscription:
			// Remove metadata for given subscriptions from user.
			user.RemoveSubscriptions(params.ObjectID)
			// Update the user.
			err = api.UpdateUser(req.Context(), user.GetID(), map[string]any{
				"subscriptions": user.GetSubscriptionMetadata(),
			})
			if err != nil {
				msg := models.NewErrorMessage("Unable to remove subscription", "Please try again.")
				template := templates.Notification(msg, 0)
				renderPartial(template).ServeHTTP(res, req)
				return models.NewAPIError(err, http.StatusInternalServerError)
			}
			// Show success notification.
			msg := models.NewSuccessMessage("Unsubscribed!", "")
			renderPartial(templates.Notification(msg, 0)).ServeHTTP(res, req)
		default:
			res.WriteHeader(http.StatusNotImplemented)
		}
		return nil
	})).ServeHTTP
}

// GetObjectIssues presents a form for entering issues about a particular object (subscription/article).
func GetObjectIssues(api *elastic.API) http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Extract request parameters.
		params := &models.ObjectParams{
			ObjectID: chi.URLParam(req, models.ParamObjectID),
			Object:   models.ObjectType(chi.URLParam(req, models.ParamObjectType)),
		}
		valid, err := params.Valid()
		if err != nil || !valid {
			renderPage(templates.NotFound(), templates.GeneratePageTitle("Unknown article")).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				http.StatusNotFound,
			)
		}
		currentURL, found := htmx.GetCurrentURL(req)
		var template templ.Component
		switch params.Object {
		case models.ObjectTypeSubscription:
			s, err := models.GetSubscriptions(req.Context(), api, params.ObjectID)
			if err != nil || len(s) == 0 {
				msg := models.NewErrorMessage(
					"Unable to create report form.",
					"The backend had issues generating the report form. Please try again.",
				)
				renderPartial(templates.ServerErrorNotification(msg)).ServeHTTP(res, req)
				return models.NewAPIError(err, http.StatusInternalServerError)
			}
			template = templates.ReportObjectIssues(s[0], models.NewObjectIssue(params, currentURL))

		case models.ObjectTypeArticle:
			// Get the current URL on which the issue is being reported.
			if !found {
				slogctx.FromCtx(req.Context()).Warn("No HX-Current-URL header found.")
			}
			i, err := models.GetArticles(req.Context(), api, params.ObjectID)
			if err != nil || len(i) == 0 {
				msg := models.NewErrorMessage(
					"Unable to create report form.",
					"The backend had issues generating the report form. Please try again.",
				)
				renderPartial(templates.ServerErrorNotification(msg)).ServeHTTP(res, req)
				return models.NewAPIError(err, http.StatusInternalServerError)
			}
			template = templates.ReportObjectIssues(i[0], models.NewObjectIssue(params, currentURL))
		default:
			res.WriteHeader(http.StatusNotImplemented)

		}
		renderPage(template, templates.GeneratePageTitle("Report an issue")).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

func SubmitObjectIssues(esapi *elastic.API, ghapi *github.Client) http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Validate the subscription issue request.
		request, valid, err := forms.DecodeForm[*models.ObjectIssueRequest](req)
		if err != nil || !valid {
			msg := models.NewErrorMessage(
				"Unable to submit issue.",
				"The backend had issues submitting the report. Please try again.",
			)
			renderPartial(templates.ServerErrorNotification(msg)).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		// Create the issue in Github.
		err = ghapi.CreateObjectIssue(req.Context(), request)
		if err != nil {
			msg := models.NewErrorMessage(
				"Unable to submit issue.",
				"The backend had issues submitting the report. Please try again.",
			)
			renderPartial(templates.ServerErrorNotification(msg))
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
