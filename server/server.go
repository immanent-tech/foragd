// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	slogctx "github.com/veqryn/slog-context"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/providers/resend"
	"github.com/immanent-tech/foragd/providers/stripe"
	"github.com/immanent-tech/foragd/server/handlers"
	"github.com/immanent-tech/foragd/server/middlewares"
	"github.com/immanent-tech/foragd/server/otel"
	"github.com/immanent-tech/foragd/server/session"
	"github.com/immanent-tech/foragd/web"
)

const (
	gracefulShutdownTimeout = 30 * time.Second
)

// Start will start the server.
//
//nolint:funlen
func Start(logger *slog.Logger) error {
	ctx, cancelFunc := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancelFunc()

	ctx = slogctx.NewCtx(ctx, logger)

	// Load the server config.
	if err := loadConfigOnce(); err != nil {
		return fmt.Errorf("unable to load server config: %w", err)
	}

	var err error

	// Set up the session manager.
	if err := session.NewSessionManager(); err != nil {
		return fmt.Errorf("unable to set up session api: %w", err)
	}

	// Set up OpenTelemetry.
	otelShutdown, err := otel.Setup(ctx)
	if err != nil {
		return fmt.Errorf("unable to set up open telemetry: %w", err)
	}
	// Handle shutdown properly so nothing leaks.
	defer func() {
		err = errors.Join(err, otelShutdown(context.Background()))
	}()

	// pubsub, err := pubsub.New(ctx)
	// if err != nil {
	// 	return fmt.Errorf("unable to configure pubsub: %w", err)
	// }

	// marshaler := cqrs.JSONMarshaler{}
	// topic := marshaler.Name(handlers.UpdatesFound{})
	// updatesHandler := pubsub.AddSSEHandler(topic, &handlers.UpdatesStream{})

	// Set up a new chi router.
	router := chi.NewRouter()

	// Health check endpoints (for GCP).
	router.Use(middleware.Heartbeat("/health-check"))

	// Standard middleware stack.
	router.Use(
		middleware.RequestID,
		middlewares.Logger,
		middleware.Recoverer,
		middlewares.SetupCORS,
		middlewares.CrossOriginProtection,
		middlewares.ContentSecurityPolicy,
		middlewares.GeneralSecurity,
		middlewares.PreventCSRF,
		middlewares.RateLimit,
		middlewares.SetupImgProxy(cfg.ImgProxy.Key, cfg.ImgProxy.Salt),
		middleware.Compress(cfg.CompressionLevel, cfg.CompressionMimetypes...),
		middleware.StripSlashes,
		middlewares.Etag,
		middlewares.Otel,
	)

	// Error handling.
	router.NotFound(handlers.HandleNotFound())
	// Image proxy.
	router.Get("/img-proxy/*", handlers.ImageProxy(cfg.ImgProxy.Prefix))
	// sitemap.xml.
	router.Handle("/sitemap.xml", handlers.HandleSitemap())
	// Static content.
	router.Handle("/robots.txt", handlers.StaticFileHandler(http.FS(web.StaticContentFS)))
	router.Handle("/favicon.ico", handlers.StaticFileHandler(http.FS(web.StaticContentFS)))
	router.Handle("/content/*", handlers.StaticFileHandler(http.FS(web.StaticContentFS)))
	router.Handle("/.well-known/*", handlers.StaticFileHandler(http.FS(web.StaticContentFS)))

	// Avatars
	router.Get("/img/avatar/*", handlers.LoadCachedImage)
	// User custom subscription images
	router.Get("/img/subscription/*", handlers.LoadCachedImage)
	// User uploaded screenshots
	router.Get("/img/screenshots/*", handlers.LoadCachedImage)
	// Handle incoming webhook requests from Stripe.
	router.Post("/checkout/webhooks", stripe.HandleWebhook)
	// Handle incoming webhook requests from Resend
	router.Post("/mail/webhooks", resend.HandleWebhook)

	// External Pages.
	router.Group(func(r chi.Router) {
		r.Use(middlewares.PushCriticalAssets)
		// Landing and features.
		r.Get("/", handlers.HandleLanding())
		r.Get("/features", handlers.HandleFeatures())
		r.With(middlewares.RequireHTMX).Get("/features/collect", handlers.HandleFeaturesCollect())
		r.With(middlewares.RequireHTMX).Get("/features/curate", handlers.HandleFeaturesCurate())
		r.With(middlewares.RequireHTMX).Get("/features/consume", handlers.HandleFeaturesConsume())
		// About.
		r.Get("/about", handlers.HandleAbout())
		// Feed Viewer.
		r.Get("/viewer", handlers.HandleViewer())
		r.Get("/viewer/url/*", handlers.HandleViewer())
		r.With(middlewares.RequireHTMX).Post("/viewer", handlers.HandleViewer())
		// Help documentation.
		r.Get("/help", handlers.DocumentationHandler())
		// Policy documentation (i.e., terms of service, privacy).
		r.Get("/policies/*", handlers.PolicyDocsHandler())
		// Posts index.
		r.Get("/posts", handlers.HandlePosts())
		r.Get("/blog", handlers.HandlePosts())
		// Individual posts.
		r.Get("/posts/*", handlers.HandlePosts())
		r.Get("/blog/*", handlers.HandlePosts())
		// Comparison pages.
		r.Get("/compare/{service}", handlers.HandleComparison())
		// Posts RSS feed.
		r.Get("/feed", handlers.HandlePostsFeed())
		// Sign-up/Login routes.
		r.Group(func(r chi.Router) {
			r.Use(
				session.LoadAndSave,
			)
			if !cfg.BlockSignup {
				r.Get("/signup", handlers.HandleLogin)
			} else {
				slogctx.FromCtx(ctx).Warn("Signups have been BLOCKED by configuration.")
			}
			if !cfg.BlockLogin {
				r.Get("/login", handlers.HandleLogin)
				r.Get("/login/callback", handlers.HandleLoginCallback)
				r.Get("/login/error", handlers.HandleLoginError)
			} else {
				slogctx.FromCtx(ctx).Warn("Logins have been BLOCKED by configuration.")
			}
		})
		// User routes that don't required authentication.
		r.Get("/unsubscribe/{token}", handlers.HandleUserUnsubscribe())
		r.Post("/unsubscribe/{token}", handlers.HandleUserUnsubscribe())
	})

	// Authenticated routes.
	router.Group(func(r chi.Router) {
		r.Use(
			middlewares.SetupHTMX,
			session.LoadAndSave,
			middlewares.RequireUserAuth,
			middlewares.PushCriticalAssets,
		)
		r.Get("/login/refresh", handlers.HandleRefreshToken)
		// Payment routes (Stripe).
		r.Route("/checkout", func(r chi.Router) {
			r.Get("/choose-plan", handlers.HandleChooseSubscriptionPlan())
			r.Post("/", handlers.HandleSubscriptionPlanCheckout())
			r.Get("/success", handlers.HandleAccountSuccess())
			r.Get("/cancel", handlers.HandleLanding())
		})
		r.Get("/home", handlers.HandleHome())
		// r.Get("/home/updates", handlers.WatchHome())
		// Searching.
		r.Route("/search", func(r chi.Router) {
			r.Get("/", handlers.HandleSearchResults())
			r.With(middlewares.RequireHTMX).Post("/suggestions", handlers.HandleSearchSuggestions())
			r.With(middlewares.RequireHTMX).Post("/paginate", handlers.HandleSearchResults())
			r.With(middlewares.RequireHTMX).
				Post("/subscription/suggestions", handlers.GetSubscriptionFilterSuggestions())
			r.With(middlewares.RequireHTMX).Post("/subscription", handlers.AddSubscriptionFilter())
			r.Post("/updates", handlers.HandleSearchUpdates())
		})
		r.Route("/action", func(r chi.Router) {
			r.With(middlewares.RequireHTMX).
				Post("/subscription/suggestions", handlers.GetSubscriptionActionSuggestions())
		})
		// Issues.
		r.With(middlewares.RequireHTMX).Get("/issue/{object}/{id}", handlers.HandleReportObjectIssue())
		r.With(middlewares.RequireHTMX).Post("/issue/{object}/{id}", handlers.HandleSubmitObjectIssue())
		// Subscription specific.
		r.Route("/list/subscriptions", func(r chi.Router) {
			r.Get("/", handlers.HandleListSubscriptions())
			r.With(middlewares.RequireHTMX).Post("/paginate", handlers.HandleListSubscriptions())
			r.With(middlewares.RequireHTMX).Post("/mark/{mark}", handlers.HandleMarkSubscriptions())
			r.Post("/updates", handlers.HandleListSubscriptionsUpdates())
			r.With(middlewares.RequireHTMX).Get("/categories", handlers.ListCategories())
		})
		r.With(middlewares.RequireHTMX).
			Post("/mark/subscription/{subscription_id}", handlers.HandleMarkSubscription())
		r.With(middlewares.RequireHTMX).
			Post("/favorite/subscription/{subscription_id}", handlers.HandleFavoriteSubscription())
		r.With(middlewares.RequireHTMX).
			Post("/remove/subscription/{subscription_id}", handlers.HandleRemoveSubscription())
		r.With(middlewares.RequireHTMX).
			Delete("/remove/subscription/{subscription_id}", handlers.HandleRemoveSubscription())
		r.Route("/subscription", func(r chi.Router) {
			r.Route("/add", func(r chi.Router) {
				r.Get("/", handlers.HandleAddSubscription())
				r.With(middlewares.RequireHTMX).Post("/suggestions", handlers.HandleAddSubscriptionSuggestions())
				r.With(middlewares.RequireHTMX).Post("/feed", handlers.HandleAddNewFeedSubscription())
				// Add search subscription.
				r.Get("/search", handlers.HandleAddSearchSubscription())
				r.With(middlewares.RequireHTMX).Post("/search", handlers.HandleAddSearchSubscription())
				// Add group subscription.
				r.Get("/group", handlers.HandleAddGroupSubscription())
				r.With(middlewares.RequireHTMX).Post("/group", handlers.HandleAddGroupSubscription())
			})
			r.Get("/edit/{subscription_id}", handlers.HandleEditSubscription())
			r.With(middlewares.RequireHTMX).Post("/save/{subscription_id}", handlers.HandleSaveSubscription())
			// Group subscription management.
			r.Route("/group", func(r chi.Router) {
				r.With(middlewares.RequireHTMX).Post("/add", handlers.HandleAddSubscriptionToGroup())
			})
			// Search subscription management.
			r.Route("/search", func(r chi.Router) {
				r.With(middlewares.RequireHTMX).Post("/suggest", handlers.HandleSuggestSubscriptionForSearch())
				r.With(middlewares.RequireHTMX).Post("/add", handlers.HandleAddSubscriptionToSearch())
			})
			// Subscription category management.
			r.Route("/category", func(r chi.Router) {
				r.With(middlewares.RequireHTMX).Post("/", handlers.HandleSubscriptionCategories())
			})
		})

		// Article specific.
		r.Route("/list/articles", func(r chi.Router) {
			r.Get("/", handlers.HandleListArticles())
			r.With(middlewares.RequireHTMX).Post("/paginate", handlers.HandleListArticles())
			r.With(middlewares.RequireHTMX).Post("/mark/{mark}", handlers.MarkArticles())
			r.Post("/updates", handlers.HandleListArticlesUpdates())
			r.With(middlewares.RequireHTMX).Get("/categories", handlers.ListCategories())
		})
		r.With(middlewares.RequireHTMX).Post("/mark/article/{item_id}", handlers.MarkArticle())
		r.With(middlewares.RequireHTMX).Post("/favorite/article/{item_id}", handlers.FavoriteArticle())
		r.With(middlewares.RequireHTMX).Post("/share/article/{item_id}", handlers.ShareArticle())
		r.Get("/view/article/{item_id}", handlers.HandleViewArticle())
		r.Get("/view/article/{item_id}/similar", handlers.HandleFindSimilarArticles())
		// General.
		r.With(middlewares.RequireHTMX).Get("/issue", handlers.HandleReportPageIssue())
		r.With(middlewares.RequireHTMX).Post("/issue", handlers.HandleSubmitPageIssue())
		// Favorite specific.
		r.Route("/list/favorites", func(r chi.Router) {
			r.Get("/", handlers.HandleListFavorites())
		})

		// User routes.
		r.Route("/user", func(r chi.Router) {
			r.Get("/account-issue", handlers.HandleAccountIssue())
			r.Post("/feedset", handlers.HandleAddFeedset(web.StaticContentFS))
			// Import/export.
			r.Get("/import", handlers.HandleImportSubscriptions())
			r.With(middlewares.RequireHTMX).Post("/import", handlers.HandleImportSubscriptions())
			r.Get("/export", handlers.HandleExportSubscriptions())
			r.Post("/export", handlers.HandleExportSubscriptions())
			// Settings.
			r.Route("/settings", func(r chi.Router) {
				r.Get("/", handlers.ShowSettings())
				r.With(middlewares.RequireHTMX).Get("/display", handlers.HandleShowDisplaySettings())
				r.With(middlewares.RequireHTMX).Post("/display", handlers.HandleSaveDisplaySettings())
				r.With(middlewares.RequireHTMX).Get("/account", handlers.HandleShowAccountSettings())
				r.With(middlewares.RequireHTMX).Post("/account", handlers.HandleSaveAccountSettings())
				r.Get("/subscription", handlers.HandleManageAccountSubscription())
				r.With(middlewares.RequireHTMX).Post("/password", handlers.HandleChangePassword())
				r.With(middlewares.RequireHTMX).Post("/subscriptionemail", handlers.HandleGenerateSubscriptionEmail())
			})
			r.Get("/deactivate", handlers.HandleDeactivateAccount())
			r.With(middlewares.RequireHTMX).Post("/deactivate/cancel", handlers.HandleCancelDeactivation())
		})
		r.Get("/logout", handlers.Logout)
	})

	h2s := &http2.Server{}
	svr := &http.Server{
		Handler:           h2c.NewHandler(router, h2s),
		Addr:              net.JoinHostPort(cfg.Host, strconv.FormatUint(cfg.Port, 10)),
		ReadHeaderTimeout: cfg.ReadTimeout.Duration(),
		// ! Setting timeouts will break SSE.
		// ReadTimeout:  cfg.ReadTimeout.Duration(),
		// WriteTimeout: cfg.WriteTimeout.Duration(),
		// IdleTimeout:  cfg.IdleTimeout.Duration(),
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}

	err = http2.ConfigureServer(svr, h2s)
	if err != nil {
		return fmt.Errorf("unable to configure server for H2C: %w", err)
	}

	// go func() {
	// 	err := pubsub.StartEventsRouter(ctx)
	// 	if err != nil {
	// 		slogctx.FromCtx(ctx).Error("Unable to start pubsub events router",
	// 			slog.Any("error", err),
	// 		)
	// 	}
	// }()

	// go func() {
	// 	err := pubsub.StartSSERouter(ctx)
	// 	if err != nil {
	// 		slogctx.FromCtx(ctx).Error("Unable to start pubsub sse router",
	// 			slog.Any("error", err),
	// 		)
	// 	}
	// }()

	logger.Info("Starting server...",
		slog.String("address", svr.Addr),
		slog.String("version", config.Version),
		slog.Time("start_time", time.Now()),
	)

	// And we serve HTTP until the world ends.
	go func() {
		var err error
		if cfg.CertFile != "" && cfg.KeyFile != "" {
			logger.Info("Using https.",
				slog.String("certificate file", cfg.CertFile),
				slog.String("key file", cfg.KeyFile),
			)
			err = svr.ListenAndServeTLS(cfg.CertFile, cfg.KeyFile)
		} else {
			logger.Info("Using http.")
			err = svr.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			logger.Error("Could not listen.",
				slog.Any("error", err),
			)
		}
	}()

	<-ctx.Done()

	// Create shutdown context with 30-second timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
	defer cancel()

	// Trigger graceful shutdown
	if err := svr.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server failed to shutdown gracefully.",
			slog.Any("error", err),
			slog.Time("stop_time", time.Now()),
		)
	}

	logger.Info("Server shutdown gracefully",
		slog.Time("stop_time", time.Now()),
	)

	return nil
}
