// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"
	"github.com/go-shiori/go-readability"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/validation"
	"github.com/immanent-tech/foragd/web/templates"
)

func fetchArticleRemoteContent(url string) (string, error) {
	remote, err := readability.FromURL(url, 30*time.Second)
	if err != nil {
		return "", fmt.Errorf("failed to parse content for %s, %w", url, err)
	}
	content := validation.SanitizeString(remote.Content)
	return content, nil
}

// MarkArticle handles marking an article as read/unread and updates the UI accordingly.
func MarkArticle(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Extract request values.
		request := &models.MarkArticleRequest{
			SubscriptionID: req.FormValue(models.ParamSubscriptionID),
			ItemID:         chi.URLParam(req, models.ParamItemID),
			Mark:           models.Mark(chi.URLParam(req, models.ParamMark)),
			View:           models.View(req.FormValue(models.ParamView)),
		}
		err := request.Valid()
		if err != nil {
			res.Header().Add(htmx.HeaderReswap, "none")
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to mark article", "This might be a temporary issue, please try again."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("%w: %w", ErrInvalidRequestParams, err), http.StatusUnprocessableEntity)
		}
		// Mark articles appropriately.
		err = models.MarkArticles(req.Context(), api, request.Mark, request.SubscriptionID, request.ItemID)
		if err != nil {
			res.Header().Add(htmx.HeaderReswap, "none")
			renderPartial(
				templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to mark objects", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to update user: %w", err), http.StatusInternalServerError)
		}
		// Generate appropriate swap content based on target header.
		switch req.Header.Get(htmx.HeaderTarget) {
		case request.ItemID: // Swap target is card.
			// Update UI according to current view.
			if request.View != models.ViewAll {
				res.Header().Add(htmx.HeaderReswap, "delete transition:true")
			} else {
				res.Header().Add(htmx.HeaderReswap, "outerHTML transition:true")
				// Get updated article.
				articles, err := models.GetArticles(req.Context(), api, request.ItemID)
				if err != nil || len(articles) == 0 || len(articles) > 1 {
					res.Header().Add(htmx.HeaderReswap, "none")
					renderPartial(
						templates.ServerErrorNotification(
							models.NewErrorMessage("Unable to mark objects", "This might be a temporary error, please try again.")),
					).ServeHTTP(res, req)
					return models.NewAPIError(fmt.Errorf("could not retrieve updated articles: %w", err), http.StatusInternalServerError)
				}
				// Render new article card.
				renderPartial(templates.ArticleCard(articles[0])).ServeHTTP(res, req)
			}
		case "mark_" + request.ItemID: // Swap target is link (viewing article).
			if request.Mark == models.MarkRead {
				renderPartial(templates.UpdateViewArticleMark(request.ItemID, false)).ServeHTTP(res, req)
			} else {
				renderPartial(templates.UpdateViewArticleMark(request.ItemID, true)).ServeHTTP(res, req)
			}
		}
		return nil
	})).ServeHTTP
}
