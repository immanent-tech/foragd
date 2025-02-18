// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"
	"slices"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"

	"github.com/joshuar/go-templ-daisyui/display/text"
	"github.com/joshuar/go-templ-daisyui/navigation/menu"

	"github.com/joshuar/go-feed-me/internal/app/server/params"
	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/web/templates"
	"github.com/joshuar/go-feed-me/web/templates/layouts/home"
	"github.com/joshuar/go-feed-me/web/templates/partials"
)

const (
	showFeedsPath       = "/home/feeds"
	markItemsReadPath   = "/home/markread/items"
	markItemsUnreadPath = "/home/markread/items"
	showItemPath        = "/home/show"
)

var ErrGeneratePageNavigationFailed = errors.New("error occurred while generating page navigation")

func HomeMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		ctx := req.Context()

		params := req.URL.Query()
		if !params.Has("count") {
			params.Set("count", "10")
		}

		if !params.Has("view") {
			params.Set("view", "unread")
		}

		req.URL.RawQuery = params.Encode()

		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

func (s Server) HandleHome(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) HandleShowFeeds(res http.ResponseWriter, req *http.Request, reqParams HandleShowFeedsParams) {
	filters, err := models.CreateFilters(reqParams)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
	}

	// Save list feeds filters in session storage.
	// session.SaveListFeedsFilters(req.Context(), filters)

	feedCh, err := s.API.elastic.UserActionGetFeeds(req.Context(), *filters)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Could not retrieve feeds.",
			slog.Any("error", err))

		// if err := layout.Render(req.Context(), res, htmx.NewResponse(), htmx.IsHTMX(req)); err != nil {
		// 	logging.FromContext(req.Context()).Error("Show feeds failed.",
		// 		slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)
		// }

		return
	}

	_, err = s.API.elastic.UserActionGetFeedCategories(req.Context())
	if err != nil {
		logging.FromContext(req.Context()).Warn("Could not retrieve feeds.",
			slog.Any("error", err))
	}

	var feeds []*templates.Component
	categoryFilters := partials.NewCategoryFilter("/home/feeds", templ.Attributes{
		"hx-target":   "#content",
		"hx-push-url": "true",
	}, *filters)

	for feed := range feedCh {
		feedBasePath := "/home/feed/" + feed.GetID() + "/items"
		component, err := templates.NewComponent(feed,
			templates.DisplayAs(templates.FeedCard),
			templates.WithAttributes(templ.Attributes{
				"hx-target":   "#content",
				"hx-push-url": "true",
				"hx-get":      params.BuildURL(feedBasePath),
			}),
			templates.WithActions(
				templates.NewAction("Mark Feed Read",
					templates.WithActionAttributes(templ.Attributes{
						"hx-post": params.BuildURL(feedBasePath, params.WithMark(models.MarkRead)),
					}),
				),
				templates.NewAction("Mark Feed Unread",
					templates.WithActionAttributes(templ.Attributes{
						"hx-post": params.BuildURL(feedBasePath, params.WithMark(models.MarkUnread)),
					}),
				),
			),
		)
		if err != nil {
			logging.FromContext(req.Context()).Warn("Could not create card component for feed.",
				slog.String("feed_id", feed.GetID()),
				slog.Any("error", err))

			continue
		}

		feeds = append(feeds, component)

		for _, category := range feed.GetCategories() {
			var active bool
			if reqParams.Categories != nil {
				if slices.Contains(*reqParams.Categories, category) {
					active = true
				}
			}
			categoryFilters.Add(category, active)
		}
	}

	layout := home.BuildLayout(
		home.WithBreadCrumbs(home.BuildCrumb("Feeds", text.Semibold, nil)),
		home.WithHeader(home.FeedsHeader(*categoryFilters)),
		home.WithSideBar(
			menu.WithID("drawer_menu"),
			menu.WithItems(
				partials.AddSubscriptionButton().Show(),
				text.Build("View:", text.WithTextWeight(text.Semibold)).Show(),
				partials.StateFilter(reqParams.View, createViewFilters("/home/feeds", reqParams.Count, nil)).Show(),
			),
			menu.WithExtraAttributes(templ.Attributes{
				"hx-target":   "#drawer_menu",
				"hx-swap-oob": "true",
			}),
		),
	)

	if len(feeds) > 0 {
		home.WithContent(feeds...)(layout)
	}

	if err := layout.Render(req.Context(), res, htmx.NewResponse(), htmx.IsHTMX(req)); err != nil {
		logging.FromContext(req.Context()).Error("Show feeds failed.",
			slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

func (s Server) HandleMarkFeeds(res http.ResponseWriter, req *http.Request, reqParams HandleMarkFeedsParams) {
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) HandleShowFeedItems(res http.ResponseWriter, req *http.Request, feedID FeedID, reqParams HandleShowFeedItemsParams) {
	filters, err := models.CreateFilters(reqParams)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)

		return
	}
	// Add the feed ID.
	filters.FeedIDs = append(filters.FeedIDs, feedID)

	// Get the feed details.
	feed, err := s.API.elastic.GetFeedByID(req.Context(), feedID)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", err))
		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)

		return
	}

	// Save list items filters in session storage.
	// session.SaveListItemsFilters(req.Context(), filters)

	layout := home.BuildLayout(
		home.WithSideBar(
			menu.WithID("drawer_menu"),
			menu.WithItems(
				partials.AddSubscriptionButton().Show(),
				text.Build("View:", text.WithTextWeight(text.Semibold)).Show(),
				partials.StateFilter(reqParams.View, createViewFilters("/home/feed/"+feedID+"/items", reqParams.Count, *reqParams.Categories)).Show(),
			),
			menu.WithExtraAttributes(templ.Attributes{
				"hx-target":   "#drawer_menu",
				"hx-swap-oob": "true",
			}),
		),
	)

	// Build breadcrumbs.
	var crumbs []templ.Component
	// Build a feeds breadcrumb using any stored feeds filters.
	// feedFilters, err := session.LoadListFeedsFilters(req.Context())
	// if err != nil {
	// 	crumbs = append(crumbs, home.BuildCrumb("Feeds", text.Normal, templ.Attributes{
	// 		"hx-get":      params.BuildURL("/home/feeds"),
	// 		"hx-target":   "#content",
	// 		"hx-push-url": "true",
	// 	}))
	// } else {
	crumbs = append(crumbs, home.BuildCrumb("Feeds", text.Normal, templ.Attributes{
		"hx-get":      "/home/feeds",
		"hx-target":   "#content",
		"hx-push-url": "true",
	}))
	crumbs = append(crumbs, home.BuildCrumb(feed.GetTitle(), text.Normal, templ.Attributes{
		"hx-get":      params.BuildURL("/home/feed/" + feed.GetID()),
		"hx-target":   "#content",
		"hx-push-url": "true",
	}))

	// }
	// Build an items breadcrumb.
	crumbs = append(crumbs, home.BuildCrumb("Items", text.Semibold, nil))
	// Create the breadcrumbs.
	home.WithBreadCrumbs(crumbs...)(layout)

	itemCh, pagination, err := s.API.elastic.UserActionGetItems(req.Context(), *filters)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Could not retrieve items.",
			slog.Any("error", err))

		if err := layout.Render(req.Context(), res, htmx.NewResponse(), htmx.IsHTMX(req)); err != nil {
			logging.FromContext(req.Context()).Error("Show feeds failed.",
				slog.Any("error", err))
			http.Error(res, err.Error(), http.StatusInternalServerError)
		}

		return
	}

	items := make([]*templates.Component, 0, filters.Count)

	idx := 0

	for item := range itemCh {
		component, err := templates.NewComponent(item,
			templates.DisplayAs(templates.ItemCard),
			templates.WithAttributes(templ.Attributes{
				"hx-target":   "#content",
				"hx-push-url": "true",
				"hx-get":      filepath.Join(showItemPath, item.GetFeedID(), item.GetID()),
			}),
			// templates.WithActions(
			// 	templates.NewAction("Mark Read",
			// 		templates.WithActionAttributes(templ.Attributes{
			// 			"hx-post": genActionBasePath(models.Markread, "feeds") + "?feeds=" + feed.GetID(),
			// 		}),
			// 	),
			// 	templates.NewAction("Mark Feed Unread",
			// 		templates.WithActionAttributes(templ.Attributes{
			// 			"hx-post": genActionBasePath(models.Markunread, "feeds") + "?feeds=" + feed.GetID(),
			// 		}),
			// 	),
			// ),
		)
		if err != nil {
			logging.FromContext(req.Context()).Warn("Could not create card component for item.",
				slog.String("items_id", item.GetID()),
				slog.Any("error", err))

			continue
		}

		if idx == len(itemCh)-1 && pagination != "" && len(itemCh) == filters.Count {
			component.AddAttributes(templ.Attributes{
				"hx-get":       filepath.Join("home", "show", "items") + "?pagination=" + pagination,
				"hx-trigger":   "revealed",
				"hx-swap":      "afterend",
				"hx-push-url":  "false",
				"hx-indicator": "#content-loading",
			})
		}

		items = append(items, component)
		idx++
	}

	if len(items) > 0 {
		home.WithContent(items...)(layout)
	}

	if err := layout.Render(req.Context(), res, htmx.NewResponse(), htmx.IsHTMX(req)); err != nil {
		logging.FromContext(req.Context()).Error("Show feeds failed.",
			slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

func (s Server) HandleMarkFeedItems(res http.ResponseWriter, req *http.Request, feed FeedID, reqParams HandleMarkFeedItemsParams) {
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) HandleShowItem(res http.ResponseWriter, req *http.Request, feed FeedID, item ItemID) {
	layout := home.BuildLayout(
		home.WithBreadCrumbs(home.BuildCrumb("Feeds", text.Semibold, nil)),
		home.WithSideBar(
			menu.WithID("drawer_menu"),
			menu.WithExtraAttributes(templ.Attributes{
				"hx-target":   "#drawer_menu",
				"hx-swap-oob": "true",
			}),
		),
	)

	// Build breadcrumbs.
	var crumbs []templ.Component
	// Build a feeds breadcrumb using any stored feeds filters.
	// feedFilters, err := session.LoadListFeedsFilters(req.Context())
	// if err != nil {
	// 	crumbs = append(crumbs, home.BuildCrumb("Feeds", text.Normal, templ.Attributes{
	// 		"hx-get":      showFeedsPath,
	// 		"hx-target":   "#content",
	// 		"hx-push-url": "true",
	// 	}))
	// } else {
	crumbs = append(crumbs, home.BuildCrumb("Feeds", text.Normal, templ.Attributes{
		"hx-get":      showFeedsPath,
		"hx-target":   "#content",
		"hx-push-url": "true",
	}))
	// }
	// Build a items breadcrumb using any stored items filters.
	// itemFilters, err := session.LoadListItemsFilters(req.Context())
	// if err != nil {
	// 	crumbs = append(crumbs, home.BuildCrumb("Items", text.Normal, templ.Attributes{
	// 		"hx-get":      itemFilters.BuildURL(showItemsPath),
	// 		"hx-target":   "#content",
	// 		"hx-push-url": "true",
	// 	}))
	// } else {
	crumbs = append(crumbs, home.BuildCrumb("Items", text.Normal, templ.Attributes{
		"hx-get":      "/home/feeds",
		"hx-target":   "#content",
		"hx-push-url": "true",
	}))
	// }
	// Add the article breadcrumb.
	crumbs = append(crumbs, home.BuildCrumb("Article", text.Semibold, nil))
	// Create the breadcrumbs.
	home.WithBreadCrumbs(crumbs...)(layout)

	details, found, err := s.API.elastic.UserActionGetItem(req.Context(), feed, item)
	if err != nil || !found {
		logging.FromContext(req.Context()).Warn("Could not retrieve item.",
			slog.Any("error", err))
		if err := layout.Render(req.Context(), res, htmx.NewResponse(), htmx.IsHTMX(req)); err != nil {
			logging.FromContext(req.Context()).Error("Show item failed.",
				slog.Any("error", err))
			http.Error(res, err.Error(), http.StatusInternalServerError)
		}

		return
	}

	component, err := templates.NewComponent(details,
		templates.DisplayAs(templates.ItemArticle),
	)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Could not retrieve items.",
			slog.Any("error", err))

		if err := layout.Render(req.Context(), res, htmx.NewResponse(), htmx.IsHTMX(req)); err != nil {
			logging.FromContext(req.Context()).Error("Show feeds failed.",
				slog.Any("error", err))
			http.Error(res, err.Error(), http.StatusInternalServerError)
		}

		return
	}

	home.WithContent(component)(layout)

	if err := layout.Render(req.Context(), res, htmx.NewResponse(), htmx.IsHTMX(req)); err != nil {
		logging.FromContext(req.Context()).Error("Show feeds failed.",
			slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

func (s Server) HandleMarkItem(res http.ResponseWriter, req *http.Request, feed FeedID, item ItemID, params HandleMarkItemParams) {
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) HandleSaveItem(res http.ResponseWriter, req *http.Request, feed FeedID, item ItemID) {
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) HandleUnsaveItem(res http.ResponseWriter, req *http.Request, feed FeedID, item ItemID) {
	res.WriteHeader(http.StatusNotImplemented)
}

func createViewFilters(path string, count Count, categories []models.Category) map[models.View]string {
	return map[View]string{
		models.ViewRead: params.BuildURL(path,
			params.WithView(models.ViewRead),
			params.WithCategories(categories...),
			params.WithCount(count)),
		models.ViewUnread: params.BuildURL(path,
			params.WithView(models.ViewUnread),
			params.WithCategories(categories...),
			params.WithCount(count)),
		models.ViewAll: params.BuildURL(path,
			params.WithView(models.ViewAll),
			params.WithCategories(categories...),
			params.WithCount(count)),
	}
}
