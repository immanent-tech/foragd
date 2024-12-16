// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//revive:disable:get-return
package handlers

import (
	"log/slog"
	"net/http"

	"github.com/angelofallars/htmx-go"

	components "github.com/joshuar/go-templ-daisyui"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/web/templates/content"
)

// ShowFeeds displays the details for the given feeds.
func ShowFeeds(res http.ResponseWriter, req *http.Request, cache models.Cache, db models.DB) {
	// Get all subscribed feeds.
	feeds, err := models.GetSubcribedFeeds(req.Context(), cache, db)
	if err != nil {
		logging.FromContext(req.Context()).
			Error("Could not add item.", slog.Any("error", err))
	}

	feedCards := make([]components.Card, len(feeds))

	// Generate cards for each feed.
	for i, feed := range feeds {
		feedCards[i] = feed.AsCardSummary()
	}

	// Render the list of feed cards.
	if err := htmx.NewResponse().
		RenderTempl(req.Context(), res, content.ShowFeeds(feedCards...)); err != nil {
		logging.LogReq(req, http.StatusInternalServerError).Error("showAllFeeds: cannot render template.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)

		return
	}
}

// showAllItems shows a list of all items from all subscribed feeds as cards.
func ShowItems(res http.ResponseWriter, req *http.Request, cache models.Cache, db models.DB) {
	feedIDs := FeedsFromCtx(req.Context())

	// Get all subscribed feeds.
	feeds, err := models.GetSubcribedFeeds(req.Context(), cache, db, feedIDs...)
	if err != nil {
		logging.FromContext(req.Context()).
			Error("Could not add item.", slog.Any("error", err))
	}

	subs := make([]string, len(feeds))

	// Get a list of the feed IDs.
	for i, feed := range feeds {
		subs[i] = feed.ID
	}
	// Get all feed items for all subscribed feeds.
	items, err := cache.GetFeedItems(req.Context(), subs...)
	if err != nil {
		logging.FromContext(req.Context()).
			Error("Could not show feed items.", slog.Any("error", err))
	}

	itemCards := make([]components.Card, len(items))

	// Create item cards.
	for i, item := range items {
		itemCards[i] = item.AsCardSummary()
	}
	// Render the list of feed cards.
	if err := htmx.NewResponse().
		RenderTempl(req.Context(), res, content.ShowFeedItems(itemCards...)); err != nil {
		logging.LogReq(req, http.StatusInternalServerError).Error("showFeedSummary: cannot render template.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)

		return
	}
}

// GetFeedItemHandler handles /home/feed/item endpoints.
func ShowItem(res http.ResponseWriter, req *http.Request, feedID string, itemID string, cache models.Cache, db models.DB) {
	item, err := models.GetItem(req.Context(), db, cache, feedID, itemID)
	if err != nil {
		logging.FromContext(req.Context()).
			Error("Could not get item.", slog.Any("error", err))
	}

	// Render the list of feed cards.
	if err := htmx.NewResponse().
		RenderTempl(req.Context(), res, content.ShowItem(item)); err != nil {
		logging.LogReq(req, http.StatusInternalServerError).Error("showFeedSummary: cannot render template.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)

		return
	}

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
