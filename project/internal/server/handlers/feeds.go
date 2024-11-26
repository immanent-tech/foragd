// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//revive:disable:get-return
package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/lxzan/gws"
	"github.com/yassinebenaid/godump"

	components "github.com/joshuar/go-templ-daisyui"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/web/templates/partials"
)

// GetFeedHandler handles /home/feed endpoints.
func GetFeedHandler(res http.ResponseWriter, req *http.Request, feedID string, db dbAPI, cache cacheAPI) {
	logging.LogReq(req, http.StatusAccepted).Info("processing request")

	if feedID == "_all" || feedID == "" {
		// No feedID given, or special "_all" value requested. Display all
		// subscribed feeds.
		showAllFeeds(res, req, db)
	} else {
		// Display a summary for the specified feed.
		showItems(res, req, feedID, cache)
	}
}

// showAllFeeds shows a list of all subscribed feeds as cards.
func showAllFeeds(res http.ResponseWriter, req *http.Request, db dbAPI) {
	var feedCards []components.Card
	// Get all subscribed feeds.
	feeds, err := db.GetSubscribedFeeds(req.Context())
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
func showAllItems(res http.ResponseWriter, req *http.Request, db dbAPI, cache cacheAPI) {
	var (
		itemCards []components.Card
		feedIDs   []string
	)
	// Get all subscribed feeds.
	feeds, err := db.GetSubscribedFeeds(req.Context())
	if err != nil {
		logging.FromContext(req.Context()).
			Error("Could not add item.", slog.Any("error", err))
	}
	// Get a list of the feed IDs.
	for _, feed := range feeds {
		feedIDs = append(feedIDs, feed.ID)
	}
	// Get all feed items for all subscribed feeds.
	items, err := cache.GetFeedItemsSummary(req.Context(), feedIDs...)
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
func showItems(res http.ResponseWriter, req *http.Request, feedID string, cache cacheAPI) {
	var itemCards []components.Card

	items, err := cache.GetFeedItemsSummary(req.Context(), feedID)
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
	card := components.NewCard(feed.ID,
		components.WithCardLayout(components.CardLayoutSide),
		components.WithTitle(feed.Title, components.H2),
		components.WithCardShadow(components.SM),
		components.CardAttributes(templ.Attributes{
			"hx-target": "#secondaryPane",
			"hx-get":    "/feed/" + feed.ID,
		}),
		components.WithBody(templ.Raw(feed.Description)),
	)

	if feed.ImageURL != nil {
		image := components.NewImage(
			components.WithURL(*feed.ImageURL),
		)

		if feed.ImageTitle != nil {
			image.Alt = *feed.ImageTitle
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

	return card
}

func newItemCard(item models.APIItem) components.Card {
	card := components.NewCard(item.ID,
		components.WithCardLayout(components.CardLayoutSide),
		components.WithTitle(item.Title, components.H2),
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
func GetFeedItemHandler(res http.ResponseWriter, req *http.Request, feedID string, itemID string, db dbAPI, cache cacheAPI) {
	if feedID == "_all" && itemID == "_all" {
		slog.Info("here")
		showAllItems(res, req, db, cache)
	}
}

func HomeFeed(res http.ResponseWriter, req *http.Request, websocket *gws.Upgrader) {
	socket, err := websocket.Upgrade(res, req)
	if err != nil {
		return
	}

	// go func() {
	// 	<-req.Context().Done()
	// 	socket.WriteClose(1000, []byte(`closing websocket`))
	// }()

	go func() {
		socket.ReadLoop() // Blocking prevents the context from being GC.
	}()

	for i := range 5 {
		socket.WriteString(`<div hx-swap-oob="beforeend:#items"><div class="join-item">Button</div></div>`)
		time.Sleep(time.Second)
		i++
	}
}

const (
	PingInterval = 5 * time.Second
	PingWait     = 10 * time.Second
)

type FeedItemWebsocketHandler struct{}

func (c *FeedItemWebsocketHandler) OnOpen(socket *gws.Conn) {
	slog.Debug("New connection.")
	// _ = socket.SetDeadline(time.Now().Add(PingInterval + PingWait))
}

func (c *FeedItemWebsocketHandler) OnClose(socket *gws.Conn, err error) {
	slog.Debug("Closing connection.")
}

func (c *FeedItemWebsocketHandler) OnPing(socket *gws.Conn, payload []byte) {
	// _ = socket.SetDeadline(time.Now().Add(PingInterval + PingWait))
	_ = socket.WritePong(nil)
}

func (c *FeedItemWebsocketHandler) OnPong(socket *gws.Conn, payload []byte) {}

func (c *FeedItemWebsocketHandler) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()
	godump.Dump(string(message.Bytes()))
	socket.WriteString(`<div hx-swap-oob="beforeend:#items"><div class="join-item">Button</div></div>`)
	// socket.WriteMessage(message.Opcode, message.Bytes())
	// socket.WriteString("hello")
}
