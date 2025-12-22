// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"codeberg.org/readeck/go-readability/v2"
	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/server/forms"
	"github.com/immanent-tech/foragd/validation"
	"github.com/immanent-tech/foragd/web/templates"
)

// ListArticles handles fetching articles based on the given page filters and displaying them.
func ListArticles(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.Append(parseFilters).
		ThenFunc(func(res http.ResponseWriter, req *http.Request) {
			list := func(res http.ResponseWriter, req *http.Request) error {
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
					return &models.APIError{
						InternalError: fmt.Errorf("unable to list articles: %w", err),
						StatusCode:    http.StatusInternalServerError,
					}
				}
				// Render appropriate content.
				subscriptionID := req.FormValue(models.ParamSubscriptionID)
				// If the list of articles is from a single subscription, update the page tile to include the subscription
				// name.
				if len(articles) > 0 && subscriptionID != "" {
					ctx = templates.PageTitleToCtx(ctx, articles[0].GetFeedTitle()+" | Articles")
				}
				template = templates.ListArticles(subscriptionID, articles, filters, pagination)
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
			}
			switch req.Method {
			case http.MethodGet:
				showOnError(list).ServeHTTP(res, req)
			case http.MethodPost:
				notifyOnError(list).ServeHTTP(res, req)
			}
		}).ServeHTTP
}

// PaginateArticles handles a request to list more articles.
func PaginateArticles(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.Append(parseFilters).
		ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
			filters := models.PageFiltersFromCtx(req.Context(), req.URL.Path)
			pagination := req.FormValue(models.ParamPagination)
			var (
				articles models.Articles
				err      error
			)

			// Get articles matching filters.
			articles, pagination, err = api.FilterArticles(req.Context(), filters, pagination)
			if err != nil && !errors.Is(err, elastic.ErrNotFound) {
				return &models.APIError{
					InternalError: fmt.Errorf("unable to list articles: %w", err),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Could not list more articles",
						"This might be temporary, please try again.",
					),
				}
			}

			// If there are articles to show, render the articles. Else, return StatusNoContent.
			if len(articles) > 0 {
				renderPartial(templates.PaginateArticles(articles, pagination)).ServeHTTP(res, req)
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
	return defaultHandlerChain.ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
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
			return &models.APIError{
				InternalError: fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to mark article",
					"This might be a temporary issue, please try again.",
				),
			}
		}

		// Mark articles.
		for subscriptionID, itemIDs := range request.Metadata {
			err = markArticles(req.Context(), api, request.Mark, subscriptionID, itemIDs...)
			if err != nil {
				return &models.APIError{
					InternalError: fmt.Errorf("unable to update user: %w", err),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Unable to mark article",
						"This might be a temporary issue, please try again.",
					),
				}
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
					return &models.APIError{
						InternalError: fmt.Errorf("could not retrieve updated articles: %w", err),
						StatusCode:    http.StatusInternalServerError,
						UserMessage:   models.NewErrorMessage("Unable to mark objects", "This might be a temporary error, please try again."),
					}
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
		res.WriteHeader(http.StatusOK)
		return nil
	})).ServeHTTP
}

// MarkArticles handles marking multiple articles as read/unread and updating the UI appropriately.
func MarkArticles(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
		// Decode request parameters.
		request, valid, err := forms.DecodeForm[*models.MarkArticlesRequest](req)
		if err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("decode mark articles request: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to mark articles.",
					"This might be a temporary error, please try again.",
				),
			}
		}
		if !valid {
			return &models.APIError{
				InternalError: fmt.Errorf("validate mark articles request: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to mark articles.",
					"This might be a temporary error, please try again.",
				),
			}
		}

		// Mark Articles.
		for subscriptionID, itemIDs := range request.Metadata {
			if err = markArticles(req.Context(), api, request.Mark, subscriptionID, itemIDs...); err != nil {
				return &models.APIError{
					InternalError: fmt.Errorf("mark subscriptions: %w", err),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Unable to mark articles.",
						"This might be a temporary error, please try again.",
					),
				}
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
			return &models.APIError{
				InternalError: fmt.Errorf("mark subscriptions: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to mark articles.",
					"This might be a temporary error, please try again.",
				),
			}
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

var articleBufPool = sync.Pool{
	New: func() any {
		var buf bytes.Buffer
		return &buf
	},
}

// ExtractArticleFromURL fetches the text content of the given URL and attempts to extract the main article content from
// it.
func extractArticleFromURL(url string) (string, error) {
	remote, err := readability.FromURL(url, models.DefaultHTTPRequestTimeout)
	if err != nil {
		return "", fmt.Errorf("extract article from url %s: %w", url, err)
	}

	articleBufPtr := articleBufPool.Get().(*bytes.Buffer)
	articleBuf := *articleBufPtr
	defer func() {
		articleBufPtr.Reset()
		imgBufPool.Put(articleBufPtr)
	}()

	if err := remote.RenderHTML(&articleBuf); err != nil {
		return "", fmt.Errorf("render article html: %w", err)
	}
	content := validation.SanitizeString(articleBuf.String())
	return content, nil
}
