// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//nolint:lll
package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"

	components "github.com/joshuar/go-templ-daisyui"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/server/session"
	"github.com/joshuar/go-feed-me/web/templates/layouts"
	"github.com/joshuar/go-feed-me/web/templates/partials/content"
)

const (
	showFeedsPath     = "/home/show/feeds"
	markFeedsReadPath = "/home/mark/feeds/read"
	showItemsPath     = "/home/show/items"
	markItemsReadPath = "/home/mark/items/read"
	showArticlePath   = "/home/show/article"
	markArticlePath   = "/home/mark/article/read"

	defaultCount = 10
)

// ShowList will show a list of feeds or items, with filtering applied.
//
// `GET /home/show/{list}`.
func (s Server) ShowList(res http.ResponseWriter, req *http.Request, list ShowListParamsList, params ShowListParams) {
	var (
		cards   []templ.Component
		filters models.APISearchFilters
	)

	logger := logging.NewHandlerLogger("ShowList", req)
	ctx := req.Context()

	if params.Feeds != nil {
		filters.FeedIDs = *params.Feeds
	}

	if params.Categories != nil {
		filters.Categories = *params.Categories
	}

	if params.Count != nil {
		filters.Count = *params.Count
	} else {
		filters.Count = defaultCount
	}

	if params.Pagination != nil {
		if pagination, err := url.QueryUnescape(*params.Pagination); err != nil {
			slog.Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
		} else {
			filters.Pagination = []byte(pagination)
		}
	}

	// Bail if an invalid show parameter is requested.
	if list != ShowListParamsListFeeds && list != ShowListParamsListItems {
		logger.Error("Bad request.",
			slog.String("list", string(list)),
			slog.Any("error", ErrInvalidQueryParams))
		res.WriteHeader(http.StatusBadRequest)

		return
	}

	// Get all subscribed feeds.
	feeds, err := s.API.elastic.UserActionGetFeeds(ctx, filters)
	if err != nil && errors.Is(err, models.ErrNoSubscriptions) {
		if err = renderHome(ctx, res, req, content.ShowEmptyContent()); err != nil {
			logger.Error("Cannot display content.", slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
			http.Error(res, "Problem!", http.StatusInternalServerError)
		}

		return
	}

	if err != nil {
		logger.Error("Cannot display content.", slog.Any("error", err))
		http.Error(res, "Problem!", http.StatusInternalServerError)

		return
	}

	switch list {
	case ShowListParamsListFeeds:
		// Save list feeds filters in session storage.
		session.SaveListFeedsFilters(ctx, filters)

		ctx = content.NavigationToCtx(ctx, content.NavigationLinks{
			RefreshPath:         generateActionLink(ctx, showFeedsPath),
			MarkReadPath:        generateActionLink(ctx, markFeedsReadPath),
			ActionBasePath:      showFeedsPath,
			ChildActionBasePath: showItemsPath,
			Count:               filters.Count,
		})

		for _, feed := range feeds {
			// Else, generate a feed card for display.
			card, err := content.NewCard(&feed, 0)
			if err != nil {
				logger.Warn("Could not render item as card.", slog.Any("error", err))
				continue
			}
			// Add the URL for fetching items for this feed.
			card.Body.AddAttributes(templ.Attributes{
				"hx-get": showItemsPath + "?feeds=" + feed.GetID(),
			})
			// Append to the list of feed cards.
			cards = append(cards, components.Card(components.FromCardProps(card)))
		}

	case ShowListParamsListItems:
		// Save list items filters in session storage.
		session.SaveListItemsFilters(ctx, filters)

		items, pagination, err := s.API.elastic.UserActionGetItems(ctx, filters)
		if err != nil {
			logger.Warn("Could not retrieve items.", slog.Any("error", err))
		}

		ctx = content.NavigationToCtx(ctx, content.NavigationLinks{
			BackPath:            generateActionLink(ctx, showFeedsPath),
			RefreshPath:         generateActionLink(ctx, showItemsPath),
			MarkReadPath:        generateActionLink(ctx, markArticlePath),
			Pagination:          generatePagination(ctx, showItemsPath, pagination),
			Count:               filters.Count,
			ActionBasePath:      showItemsPath,
			ChildActionBasePath: showArticlePath,
		})

		for i, item := range items {
			// Create item card properties.
			card, err := content.NewCard(&item, 0)
			if err != nil {
				logger.Warn("Could not render item as card.", slog.Any("error", err))
				continue
			}
			// Generate URL for fetching article for item.
			get, err := url.JoinPath(showArticlePath, item.GetFeedID(), item.GetID())
			if err != nil {
				logger.Warn("Could not render item as card.", slog.Any("error", err))
				continue
			}
			// Add the URL for fetching the item article.
			card.Body.AddAttributes(templ.Attributes{
				"hx-get": get,
			})

			if i == len(items)-1 && pagination != nil && len(items) == filters.Count {
				slog.Info("last item", slog.Any("item", item))
				card.AddAttributes(templ.Attributes{
					"hx-get":       generatePagination(ctx, showItemsPath, pagination),
					"hx-trigger":   "revealed",
					"hx-swap":      "afterend",
					"hx-push-url":  "false",
					"hx-indicator": "#content-loading",
				})
			}
			// Append to the list of item cards.
			cards = append(cards, components.Card(components.FromCardProps(card)))
		}
	}

	if err := renderHome(ctx, res, req, content.ShowContent(cards...)); err != nil {
		logger.Error("Cannot display content.", slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
	}
}

func renderHome(ctx context.Context, res http.ResponseWriter, req *http.Request, pageContent templ.Component) error {
	var page templ.Component

	if !htmx.IsHTMX(req) {
		// Full page when not htmx.
		page = layouts.Page("Go Feed Me - Home",
			layouts.WithPageDescription("Your home."),
			layouts.WithPageKeywords("feeds", "atom", "jsonfeed", "rss", "feed reader", "news", "current affairs"),
			layouts.WithPageContent(layouts.HomeLayout(pageContent)))
	} else {
		// Partial content otherwise.
		page = pageContent
	}

	return htmx.NewResponse().RenderTempl(ctx, res, page)
}

// MarkList will mark a list of feeds or items, with filtering applied, as read
// or unread.
//
// `POST /home/mark/{list}/{action}`.
func (s Server) MarkList(res http.ResponseWriter, req *http.Request, list MarkListParamsList, action MarkListParamsAction, params MarkListParams) {
	var (
		filters     models.APISearchFilters
		items       []models.APIItem
		pagination  []byte
		err         error
		unreadItems []models.APIReadItem
	)

	logger := logging.NewHandlerLogger("MarkListRead", req)
	ctx := req.Context()

	if params.Feeds != nil {
		filters.FeedIDs = *params.Feeds
	}

	if params.Categories != nil {
		filters.Categories = *params.Categories
	}

	filters.Count = 100

	// Fetch the unread items with the given filters. Paginate through the
	// results, collecting into unreadItems.
	for {
		if pagination != nil {
			filters.Pagination = pagination
		}

		items, pagination, err = s.API.elastic.UserActionGetItems(ctx, filters)
		if err != nil {
			logger.Warn("Could not retrieve items.", slog.Any("error", err))
		}

		if len(items) == 0 {
			break
		}

		for _, item := range items {
			unreadItems = append(unreadItems, models.APIReadItem{
				ItemID: item.ID,
				FeedID: item.FeedID,
			})
		}
	}

	// Mark all unreadItems as read.
	if err := s.API.elastic.UserActionMarkItemsRead(req.Context(), unreadItems...); err != nil {
		logger.Warn("Could not mark item as read.", slog.Any("error", err))
		return
	}

	if _, err := res.Write(nil); err != nil {
		logger.Error("Failed to write response.", slog.Any("error", err))
	}
}

// ShowArticle will show an item as an article.
//
// `GET /home/show/article/{feedID}/{itemID}`.
func (s Server) ShowArticle(res http.ResponseWriter, req *http.Request, feedID FeedID, itemID ItemID) {
	var page templ.Component

	logger := logging.NewHandlerLogger("ShowArticle", req)

	ctx := req.Context()

	ctx = content.NavigationToCtx(ctx, content.NavigationLinks{
		BackPath:       generateActionLink(req.Context(), showItemsPath),
		ActionBasePath: showArticlePath,
	})

	item, found, err := s.API.elastic.UserActionGetItem(ctx, feedID, itemID)
	if err != nil || !found {
		logger.Error("Could not get item.", slog.Any("error", err))
		http.Error(res, "Not found!.", http.StatusNotFound)

		return
	}

	if !htmx.IsHTMX(req) {
		page = layouts.Page("Go Feed Me - Home",
			layouts.WithPageDescription("Your home."),
			layouts.WithPageKeywords("feeds", "atom", "jsonfeed", "rss", "feed reader", "news", "current affairs"),
			layouts.WithPageContent(layouts.HomeLayout(content.ShowArticle(&item))))
	} else {
		page = content.ShowArticle(&item)
	}

	if err := htmx.NewResponse().RenderTempl(ctx, res, page); err != nil {
		logger.Error("Cannot display content.", slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
	}
}

func (s Server) ShowArticleMenu(res http.ResponseWriter, req *http.Request, feed FeedID, item ItemID) {
	logger := logging.NewHandlerLogger("ShowArticleMenu", req)
	logger.Warn("Unimplmented.")
	res.WriteHeader(http.StatusNotImplemented)
}

// MarkArticle will mark an article as read/unread.
//
// `POST /home/mark/article/{action}/{feedID}/{itemID}`.
func (s Server) MarkArticle(res http.ResponseWriter, req *http.Request, action MarkArticleParamsAction, feed FeedID, item ItemID) {
	logger := logging.NewHandlerLogger("MarkArticle", req)

	switch action {
	case MarkArticleParamsActionRead:
		item := models.APIReadItem{
			ItemID: item,
			FeedID: feed,
		}

		if err := s.API.elastic.UserActionMarkItemsRead(req.Context(), item); err != nil {
			logger.Warn("Could not mark item as read.", slog.Any("error", err))
			return
		}

		if _, err := res.Write(nil); err != nil {
			logger.Error("Failed to write response.", slog.Any("error", err))
		}
	default:
		logger.Warn("Unimplmented.")
		res.WriteHeader(http.StatusNotImplemented)
	}
}

// generateActionLink creates a URL string that can be used for actions that
// manipulate the current page.
func generateActionLink(ctx context.Context, path string) string {
	var (
		filters models.APISearchFilters
		err     error
	)

	switch path {
	case showFeedsPath, markFeedsReadPath:
		filters, err = session.LoadListFeedsFilters(ctx)
	case showItemsPath, markItemsReadPath:
		filters, err = session.LoadListItemsFilters(ctx)
	}

	if err != nil {
		logging.FromContext(ctx).Warn("Could not generate backlink.",
			slog.Any("error", err))
		return path
	}

	backlink, err := filters.GenerateURL(path)
	if err != nil {
		logging.FromContext(ctx).Warn("Could not generate backlink.",
			slog.Any("error", err))
		return path
	}

	return backlink.String()
}

// generatePagination generates a URL string with an updated pagination value.
func generatePagination(ctx context.Context, path string, pagination []byte) string {
	var (
		filters models.APISearchFilters
		err     error
	)

	switch path {
	case showFeedsPath:
		filters, err = session.LoadListFeedsFilters(ctx)
	case showItemsPath:
		filters, err = session.LoadListItemsFilters(ctx)
	}

	if err != nil {
		logging.FromContext(ctx).Warn("Could not generate pagination link.",
			slog.Any("error", err))
		return path
	}

	paginationLink, err := filters.GenerateURL(path)
	if err != nil {
		logging.FromContext(ctx).Warn("Could not generate pagination link.",
			slog.Any("error", err))
		return path
	}

	q := paginationLink.Query()
	q.Del("pagination")
	q.Add("pagination", url.QueryEscape(string(pagination)))
	paginationLink.RawQuery = q.Encode()

	return paginationLink.String()
}
