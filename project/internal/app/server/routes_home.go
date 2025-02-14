// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"

	"github.com/joshuar/go-templ-daisyui/display/text"
	"github.com/joshuar/go-templ-daisyui/navigation/menu"

	"github.com/joshuar/go-feed-me/internal/app/server/session"
	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/web/templates"
	"github.com/joshuar/go-feed-me/web/templates/layouts/home"
	"github.com/joshuar/go-feed-me/web/templates/partials"
)

const (
	showFeedsPath       = "/home/show/feeds"
	showItemsPath       = "/home/show/items"
	markItemsReadPath   = "/home/markread/items"
	markItemsUnreadPath = "/home/markread/items"
	showItemPath        = "/home/show"
)

var ErrGeneratePageNavigationFailed = errors.New("error occurred while generating page navigation")

// HomeMiddleware performs some common functionality for /home routes.
//
// - Load the request context with appropriate values.
//
// - Enforce htmx-only routes, where applicable.
func HomeMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		ctx := req.Context()

		switch req.Method {
		case http.MethodPost:
			if !htmx.IsHTMX(req) {
				logging.FromContext(ctx).Error("HTMX required.")
				http.Error(res, "HTMX required.", http.StatusNotAcceptable)

				return
			}
			// case http.MethodGet:
			// 	nav, err := createPageNavigation(req)
			// 	if err != nil {
			// 		logging.FromContext(ctx).Error("Bad request.",
			// 			slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
			// 		res.WriteHeader(http.StatusNotAcceptable)

			// 		return
			// 	}

			// 	ctx = models.PageNavigationToCtx(ctx, nav)
		}

		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

// (GET /home/show/feeds)
func (s Server) HandleShowObjects(res http.ResponseWriter, req *http.Request, object Type, params HandleShowObjectsParams) {
	filters, err := models.CreateFilters(params)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
	}

	switch object {
	case Feeds:
		s.showFeeds(res, req, filters)
	case Items:
		s.showItems(res, req, filters)
	}
}

func (s Server) showFeeds(res http.ResponseWriter, req *http.Request, filters *models.APIFilters) {
	// Save list feeds filters in session storage.
	session.SaveListFeedsFilters(req.Context(), filters)

	layout := home.BuildLayout(
		home.WithBreadCrumbs(home.BuildCrumb("Feeds", text.Semibold, nil)),
		home.WithSideBar(
			menu.WithID("drawer_menu"),
			menu.WithItems(
				partials.AddSubscriptionButton().Show(),
				text.Build("View:", text.WithTextWeight(text.Semibold)).Show(),
				partials.StateFilter(showFeedsPath, filters).Show(),
			),
			menu.WithExtraAttributes(templ.Attributes{
				"hx-target":   "#drawer_menu",
				"hx-swap-oob": "true",
			}),
		),
	)

	feedCh, err := s.API.elastic.UserActionGetFeeds(req.Context(), *filters)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Could not retrieve feeds.",
			slog.Any("error", err))

		if err := layout.Render(req.Context(), res, htmx.NewResponse(), htmx.IsHTMX(req)); err != nil {
			logging.FromContext(req.Context()).Error("Show feeds failed.",
				slog.Any("error", err))
			http.Error(res, err.Error(), http.StatusInternalServerError)
		}

		return
	}

	feeds := make([]*templates.Component, 0, filters.Count)

	for feed := range feedCh {
		component, err := templates.NewComponent(feed,
			templates.DisplayAs(templates.FeedCard),
			templates.WithAttributes(templ.Attributes{
				"hx-target":   "#content",
				"hx-push-url": "true",
				"hx-get": filters.BuildURL(showItemsPath,
					models.ExcludeCategories(),
					models.ExcludeItems(),
					models.WithFeeds(feed.GetID()),
				).String(),
			}),
			templates.WithActions(
				templates.NewAction("Mark Feed Read",
					templates.WithActionAttributes(templ.Attributes{
						"hx-post": filters.BuildURL(markItemsReadPath,
							models.ExcludeCategories(),
							models.ExcludeItems(),
							models.WithFeeds(feed.GetID()),
						).String(),
					}),
				),
				templates.NewAction("Mark Feed Unread",
					templates.WithActionAttributes(templ.Attributes{
						"hx-post": filters.BuildURL(markItemsUnreadPath,
							models.ExcludeCategories(),
							models.ExcludeItems(),
							models.WithFeeds(feed.GetID()),
						).String(),
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
	}

	if len(feeds) > 0 {
		home.WithContent(feeds...)(layout)
	}

	if err := layout.Render(req.Context(), res, htmx.NewResponse(), htmx.IsHTMX(req)); err != nil {
		logging.FromContext(req.Context()).Error("Show feeds failed.",
			slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

// (GET /home/show/items)
func (s Server) showItems(res http.ResponseWriter, req *http.Request, filters *models.APIFilters) {
	// Save list items filters in session storage.
	session.SaveListItemsFilters(req.Context(), filters)

	layout := home.BuildLayout(
		home.WithSideBar(
			menu.WithID("drawer_menu"),
			menu.WithItems(
				partials.AddSubscriptionButton().Show(),
				text.Build("View:", text.WithTextWeight(text.Semibold)).Show(),
				partials.StateFilter(showItemsPath, filters).Show(),
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
	feedFilters, err := session.LoadListFeedsFilters(req.Context())
	if err != nil {
		crumbs = append(crumbs, home.BuildCrumb("Feeds", text.Normal, templ.Attributes{
			"hx-get":      feedFilters.BuildURL(showFeedsPath),
			"hx-target":   "#content",
			"hx-push-url": "true",
		}))
	} else {
		crumbs = append(crumbs, home.BuildCrumb("Feeds", text.Normal, templ.Attributes{
			"hx-get":      showFeedsPath,
			"hx-target":   "#content",
			"hx-push-url": "true",
		}))
	}
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

func (s Server) HandlePaginateObjects(w http.ResponseWriter, r *http.Request, pType Type, pagination Pagination, params HandlePaginateObjectsParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

func (s Server) HandleActionObjects(w http.ResponseWriter, r *http.Request, objectAction Action, objectType ObjectType, params HandleActionObjectsParams) {
	w.WriteHeader(http.StatusNotImplemented)
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
	feedFilters, err := session.LoadListFeedsFilters(req.Context())
	if err != nil {
		crumbs = append(crumbs, home.BuildCrumb("Feeds", text.Normal, templ.Attributes{
			"hx-get":      feedFilters.BuildURL(showFeedsPath),
			"hx-target":   "#content",
			"hx-push-url": "true",
		}))
	} else {
		crumbs = append(crumbs, home.BuildCrumb("Feeds", text.Normal, templ.Attributes{
			"hx-get":      showFeedsPath,
			"hx-target":   "#content",
			"hx-push-url": "true",
		}))
	}
	// Build a items breadcrumb using any stored items filters.
	itemFilters, err := session.LoadListItemsFilters(req.Context())
	if err != nil {
		crumbs = append(crumbs, home.BuildCrumb("Items", text.Normal, templ.Attributes{
			"hx-get":      itemFilters.BuildURL(showItemsPath),
			"hx-target":   "#content",
			"hx-push-url": "true",
		}))
	} else {
		crumbs = append(crumbs, home.BuildCrumb("Items", text.Normal, templ.Attributes{
			"hx-get":      showItemsPath,
			"hx-target":   "#content",
			"hx-push-url": "true",
		}))
	}
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

func (s Server) HandleActionItem(w http.ResponseWriter, r *http.Request, objectAction Action, feed FeedID, item ItemID) {
	w.WriteHeader(http.StatusNotImplemented)
}
