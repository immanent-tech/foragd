// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"

	"github.com/immanent-tech/foragd/server/session"
)

const (
	listSubscriptionFiltersKey = "listSubscriptionFilters"
	listArticleFiltersKey      = "listArticleFilters"
)

func GetListSubscriptionFiltersFromSession(ctx context.Context) *ListFilters {
	restored, err := session.Restore[ListFilters](ctx, listSubscriptionFiltersKey)
	if err != nil {
		// Use new filters if unable to restore from session or form data.
		restored = NewListDisplayFilters()
	}

	return &restored
}

func StoreListSubscriptionFiltersInSession(ctx context.Context, filters ListFilters) error {
	return session.Save(ctx, listSubscriptionFiltersKey, filters)
}

func GetListArticleFiltersFromSession(ctx context.Context) *ListFilters {
	restored, err := session.Restore[ListFilters](ctx, listArticleFiltersKey)
	if err != nil {
		// Use new filters if unable to restore from session or form data.
		restored = NewListDisplayFilters()
	}

	return &restored
}

func StoreListArticleFiltersInSession(ctx context.Context, filters ListFilters) error {
	return session.Save(ctx, listArticleFiltersKey, filters)
}
