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
		err := params.Valid()
		if err != nil {
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

// FindSimilar handles finding objects similar to the given objects and showing the results.
func FindSimilar(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Extract request parameters.
		params := &models.ObjectParams{
			ObjectID: chi.URLParam(req, models.ParamObjectID),
			Object:   models.ObjectType(chi.URLParam(req, models.ParamObjectType)),
		}
		err := params.Valid()
		if err != nil {
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
		err := params.Valid()
		if err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Server could not complete request", "This might be temporary, please try again."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("%w: %w", ErrInvalidRequestParams, err), http.StatusUnprocessableEntity)
		}
		currentURL, found := htmx.GetCurrentURL(req)
		if !found {
			slogctx.FromCtx(req.Context()).Warn("No HX-Current-URL header found.")
		}
		template := templates.ReportObjectIssues(string(params.Object), params.ObjectID, models.NewObjectIssue(params, currentURL))
		renderPage(template, templates.GeneratePageTitle("Report an issue")).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// SubmitObjectIssues handles processing the issue form and creating a github issue with the details.
func SubmitObjectIssues(esapi *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Extract the issue request details.
		request, valid, err := forms.DecodeForm[*models.ObjectIssueRequest](req)
		if err != nil || !valid {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Server could not complete request", "This might be temporary, please try again."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("%w: %w", ErrInvalidRequestParams, err), http.StatusUnprocessableEntity)
		}
		// // Extract any attached screenshot.
		// screenshot, err := forms.DecodeMultipartFile2(req, "screenshot")
		// // Validate the screenshot is an image file.
		// mimeType, err := screenshot.ParseMimetype()
		// if err != nil {
		// 	renderPartial(templates.ServerErrorNotification(
		// 		models.NewErrorMessage("Invalid request data", "This might be temporary, please try again."),
		// 	)).ServeHTTP(res, req)
		// 	return models.NewAPIError(fmt.Errorf("%w: %w", ErrInvalidRequestParams, err), http.StatusUnprocessableEntity)
		// }
		// if !slices.Contains([]string{"image/jpeg", "image/png"}, mimeType) {
		// 	renderPartial(templates.ServerErrorNotification(
		// 		models.NewErrorMessage("Invalid request data", "This might be temporary, please try again."),
		// 	)).ServeHTTP(res, req)
		// 	return models.NewAPIError(fmt.Errorf("%w: %w", ErrInvalidRequestParams, err), http.StatusUnprocessableEntity)
		// }

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
