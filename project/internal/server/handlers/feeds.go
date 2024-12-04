// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//revive:disable:get-return
package handlers

import (
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"

	components "github.com/joshuar/go-templ-daisyui"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/web/templates/partials"
)

// ShowFeeds displays the details for the given feeds.
func ShowFeeds(res http.ResponseWriter, req *http.Request, cache models.Cache, db models.DB, filters *models.Filters) {
	var feedCards []components.Card
	// Get all subscribed feeds.
	feeds, err := models.GetSubcribedFeeds(req.Context(), cache, db, filters.Feeds...)
	if err != nil {
		logging.FromContext(req.Context()).
			Error("Could not add item.", slog.Any("error", err))
	}
	// Generate cards for each feed.
	for _, feed := range feeds {
		feedCards = append(feedCards, newFeedCard(feed))
	}
	// Render the list of feed cards.
	if err := htmx.NewResponse().
		RenderTempl(req.Context(), res, partials.ShowCardList(feedCards...)); err != nil {
		logging.LogReq(req, http.StatusInternalServerError).Error("showAllFeeds: cannot render template.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)

		return
	}
}

// showAllItems shows a list of all items from all subscribed feeds as cards.
func showAllItems(res http.ResponseWriter, req *http.Request, cache models.Cache, db models.DB) {
	var (
		itemCards []components.Card
		feedIDs   []string
	)
	// Get all subscribed feeds.
	feeds, err := models.GetSubcribedFeeds(req.Context(), cache, db)
	if err != nil {
		logging.FromContext(req.Context()).
			Error("Could not add item.", slog.Any("error", err))
	}
	// Get a list of the feed IDs.
	for _, feed := range feeds {
		feedIDs = append(feedIDs, feed.ID)
	}
	// Get all feed items for all subscribed feeds.
	items, err := cache.GetFeedItems(req.Context(), feedIDs...)
	if err != nil {
		logging.FromContext(req.Context()).
			Error("Could not show feed items.", slog.Any("error", err))
	}
	// Create item cards.
	for _, item := range items {
		itemCards = append(itemCards, newItemCard(item))
	}
	// Render the list of feed cards.
	if err := htmx.NewResponse().
		RenderTempl(req.Context(), res, partials.ShowCardList(itemCards...)); err != nil {
		logging.LogReq(req, http.StatusInternalServerError).Error("showFeedSummary: cannot render template.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)

		return
	}
}

// showItems shows a list of items for a given feed as cards.
func showItems(res http.ResponseWriter, req *http.Request, cache models.Cache, feedID string) {
	var itemCards []components.Card

	items, err := cache.GetFeedItems(req.Context(), feedID)
	if err != nil {
		logging.FromContext(req.Context()).
			Error("Could not show feed items.", slog.Any("error", err))
	}

	for _, item := range items {
		itemCards = append(itemCards, newItemCard(item))
	}

	// Render the list of feed cards.
	if err := htmx.NewResponse().
		RenderTempl(req.Context(), res, partials.ShowCardList(itemCards...)); err != nil {
		logging.LogReq(req, http.StatusInternalServerError).Error("showFeedSummary: cannot render template.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)

		return
	}
}

// func showFeed(res http.ResponseWriter, req *http.Request, feedID string, db dbAPI) {
// 	subscriptions, err := db.GetAllSubscriptions(req.Context())
// 	if err != nil {
// 		logging.FromContext(req.Context()).
// 			Error("Could not add item.", slog.Any("error", err))
// 	}
// 	godump.Dump(subscriptions)
// }

func newFeedCard(feed models.APIFeed) components.Card {
	card := components.NewCard(
		components.WithCardLayout(components.CardLayoutSide),
		components.WithTitle(feed.Title),
		components.WithCardShadow(components.SM),
		components.WithID[components.Card](feed.ID),
		components.WithAttributes[components.Card](templ.Attributes{
			"hx-target":  "#content",
			"hx-get":     "/list/items",
			"hx-include": "[id='" + feed.ID + "']",
		}),
		components.WithBody(templ.Raw(feed.Description)),
	)

	if feed.Image != nil {
		image := components.NewImage(
			components.WithURL(feed.Image.URL),
		)

		if feed.Image.Title != "" {
			image.Alt = feed.Image.Title
		}

		card.Image = &image
	}

	if len(feed.Categories) > 0 {
		var categories []components.Badge
		for _, c := range feed.Categories {
			categories = append(categories, components.NewBadge(c))
		}
		card.Badges = categories
	}

	// card.Badges = append(card.Badges, components.NewBadge(feed.LastFetched.String()))

	return card
}

func newItemCard(item models.APIItem) components.Card {
	card := components.NewCard(
		components.WithCardLayout(components.CardLayoutSide),
		components.WithTitle(item.Title),
		components.WithCardShadow(components.XL),
		// components.WithBody(partials.FeedItemDescription(item.Description)),
		// components.CardClasses("btn"),
		// components.CardAttributes(templ.Attributes{
		// 	"hx-target": "#secondaryPane",
		// 	"hx-get":    "/feed/" + feed.ID,
		// }),
	)

	if item.Image != nil {
		image := components.NewImage(
			components.WithURL(item.Image.URL),
			components.WithAltText(item.Image.Title),
			components.WithClasses[components.Image]("max-h-full"),
		)
		card.Image = &image
	}

	if len(item.Categories) > 0 {
		var categories []components.Badge
		for _, c := range item.Categories {
			categories = append(categories, components.NewBadge(c))
		}
		card.Badges = categories
	}

	return card
}

// GetFeedItemHandler handles /home/feed/item endpoints.
func GetFeedItemHandler(res http.ResponseWriter, req *http.Request, feedID string, itemID string, cache models.Cache, db models.DB) {
	// if feedID == paramAll && itemID == "_all" {
	// 	showAllItems(res, req, cache, db)
	// }
}

// func HomeFeed(res http.ResponseWriter, req *http.Request, websocket *gws.Upgrader) {
// 	socket, err := websocket.Upgrade(res, req)
// 	if err != nil {
// 		return
// 	}

// 	// go func() {
// 	// 	<-req.Context().Done()
// 	// 	socket.WriteClose(1000, []byte(`closing websocket`))
// 	// }()

// 	go func() {
// 		socket.ReadLoop() // Blocking prevents the context from being GC.
// 	}()

// 	for i := range 5 {
// 		socket.WriteString(`<div hx-swap-oob="beforeend:#items"><div class="join-item">Button</div></div>`)
// 		time.Sleep(time.Second)
// 		i++
// 	}
// }

// const (
// 	PingInterval = 5 * time.Second
// 	PingWait     = 10 * time.Second
// )

// type FeedItemWebsocketHandler struct{}

// func (c *FeedItemWebsocketHandler) OnOpen(socket *gws.Conn) {
// 	slog.Debug("New connection.")
// 	// _ = socket.SetDeadline(time.Now().Add(PingInterval + PingWait))
// }

// func (c *FeedItemWebsocketHandler) OnClose(socket *gws.Conn, err error) {
// 	slog.Debug("Closing connection.")
// }

// func (c *FeedItemWebsocketHandler) OnPing(socket *gws.Conn, payload []byte) {
// 	// _ = socket.SetDeadline(time.Now().Add(PingInterval + PingWait))
// 	_ = socket.WritePong(nil)
// }

// func (c *FeedItemWebsocketHandler) OnPong(socket *gws.Conn, payload []byte) {}

// func (c *FeedItemWebsocketHandler) OnMessage(socket *gws.Conn, message *gws.Message) {
// 	defer message.Close()
// 	godump.Dump(string(message.Bytes()))
// 	socket.WriteString(`<div hx-swap-oob="beforeend:#items"><div class="join-item">Button</div></div>`)
// 	// socket.WriteMessage(message.Opcode, message.Bytes())
// 	// socket.WriteString("hello")
// }
