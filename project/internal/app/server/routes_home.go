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

	"github.com/joshuar/go-feed-me/internal/app/server/session"
	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/web/templates"
	"github.com/joshuar/go-feed-me/web/templates/layouts"
	"github.com/joshuar/go-feed-me/web/templates/panes"
	"github.com/joshuar/go-feed-me/web/templates/partials/drawer"
)

// Navigation paths.
const (
	homeBasePath = "/home"

	showFeedsBasePath = homeBasePath + "/show/feeds"
	showItemsBasePath = homeBasePath + "/show/items"
	showItemBasePath  = homeBasePath + "/show/"
	setFeedsBasePath  = homeBasePath + "/markread/list/feeds"
	setItemsBasePath  = homeBasePath + "/markread/list/items"
	setItemBasePath   = homeBasePath + "/markread/item"
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

	feedCh, err := s.API.elastic.UserActionGetFeeds(req.Context(), *filters)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Could not retrieve feeds.",
			slog.Any("error", err))
		renderHome(res, req, panes.EmptyContent())

		return
	}

	feeds := make([]templ.Component, 0, filters.GetCount())

	for feed := range feedCh {
		component, err := templates.NewComponent(feed,
			templates.DisplayAs(templates.FeedCard),
			templates.WithAttributes(templ.Attributes{
				"hx-target":   "#content",
				"hx-push-url": "true",
				"hx-get":      filepath.Join("/home", "show", "items") + "?feeds=" + feed.GetID(),
			}),
			templates.WithActions(
				templates.NewAction("Mark Feed Read",
					templates.WithActionAttributes(templ.Attributes{
						"hx-post": filepath.Join(string(Markread), "feeds") + "?feeds=" + feed.GetID(),
					}),
				),
				templates.NewAction("Mark Feed Unread",
					templates.WithActionAttributes(templ.Attributes{
						"hx-post": filepath.Join(string(Markunread), "feeds") + "?feeds=" + feed.GetID(),
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

		card, err := component.Show()
		if err != nil {
			logging.FromContext(req.Context()).Warn("Could not render card for feed.",
				slog.String("feed_id", feed.GetID()),
				slog.Any("error", err))

			continue
		}

		feeds = append(feeds, card)
	}

	if len(feeds) == 0 {
		renderHome(res, req, panes.EmptyContent())
	}

	renderHome(res, req, feeds...)
}

// (GET /home/show/items)
func (s Server) showItems(res http.ResponseWriter, req *http.Request, filters *models.APIFilters) {
	// Save list items filters in session storage.
	session.SaveListItemsFilters(req.Context(), filters)

	itemCh, pagination, err := s.API.elastic.UserActionGetItems(req.Context(), *filters)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Could not retrieve items.",
			slog.Any("error", err))
		renderHome(res, req, panes.EmptyContent())

		return
	}

	items := make([]templ.Component, 0, filters.GetCount())

	idx := 0

	for item := range itemCh {
		component, err := templates.NewComponent(item,
			templates.DisplayAs(templates.ItemCard),
			templates.WithAttributes(templ.Attributes{
				"hx-target":   "#content",
				"hx-push-url": "true",
				"hx-get":      filepath.Join("home", "show", item.GetFeedID(), item.GetID()),
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
			logging.FromContext(req.Context()).Warn("Could not create card component for feed.",
				slog.String("feed_id", item.GetID()),
				slog.Any("error", err))

			continue
		}

		if idx == len(itemCh)-1 && pagination != "" && len(itemCh) == filters.GetCount() {
			component.AddAttributes(templ.Attributes{
				"hx-get":       filepath.Join("home", "show", "items") + "?pagination=" + pagination,
				"hx-trigger":   "revealed",
				"hx-swap":      "afterend",
				"hx-push-url":  "false",
				"hx-indicator": "#content-loading",
			})
		}

		card, err := component.Show()
		if err != nil {
			logging.FromContext(req.Context()).Warn("Could not render card for feed.",
				slog.String("feed_id", item.GetID()),
				slog.Any("error", err))

			continue
		}

		items = append(items, card)
		idx++
	}

	if len(items) == 0 {
		renderHome(res, req, panes.EmptyContent())
	}

	renderHome(res, req, items...)
}

func renderHome(res http.ResponseWriter, req *http.Request, items ...templ.Component) {
	if !htmx.IsHTMX(req) {
		// Regular request, load full page.
		if err := layouts.Page("Go Feed Me - Home",
			layouts.WithPageDescription("Your home."),
			layouts.WithPageKeywords("feeds", "atom", "jsonfeed", "rss", "feed reader", "news", "current affairs"),
			layouts.WithPageContent(layouts.HomeLayout(panes.AllContent(panes.AppBarTop(), panes.Header(), panes.Footer(), items...), buildSideDrawer()))).
			Render(req.Context(), res); err != nil {
			logging.FromContext(req.Context()).Error("Cannot display content.", slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
			http.Error(res, "Problem!", http.StatusInternalServerError)
		}
	} else {
		// HTMX request, load cards and update header/footer.
		resp := htmx.NewResponse()
		if err := resp.RenderTempl(req.Context(), res, panes.Items(items...)); err != nil {
			logging.FromContext(req.Context()).Error("Cannot display content.", slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
			http.Error(res, "Problem!", http.StatusInternalServerError)
		}

		if err := resp.RenderTempl(req.Context(), res, panes.Header()); err != nil {
			logging.FromContext(req.Context()).Error("Cannot display content.", slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
			http.Error(res, "Problem!", http.StatusInternalServerError)
		}

		if err := resp.RenderTempl(req.Context(), res, panes.Footer()); err != nil {
			logging.FromContext(req.Context()).Error("Cannot display content.", slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
			http.Error(res, "Problem!", http.StatusInternalServerError)
		}

		if err := resp.RenderTempl(req.Context(), res, buildSideDrawer()); err != nil {
			logging.FromContext(req.Context()).Error("Cannot display content.", slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
			http.Error(res, "Problem!", http.StatusInternalServerError)
		}
	}
}

func (s Server) HandlePaginateObjects(w http.ResponseWriter, r *http.Request, pType Type, pagination Pagination, params HandlePaginateObjectsParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

func (s Server) HandleActionObjects(w http.ResponseWriter, r *http.Request, objectAction Action, objectType ObjectType, params HandleActionObjectsParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

func (s Server) HandleShowItem(w http.ResponseWriter, r *http.Request, feed FeedID, item ItemID) {
	w.WriteHeader(http.StatusNotImplemented)
}

func (s Server) HandleActionItem(w http.ResponseWriter, r *http.Request, objectAction Action, feed FeedID, item ItemID) {
	w.WriteHeader(http.StatusNotImplemented)
}

func buildSideDrawer() templ.Component {
	return drawer.BuildSide(
		drawer.WithID("drawer_menu"),
		drawer.WithActionButtons(
			drawer.AddSubscriptionButton(templ.Attributes{
				"hx-get":    "/subscription/new",
				"hx-target": "#command_modal",
				"_":         "on htmx:afterOnLoad wait 10ms then add .modal-open to #add_subscription_modal",
			}),
		),
	).Show()
}
