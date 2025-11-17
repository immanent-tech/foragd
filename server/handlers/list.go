// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/web/templates"
)

// ShowList handles displaying or paginating a list of objects (subscriptions/articles) as cards in a grid layout.
//
//nolint:gocognit,gocyclo,funlen
func ShowList(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.Append(parseFilters, setCacheControl).ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		listType := chi.RouteContext(req.Context()).URLParam(models.ParamListType)
		filters := models.PageFiltersFromCtx(req.Context(), req.URL.Path)
		pagination := req.FormValue(models.ParamPagination)
		// Redirect to include query parameters in address bar.
		if req.Method == http.MethodGet && len(req.URL.Query()) == 0 && listType != "favorites" {
			if IsHTMX(req) {
				res.Header().Set(htmx.HeaderPushURL, req.URL.Path+"?"+filters.QueryString())
			} else {
				http.Redirect(res, req, req.URL.Path+"?"+filters.QueryString(), http.StatusSeeOther)
			}
		}
		var (
			template  templ.Component
			pageTitle string
		)
		// Render list based on type.
		switch listType {
		case "subscriptions":
			// Remove any subscription filters if this is a history restore request (i.e. back button clicked).
			if htmx.IsHistoryRestoreRequest(req) {
				filters.Subscriptions = nil
			}
			// Get subscriptions matching filters.
			subscriptions, pagination, err := models.FilterSubscriptions(req.Context(), api, filters, pagination)
			if err != nil && !errors.Is(err, models.ErrNotFound) {
				msg := models.NewErrorMessage("Server could not complete request!", "This might be temporary, please try again.")
				switch req.Method {
				case http.MethodGet:
					renderPage(templates.ErrorPage(msg), templates.GeneratePageTitle(pageTitle)).ServeHTTP(res, req)
				case http.MethodPost:
					template := templates.ServerErrorNotification(msg)
					renderPartial(template).ServeHTTP(res, req)
				}
				return models.NewAPIError(fmt.Errorf("unable to list subscriptions: %w", err), http.StatusInternalServerError)
			}
			// Render appropriate content.
			switch req.Method {
			case http.MethodGet:
				template = templates.SubscriptionsGrid(pagination, subscriptions...)
			case http.MethodPost:
				if len(subscriptions) > 0 {
					template = templates.SubscriptionsList(pagination, subscriptions...)
				} else {
					res.WriteHeader(http.StatusNoContent)
					return nil
				}
			}
		case "articles":
			// Get articles matching filters.
			articles, pagination, err := models.FilterArticles(req.Context(), api, filters, pagination)
			if err != nil && !errors.Is(err, models.ErrNotFound) {
				msg := models.NewErrorMessage("Server could not complete request!", "This might be temporary, please try again.")
				switch req.Method {
				case http.MethodGet:
					renderPage(templates.ErrorPage(msg), templates.GeneratePageTitle(pageTitle)).ServeHTTP(res, req)
				case http.MethodPost:
					template := templates.ServerErrorNotification(msg)
					renderPartial(template).ServeHTTP(res, req)
				}
				return models.NewAPIError(fmt.Errorf("unable to list articles: %w", err), http.StatusInternalServerError)
			}
			// Render appropriate content.
			switch req.Method {
			case http.MethodGet:
				template = templates.ArticlesGrid(articles, pagination)
			case http.MethodPost:
				if len(articles) > 0 {
					template = templates.ArticlesList(articles, pagination)
				} else {
					res.WriteHeader(http.StatusNoContent)
					return nil
				}
			}
		case "favorites":
			// Get favorite articles.
			articles, err := models.GetUserFavoriteArticles(req.Context(), api)
			if err != nil && !errors.Is(err, models.ErrNotFound) {
				msg := models.NewErrorMessage("Server could not complete request!", "This might be temporary, please try again.")
				switch req.Method {
				case http.MethodGet:
					renderPage(templates.ErrorPage(msg), templates.GeneratePageTitle(pageTitle)).ServeHTTP(res, req)
				case http.MethodPost:
					template := templates.ServerErrorNotification(msg)
					renderPartial(template).ServeHTTP(res, req)
				}
				return models.NewAPIError(fmt.Errorf("could not fetch user info from context: %w", err), http.StatusInternalServerError)
			}
			// Get favorite feed and search subscriptions
			subscriptions, err := models.GetUserFavoriteSubscriptions(req.Context(), api)
			if err != nil && !errors.Is(err, models.ErrNotFound) {
				msg := models.NewErrorMessage("Server could not complete request!", "This might be temporary, please try again.")
				switch req.Method {
				case http.MethodGet:
					renderPage(templates.ErrorPage(msg), templates.GeneratePageTitle(pageTitle)).ServeHTTP(res, req)
				case http.MethodPost:
					template := templates.ServerErrorNotification(msg)
					renderPartial(template).ServeHTTP(res, req)
				}
				return models.NewAPIError(fmt.Errorf("could not fetch user info from context: %w", err), http.StatusInternalServerError)
			}
			if len(subscriptions) > 0 || len(articles) > 0 {
				template = templates.FavoritesLayout(subscriptions, articles)
			} else {
				template = templates.EmptyContent()
			}
		default:
			slogctx.FromCtx(req.Context()).Error("Unsupported list type requested.",
				slog.String("type", listType))
			res.WriteHeader(http.StatusNotFound)
			return nil
		}
		// Choose rendering method based on method (get = page, post = partial).
		switch req.Method {
		case http.MethodGet:
			renderPage(template, templates.GeneratePageTitle(pageTitle)).ServeHTTP(res, req)
		case http.MethodPost:
			renderPartial(template).ServeHTTP(res, req)
		}
		return nil
	})).ServeHTTP
}

// WatchList handles watching a list of object for any updates and rendering a notification to the user to refresh the page.
func WatchList(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.Append(parseFilters).ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		filters := models.PageFiltersFromCtx(req.Context(), req.URL.Path)
		query, err := models.BuildItemsQuery(req.Context(), api, filters)
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Cannot generate query for updates.",
				slog.Any("error", err))
			res.WriteHeader(http.StatusInternalServerError)
			return
		}
		// Watch list for updates.
		watchForUpdates(api, query).ServeHTTP(res, req)
	}).ServeHTTP
}
