// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

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
	"github.com/joshuar/go-feed-me/web/templates/partials"
)

// Feed handles the routes for fetching feed details for a single, or all
// subscribed feeds.
func Feed(res http.ResponseWriter, req *http.Request, feedID string, storeAPI dataStore, cacheAPI cache) {
	logging.LogReq(req, http.StatusAccepted).Info("processing request")

	// No feedID, display all subscribed feeds.
	if feedID == "_all" || feedID == "" {
		showAllFeeds(res, req, storeAPI)
	} else {
		// Display a summary for the specified feed.
		showFeedSummary(res, req, feedID, cacheAPI)
	}
}

// showAllFeeds renders a list of subscribed feeds.
func showAllFeeds(res http.ResponseWriter, req *http.Request, storeAPI dataStore) {
	var feedCards []templ.Component

	feeds, err := storeAPI.GetSubscribedFeeds(req.Context())
	if err != nil {
		logging.FromContext(req.Context()).
			Error("Could not add item.", slog.Any("error", err))
	}

	// Generate cards for each feed.
	for _, feed := range feeds {
		card := components.NewCard(feed.ID,
			components.WithCardLayout(components.CardLayoutSide),
			components.WithTitle(feed.Title),
			components.CardClasses("btn"),
			components.CardAttributes(templ.Attributes{
				"hx-target": "#secondaryPane",
				"hx-get":    "/feed/" + feed.ID,
			}),
		)

		if feed.ImageURL != "" {
			image := components.NewImage(
				components.WithURL(feed.ImageURL),
				components.WithAltText(feed.ImageTitle),
			)
			card.Image = &image
		}

		feedCards = append(feedCards, card.Show())
	}

	// Combine feed cards into a list.
	cardList := components.NewList(components.ListUnordered,
		components.WithItems(feedCards...))

	// Render the list of feed cards.
	if err := htmx.NewResponse().
		RenderTempl(req.Context(), res, cardList.Show()); err != nil {
		logging.LogReq(req, http.StatusInternalServerError).Error("showAllFeeds: cannot render template.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)

		return
	}
}

func showFeedSummary(res http.ResponseWriter, req *http.Request, feedID string, cacheAPI cache) {
	slog.Info("searching...")
	items, err := cacheAPI.GetFeedItemsSummary(req.Context(), feedID)
	if err != nil {
		logging.FromContext(req.Context()).
			Error("Could not show feed items.", slog.Any("error", err))
	}

	var itemCards []components.Card

	for _, item := range items {
		card := components.NewCard(item.ID,
			components.WithCardLayout(components.CardLayoutSide),
			components.WithTitle(item.Title),
			// components.CardClasses("btn"),
			// components.CardAttributes(templ.Attributes{
			// 	"hx-target": "#secondaryPane",
			// 	"hx-get":    "/feed/" + feed.ID,
			// }),
		)

		if item.Image.URL != "" {
			image := components.NewImage(
				components.WithURL(item.Image.URL),
				components.WithAltText(item.Image.Title),
			)
			card.Image = &image
		}

		itemCards = append(itemCards, card)
	}

	// Render the list of feed cards.
	if err := htmx.NewResponse().
		RenderTempl(req.Context(), res, partials.FeedItemsSummary(itemCards...)); err != nil {
		logging.LogReq(req, http.StatusInternalServerError).Error("showFeedSummary: cannot render template.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)

		return
	}
}

func showFeed(res http.ResponseWriter, req *http.Request, feedID string, storeAPI dataStore) {
	subscriptions, err := storeAPI.GetAllSubscriptions(req.Context())
	if err != nil {
		logging.FromContext(req.Context()).
			Error("Could not add item.", slog.Any("error", err))
	}
	godump.Dump(subscriptions)
}

func FeedItem(res http.ResponseWriter, req *http.Request, feedID string, itemID string, storeAPI dataStore) {
	res.WriteHeader(http.StatusNotImplemented)
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
