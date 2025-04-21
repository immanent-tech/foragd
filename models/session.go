// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"

	"github.com/go-chi/chi/v5"
)

const (
	feedFiltersSessionKey = "feed_filters"
	itemFiltersSessionKey = "item_filters"
)

type SessionAPI interface {
	Put(ctx context.Context, key string, value any)
	Get(ctx context.Context, key string) any
}

type Session struct {
	api SessionAPI
}

func NewSession(api SessionAPI) *Session {
	return &Session{api: api}
}

func (s *Session) GetFilters(ctx context.Context) Filters {
	route := chi.RouteContext(ctx).RoutePattern()
	switch route {
	case FeedsRoute:
		return s.GetFeedFilters(ctx)
	case ItemsRoute:
		return s.GetItemFilters(ctx)
	default:
		return *NewFilters()
	}
}

func (s *Session) GetFeedFilters(ctx context.Context) Filters {
	if filters, found := s.api.Get(ctx, feedFiltersSessionKey).(Filters); found {
		return filters
	}
	return *NewFilters()
}

func (s *Session) GetItemFilters(ctx context.Context) Filters {
	if filters, found := s.api.Get(ctx, itemFiltersSessionKey).(Filters); found {
		return filters
	}
	return *NewFilters()
}
