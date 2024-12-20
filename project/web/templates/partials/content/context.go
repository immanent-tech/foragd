// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package content

import (
	"context"
	"errors"
)

type contextKey string

const (
	backLinkCtxKey contextKey = "backlink"
	triggerCtxKey  contextKey = "trigger"
)

var ErrNotInCtx = errors.New("not found in context")

func BackLinkToCtx(ctx context.Context, link string) context.Context {
	newCtx := context.WithValue(ctx, backLinkCtxKey, link)

	return newCtx
}

func BackLinkFromCtx(ctx context.Context) string {
	link, ok := ctx.Value(backLinkCtxKey).(string)
	if !ok {
		return ""
	}

	return link
}

func TriggerToCtx(ctx context.Context, trigger string) context.Context {
	newCtx := context.WithValue(ctx, triggerCtxKey, trigger)

	return newCtx
}

func TriggerFromCtx(ctx context.Context) string {
	trigger, ok := ctx.Value(triggerCtxKey).(string)
	if !ok {
		return ""
	}

	return trigger
}
