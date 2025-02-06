// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"

	components "github.com/joshuar/go-templ-daisyui"

	"github.com/joshuar/go-feed-me/internal/app/server/session"
	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/web/templates/layouts"
	"github.com/joshuar/go-feed-me/web/templates/panes"
)

// Navigation paths.
const (
	homeBasePath      = "/home"
	showFeedsBasePath = homeBasePath + "/show/list/" + "/feeds"
	showItemsBasePath = homeBasePath + "/show/list/" + "/items"
	showItemBasePath  = homeBasePath + "/show/item"
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
		case http.MethodGet:
			nav, err := createPageNavigation(req)
			if err != nil {
				logging.FromContext(ctx).Error("Bad request.",
					slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
				res.WriteHeader(http.StatusNotAcceptable)

				return
			}

			ctx = models.PageNavigationToCtx(ctx, nav)
		}

		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

// ShowList will show a list of feeds or items, with filtering applied.
//
// `GET /home/show/{list}`.
func (s Server) ShowList(res http.ResponseWriter, req *http.Request, action Action, list List, params ShowListParams) {
	var (
		page  templ.Component
		cards templ.Component
	)

	filters, err := models.CreateFilters(params)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
	}

	switch list {
	case models.Feeds:
		// Save list feeds filters in session storage.
		session.SaveListFeedsFilters(req.Context(), filters)

		cards = renderFeedCards(req.Context(), s.API.elastic, filters)

	case models.Items:
		// Save list items filters in session storage.
		session.SaveListItemsFilters(req.Context(), filters)

		cards, _ = renderItemCards(req.Context(), s.API.elastic, filters)
	default:
		logging.FromContext(req.Context()).Error("Bad request.",
			slog.String("list", string(list)),
			slog.Any("error", ErrInvalidQueryParams))
		res.WriteHeader(http.StatusBadRequest)

		return
	}

	if !htmx.IsHTMX(req) {
		// Regular request, load full page.
		page = layouts.Page("Go Feed Me - Home",
			layouts.WithPageDescription("Your home."),
			layouts.WithPageKeywords("feeds", "atom", "jsonfeed", "rss", "feed reader", "news", "current affairs"),
			layouts.WithPageContent(layouts.HomeLayout(panes.AllContent(panes.AppBarTop(), panes.Header(), panes.Footer(), cards), panes.Drawer())))
		if err := page.Render(req.Context(), res); err != nil {
			logging.FromContext(req.Context()).Error("Cannot display content.", slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
			http.Error(res, "Problem!", http.StatusInternalServerError)
		}
	} else {
		// HTMX request, load cards and update header/footer.
		resp := htmx.NewResponse()
		if err := resp.RenderTempl(req.Context(), res, cards); err != nil {
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

		if err := resp.RenderTempl(req.Context(), res, panes.Drawer()); err != nil {
			logging.FromContext(req.Context()).Error("Cannot display content.", slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
			http.Error(res, "Problem!", http.StatusInternalServerError)
		}
	}
}

func renderFeedCards(ctx context.Context, api models.UserActionsAPI, filters *models.APIFilters) templ.Component {
	feedCh, err := api.UserActionGetFeeds(ctx, *filters)
	if err != nil {
		logging.FromContext(ctx).Warn("Could not retrieve feeds.",
			slog.Any("error", err))
		return panes.EmptyContent()
	}

	cards := make([]templ.Component, 0, filters.GetCount())

	for feed := range feedCh {
		// Else, generate a feed card for display.
		card, err := panes.NewCard(ctx, &feed)
		if err != nil {
			logging.FromContext(ctx).Warn("Could not render item as card.",
				slog.Any("error", err))
			continue
		}
		// Add the URL for fetching items for this feed.
		card.Body.AddAttributes(templ.Attributes{
			"hx-get": showItemsBasePath + "?feeds=" + feed.GetID(),
		})
		// Append to the list of feed cards.
		cards = append(cards, components.Card(components.FromCardProps(card)))
	}

	if len(cards) == 0 {
		return panes.EmptyContent()
	}

	return components.ComponentArray(cards...)
}

func renderItemCards(ctx context.Context, api models.UserActionsAPI, filters *models.APIFilters) (templ.Component, models.Pagination) {
	itemCh, pagination, err := api.UserActionGetItems(ctx, *filters)
	if err != nil {
		logging.FromContext(ctx).Warn("Could not retrieve items.",
			slog.Any("error", err))
		return panes.EmptyContent(), pagination
	}

	ctx = models.ItemSetBasePathToCtx(ctx, setItemBasePath)

	cards := make([]templ.Component, 0, filters.GetCount())

	idx := 0

	for item := range itemCh {
		// Create item card properties.
		card, err := panes.NewCard(ctx, &item)
		if err != nil {
			logging.FromContext(ctx).Warn("Could not render item as card.",
				slog.Any("error", err))
			continue
		}
		// Generate URL for fetching article for item.
		get, err := url.JoinPath(showItemBasePath, item.GetFeedID(), item.GetID())
		if err != nil {
			logging.FromContext(ctx).Warn("Could not render item as card.",
				slog.Any("error", err))
			continue
		}
		// Add the URL for fetching the item article.
		card.Body.AddAttributes(templ.Attributes{
			"hx-get": get,
		})

		if idx == len(itemCh)-1 && pagination != "" && len(itemCh) == filters.GetCount() {
			paginationURL := *models.SetQueryParams(
				models.PageNavigationFromCtx(ctx).Current,
				map[string]string{
					"pagination": pagination,
				},
			)
			card.AddAttributes(templ.Attributes{
				"hx-get":       paginationURL.String(),
				"hx-trigger":   "revealed",
				"hx-swap":      "afterend",
				"hx-push-url":  "false",
				"hx-indicator": "#content-loading",
			})
		}
		// Append to the list of item cards.
		cards = append(cards, components.Card(components.FromCardProps(card)))
		idx++
	}

	if len(cards) == 0 {
		return panes.EmptyContent(), pagination
	}

	return components.ComponentArray(cards...), pagination
}

// MarkList will mark a list of feeds or items, with filtering applied, as read
// or unread.
//
// `POST /home/mark/{list}/{action}`.
func (s Server) ActionList(res http.ResponseWriter, req *http.Request, action Action, list List, params ActionListParams) {
	var err error

	filters, err := models.CreateFilters(params)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
	}

	switch {
	case len(filters.GetItemsIDs()) > 0:
		err = s.API.elastic.UserActionMarkItems(req.Context(), action, filters.GetItemsIDs())
	case len(filters.GetSubscriptionIDs()) > 0:
		err = s.API.elastic.UserActionMarkFeeds(req.Context(), action, filters.GetSubscriptionIDs())
	}

	if err != nil {
		logging.FromContext(req.Context()).Warn("Could not mark as read.", slog.Any("error", err))
		return
	}

	if _, err := res.Write(nil); err != nil {
		logging.FromContext(req.Context()).Error("Failed to write response.", slog.Any("error", err))
	}
}

func (s Server) ShowItem(res http.ResponseWriter, req *http.Request, action Action, feedID FeedID, itemID ItemID) {
	var page templ.Component

	item, found, err := s.API.elastic.UserActionGetItem(req.Context(), feedID, itemID)
	if err != nil || !found {
		logging.FromContext(req.Context()).Error("Could not get item.", slog.Any("error", err))
		http.Error(res, "Not found!.", http.StatusNotFound)

		return
	}

	if !htmx.IsHTMX(req) {
		page = layouts.Page("Go Feed Me - Home",
			layouts.WithPageDescription("Your home."),
			layouts.WithPageKeywords("feeds", "atom", "jsonfeed", "rss", "feed reader", "news", "current affairs"),
			layouts.WithPageContent(layouts.HomeLayout(panes.Article(false, &item), panes.Drawer())))
	} else {
		page = panes.Article(true, &item)
	}

	if err := htmx.NewResponse().RenderTempl(req.Context(), res, page); err != nil {
		logging.FromContext(req.Context()).Error("Cannot display content.", slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
	}
}

// MarkArticle will mark an article as read/unread.
func (s Server) ActionItem(res http.ResponseWriter, req *http.Request, action Action, feed FeedID, item ItemID) {
	if action == models.Show {
		logging.FromContext(req.Context()).Warn("Unsupported action.",
			slog.String("action", string(action)))
		return
	}

	switch action {
	case models.Markread, models.Markunread:
		if err := s.API.elastic.UserActionMarkItems(req.Context(), action, []string{item}); err != nil {
			logging.FromContext(req.Context()).Warn("Could not set item state.", slog.Any("error", err))
		}
	default:
		res.WriteHeader(http.StatusNotImplemented)
	}

	if _, err := res.Write(nil); err != nil {
		logging.FromContext(req.Context()).Error("Failed to write response.", slog.Any("error", err))
	}
}

func createPageNavigation(req *http.Request) (*models.APIPageNavigation, error) {
	navigation := &models.APIPageNavigation{
		Current: *req.URL,
		Action:  *req.URL,
	}

	switch {
	case strings.HasPrefix(req.URL.Path, showFeedsBasePath):
		navigation.Action.Path = setFeedsBasePath

		childURL, err := url.Parse(showItemsBasePath)
		if err != nil {
			return navigation, errors.Join(ErrGeneratePageNavigationFailed, err)
		}

		navigation.Child = *childURL
	case strings.HasPrefix(req.URL.Path, showItemsBasePath):
		navigation.Action.Path = setItemsBasePath

		parentFilters, err := session.LoadListFeedsFilters(req.Context())
		if err != nil {
			return navigation, errors.Join(ErrGeneratePageNavigationFailed, err)
		}

		parentURL, err := parentFilters.GenerateURL(showFeedsBasePath)
		if err != nil {
			return navigation, errors.Join(ErrGeneratePageNavigationFailed, err)
		}

		navigation.Parent = *parentURL

		childURL, err := url.Parse(showItemBasePath)
		if err != nil {
			return navigation, errors.Join(ErrGeneratePageNavigationFailed, err)
		}

		navigation.Child = *childURL
	case strings.HasPrefix(req.URL.Path, showItemBasePath):
		parentFilters, err := session.LoadListItemsFilters(req.Context())
		if err != nil {
			return navigation, errors.Join(ErrGeneratePageNavigationFailed, err)
		}

		parentURL, err := parentFilters.GenerateURL(showItemsBasePath)
		if err != nil {
			return navigation, errors.Join(ErrGeneratePageNavigationFailed, err)
		}

		navigation.Parent = *parentURL
	}

	logging.FromContext(req.Context()).
		Debug("Generated page navigation.",
			slog.Any("parent", navigation.Parent.String()),
			slog.Any("current", navigation.Current.String()),
			slog.Any("child", navigation.Child.String()),
		)

	return navigation, nil
}
