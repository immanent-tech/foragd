/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package service

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	slogctx "github.com/veqryn/slog-context"
	"github.com/zeebo/xxh3"

	osbulk "github.com/immanent-tech/foragd/providers/opensearch/bulk"
)

// LogIndexer is a service to write logs to the logs index.
type LogIndexer struct {
	indexer *osbulk.Indexer
}

func NewLogIndexer(indexer *osbulk.Indexer) *LogIndexer {
	return &LogIndexer{indexer: indexer}
}

func (l *LogIndexer) Log(ctx context.Context, msg any) error {
	id := "log_" + strconv.FormatUint(xxh3.Hash([]byte(time.Now().String())), 10)

	if err := l.indexer.Add(
		ctx,
		osbulk.NewAction(osbulk.ActionCreate, osbulk.ToIndex("logs"), osbulk.WithID(id), osbulk.WithDoc(msg)),
	); err != nil {
		slogctx.FromCtx(ctx).Warn("Unable to record feed status.",
			slog.Any("error", err),
		)
	}
	return nil
}
