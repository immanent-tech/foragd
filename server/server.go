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

	"github.com/immanent-tech/foragd/config"
	gcp "github.com/immanent-tech/foragd/providers/google"
	"github.com/immanent-tech/foragd/providers/google/android"
	"github.com/immanent-tech/foragd/server/assets"
	"github.com/immanent-tech/foragd/server/cache"
	"github.com/immanent-tech/foragd/server/handlers"
	"github.com/immanent-tech/foragd/server/imgproxy"
	"github.com/immanent-tech/foragd/server/middlewares"
	"github.com/immanent-tech/foragd/server/middlewares/security"
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

	if config.IsProduction() {
		// Start the error client.
		if err := gcp.InitErrorClient(ctx); err != nil {
			return fmt.Errorf("init error client: %w", err)
		}
	}

	// Set up the session manager.
	if err := session.NewSessionManager(); err != nil {
		return fmt.Errorf("unable to set up session api: %w", err)
	}

	if err := assets.New(web.StaticContentFS, "content"); err != nil {
		return fmt.Errorf("load assets: %w", err)
	}

	// Set up OpenTelemetry if specified in the config.
	if cfg.EnableOTEL {
		otelShutdown, err := otel.Setup(ctx)
		if err != nil {
			return fmt.Errorf("unable to set up open telemetry: %w", err)
		}
		// Handle shutdown properly so nothing leaks.
		defer func() {
			err = errors.Join(err, otelShutdown(context.Background()))
		}()
		slogctx.FromCtx(ctx).Debug("Open Telemetry instrumentation is enabled.")
	} else {
		slogctx.FromCtx(ctx).Debug("Open Telemetry instrumentation is disabled.")
	}

	// Set up a new chi router.
	router := chi.NewRouter()

	// Health check endpoints (for GCP).
	router.Use(middleware.Heartbeat("/health-check"))

	// Standard middleware stack.
	router.Use(
		middleware.RequestID,
		middlewares.Logger,
		middlewares.Recoverer,
		security.SetupCORS,
		security.CrossOriginProtection,
		security.ContentSecurityPolicy,
		security.GeneralSecurity,
		security.PreventCSRF,
		// middlewares.RateLimit,
		middleware.Compress(cfg.CompressionLevel, cfg.CompressionMimetypes...),
		middleware.StripSlashes,
		middlewares.Etag,
		middlewares.SetClient,
		middlewares.Otel,
	)

	// Error handling.
	router.NotFound(handlers.HandleNotFound())
	// sitemap.xml.
	router.Handle("/sitemap.xml", handlers.HandleSitemap())
	// Static content.
	router.Handle("/robots.txt", assets.HandleAssets("", true))
	router.Handle("/favicon.ico", assets.HandleAssets("", true))
	router.Handle("/favicon.svg", assets.HandleAssets("", true))
	router.Handle("/.well-known/*", assets.HandleAssets("", true))
	router.Handle("/security.txt", http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		http.Redirect(res, req, "/.well-known/security.txt", http.StatusMovedPermanently)
	}))
	router.Handle("/assets/*", assets.HandleAssets("/assets/", false)) // hashed filenames.
	router.Handle("/fonts/*", assets.HandleAssets("", true))
	router.Handle("/content/*", assets.HandleAssets("/content/", true))

	// Image proxy.
	router.Get("/img-proxy/*", imgproxy.HandleImage())
	// Avatars
	router.Get("/img/avatar/*", cache.HandleImage)
	// User custom subscription images.
	router.Get("/img/subscription/*", cache.HandleImage)
	// User uploaded screenshots.
	router.Get("/img/screenshots/*", cache.HandleImage)

	// Handle incoming webhooks from Resend
	router.Post("/mail/webhooks", handlers.HandleResendWebhook)
	// Handle incoming webhooks from Paddle.
	router.Post("/webhooks/paddle", handlers.HandlePaddleWebhook)
	// Handle incoming Google Play Real Time Developer Notifications.
	router.Post("/webhooks/googleplay", android.HandleRTDN)

	// External Pages.
	router.Group(func(r chi.Router) {
		r.Use(middlewares.PushCriticalAssets)
		// Landing and features.
		r.Get("/", handlers.HandleLanding())
		r.Get("/features", handlers.HandleFeatures())
		r.Get("/features/collect", handlers.HandleFeaturesCollect())
		r.Get("/features/curate", handlers.HandleFeaturesCurate())
		r.Get("/features/consume", handlers.HandleFeaturesConsume())
		// About.
		r.Get("/about", handlers.HandleAbout())
		// Contact.
		r.Get("/contact", handlers.HandleContact())
		r.With(middlewares.RequireHTMX).Post("/contact", handlers.HandleSubmitContact())
		r.Get("/forget-me", handlers.HandleForgetMe())
		r.With(middlewares.RequireHTMX).Post("/forget-me", handlers.HandleSubmitContact())
		// Feed Viewer.
		r.Get("/viewer", handlers.HandleViewer())
		r.Get("/viewer/url/*", handlers.HandleViewer())
		r.With(middlewares.RequireHTMX).Post("/viewer", handlers.HandleViewer())
		// Help documentation.
		r.Get("/docs", handlers.DocumentationHandler())
		// Policy documentation (i.e., terms of service, privacy).
		r.Get("/policies/*", handlers.PolicyDocsHandler())
		// Posts index.
		r.Get("/blog", handlers.HandlePosts())
		r.Get("/posts", http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			http.Redirect(res, req, "/blog", http.StatusMovedPermanently)
		}))
		// Individual posts.
		r.Get("/blog/*", handlers.HandlePosts())
		r.Get("/posts/*", func(w http.ResponseWriter, r *http.Request) {
			wildcardPath := chi.URLParam(r, "*")
			http.Redirect(w, r, "/blog/"+wildcardPath, http.StatusMovedPermanently)
		})
		// Comparison pages.
		r.Get("/compare/{service}", handlers.HandleComparison())
		// Changelog.
		r.Get("/changelog", handlers.HandleChangelog())
		r.Get("/changelog/feed", handlers.HandleChangelogFeed())
		// Posts RSS feed.
		r.Get("/feed", handlers.HandlePostsFeed())
		// Help.
		r.Get("/help", handlers.DocumentationHandler())
		// Sign-up/Login routes.
		r.Group(func(r chi.Router) {
			r.Use(
				session.LoadAndSave,
				middlewares.SetupHTMX,
			)
			r.Get("/signup", handlers.HandleLogin)
			r.Get("/login", handlers.HandleLogin)
			r.Get("/login/callback", handlers.HandleLoginCallback)
			r.Get("/login/error", handlers.HandleLoginError)
			r.Get("/logout", handlers.Logout)
			r.Get("/account-issue", handlers.HandleAccountIssue())
		})
		// Web payment routes.
		r.Group(func(r chi.Router) {
			r.Use(
				session.LoadAndSave,
				middlewares.SetupHTMX,
				middlewares.ExtractUserFromSession,
			)
			r.Route("/checkout", func(r chi.Router) {
				r.Get("/", handlers.HandleChooseSubscription())
				r.With(middlewares.RequireHTMX).Post("/", handlers.HandlePurchaseSubscription())
				r.Get("/success", handlers.HandlePurchaseSubscriptionSuccess())
			})
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
			middlewares.ExtractUserFromSession,
			middlewares.RequireValidUser,
			middlewares.PushCriticalAssets,
			handlers.ValidateSubscriptionLimits,
		)
		// Manual login refresh.
		r.Get("/login/refresh", handlers.HandleRefreshToken)
		r.Get(handlers.RouteHome, handlers.HandleHome())
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
				r.With(middlewares.RequireHTMX).Post("/suggestions", handlers.HandleSuggestFeeds())
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
		r.Get("/issue", handlers.HandleReportIssue())
		r.With(middlewares.RequireHTMX).Post("/issue", handlers.HandleSubmitIssue())
		r.Get("/docs", handlers.DocumentationHandler())
		// Favorite specific.
		r.Route("/list/favorites", func(r chi.Router) {
			r.Get("/", handlers.HandleListFavorites())
		})

		// User routes.
		r.Route("/user", func(r chi.Router) {
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
				r.With(middlewares.RequireHTMX).Get("/subscriptions", handlers.HandleShowSubscriptionsSettings())
				r.With(middlewares.RequireHTMX).Get("/subscriptions", handlers.HandleShowSubscriptionsSettings())
				r.Get("/subscription", handlers.HandleManageAccountSubscription())
				r.With(middlewares.RequireHTMX).Post("/password", handlers.HandleChangePassword())
				r.With(middlewares.RequireHTMX).Post("/subscriptionemail", handlers.HandleGenerateSubscriptionEmail())
			})
			r.With(middlewares.RequireHTMX).Get("/deactivate", handlers.HandleDeactivateAccount())
		})
	})

	svr := &http.Server{
		Protocols:         new(http.Protocols),
		Handler:           router,
		Addr:              net.JoinHostPort(cfg.Host, strconv.FormatUint(cfg.Port, 10)),
		ReadHeaderTimeout: cfg.ReadTimeout.Duration(),
		ReadTimeout:       cfg.ReadTimeout.Duration(),
		WriteTimeout:      cfg.WriteTimeout.Duration(),
		IdleTimeout:       cfg.IdleTimeout.Duration(),
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}
	svr.Protocols.SetUnencryptedHTTP2(true) // Enable H2C (HTTP/2 cleartext)
	svr.Protocols.SetHTTP1(true)            // Enable HTTP/1.1
	svr.Protocols.SetHTTP2(false)           // Explicitly disable encrypted HTTP/2 (HTTPS)

	logger.Info("Starting server...",
		slog.String("address", svr.Addr),
		slog.String("version", config.GetVersion()),
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
