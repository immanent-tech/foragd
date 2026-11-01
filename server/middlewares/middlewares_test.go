/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

//go:generate go tool mockery
package middlewares_test

import (
	"context"
)

type moqSessionMap struct {
	MoqSessionManager

	state map[string]any
}

func newMoqSessionMap(state map[string]any) *moqSessionMap {
	session := &moqSessionMap{}
	if state != nil {
		session.state = state
	} else {
		session.state = make(map[string]any)
	}
	session.PutFunc = func(ctx context.Context, key string, value any) {
		session.state[key] = value
	}
	session.GetFunc = func(ctx context.Context, key string) any {
		return session.state[key]
	}
	return session
}
