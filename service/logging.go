// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/immanent-tech/go-base/config"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic/bulk"
)

type feedStatusLogMsg struct {
	*models.FeedStatus

	Labels map[string]string `json:"labels"`
}

func newFeedStatusMsg(id models.FeedID) *feedStatusLogMsg {
	return &feedStatusLogMsg{
		FeedStatus: &models.FeedStatus{
			Timestamp: time.Now().UTC(),
			FeedID:    id,
		},
		Labels: map[string]string{
			"env":  config.GetEnvironment().String(),
			"type": "feed-status",
		},
	}
}

func (l *feedStatusLogMsg) log(ctx context.Context) error {
	if err := bulk.AddAction(ctx,
		bulk.NewAction(
			l,
			bulk.AsOperation[string](bulk.OpIndex),
			bulk.ToIndex[string]("logs"),
		),
	); err != nil {
		return fmt.Errorf("add bulk action: %w", err)
	}
	return nil
}

func logZyteError(ctx context.Context, err error, feedURL string, details *models.Feed) {
	logMsg := newFeedStatusMsg(details.GetID())
	logMsg.FeedStatus.URL = feedURL
	if apiErr, ok := errors.AsType[*models.APIError](err); ok {
		logMsg.StatusCode = apiErr.StatusCode
		logMsg.StatusMessage = new(apiErr.Error())
	} else {
		logMsg.StatusCode = http.StatusInternalServerError
		logMsg.StatusMessage = new(err.Error())
	}
	if err := logMsg.log(ctx); err != nil {
		slogctx.FromCtx(ctx).Warn("Unable to record feed status.",
			slog.Any("error", err),
		)
	}
}

func logGeneralError(ctx context.Context, err error, feedURL string, details *models.Feed) {
	logMsg := newFeedStatusMsg(details.GetID())
	logMsg.FeedStatus.URL = feedURL
	if apiErr, ok := errors.AsType[*models.APIError](err); ok {
		logMsg.StatusCode = apiErr.StatusCode
		logMsg.StatusMessage = new(apiErr.Error())
	} else {
		logMsg.StatusCode = http.StatusInternalServerError
		logMsg.StatusMessage = new(err.Error())
	}
	if err := logMsg.log(ctx); err != nil {
		slogctx.FromCtx(ctx).Warn("Unable to record feed status.",
			slog.Any("error", err),
		)
	}
}
