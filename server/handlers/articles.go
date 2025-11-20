// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"
	"github.com/go-shiori/go-readability"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/server/forms"
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
		subscriptionID := req.FormValue(models.ParamSubscriptionID)
		itemID := chi.URLParam(req, models.ParamItemID)
		request := &models.MarkArticlesRequest{
			Metadata: map[models.SubscriptionID][]models.ItemID{subscriptionID: {itemID}},
			Mark:     models.Mark(chi.URLParam(req, models.ParamMark)),
			View:     models.View(req.FormValue(models.ParamView)),
		}
		err := request.Valid()
		if err != nil {
			res.Header().Add(htmx.HeaderReswap, "none")
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to mark article", "This might be a temporary issue, please try again."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("%w: %w", ErrInvalidRequestParams, err), http.StatusUnprocessableEntity)
		}

		// Mark articles.
		for subscriptionID, itemIDs := range request.Metadata {
			err = markArticles(req.Context(), api, request.Mark, subscriptionID, itemIDs...)
			if err != nil {
				res.Header().Add(htmx.HeaderReswap, "none")
				renderPartial(
					templates.ServerErrorNotification(
						models.NewErrorMessage("Unable to mark objects", "This might be a temporary error, please try again.")),
				).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("unable to update user: %w", err), http.StatusInternalServerError)
			}
		}

		// Generate appropriate swap content based on target header.
		switch req.Header.Get(htmx.HeaderTarget) {
		case itemID: // Swap target is card.
			// Update UI according to current view.
			if request.View != models.ViewAll {
				res.Header().Add(htmx.HeaderReswap, "delete transition:true")
			} else {
				res.Header().Add(htmx.HeaderReswap, "outerHTML transition:true")
				// Get updated article.
				articles, err := models.GetArticles(req.Context(), api, itemID)
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
		case "mark_" + itemID: // Swap target is link (viewing article).
			if request.Mark == models.MarkRead {
				renderPartial(templates.UpdateViewArticleMark(itemID, false)).ServeHTTP(res, req)
			} else {
				renderPartial(templates.UpdateViewArticleMark(itemID, true)).ServeHTTP(res, req)
			}
		}
		return nil
	})).ServeHTTP
}

// MarkArticles handles marking multiple articles as read/unread and updating the UI appropriately.
func MarkArticles(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Decode request parameters.
		request, valid, err := forms.DecodeForm[*models.MarkArticlesRequest](req)
		if err != nil {
			renderPartial(templates.ServerErrorNotification(models.NewErrorMessage("Unable to mark articles.", "This might be a temporary error, please try again."))).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("mark subscriptions: %w", err), http.StatusInternalServerError)
		}
		if !valid {
			renderPartial(templates.ServerErrorNotification(models.NewErrorMessage("Unable to mark articles.", "This might be a temporary error, please try again."))).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("mark subscriptions: %w", err), http.StatusUnprocessableEntity)
		}

		// Mark Articles.
		for subscriptionID, itemIDs := range request.Metadata {
			err = markArticles(req.Context(), api, request.Mark, subscriptionID, itemIDs...)
			if err != nil {
				renderPartial(
					templates.ServerErrorNotification(
						models.NewErrorMessage("Unable to mark articles", "This might be a temporary problem, please try again")),
				).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("mark articles: %w", err), http.StatusInternalServerError)
			}
		}

		currentURL, found := htmx.GetCurrentURL(req)
		if !found {
			err = SetRedirect(req.Context(), res, HXLocationRequest{
				Path:   "/home",
				Target: templates.ContentID.Target(),
				Swap:   "innerHTML transition:true",
			})
		} else {
			err = SetRedirect(req.Context(), res, HXLocationRequest{
				Path:   currentURL,
				Target: templates.ContentID.Target(),
				Swap:   "innerHTML transition:true",
			})
		}
		if err != nil {
			renderPartial(
				templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to mark articles", "This might be a temporary problem, please try again")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("mark articles: %w", err), http.StatusInternalServerError)
		}

		res.WriteHeader(http.StatusOK)
		return nil
	})).ServeHTTP
}

// markArticles will mark Articles for a Subscription as appropriate. Marking Articles involves updating the User object
// with an ItemState that tracks the mark status for the underlying Item an Article represents.
func markArticles(ctx context.Context, api *elastic.API, mark models.Mark, subscriptionID models.SubscriptionID, itemIDs ...models.ItemID) error {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return fmt.Errorf("mark articles: get user data: %w", models.ErrNoUserCtx)
	}
	query := query.Bool(
		query.Filter(
			query.Term("user_id", user.GetID()),
			query.Terms("subscription_id", subscriptionID),
		),
	)
	subscriptions, _, err := api.SearchSubscriptions(ctx, query, 1, nil, nil)
	switch {
	case err != nil:
		return fmt.Errorf("mark articles: get subscriptions: %w", err)
	case len(subscriptions) == 0:
		return fmt.Errorf("mark articles: get subscriptions: %w", models.ErrNotFound)
	case len(subscriptions) != 1:
		return fmt.Errorf("mark articles: get subscriptions: %w", models.ErrInvalidAPIResult)
	}
	subscriptions[0].MarkItems(mark, itemIDs...)

	_, err = api.UpdateSubscriptions(ctx, subscriptions[0])
	if err != nil {
		return fmt.Errorf("mark articles: update subscription data: %w", err)
	}

	return nil
}
