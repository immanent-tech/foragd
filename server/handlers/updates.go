// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/ThreeDotsLabs/watermill/components/cqrs"
	"github.com/ThreeDotsLabs/watermill/message"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/templates"
)

type UpdatesHandler struct {
	eventBus *cqrs.EventBus
}

func (b *UpdatesHandler) Handle(req *http.Request) error {
	// Get user details.
	user := models.UserFromCtx(req.Context())
	if user == nil {
		return fmt.Errorf("get user details: %w", models.ErrCtxValueNotFound)
	}
	// Retrieve current filters.
	filters := models.NewListDisplayFilters()
	// Create a query to find new items.
	query, err := models.BuildItemsQuery(req.Context(), &filters)
	if err != nil {
		return fmt.Errorf("build updates query: %w", err)
	}

	// Set up counters.
	var (
		currentCount int64
		prevCount    int64
	)

	// Get an initial count.
	prevCount, err = models.CountItems(req.Context(), query)
	if err != nil {
		return fmt.Errorf("count items: %w", err)
	}

	// Check for updates and publish to queue.
	go func() {
		for {
			select {
			case <-req.Context().Done():
				return
			default:
				slogctx.FromCtx(req.Context()).Debug("Checking for updates...")
				currentCount, err = models.CountItems(req.Context(), query)
				if err != nil {
					slogctx.FromCtx(req.Context()).Warn("Cannot get updates count.",
						slog.Any("error", err))
					continue
				}
				// Show updates toast if new items found.
				if currentCount > prevCount {
					if err := b.eventBus.Publish(req.Context(), &UpdatesFound{
						UserID: user.GetID(),
						Count:  currentCount,
					}); err != nil {
						slogctx.FromCtx(req.Context()).Error("Unable to publish update.",
							slog.Any("error", err),
						)
					}
				}
				time.Sleep(user.GetUpdatesFrequency())
			}
		}
	}()

	return nil
}

type UpdatesFound struct {
	UserID models.UserID
	Count  int64
}

type UpdatesStream struct{}

func (s *UpdatesStream) InitialStreamResponse(res http.ResponseWriter, req *http.Request) (response any, ok bool) {
	// Get user details.
	user := models.UserFromCtx(req.Context())
	if user == nil {
		res.WriteHeader(http.StatusBadRequest)
		return nil, false
	}

	resp, err := s.getResponse(req.Context(), user.GetID(), nil)
	if err != nil {
		slogctx.FromCtx(req.Context()).Error("Could not get stream response.",
			slog.Any("error", err),
		)
		res.WriteHeader(http.StatusInternalServerError)
		return nil, false
	}

	return resp, true
}

func (s *UpdatesStream) NextStreamResponse(req *http.Request, msg *message.Message) (response interface{}, ok bool) {
	// Get user details.
	user := models.UserFromCtx(req.Context())
	if user == nil {
		slogctx.FromCtx(req.Context()).Error("Get user.",
			slog.Any("error", models.ErrCtxValueNotFound),
		)
		return nil, false
	}

	var event UpdatesFound
	err := json.Unmarshal(msg.Payload, &event)
	if err != nil {
		slogctx.FromCtx(req.Context()).Error("Could not unmarshal message.",
			slog.Any("error", err),
		)
		return "keep-alive", false
	}

	resp, err := s.getResponse(req.Context(), user.GetID(), nil)
	if err != nil {
		slogctx.FromCtx(req.Context()).Error("Could not get stream response.",
			slog.Any("error", err),
		)
		return "keep-alive", false
	}

	return resp, true
}

func (s *UpdatesStream) getResponse(ctx context.Context, userID models.UserID, event *UpdatesFound) (any, error) {
	respBuf, ok := bufPool.Get().(*bytes.Buffer)
	if !ok {
		return nil, fmt.Errorf("could not get buffer")
	}
	respBuf.Reset()
	defer bufPool.Put(respBuf)

	if event != nil && userID == event.UserID {
		template := bufio.NewWriter(respBuf)
		if err := templates.UpdatesToast().Render(ctx, template); err != nil {
			return nil, fmt.Errorf("write template: %w", err)
		}
		if err := template.Flush(); err != nil {
			return nil, fmt.Errorf("write template: %w", err)
		}
		return respBuf.String(), nil
	}

	return "keep-alive", nil

}
