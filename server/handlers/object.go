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
	return defaultHandlerChain.ThenFunc(showOnError(func(res http.ResponseWriter, req *http.Request) error {
		// Extract request parameters.
		params := &models.ObjectParams{
			ObjectID: chi.URLParam(req, models.ParamObjectID),
			Object:   models.ObjectType(chi.URLParam(req, models.ParamObjectType)),
		}
		if err := params.Valid(); err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to view object",
					"There was a problem with the request. Please try again.",
				),
			}
		}
		switch params.Object {
		case models.ObjectTypeArticle:
			articles, err := api.GetArticles(req.Context(), params.ObjectID)
			if err != nil {
				return &models.APIError{
					InternalError: fmt.Errorf("get article content: %w", err),
					StatusCode:    http.StatusUnprocessableEntity,
					UserMessage: models.NewErrorMessage(
						"Unable to view object",
						"There was a problem with the request. Please try again.",
					),
				}
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
				if content, err := models.ExtractArticleFromURL(article.GetLink()); err != nil {
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
	return defaultHandlerChain.ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
		// Extract request parameters.
		params := &models.ObjectParams{
			ObjectID: chi.URLParam(req, models.ParamObjectID),
			Object:   models.ObjectType(chi.URLParam(req, models.ParamObjectType)),
		}
		if err := params.Valid(); err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("decode request: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to find similar",
					"There was a problem with the request. Please try again.",
				),
			}
		}
		switch params.Object {
		case models.ObjectTypeArticle:
			articles, err := api.FindSimilarArticles(req.Context(), params.ObjectID)
			if err != nil {
				return &models.APIError{
					InternalError: fmt.Errorf("find similar articles: %w", err),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Unable to find similar",
						"There was a problem with the request. Please try again.",
					),
				}
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
