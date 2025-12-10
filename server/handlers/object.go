// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
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
		if err := params.Valid(); err != nil {
			renderPage(
				wrapContent(req, templates.NotFound()),
			).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				http.StatusUnprocessableEntity,
			)
		}
		switch params.Object {
		case models.ObjectTypeArticle:
			articles, err := api.GetArticles(req.Context(), params.ObjectID)
			if err != nil {
				msg := models.NewErrorMessage(
					"Server could not complete request!",
					"This might be temporary, please try again.",
				)
				renderPage(
					wrapContent(req, templates.ErrorPage(msg)),
				).ServeHTTP(res, req)
				return models.NewAPIError(
					fmt.Errorf("unable to fetch article content: %w", err),
					http.StatusInternalServerError,
				)
			}
			article := articles[0]
			ctx := templates.PageTitleToCtx(req.Context(), article.GetTitle()+" | "+article.GetFeedTitle()+" | ")
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
				if content, err := models.ExtractTextFromURL(article.GetLink()); err != nil {
					// Couldn't fetch remote article content, show an error message.
					remoteContentErrMsg = templates.Notification(
						models.NewErrorMessage("Unable to fetch article remote content", ""), 0,
					)
					article.ShowFullContent = false
				} else {
					if content == article.Content {
						// Remote article content is the same as feed content, show an info message.
						remoteContentErrMsg = templates.Notification(
							models.NewInfoMessage("No remote content available", "Page returned existing content."), templates.DefaultNotificationTimeout,
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
			renderPage(wrapContent(req.WithContext(ctx), template)).ServeHTTP(res, req.WithContext(ctx))
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
		if err := params.Valid(); err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage(
					"Server could not complete request!",
					"This might be temporary, please try again.",
				),
			)).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				http.StatusUnprocessableEntity,
			)
		}
		switch params.Object {
		case models.ObjectTypeArticle:
			articles, err := api.FindSimilarArticles(req.Context(), params.ObjectID)
			if err != nil {
				renderPartial(templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to find similar articles", ""),
				)).ServeHTTP(res, req)
				return models.NewAPIError(
					fmt.Errorf("find similar articles request failed: %w", err),
					http.StatusUnprocessableEntity,
				)
			}
			// Show results.
			var template templ.Component
			if len(articles) > 0 {
				template = templates.SimilarArticles(articles)
			} else {
				template = templates.NoSearchResults()
			}
			ctx := templates.PageTitleToCtx(req.Context(), "Similar Articles")
			renderPage(wrapContent(req.WithContext(ctx), template)).ServeHTTP(res, req.WithContext(ctx))
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
