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
				http.StatusUnprocessableEntity,
			)
		}
		switch params.Object {
		case models.ObjectTypeArticle:
			articles, err := models.GetArticles(req.Context(), api, params.ObjectID)
			if err != nil {
				msg := models.NewErrorMessage("Server could not complete request!", "This might be temporary, please try again.")
				renderPage(templates.ErrorPage(msg), templates.GeneratePageTitle("View Article")).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("unable to fetch article content: %w", err), http.StatusInternalServerError)
			}
			article := articles[0]
			pageTitle := templates.GeneratePageTitle(article.GetTitle())
			// Get the "show_full_content" value and override the article value.
			fullContent, err := strconv.ParseBool(req.FormValue(models.ParamFullArticleContent))
			if err != nil || !fullContent {
				article.ShowFullContent = false
			} else if fullContent {
				article.ShowFullContent = fullContent
			}
			var remoteContentErrMsg templ.Component
			// Fetch and set remote content if required.
			if article.ShowFullContent {
				content, err := fetchArticleRemoteContent(article.GetLink())
				if err != nil {
					// Couldn't fetch remote article content, show an error message.
					remoteContentErrMsg = templates.Notification(
						models.NewErrorMessage("Unable to fetch article remote content", ""), 0,
					)
					article.ShowFullContent = false
				} else {
					if content == article.Content {
						// Remote article content is the same as feed content, show an info message.
						remoteContentErrMsg = templates.Notification(
							models.NewInfoMessage("No remote content available", "Page returned existing content."), 10*time.Second,
						)
					}
					article.Content = content
				}
			}
			// Render appropriate content.
			var template templ.Component
			if remoteContentErrMsg != nil {
				template = templ.Join(templates.ArticleContent(article), remoteContentErrMsg)
			} else {
				template = templates.ArticleContent(article)
			}
			renderPage(template, pageTitle).ServeHTTP(res, req)
		default:
			res.WriteHeader(http.StatusNotImplemented)
		}
		return nil
	})).ServeHTTP
}

// MarkObject handles marking an object as read or unread and updating the UI appropriately.
func MarkObject(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Extract request parameters.
		params := &models.MarkObjectParams{
			ObjectID: chi.URLParam(req, models.ParamObjectID),
			Object:   models.ObjectType(chi.URLParam(req, models.ParamObjectType)),
			Mark:     models.Mark(chi.URLParam(req, models.ParamMark)),
		}
		valid, err := params.Valid()
		if err != nil || !valid {
			renderPartial(
				templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to mark objects", "The request data is invalid.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("mark object validation error: %w", err), http.StatusUnprocessableEntity)
		}
		// Extract user data.
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			renderPartial(
				templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to mark objects", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to retrieve user data: %w", err), http.StatusInternalServerError)
		}
		switch params.Object {
		case models.ObjectTypeSubscription:
			user.MarkSubscriptions(params.Mark, params.ObjectID)
			// Update the user object.
			err = api.UpdateUser(req.Context(), user.GetID(), map[string]any{
				"subscriptions": user.Subscriptions,
			})
			if err != nil {
				renderPartial(
					templates.ServerErrorNotification(
						models.NewErrorMessage("Unable to mark objects", "This might be a temporary error, please try again.")),
				).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("unable to update user: %w", err), http.StatusInternalServerError)
			}
			// Client side refresh of page.
			SetRedirect(req.Context(), "/list/subscriptions", models.PageFiltersFromCtx(req.Context(), "/list/subscriptions"), res)
			res.WriteHeader(http.StatusOK)
		case models.ObjectTypeArticle:
			subscriptionID := req.FormValue(models.ParamSubscriptionID)
			if subscriptionID == "" {
				renderPartial(
					templates.ServerErrorNotification(
						models.NewErrorMessage("Unable to mark objects", "This might be a temporary error, please try again.")),
				).ServeHTTP(res, req)
				return models.NewAPIError(ErrInvalidRequestParams, http.StatusBadRequest)
			}
			err = models.MarkArticles(req.Context(), api, params.Mark, subscriptionID, params.ObjectID)
			if err != nil {
				renderPartial(
					templates.ServerErrorNotification(
						models.NewErrorMessage("Unable to mark objects", "This might be a temporary error, please try again.")),
				).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("unable to update user: %w", err), http.StatusInternalServerError)
			}
			// Get updated articles.
			articles, err := models.GetArticles(req.Context(), api, params.ObjectID)
			if err != nil || len(articles) == 0 || len(articles) > 1 {
				renderPartial(
					templates.ServerErrorNotification(
						models.NewErrorMessage("Unable to mark objects", "This might be a temporary error, please try again.")),
				).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("could not retrieve updated articles: %w", err), http.StatusInternalServerError)
			}
			// Generate appropriate swap content based on target header.
			switch req.Header.Get(htmx.HeaderTarget) {
			case params.ObjectID:
				// Swap target is card.
				renderPartial(templates.ArticleCard(articles[0])).ServeHTTP(res, req)
			case "mark_" + params.ObjectID:
				// Swap target is link.
				if len(articles) == 1 {
					renderPartial(templates.UpdateViewArticleMark(articles[0])).ServeHTTP(res, req)
				}
				res.WriteHeader(http.StatusNoContent)
			}
		default:
			res.WriteHeader(http.StatusNotImplemented)
		}
		return nil
	})).ServeHTTP
}

