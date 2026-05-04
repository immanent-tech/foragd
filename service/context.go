// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package service

import (
	"context"

	"github.com/immanent-tech/foragd/models"
)

const (
	listFiltersCtxKey contextKey = "listFilters"
)

type contextKey string

// ListFiltersToCtx stores list filters in the context.
func ListFiltersToCtx(ctx context.Context, filters models.ListFilters) context.Context {
	return context.WithValue(ctx, listFiltersCtxKey, filters)
}

// ListFiltersFromCtx retrieves list filters from the context. If none are found, new list filters are returned.
func ListFiltersFromCtx(ctx context.Context) models.ListFilters {
	filters, found := ctx.Value(listFiltersCtxKey).(models.ListFilters)
	if !found {
		return models.NewListDisplayFilters()
	}
	return filters
}
