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

func FeedManagementAPIToCtx(ctx context.Context, api DataAPI) context.Context {
	return context.WithValue(ctx, feedManagementAPICtxKey, api)
}

func FeedManagementAPIFromCtx(ctx context.Context) DataAPI {
	api, found := ctx.Value(feedManagementAPICtxKey).(DataAPI)
	if !found {
		return nil
	}

	return api
}