// FindSimilar handles finding objects similar to the given objects and showing the results.
func FindSimilar(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Extract request parameters.
		params := &models.ObjectParams{
			ObjectID: chi.URLParam(req, models.ParamObjectID),
			Object:   models.ObjectType(chi.URLParam(req, models.ParamObjectType)),
		}
		valid, err := params.Valid()
		if err != nil || !valid {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Server could not complete request!", "This might be temporary, please try again."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("%w: %w", ErrInvalidRequestParams, err), http.StatusUnprocessableEntity)
		}
		switch params.Object {
		case models.ObjectTypeArticle:
			articles, err := models.FindSimilarArticles(req.Context(), api, params.ObjectID)
			if err != nil {
				renderPartial(templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to find similar articles", ""),
				)).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("find similar articles request failed: %w", err), http.StatusUnprocessableEntity)
			}
			// Show results.
			var template templ.Component
			if len(articles) > 0 {
				template = templates.SimilarArticles(articles)
			} else {
				template = templates.NoSearchResults()
			}
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
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Extract request parameters.
		params := &models.ObjectParams{
			ObjectID: chi.URLParam(req, models.ParamObjectID),
			Object:   models.ObjectType(chi.URLParam(req, models.ParamObjectType)),
		}
		valid, err := params.Valid()
		if err != nil || !valid {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Server could not complete request!", "This might be temporary, please try again."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("%w: %w", ErrInvalidRequestParams, err), http.StatusUnprocessableEntity)
		}
		switch params.Object {
		case models.ObjectTypeSubscription:
			subscriptions, err := models.GetSubscriptions(req.Context(), api, params.ObjectID)
			if err != nil || len(subscriptions) == 0 || len(subscriptions) > 1 {
				renderPartial(templates.ServerErrorNotification(
					models.NewErrorMessage("Server could not complete request!", "This might be temporary, please try again."),
				)).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("could not retrieve subscriptions: %w", err), http.StatusInternalServerError)
			}
			renderPartial(templates.RemoveObjectModal[models.SubscriptionID](subscriptions[0])).ServeHTTP(res, req)
		default:
			res.WriteHeader(http.StatusNotImplemented)
		}
		return nil
	})).ServeHTTP
}

