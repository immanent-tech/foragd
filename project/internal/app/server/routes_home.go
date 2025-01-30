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
	"github.com/joshuar/go-feed-me/internal/config"
	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic/schema"
	"github.com/joshuar/go-feed-me/web/templates/layouts"
	"github.com/joshuar/go-feed-me/web/templates/panes"
)

// Navigation paths.
const (
	homeBasePath        = "/home"
	showBasePath        = homeBasePath + "/show"
	setBasePath         = homeBasePath + "/set"
	showFeedsBasePath   = showBasePath + "/feeds"
	showItemsBasePath   = showBasePath + "/items"
	showArticleBasePath = showBasePath + "/article"
	setFeedsBasePath    = setBasePath + "/feeds"
	setItemsBasePath    = setBasePath + "/items"
	setArticleBasePath  = setBasePath + "/article"
)

var ErrGeneratePageNavigationFailed = errors.New("error occurred while generating page navigation")

// ShowList will show a list of feeds or items, with filtering applied.
//
// `GET /home/show/{list}`.
func (s Server) ShowList(res http.ResponseWriter, req *http.Request, list List, params ShowListParams) {
	var (
		page  templ.Component
		cards templ.Component
	)

	logger := logging.NewHandlerLogger("ShowList", req)

	nav, err := createPageNavigation(req)
	if err != nil {
		logger.Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
	}

	// Load up a context with required values.
	ctx := req.Context()
	ctx = models.PageNavigationToCtx(ctx, nav)
	ctx = elastic.FeedsIndexToCtx(ctx, schema.FeedsSchemaPrefix)
	ctx = elastic.ItemsIndexToCtx(ctx, schema.FeedItemsSchemaPrefix+"_"+config.Environment())
	ctx = elastic.UserIndexToCtx(ctx, schema.UsersSchemaPrefix)

	filters, err := models.CreateFilters(params)
	if err != nil {
		logger.Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
	}

	switch list {
	case models.Feeds:
		// Save list feeds filters in session storage.
		session.SaveListFeedsFilters(ctx, filters)

		cards = renderFeedCards(ctx, s.API.elastic, filters)

	case models.Items:
		// Save list items filters in session storage.
		session.SaveListItemsFilters(ctx, filters)

		cards, _ = renderItemCards(ctx, s.API.elastic, filters)
	default:
		logger.Error("Bad request.",
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
		if err := page.Render(ctx, res); err != nil {
			logger.Error("Cannot display content.", slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
			http.Error(res, "Problem!", http.StatusInternalServerError)
		}
	} else {
		// HTMX request, load cards and update header/footer.
		resp := htmx.NewResponse()
		if err := resp.RenderTempl(ctx, res, cards); err != nil {
			logger.Error("Cannot display content.", slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
			http.Error(res, "Problem!", http.StatusInternalServerError)
		}

		if err := resp.RenderTempl(ctx, res, panes.Header()); err != nil {
			logger.Error("Cannot display content.", slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
			http.Error(res, "Problem!", http.StatusInternalServerError)
		}

		if err := resp.RenderTempl(ctx, res, panes.Footer()); err != nil {
			logger.Error("Cannot display content.", slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
			http.Error(res, "Problem!", http.StatusInternalServerError)
		}

		if err := resp.RenderTempl(ctx, res, panes.Drawer()); err != nil {
			logger.Error("Cannot display content.", slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
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
		get, err := url.JoinPath(showArticleBasePath, item.GetFeedID(), item.GetID())
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
func (s Server) SetListState(res http.ResponseWriter, req *http.Request, list List, state State, params SetListStateParams) {
	var err error

	logger := logging.NewHandlerLogger("MarkListRead", req)

	// Set up context.
	ctx := req.Context()
	ctx = elastic.UserIndexToCtx(ctx, schema.UsersSchemaPrefix)
	ctx = elastic.ItemsIndexToCtx(ctx, schema.FeedItemsSchemaPrefix+"_"+config.Environment())

	filters, err := models.CreateFilters(params)
	if err != nil {
		logger.Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
	}

	switch {
	case len(filters.GetItemsIDs()) > 0:
		err = s.API.elastic.UserActionMarkItems(ctx, state, filters.GetItemsIDs())
	case len(filters.GetFeedIDs()) > 0:
		err = s.API.elastic.UserActionMarkFeeds(ctx, state, filters.GetFeedIDs())
	}

	if err != nil {
		logger.Warn("Could not mark as read.", slog.Any("error", err))
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

	nav, err := createPageNavigation(req)
	if err != nil {
		logger.Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
	}

	ctx := req.Context()
	ctx = models.PageNavigationToCtx(ctx, nav)
	ctx = elastic.ItemsIndexToCtx(ctx, schema.FeedItemsSchemaPrefix+"_"+config.Environment())

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
			layouts.WithPageContent(layouts.HomeLayout(panes.Article(false, &item), panes.Drawer())))
	} else {
		page = panes.Article(true, &item)
	}

	if err := htmx.NewResponse().RenderTempl(ctx, res, page); err != nil {
		logger.Error("Cannot display content.", slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
	}
}

// MarkArticle will mark an article as read/unread.
func (s Server) SetArticleState(res http.ResponseWriter, req *http.Request, feed FeedID, item ItemID, state State) {
	logger := logging.NewHandlerLogger("MarkArticle", req)

	ctx := elastic.UserIndexToCtx(req.Context(), schema.UsersSchemaPrefix)

	if err := s.API.elastic.UserActionMarkItems(ctx, state, []string{item}); err != nil {
		logger.Warn("Could not mark item as read.", slog.Any("error", err))
		return
	}

	if _, err := res.Write(nil); err != nil {
		logger.Error("Failed to write response.", slog.Any("error", err))
	}
}

func (s Server) ActionArticle(res http.ResponseWriter, req *http.Request, action models.APIAction, feed FeedID, item ItemID) {
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

		childURL, err := url.Parse(showArticleBasePath)
		if err != nil {
			return navigation, errors.Join(ErrGeneratePageNavigationFailed, err)
		}

		navigation.Child = *childURL
	case strings.HasPrefix(req.URL.Path, showArticleBasePath):
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
