/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package cli

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/go-resty/resty/v2"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/providers/ollama"
	"github.com/immanent-tech/foragd/providers/opensearch"
	"github.com/immanent-tech/foragd/providers/opensearch/bulk"
	"github.com/immanent-tech/foragd/service"
)

type services struct {
	items         *service.ItemService
	feeds         *service.FeedService
	users         *service.UserService
	subscriptions *service.SubscriptionService
	imports       *service.ImportService
	httpClient    *resty.Client
}

func loadServices(ctx context.Context) (*services, error) {
	osc, err := opensearch.GetClient()
	if err != nil {
		slogctx.Warn(ctx, "Failed to load opensearch client", slog.Any("error", err))
		// return nil, fmt.Errorf("load opensearch client: %w", err)
	}
	indexer, err := bulk.New()
	if err != nil {
		slogctx.Warn(ctx, "Failed to load bulk indexer", slog.Any("error", err))
	}
	httpClient := service.NewHTTPClient()
	ollamaSvc, err := ollama.NewClient()
	if err != nil {
		return nil, fmt.Errorf("load ollama client: %w", err)
	}
	itemsSvc, err := service.LoadItemsService(ctx, osc, indexer, httpClient)
	if err != nil {
		return nil, fmt.Errorf("load items service: %w", err)
	}
	feedsSvc, err := service.LoadFeedsService(ctx, osc, indexer, httpClient, ollamaSvc, itemsSvc)
	if err != nil {
		return nil, fmt.Errorf("load feeds service: %w", err)
	}
	usersSvc, err := service.LoadUsersService(ctx, osc)
	if err != nil {
		return nil, fmt.Errorf("load users service: %w", err)
	}
	subscriptionsSvc, err := service.LoadSubscriptionsService(ctx, osc, indexer, itemsSvc, feedsSvc, usersSvc)
	if err != nil {
		return nil, fmt.Errorf("load subscriptions service: %w", err)
	}
	importsSvc, err := service.LoadImportsService(ctx, osc, indexer, feedsSvc, subscriptionsSvc, usersSvc)
	if err != nil {
		return nil, fmt.Errorf("load imports service: %w", err)
	}

	return &services{
		items:         itemsSvc,
		feeds:         feedsSvc,
		users:         usersSvc,
		subscriptions: subscriptionsSvc,
		imports:       importsSvc,
		httpClient:    httpClient,
	}, nil
}
