// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/server/forms"
	"github.com/immanent-tech/foragd/web/templates"
)

// ListArticles handles fetching articles based on the given page filters and displaying them.
func ListArticles(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.Append(parseFilters, setCacheControl).
		ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
			filters := models.PageFiltersFromCtx(req.Context(), req.URL.Path)
			pagination := req.FormValue(models.ParamPagination)
			ctx := templates.PageTitleToCtx(req.Context(), "Articles")
			// Redirect to include query parameters in address bar.
			if len(req.URL.Query()) == 0 {
				if htmx.IsHTMX(req) {
					res.Header().Set(htmx.HeaderPushURL, req.URL.Path+"?"+filters.QueryString())
				} else {
					http.Redirect(res, req.WithContext(ctx), req.URL.Path+"?"+filters.QueryString(), http.StatusSeeOther)
				}
			}
			var (
				articles models.Articles
				err      error
				template templ.Component
			)

			// Get articles matching filters.
			articles, pagination, err = api.FilterArticles(ctx, filters, pagination)
			if err != nil && !errors.Is(err, elastic.ErrNotFound) {
				msg := models.NewErrorMessage(
					"Server could not complete request!",
					"This might be temporary, please try again.",
				)
				switch req.Method {
				case http.MethodGet:
					renderPage(templates.ErrorPage(msg)).ServeHTTP(res, req.WithContext(ctx))
				case http.MethodPost:
					renderPartial(templates.ServerErrorNotification(msg)).ServeHTTP(res, req.WithContext(ctx))
				}
				return models.NewAPIError(
					fmt.Errorf("unable to list articles: %w", err),
					http.StatusInternalServerError,
				)
			}
			// Render appropriate content.
			subscriptionID := req.FormValue(models.ParamSubscriptionID)
			// If the list of articles is from a single subscription, update the page tile to include the subscription
			// name.
			if subscriptionID != "" {
				ctx = templates.PageTitleToCtx(ctx, articles[0].GetFeedTitle()+" | Articles")
			}
			template = templates.ArticlesGrid(subscriptionID, articles, pagination)
			// Choose rendering method based on method (get = page, post = partial).
			switch req.Method {
			case http.MethodGet:
				renderPage(
					wrapContent(req.WithContext(ctx), template),
				).ServeHTTP(res, req.WithContext(ctx))
			case http.MethodPost:
				renderPartial(template).ServeHTTP(res, req.WithContext(ctx))
			}
			return nil
		})).
		ServeHTTP
}

// PaginateArticles handles a request to list more articles.
//
//nolint:dupl // this is not a duplicate.
func PaginateArticles(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.Append(parseFilters).
		ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
			filters := models.PageFiltersFromCtx(req.Context(), req.URL.Path)
			pagination := req.FormValue(models.ParamPagination)
			var (
				articles models.Articles
				err      error
			)

			// Get articles matching filters.
			articles, pagination, err = api.FilterArticles(req.Context(), filters, pagination)
			if err != nil && !errors.Is(err, elastic.ErrNotFound) {
				msg := models.NewErrorMessage(
					"Server could not complete request!",
					"This might be temporary, please try again.",
				)
				renderPartial(templates.ServerErrorNotification(msg)).ServeHTTP(res, req)
				return models.NewAPIError(
					fmt.Errorf("unable to list articles: %w", err),
					http.StatusInternalServerError,
				)
			}

			// If there are articles to show, render the articles. Else, return StatusNoContent.
			if len(articles) > 0 {
				renderPartial(templates.Articles(articles, pagination)).ServeHTTP(res, req)
			} else {
				res.WriteHeader(http.StatusNoContent)
				return nil
			}
			return nil
		})).
		ServeHTTP
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
			return models.NewAPIError(
				fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				http.StatusUnprocessableEntity,
			)
		}

		// Mark articles.
		for subscriptionID, itemIDs := range request.Metadata {
			err = markArticles(req.Context(), api, request.Mark, subscriptionID, itemIDs...)
			if err != nil {
				res.Header().Add(htmx.HeaderReswap, "none")
				renderPartial(
					templates.ServerErrorNotification(
						models.NewErrorMessage(
							"Unable to mark objects",
							"This might be a temporary error, please try again.",
						),
					),
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
				articles, err := api.GetArticles(req.Context(), itemID)
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
			renderPartial(
				templates.ServerErrorNotification(
					models.NewErrorMessage(
						"Unable to mark articles.",
						"This might be a temporary error, please try again.",
					),
				),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("mark subscriptions: %w", err), http.StatusInternalServerError)
		}
		if !valid {
			renderPartial(
				templates.ServerErrorNotification(
					models.NewErrorMessage(
						"Unable to mark articles.",
						"This might be a temporary error, please try again.",
					),
				),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("mark subscriptions: %w", err), http.StatusUnprocessableEntity)
		}

		// Mark Articles.
		for subscriptionID, itemIDs := range request.Metadata {
			err = markArticles(req.Context(), api, request.Mark, subscriptionID, itemIDs...)
			if err != nil {
				renderPartial(
					templates.ServerErrorNotification(
						models.NewErrorMessage(
							"Unable to mark articles",
							"This might be a temporary problem, please try again",
						),
					),
				).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("mark articles: %w", err), http.StatusInternalServerError)
			}
		}

		currentURL, found := htmx.GetCurrentURL(req)
		if !found {
			err = SetRedirect(res, HXLocationRequest{
				Path:   "/home",
				Target: templates.ContentID.Target(),
				Swap:   "innerHTML show:window:top transition:true",
			})
		} else {
			err = SetRedirect(res, HXLocationRequest{
				Path:   currentURL,
				Target: templates.ContentID.Target(),
				Swap:   "innerHTML show:window:top transition:true",
			})
		}
		if err != nil {
			renderPartial(
				templates.ServerErrorNotification(
					models.NewErrorMessage(
						"Unable to mark articles",
						"This might be a temporary problem, please try again",
					),
				),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("mark articles: %w", err), http.StatusInternalServerError)
		}

		res.WriteHeader(http.StatusOK)
		return nil
	})).ServeHTTP
}

func markArticles(
	ctx context.Context,
	api *elastic.API,
	mark models.Mark,
	subscriptionID models.SubscriptionID,
	itemIDs ...models.ItemID,
) error {
	subscription, err := api.GetSubscription(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("get subscriptions: %w", err)
	}
	subscription.MarkItems(mark, itemIDs...)

	_, err = api.UpdateSubscriptions(ctx, subscription)
	if err != nil {
		return fmt.Errorf("update subscription data: %w", err)
	}

	return nil
}
