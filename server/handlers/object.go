// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"
	"github.com/justinas/alice"
	"github.com/justinas/nosurf"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/layouts"
	"github.com/immanent-tech/foragd/web/templates/partials"
)

// ViewObject handles showing an object's content (e.g. viewing article content).
func ViewObject(api *elastic.API) http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		objectType := chi.URLParam(req, models.ParamObjectType)
		id := chi.URLParam(req, models.ParamObjectID)
		if id == "" {
			slogctx.FromCtx(req.Context()).Error("No object ID provided.")
			res.WriteHeader(http.StatusNotFound)
			renderPage(layouts.NotFound(), templates.GeneratePageTitle("Unknown article")).ServeHTTP(res, req)
			return nil
		}
		switch objectType {
		case "article":
			articles, err := models.GetArticles(req.Context(), api, id)
			if err != nil {
				renderPartial(partials.Error(
					models.NewErrorMessage("Unable to fetch article content", ""),
				)).ServeHTTP(res, req)
				return fmt.Errorf("unable to view object: %w", err)
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
					renderPartial(partials.Notification(
						models.NewErrorMessage("Unable to fetch article remote content", ""), 0,
					)).ServeHTTP(res, req)
					article.ShowFullContent = false
				} else {
					if content == article.Content {
						renderPartial(partials.Notification(
							models.NewWarningMessage("Could not fetch full article content", "Page returned existing content."), 10*time.Second,
						)).ServeHTTP(res, req)
					}
					article.Content = content
				}
			}
			// Render appropriate content.
			template := partials.ArticleContent(article)
			ctx := models.CSRFTokenToCtx(req.Context(), nosurf.Token(req))
			renderPage(template, templates.GeneratePageTitle("Articles")).ServeHTTP(res, req.WithContext(ctx))
		}

		return nil
	})).ServeHTTP
}

// MarkObject handles marking an object as read or unread and updating the UI appropriately.
func MarkObject(api *elastic.API) http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Extract request parameters.
		objectType := chi.URLParam(req, models.ParamObjectType)
		id := chi.URLParam(req, models.ParamObjectID)
		mark := models.Mark(chi.URLParam(req, models.ParamMark))
		// Validate parameters.
		if id == "" || !(mark == models.MarkRead || mark == models.MarkUnread) {
			renderPartial(partials.Notification(
				models.NewErrorMessage(
					"Unable to mark article",
					"Server received invalid data.",
				), 0)).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("%w: no ID and/or invalid mark provided", ErrInvalidRequestParams),
				http.StatusBadRequest,
			)
		}
		// Extract user data.
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			return models.NewAPIError(
				err,
				http.StatusBadRequest,
			)
		}
		switch objectType {
		case "article":
			subscriptionID := req.FormValue(models.ParamSubscriptionID)
			if subscriptionID == "" {
				return models.NewAPIError(
					fmt.Errorf("%w: unknown subscription ID", ErrInvalidRequestParams),
					http.StatusBadRequest,
				)
			}
			user.MarkItems(mark, subscriptionID, id)
			// Update the user object.
			err = api.UpdateUser(req.Context(), map[string]any{
				"subscriptions": user.Subscriptions,
			})
			if err != nil {
				renderPartial(partials.Notification(
					models.NewErrorMessage(
						"Unable to mark article",
						"Could not update user data.",
					), 0))
				return models.NewAPIError(
					fmt.Errorf("unable to mark article: %w", err),
					http.StatusUnprocessableEntity)
			}
			// Get updated articles.
			s, err := models.GetArticles(req.Context(), api, id)
			if err != nil || len(s) == 0 || len(s) > 1 {
				renderPartial(partials.Notification(
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
			case id:
				// Swap target is card.
				filters := models.ListFiltersFromCtx(req.Context())
				renderPartial(partials.NewArticleContent(s[0]).Card(filters.GetView())).ServeHTTP(res, req)
			case "mark_" + id:
				// Swap target is link.
				renderPartial(partials.UpdateViewArticleMark(s[0])).ServeHTTP(res, req)
			}
		}
		return nil
	})).ServeHTTP
}

// FindSimilar handles finding objects similar to the given objects and showing the results.
func FindSimilar(api *elastic.API) http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Extract request parameters.
		objectType := chi.URLParam(req, models.ParamObjectType)
		id := chi.URLParam(req, models.ParamObjectID)
		switch objectType {
		case "article":
			articles, err := models.FindSimilarArticles(req.Context(), api, id)
			if err != nil {
				renderPartial(partials.Notification(
					models.NewErrorMessage("Unable to find similar articles", ""), 0))
				return models.NewAPIError(err, http.StatusInternalServerError)
			}
			// Show results.
			template := layouts.SimilarArticles(articles)
			renderPage(template, templates.GeneratePageTitle("Similar Articles")).ServeHTTP(res, req)
		}
		return nil
	})).ServeHTTP
}
