// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"bufio"
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"
	"github.com/goforj/godump"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/web/templates"
)

// ShowList handles displaying or paginating a list of objects (subscriptions/articles) as cards in a grid layout.
//
//nolint:gocognit
func ShowList(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.Append(parseFilters).ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		listType := chi.RouteContext(req.Context()).URLParam(models.ParamListType)
		filters := models.PageFiltersFromCtx(req.Context(), req.URL.Path)
		pagination := req.FormValue(models.ParamPagination)
		// Redirect to include query parameters in address bar.
		if req.Method == http.MethodGet && len(req.URL.Query()) == 0 {
			if IsHTMX(req) {
				res.Header().Set(htmx.HeaderReplaceUrl, req.URL.Path+"?"+filters.QueryString())
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
			if err != nil {
				return models.NewAPIError(fmt.Errorf("unable to get subscriptions: %w", err), http.StatusInternalServerError)
			}
			// Render appropriate content.
			switch req.Method {
			case http.MethodGet:
				godump.Dump(req.URL.Path)
				if len(subscriptions) > 0 {
					template = templates.SubscriptionsGrid(subscriptions, pagination)
				} else {
					template = templates.EmptyContent("/home", nil)
				}
			case http.MethodPost:
				if len(subscriptions) > 0 {
					template = templates.SubscriptionsList(subscriptions, pagination)
				} else {
					res.WriteHeader(http.StatusNoContent)
					return nil
				}
			}
		case "articles":
			// Get articles matching filters.
			articles, pagination, err := models.FilterArticles(req.Context(), api, filters, pagination)
			if err != nil {
				return fmt.Errorf("unable to get articles: %w", err)
			}
			// Render appropriate content.
			switch req.Method {
			case http.MethodGet:
				if len(articles) > 0 {
					template = templates.ArticlesGrid(articles, pagination)
				} else {
					filters.Subscriptions = nil
					template = templates.EmptyContent("/list/subscriptions", filters.Values())
				}
			case http.MethodPost:
				if len(articles) > 0 {
					template = templates.ArticlesList(articles, pagination)
				} else {
					res.WriteHeader(http.StatusNoContent)
					return nil
				}
			}
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
//
//nolint:gocognit
func WatchList(api *elastic.API) http.HandlerFunc {
	return alice.New(
		parseFilters,
	).ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "text/event-stream")
		res.Header().Set("Cache-Control", "no-cache")
		res.Header().Set("Connection", "keep-alive")
		if f, ok := res.(http.Flusher); ok {
			f.Flush()
		} else {
			slogctx.FromCtx(req.Context()).Warn("Cannot flush update stream!")
			res.WriteHeader(http.StatusNoContent)
		}
		filters := models.PageFiltersFromCtx(req.Context(), req.URL.Path)
		query, err := models.BuildItemsQuery(req.Context(), filters)
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Cannot generate query for updates.",
				slog.Any("error", err))
			res.WriteHeader(http.StatusInternalServerError)
			return
		}

		var (
			currentCount int64
			prevCount    int64
		)
		prevCount, err = api.CountItems(req.Context(), query)
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Cannot get updates count.",
				slog.Any("error", err))
			res.WriteHeader(http.StatusInternalServerError)
			return
		}

		for {
			select {
			case <-req.Context().Done():
				res.Header().Set("Connection", "close")
				res.WriteHeader(http.StatusRequestTimeout)
				return
			default:
				currentCount, err = api.CountItems(req.Context(), query)
				if err != nil {
					slogctx.FromCtx(req.Context()).Warn("Cannot get updates count.",
						slog.Any("error", err))
					continue
				}
				// Show updates toast if new items found.
				if currentCount > prevCount {
					slogctx.FromCtx(req.Context()).Debug("Subscription updates found.")
					var b bytes.Buffer //nolint:varnamelen
					template := bufio.NewWriter(&b)
					err := templates.UpdatesToast().Render(req.Context(), template)
					if err != nil {
						slogctx.FromCtx(req.Context()).Warn("Unable to render template.",
							slog.Any("error", err))
						continue
					}
					err = template.Flush()
					if err != nil {
						slogctx.FromCtx(req.Context()).Error("Failed to flush SSE message buffer.",
							slog.Any("error", err))
					}
					_, err = fmt.Fprintf(res, "data: %s\n\n", b.String())
					if err != nil {
						slogctx.FromCtx(req.Context()).Error("Failed to send update SSE message.",
							slog.Any("error", err))
					}
					if f, ok := res.(http.Flusher); ok {
						f.Flush()
					}
				}
				prevCount = currentCount
				time.Sleep(defaultUpdateInterval)
			}
		}
	}).ServeHTTP
}

// MarkList handles marking a list of objects (subscriptions/articles) as read or unread. After marking, it will
// redirect the user appropriately.
func MarkList(api *elastic.API) http.HandlerFunc {
	return alice.New(
		parseFilters,
	).ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		listType := chi.RouteContext(req.Context()).URLParam(models.ParamListType)
		filters := models.PageFiltersFromCtx(req.Context(), req.URL.Path)

		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			return fmt.Errorf("unable to mark all subscriptions: %w", err)
		}
		// Generate request details
		var (
			mark          models.Mark
			subscriptions []models.SubscriptionID
		)

		switch filters.GetView() {
		case models.ViewUnread:
			mark = models.MarkRead
		default:
			mark = models.MarkUnread
		}
		if len(filters.Subscriptions) == 0 {
			subscriptions = user.GetSubscriptionMetadata().GetIDs()
		} else {
			subscriptions = filters.GetSubscriptions()
		}
		slogctx.FromCtx(req.Context()).Debug("Marking list.",
			slog.String("list", listType),
			slog.String("mark", string(mark)),
			slog.String("subscriptions", strings.Join(subscriptions, ",")),
		)
		// Mark subscriptions.
		err = models.MarkSubscriptions(req.Context(), api, mark, subscriptions...)
		if err != nil {
			renderPartial(templates.Notification(
				models.NewErrorMessage(
					"Unable to mark objects",
					"Something went wrong, please try again",
				), 0))
			return models.NewAPIError(
				fmt.Errorf("unable to mark all objects: %w", err),
				http.StatusInternalServerError)
		}

		// Redirect depending on the current view.
		switch filters.GetView() {
		case models.ViewRead, models.ViewUnread:
			if listType == "articles" {
				filters.Subscriptions = nil
				SetRedirect(req.Context(), "/list/subscriptions", filters, res)
			} else {
				SetRedirect(req.Context(), "/home", nil, res)
			}
		case models.ViewAll:
			SetRedirect(req.Context(), "/home", nil, res)
		}
		res.WriteHeader(http.StatusOK)
		return nil
	})).ServeHTTP
}
