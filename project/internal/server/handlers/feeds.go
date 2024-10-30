// Copyright (C) 2024 Joshua Rich <joshua.rich@gmail.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/lxzan/gws"
	"github.com/yassinebenaid/godump"

	components "github.com/joshuar/go-templ-daisyui"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/server/handlers/renderers"
)

func addItemForm() components.Form {
	return components.NewForm("addItem",
		components.Info("Enter Feed Details."),
		components.FormAttributes(templ.Attributes{
			"hx-post": "/home/add",
		}),
		components.Inputs(
			components.NewInput("Name",
				components.AsFormControl(),
				components.OptionalInput(),
				components.WithInputLabel("Name"),
				components.WithPlaceholder("The Feed"),
				components.WithInputAttributes(templ.Attributes{
					"hx-post": "/home/add/validate",
				}),
			),
			components.NewInput("Link",
				components.AsFormControl(),
				components.WithInputLabel("Link"),
				components.WithPlaceholder("https://my.favourite.site/feed.rss"),
				components.WithInputAttributes(templ.Attributes{
					"hx-post": "/home/add/validate",
				}),
			),
			components.NewInput("Topics",
				components.AsFormControl(),
				components.OptionalInput(),
				components.WithInputLabel("Topics"),
				components.WithPlaceholder("CoolStuff, Memes"),
				components.WithInputAttributes(templ.Attributes{
					"hx-post": "/home/add/validate",
				}),
			),
		),
		components.Buttons(
			components.NewButton("Add", "add",
				components.ButtonAttributes(templ.Attributes{
					"_": "on click take .modal-open from #command-modal wait 200ms",
				}),
			),
		),
	)
}

func AddItem(res http.ResponseWriter, req *http.Request) {
	if err := renderers.CommandModal(req, res, addItemForm().Show()); err != nil {
		logging.FromContext(req.Context()).
			Warn("Unable to command modal.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)
	}
}

func ProcessAddItem(res http.ResponseWriter, req *http.Request, storeAPI dataStore) {
	item, problems, err := decodeForm[*models.SubscriptionRequest](req)
	if err != nil && len(problems) == 0 {
		logging.FromContext(req.Context()).
			Error("Could not decode submitted signup request.", slog.Any("error", err))
		Validate(res, req, UpdateAddItemForm)
		return
	}

	if err := storeAPI.AddSubscription(req.Context(), item); err != nil {
		logging.FromContext(req.Context()).
			Error("Could not add item.", slog.Any("error", err))
	}
}

// UpdateAddItemForm takes the user input, validation results and decorates the
// add item form with the results.
func UpdateAddItemForm(field string, item *models.SubscriptionRequest, problems models.ValidationErrors) components.Input {
	form := addItemForm()

	input, _ := form.Inputs.Get(field)

	switch field {
	case "Name":
		input.Attributes["value"] = item.Name
	case "Link":
		input.Attributes["value"] = item.Link
	case "Topics":
		input.Attributes["value"] = item.Topics
	}

	if issue, found := problems[field]; found {
		input.Error = issue
	}

	return input
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
