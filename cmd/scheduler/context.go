// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package scheduler

import (
	"context"
)

const (
	feedManagementAPICtxKey contextKey = "feedManagementAPI"
)

type contextKey string

// FeedManagementAPIToCtx stores the feed management api in the context, for use by feed jobs.
func FeedManagementAPIToCtx(ctx context.Context, api DataAPI) context.Context {
	return context.WithValue(ctx, feedManagementAPICtxKey, api)
}

// FeedManagementAPIFromCtx retrieves the feed management api in the context, for use by feed jobs.
func FeedManagementAPIFromCtx(ctx context.Context) DataAPI {
	api, found := ctx.Value(feedManagementAPICtxKey).(DataAPI)
	if !found {
		return nil
	}

	return api
}
