// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package scheduler

import (
	"context"

	"github.com/joshuar/go-feed-me/providers/elastic"
)

const (
	feedManagementAPICtxKey contextKey = "feedManagementAPI"
)

type contextKey string

// FeedManagementAPIToCtx stores the feed management api in the context, for use by feed jobs.
func FeedManagementAPIToCtx(ctx context.Context, api *elastic.API) context.Context {
	return context.WithValue(ctx, feedManagementAPICtxKey, api)
}

// FeedManagementAPIFromCtx retrieves the feed management api in the context, for use by feed jobs.
func FeedManagementAPIFromCtx(ctx context.Context) *elastic.API {
	api, found := ctx.Value(feedManagementAPICtxKey).(*elastic.API)
	if !found {
		return nil
	}

	return api
}
