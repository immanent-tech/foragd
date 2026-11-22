/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/immanent-tech/go-base/config"
	"github.com/immanent-tech/go-base/server/handlers/assets"
	"github.com/immanent-tech/go-base/server/middlewares/breadcrumbs"
	slogctx "github.com/veqryn/slog-context"
	"golang.org/x/sync/errgroup"

	"github.com/immanent-tech/foragd/providers/auth0"
	"github.com/immanent-tech/foragd/providers/ollama"
	"github.com/immanent-tech/foragd/providers/opensearch"
	"github.com/immanent-tech/foragd/providers/opensearch/bulk"
	"github.com/immanent-tech/foragd/providers/resend"
	"github.com/immanent-tech/foragd/server/cache"
	"github.com/immanent-tech/foragd/server/otel"
	"github.com/immanent-tech/foragd/server/session"
	"github.com/immanent-tech/foragd/service"
	"github.com/immanent-tech/foragd/web"
)

type services struct {
	osc           *opensearch.Client
	indexer       *bulk.Indexer
	imageCache    *cache.ImageCache
	subscriptions *service.SubscriptionService
	feeds         *service.FeedService
	users         *service.UserService
	items         *service.ItemService
	imports       *service.ImportService
	emailVerifier *resend.Verifier
	emailSender   *resend.Sender
	emailReceiver *resend.Receiver
	session       *session.Manager
	breadcrumbs   *breadcrumbs.Manager
	auth          *auth0.Authenticator
	authMgr       *auth0.Manager
	otelShutdown  func(context.Context) error
	httpClient    *resty.Client
}

func loadServices(ctx context.Context, appCfg *config.AppConfig) (*services, error) {
	// Load services that other services depend on or have dependencies.
	osc, err := opensearch.GetClient()
	if err != nil {
		slogctx.Warn(ctx, "Failed to load opensearch client", slog.Any("error", err))
		// return nil, fmt.Errorf("load opensearch client: %w", err)
	}
	indexer, err := bulk.New(osc)
	if err != nil {
		slogctx.Warn(ctx, "Failed to load bulk indexer", slog.Any("error", err))
	}
	sessionMgr, err := session.New(ctx, osc)
	if err != nil {
		return nil, fmt.Errorf("load session manager: %w", err)
	}
	authMgr, err := auth0.LoadAuthenticator(sessionMgr)
	if err != nil {
		return nil, fmt.Errorf("load authenticator: %w", err)
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

	// Load all independent components concurrently. Many of these do network I/O (OIDC discovery, management tokens,
	// Elasticsearch, etc.), so doing them serially is the main startup cost.
	g, _ := errgroup.WithContext(ctx)

	g.Go(func() error {
		// Generate asset maps.
		if err := assets.GenerateAssetMap(web.Files, "files/dist/meta.json"); err != nil {
			return fmt.Errorf("generate asset map: %w", err)
		}
		return nil
	})

	imgCacheF := loadAsync(g, "image cache", cache.NewCache)

	resendVerifierF := loadAsync(g, "resend verifier", resend.NewVerifier)
	emailSenderF := loadAsync(g, "email sender", resend.NewSender)
	// sessionManagerF := loadAsync(g, "session manager", session.Load)
	// Authenticator, used to exchange authentication with the backend service.
	// authF := loadAsync(g, "authenticator")
	// Authentication manager, used for managing user accounts and authentication settings.
	authMgrF := loadAsync(g, "user manager", auth0.LoadManager)

	// OpenTelemetry is optional: a failure is logged and replaced with a no-op shutdown.
	// NOTE: assumes otel.Setup returns func(context.Context) error; adjust if it differs.
	otelShutdownF := loadAsync(g, "opentelemetry", func() (func(context.Context) error, error) {
		shutdownFn, err := otel.Setup(ctx, appCfg)
		if err != nil {
			slogctx.Warn(ctx, "Unable to set up OpenTelemetry.", slog.Any("error", err))
			return func(context.Context) error { return nil }, nil
		}
		return shutdownFn, nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Build base services.
	services := &services{
		osc:           osc,
		indexer:       indexer,
		imageCache:    imgCacheF(),
		items:         itemsSvc,
		subscriptions: subscriptionsSvc,
		feeds:         feedsSvc,
		users:         usersSvc,
		imports:       importsSvc,
		emailVerifier: resendVerifierF(),
		emailSender:   emailSenderF(),
		session:       sessionMgr,
		auth:          authMgr,
		authMgr:       authMgrF(),
		otelShutdown:  otelShutdownF(),
		httpClient:    httpClient,
	}

	// Add services that depend on other services.
	services.breadcrumbs = breadcrumbs.New(services.session)
	services.emailReceiver = resend.NewReceiver(
		services.subscriptions,
		services.users,
		services.items,
		service.NewEmailSubscription,
	)

	return services, nil
}

func (s *services) close(ctx context.Context) error {
	var errs []error

	if err := s.imports.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := s.subscriptions.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := s.users.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := s.feeds.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := s.items.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := s.indexer.Close(ctx); err != nil {
		errs = append(errs, err)
	}
	if err := s.osc.Close(); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

// loadAsync runs fn in the errgroup and returns a getter for the result. The getter must only be called after g.Wait()
// has returned nil. The type is inferred, so no explicit service types are needed at the call site. It also logs how
// long each step took, which shows what dominates startup.
func loadAsync[T any](g *errgroup.Group, name string, fn func() (T, error)) func() T {
	var v T
	g.Go(func() error {
		start := time.Now()
		res, err := fn()
		if err != nil {
			return fmt.Errorf("load %s: %w", name, err)
		}
		v = res
		slog.Debug("Loaded component.", slog.String("component", name), slog.Duration("took", time.Since(start)))
		return nil
	})
	return func() T { return v }
}