// RemoveObject handles processing a remove object request from the user (e.g., unsubscribing from a feed).
func RemoveObject(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Extract request parameters.
		params := &models.ObjectParams{
			ObjectID: chi.URLParam(req, models.ParamObjectID),
			Object:   models.ObjectType(chi.URLParam(req, models.ParamObjectType)),
		}
		valid, err := params.Valid()
		if err != nil || !valid {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Server could not complete request", "This might be temporary, please try again."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("%w: %w", ErrInvalidRequestParams, err), http.StatusUnprocessableEntity)
		}
		// Retrieve user object.
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			renderPartial(
				templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to remove object", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to retrieve user data: %w", err), http.StatusInternalServerError)
		}
		switch params.Object {
		case models.ObjectTypeSubscription:
			// Remove metadata for given subscriptions from user.
			user.RemoveSubscriptions(params.ObjectID)
			// Update the user.
			err = api.UpdateUser(req.Context(), user.GetID(), map[string]any{
				"subscriptions": user.GetSubscriptions(),
			})
			if err != nil {
				renderPartial(
					templates.ServerErrorNotification(
						models.NewErrorMessage("Unable to remove object", "This might be a temporary error, please try again.")),
				).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("unable update user data: %w", err), http.StatusInternalServerError)
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

// func ShareObject(api *elastic.API) http.HandlerFunc {
// 	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
// 		// Extract request parameters.
// 		params := &models.ObjectParams{
// 			ObjectID: chi.URLParam(req, models.ParamObjectID),
// 			Object:   models.ObjectType(chi.URLParam(req, models.ParamObjectType)),
// 		}
// 		valid, err := params.Valid()
// 		if err != nil || !valid {
// 			renderPartial(templates.ServerErrorNotification(
// 				models.NewErrorMessage("Server could not complete request!", "This might be temporary, please try again."),
// 			)).ServeHTTP(res, req)
// 			return models.NewAPIError(fmt.Errorf("%w: %w", ErrInvalidRequestParams, err), http.StatusUnprocessableEntity)
// 		}
// 		switch params.Object {
// 		case models.ObjectTypeArticle:
// 			articles, err := models.GetArticles(req.Context(), api, params.ObjectID)
// 			if err != nil || len(articles) == 0 || len(articles) > 1 {
// 				renderPartial(templates.ServerErrorNotification(
// 					models.NewErrorMessage("Server could not complete request!", "This might be temporary, please try again."),
// 				)).ServeHTTP(res, req)
// 				return models.NewAPIError(fmt.Errorf("could not retrieve subscriptions: %w", err), http.StatusInternalServerError)
// 			}
// 			renderPartial(templates.ShareObjectModal(articles[0])).ServeHTTP(res, req)
// 		default:
// 			res.WriteHeader(http.StatusNotImplemented)
// 		}
// 		return nil
// 	})).ServeHTTP
// }

// GetObjectIssues presents a form for entering issues about a particular object (subscription/article).
func GetObjectIssues(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Extract request parameters.
		params := &models.ObjectParams{
			ObjectID: chi.URLParam(req, models.ParamObjectID),
			Object:   models.ObjectType(chi.URLParam(req, models.ParamObjectType)),
		}
		valid, err := params.Valid()
		if err != nil || !valid {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Server could not complete request", "This might be temporary, please try again."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("%w: %w", ErrInvalidRequestParams, err), http.StatusUnprocessableEntity)
		}
		currentURL, found := htmx.GetCurrentURL(req)
		var template templ.Component
		switch params.Object {
		case models.ObjectTypeSubscription:
			subscriptions, err := models.GetSubscriptions(req.Context(), api, params.ObjectID)
			if err != nil || len(subscriptions) == 0 {
				renderPartial(
					templates.ServerErrorNotification(
						models.NewErrorMessage("Unable to process request", "This might be a temporary error, please try again.")),
				).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("unable to retrieve subscription details: %w", err), http.StatusInternalServerError)
			}
			template = templates.ReportObjectIssues[models.SubscriptionID](subscriptions[0], models.NewObjectIssue(params, currentURL))
		case models.ObjectTypeArticle:
			// Get the current URL on which the issue is being reported.
			if !found {
				slogctx.FromCtx(req.Context()).Warn("No HX-Current-URL header found.")
			}
			articles, err := models.GetArticles(req.Context(), api, params.ObjectID)
			if err != nil || len(articles) == 0 {
				renderPartial(
					templates.ServerErrorNotification(
						models.NewErrorMessage("Unable to process request", "This might be a temporary error, please try again.")),
				).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("unable to retrieve article details: %w", err), http.StatusInternalServerError)
			}
			template = templates.ReportObjectIssues[models.ItemID](articles[0], models.NewObjectIssue(params, currentURL))
		default:
			res.WriteHeader(http.StatusNotImplemented)
		}
		renderPage(template, templates.GeneratePageTitle("Report an issue")).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// SubmitObjectIssues handles processing the issue form and creating a github issue with the details.
func SubmitObjectIssues(esapi *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Validate the subscription issue request.
		request, valid, err := forms.DecodeForm[*models.ObjectIssueRequest](req)
		if err != nil || !valid {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Server could not complete request", "This might be temporary, please try again."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("%w: %w", ErrInvalidRequestParams, err), http.StatusUnprocessableEntity)
		}
		err = github.Connect()
		if err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to submit issue", "This might be a temporary issue, please try again."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to connect to github: %w", err), http.StatusInternalServerError)
		}
		// Create the issue in Github.
		err = github.CreateObjectIssue(req.Context(), request)
		if err != nil {
			renderPartial(
				templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to submit issue", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to create issue in github: %w", err), http.StatusInternalServerError)
		}
		// Force refresh of page.
		msg := models.NewInfoMessage(
			"Thanks for reporting the issue!",
			"We will look into it and implement fixes as appropriate.",
		)
		renderPage(templates.IssueReportedConfirmation(msg), templates.GeneratePageTitle("Report subscription issue")).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}
