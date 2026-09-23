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
	"net"
	"net/http"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-base/client"
	"github.com/immanent-tech/go-base/config"
	"github.com/immanent-tech/go-base/logging"
	"github.com/immanent-tech/go-base/pkg/htmx"

	"github.com/immanent-tech/go-base/server/handlers/assets"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/auth0"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/bulk"
	"github.com/immanent-tech/foragd/providers/google/android"
	"github.com/immanent-tech/foragd/server/cache"
	"github.com/immanent-tech/foragd/server/handlers"
	"github.com/immanent-tech/foragd/server/imgproxy"
	"github.com/immanent-tech/foragd/server/middlewares"
	"github.com/immanent-tech/foragd/server/otel"
	"github.com/immanent-tech/foragd/server/session"
	"github.com/immanent-tech/foragd/service"
	"github.com/immanent-tech/foragd/web"

	"github.com/immanent-tech/go-base/server/middlewares/breadcrumbs"
	"github.com/immanent-tech/go-base/server/middlewares/etag"
	"github.com/immanent-tech/go-base/server/middlewares/security"
)

const (
	gracefulShutdownTimeout = 30 * time.Second
)

// Start will start the server.
//
//nolint:funlen
func Start() error {
	logger := logging.New()

	ctx, cancelFunc := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancelFunc()

	ctx = slogctx.NewCtx(ctx, logger)

	// Load the app config.
	appCfg, err := config.LoadAppConfig()
	if err != nil {
		return fmt.Errorf("load app config: %w", err)
	}

	// Load the image caches.
	imgCache, err := cache.NewCache()
	if err != nil {
		return fmt.Errorf("load image cache: %w", err)
	}

	// Load the articles cache.
	itemsCache, err := cache.NewItemsCache()
	if err != nil {
		return fmt.Errorf("load articles cache: %w", err)
	}

	// Load the http client.
	httpClient, err := client.Load()
	if err != nil {
		return fmt.Errorf("load http client: %w", err)
	}
	httpClient = httpClient.SetHeader(
		"User-Agent",
		appCfg.GetAppName()+"/"+appCfg.GetAppVersion()+" (+https://foragd.app/policies/bot)",
	)

	subscriptionSvc, err := service.LoadSubscriptionService()
	if err != nil {
		return fmt.Errorf("load subscription service: %w", err)
	}

	feedSvc, err := service.LoadFeedService()
	if err != nil {
		return fmt.Errorf("load feed service: %w", err)
	}

	userSvc, err := service.LoadUserService()
	if err != nil {
		return fmt.Errorf("load user service: %w", err)
	}

	itemSvc, err := service.LoadItemService()
	if err != nil {
		return fmt.Errorf("load items service: %w", err)
	}

	// Load the server config.
	if err := loadConfigOnce(); err != nil {
		return fmt.Errorf("unable to load server config: %w", err)
	}

	// Set up assets storage.
	if err := assets.New(web.StaticContentFS, "content"); err != nil {
		return fmt.Errorf("load assets: %w", err)
	}

	// Load the session manager.
	sessionManager, err := session.Load()
	if err != nil {
		return fmt.Errorf("load session manager: %w", err)
	}

	authenticator, err := auth0.LoadAuthenticator()
	if err != nil {
		return fmt.Errorf("load authenticator: %w", err)
	}

	breadcrumbs := breadcrumbs.New(sessionManager)

	handlerMgr := handlers.Manager{
		AppConfig:   appCfg,
		SessionMgr:  sessionManager,
		Breadcrumbs: breadcrumbs,
	}

	// Set up OpenTelemetry.
	otelShutdown, err := otel.Setup(ctx, appCfg)
	if err != nil {
		slogctx.Warn(ctx, "Unable to set up OpenTelemetry.", slog.Any("error", err))
	} else {
		defer func() {
			err = errors.Join(err, otelShutdown(context.Background()))
		}()
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
		middleware.StripSlashes,
		security.SetupCORS,
		security.CrossOriginProtection,
		security.ContentSecurityPolicy,
		security.GeneralSecurity(
			[]string{"camera=()", "microphone=()", "geolocation=self", "usb=()", "bluetooth=()", "web-share=self"},
		),
		security.PreventCSRF(
			security.WithCSRFInsecureBypassPattern("/checkout/webhooks"),
			security.WithCSRFInsecureBypassPattern("/mail/webhooks"),
		),
		middlewares.Customisation,
		// middlewares.RateLimit,
		etag.Etag,
		middlewares.SetClient,
		middlewares.Otel,
		middleware.Compress(cfg.CompressionLevel, cfg.CompressionMimetypes...),
		sessionManager.LoadAndSave,
		htmx.SetupHTMX,
	)

	// Error handling.
	router.NotFound(handlerMgr.HandleNotFound())
	// sitemap.xml.
	router.Handle("/sitemap.xml", handlerMgr.HandleSitemap())
	// Static content.
	router.Handle("/assets/files/*", assets.HandleFiles("/assets/", "/content"))
	router.Handle("/assets/*", assets.HandleAssets("/assets/")) // hashed filenames.
	router.Handle("/content/*", assets.HandleFiles("", ""))
	router.Handle("/.well-known/*", assets.HandleFiles("", "/content"))
	router.Handle("/favicon.ico", assets.HandleFiles("", "/content"))

	// Image proxy.
	router.Get("/img-proxy/*", imgproxy.HandleImage(imgCache, httpClient))
	// Avatars
	router.Get("/img/avatar/*", cache.HandleImage(imgCache))
	// User custom subscription images.
	router.Get("/img/subscription/*", cache.HandleImage(imgCache))
	// User uploaded screenshots.
	router.Get("/img/screenshots/*", cache.HandleImage(imgCache))

	// Handle incoming webhooks from Resend
	router.Post("/mail/webhooks", handlers.HandleResendWebhook(subscriptionSvc, userSvc, itemSvc))
	// Handle incoming webhooks from Paddle.
	router.Post("/webhooks/paddle", handlers.HandlePaddleWebhook(userSvc))
	// Handle incoming Google Play Real Time Developer Notifications.
	router.Post("/webhooks/googleplay", android.HandleRTDN(appCfg, userSvc))

	// External Pages.
	router.Group(func(r chi.Router) {
		// Landing and features.
		r.Get("/", handlerMgr.HandleLanding())
		r.Route("/features", func(r chi.Router) {
			r.Get("/", handlerMgr.HandleFeatures())
			r.Get("/collect", handlerMgr.HandleFeaturesCollect())
			r.Get("/curate", handlerMgr.HandleFeaturesCurate())
			r.Get("/consume", handlerMgr.HandleFeaturesConsume())
		})
		// Comparison pages.
		r.Get("/compare/{service}", handlerMgr.HandleComparison())
		// About.
		r.Get("/about", handlerMgr.HandleAbout())
		// Contact.
		r.Route("/contact", func(r chi.Router) {
			r.Get("/", handlerMgr.HandleContact())
			r.With(htmx.RequireHTMX).Post("/", handlerMgr.HandleSubmitContact())
		})
		r.Route("/forget-me", func(r chi.Router) {
			r.Get("/", handlerMgr.HandleForgetMe())
			r.With(htmx.RequireHTMX).Post("/", handlerMgr.HandleSubmitContact())
		})
		// Feed Viewer.
		r.Route("/viewer", func(r chi.Router) {
			r.Get("/", handlerMgr.HandleViewer(httpClient))
			r.Get("/url/*", handlerMgr.HandleViewer(httpClient))
			r.With(htmx.RequireHTMX).Post("/", handlerMgr.HandleViewer(httpClient))
		})
		// Feed Linter.
		r.Route("/linter", func(r chi.Router) {
			r.Get("/", handlerMgr.HandleLinter(httpClient))
			r.With(htmx.RequireHTMX).Post("/", handlerMgr.HandleLinter(httpClient))
		})
		// Help documentation.
		r.Get("/docs", handlerMgr.DocumentationHandler("/docs"))
		r.Get("/help", handlerMgr.DocumentationHandler("help"))
		// Policy documentation (i.e., terms of service, privacy).
		r.Get("/policies/*", handlerMgr.PolicyDocsHandler())
		// Blog.
		r.Route("/blog", func(r chi.Router) {
			r.Get("/", handlerMgr.HandlePosts())
			r.Get("/*", handlerMgr.HandlePosts())
		})
		// Changelog.
		r.Route("/changelog", func(r chi.Router) {
			r.Get("/", handlerMgr.HandleChangelog())
			r.Get("/feed", handlerMgr.HandleChangelogFeed())
		})
		// Posts RSS feed.
		r.Get("/feed", handlerMgr.HandlePostsFeed())
		// Sign-up/Login routes.
		r.Group(func(r chi.Router) {
			r.Get("/signup", handlerMgr.HandleLogin(authenticator))
			r.Route("/login", func(r chi.Router) {
				r.Get("/", handlerMgr.HandleLogin(authenticator))
				r.Get("/callback", handlerMgr.HandleLoginCallback(userSvc, authenticator))
				r.Get("/error", handlerMgr.HandleLoginError)
			})
			r.Get("/logout", handlerMgr.HandleLogout(authenticator))
			r.Get("/account-issue", handlerMgr.HandleAccountIssue())
		})
		// Web payment routes.
		r.Group(func(r chi.Router) {
			r.Use(
				middlewares.ExtractUserFromSession(userSvc, authenticator, sessionManager, httpClient),
			)
			r.Route("/checkout", func(r chi.Router) {
				r.Get("/", handlerMgr.HandleChooseSubscription())
				r.Post("/", handlerMgr.HandlePurchaseSubscription(userSvc))
				r.Get("/success", handlerMgr.HandlePurchaseSubscriptionSuccess())
			})
		})
		// User routes that don't required authentication.
		r.Route("/unsubscribe/{token}", func(r chi.Router) {
			r.Get("/", handlerMgr.HandleUserUnsubscribe(userSvc))
			r.Post("/", handlerMgr.HandleUserUnsubscribe(userSvc))
		})
	})

	// Authenticated routes.
	router.Group(func(r chi.Router) {
		r.Use(
			breadcrumbs.Recorder,
			middlewares.ExtractUserFromSession(userSvc, authenticator, sessionManager, httpClient),
			middlewares.RequireValidUser,
			handlers.ValidateSubscriptionLimits(userSvc, subscriptionSvc),
			middlewares.NoCache,
		)
		// Manual login refresh.
		r.Get("/login/refresh", handlerMgr.HandleRefreshToken(httpClient, authenticator))
		r.With(handlers.AllSubscriptionsCtx(subscriptionSvc)).
			Get("/home", handlerMgr.HandleHome(&service.Home{}))
		// Searching.
		r.Route("/search", func(r chi.Router) {
			r.Use(middlewares.CanonicalizeSearchParams(sessionManager))
			r.Use(handlers.AllSubscriptionsCtx(subscriptionSvc))
			r.Get("/", handlerMgr.HandleSearchResults(itemSvc))
			r.With(htmx.RequireHTMX).
				Post("/suggestions", handlerMgr.HandleSearchSuggestions(subscriptionSvc, itemSvc))
			r.With(htmx.RequireHTMX).Post("/paginate", handlerMgr.HandleSearchResults(itemSvc))
			r.With(htmx.RequireHTMX).
				Post("/subscription/suggestions", handlerMgr.GetSubscriptionFilterSuggestions(subscriptionSvc))
			r.With(htmx.RequireHTMX).Post("/subscription", handlerMgr.AddSubscriptionFilter())
			r.Post("/updates", handlerMgr.HandleSearchUpdates(itemSvc))
		})
		r.Route("/action", func(r chi.Router) {
			r.With(htmx.RequireHTMX).
				Post("/subscription/suggestions", handlerMgr.GetSubscriptionActionSuggestions(subscriptionSvc))
		})
		r.Route("/discover", func(r chi.Router) {
			r.Use(middlewares.CheckUserLimits)
			r.Get("/", handlerMgr.HandleDiscover())
			r.With(htmx.RequireHTMX).Post("/suggest", handlerMgr.HandleDiscoverSuggestions(feedSvc, httpClient))
		})
		// Subscription specific.
		r.Route("/list/subscriptions", func(r chi.Router) {
			r.Use(middlewares.CanonicalizeListFilters(sessionManager))
			r.Use(handlers.AllSubscriptionsCtx(subscriptionSvc))
			r.Get("/", handlerMgr.HandleListSubscriptions(subscriptionSvc))
			r.With(htmx.RequireHTMX).Get("/categories", handlerMgr.ListCategories(subscriptionSvc, itemSvc))
		})
		r.Route("/subscriptions", func(r chi.Router) {
			r.Use(middlewares.CanonicalizeListFilters(sessionManager))
			r.Use(handlers.AllSubscriptionsCtx(subscriptionSvc))
			// r.Get("/", handlers.HandleListSubscriptions()) // ?sort=&status=&category=&page=&per_page=
			r.Group(func(r chi.Router) {
				r.Use(htmx.RequireHTMX)
				r.Post("/paginate", handlerMgr.HandleListSubscriptions(subscriptionSvc))
				r.Post("/read", handlerMgr.HandleBulkMarkSubscriptions(subscriptionSvc, models.MarkRead))
				r.Post(
					"/unread",
					handlerMgr.HandleBulkMarkSubscriptions(subscriptionSvc, models.MarkRead),
				)
				r.Post("/updates", handlerMgr.HandleListSubscriptionsUpdates(itemSvc))
			})
			r.Route("/{subscriptionID}", func(r chi.Router) {
				r.Use(handlers.SubscriptionCtx(subscriptionSvc))
				// 	// r.Get("/", handleSubscriptionDetail) // its own article feed: ?sort=&status=&page=
				r.Group(func(r chi.Router) {
					r.Use(htmx.RequireHTMX)
					r.Post(
						"/read",
						handlerMgr.HandleMarkSubscription(
							subscriptionSvc,
							models.MarkRead,
						),
					)
					r.Post(
						"/unread",
						handlerMgr.HandleMarkSubscription(
							subscriptionSvc,
							models.MarkUnread,
						),
					)
					r.Post("/favorite", handlerMgr.HandleFavoriteSubscription(subscriptionSvc))
					r.Route("/edit", func(r chi.Router) {
						r.Get("/", handlerMgr.HandleEditSubscription(subscriptionSvc))
						r.Post("/", handlerMgr.HandleSaveSubscription(imgCache, subscriptionSvc))
					})
					r.Get("/remove", handlerMgr.HandleRemoveSubscription(subscriptionSvc))
				})
			})
		})
		r.Route("/subscription", func(r chi.Router) {
			r.Use(handlers.AllSubscriptionsCtx(subscriptionSvc))
			r.Route("/add", func(r chi.Router) {
				r.Get("/", handlerMgr.HandleAddSubscription())
				r.With(htmx.RequireHTMX).Post("/suggestions", handlerMgr.HandleSuggestFeeds(feedSvc, httpClient))
				r.With(htmx.RequireHTMX).
					Post("/feed", handlerMgr.HandleAddNewFeedSubscription(subscriptionSvc, userSvc, feedSvc, httpClient))
				// Add search subscription.
				r.Get("/search", handlerMgr.HandleAddSearchSubscription(subscriptionSvc, userSvc))
				r.With(htmx.RequireHTMX).
					Post("/search", handlerMgr.HandleAddSearchSubscription(subscriptionSvc, userSvc))
				// Add group subscription.
				r.Get("/group", handlerMgr.HandleAddGroupSubscription(subscriptionSvc, userSvc))
				r.With(htmx.RequireHTMX).
					Post("/group", handlerMgr.HandleAddGroupSubscription(subscriptionSvc, userSvc))
			})
			// Group subscription management.
			r.Route("/group", func(r chi.Router) {
				r.With(htmx.RequireHTMX).Post("/add", handlerMgr.HandleAddSubscriptionToGroup())
			})
			// Search subscription management.
			r.Route("/search", func(r chi.Router) {
				r.With(htmx.RequireHTMX).
					Post("/suggest", handlerMgr.HandleSuggestSubscriptionForSearch(subscriptionSvc))
				r.With(htmx.RequireHTMX).Post("/add", handlerMgr.HandleAddSubscriptionToSearch())
			})
			// Subscription category management.
			r.Route("/category", func(r chi.Router) {
				r.With(htmx.RequireHTMX).Post("/", handlerMgr.HandleSubscriptionCategories())
			})
		})

		// Article specific.
		r.Route("/list/articles", func(r chi.Router) {
			r.Use(middlewares.CanonicalizeListFilters(sessionManager))
			r.Use(handlers.AllSubscriptionsCtx(subscriptionSvc))
			r.Get("/", handlerMgr.HandleListArticles(itemSvc))
			r.Post("/updates", handlerMgr.HandleListArticlesUpdates(itemSvc))
			r.With(htmx.RequireHTMX).Get("/categories", handlerMgr.ListCategories(subscriptionSvc, itemSvc))
		})
		r.Route("/articles", func(r chi.Router) {
			r.Use(middlewares.CanonicalizeListFilters(sessionManager))
			r.Use(handlers.AllSubscriptionsCtx(subscriptionSvc))
			r.Group(func(r chi.Router) {
				r.Use(htmx.RequireHTMX)
				r.Post("/paginate", handlerMgr.HandleListArticles(itemSvc))
				r.Post("/read", handlerMgr.HandleBulkMarkArticles(subscriptionSvc, models.MarkRead))
				r.Post("/unread", handlerMgr.HandleBulkMarkArticles(subscriptionSvc, models.MarkRead))
			})
			r.Route("/{articleID}", func(r chi.Router) {
				r.Use(handlers.ArticleCtx(itemSvc))
				r.Get(
					"/",
					handlerMgr.HandleViewArticle(itemSvc, httpClient, itemsCache),
				)
				r.Get("/similar", handlerMgr.HandleFindSimilarArticles(itemSvc))
				r.Group(func(r chi.Router) {
					r.Use(htmx.RequireHTMX)
					r.Get("/next", handlerMgr.HandleBrowseArticles(subscriptionSvc, itemSvc, "next"))
					r.Get("/prev", handlerMgr.HandleBrowseArticles(subscriptionSvc, itemSvc, "prev"))
					r.Post("/read", handlerMgr.HandleMarkArticle(subscriptionSvc, models.MarkRead))
					r.Post("/unread", handlerMgr.HandleMarkArticle(subscriptionSvc, models.MarkUnread))
					r.Post("/favorite", handlerMgr.HandleFavoriteArticle(itemSvc, userSvc))
					r.Post("/share", handlerMgr.HandleShareArticle())
				})
			})
		})
		// Favorites.
		r.Route("/favorites", func(r chi.Router) {
			r.Use(handlers.AllSubscriptionsCtx(subscriptionSvc))
			r.Get("/", handlerMgr.HandleListFavorites(subscriptionSvc, itemSvc))
		})
		// Map
		r.Route("/map", func(r chi.Router) {
			r.Use(middlewares.CanonicalizeListFilters(sessionManager))
			r.Use(handlers.AllSubscriptionsCtx(subscriptionSvc))
			r.Get("/", handlerMgr.HandleMap(itemSvc))
			r.With(htmx.RequireHTMX).Post("/updates", handlerMgr.HandleMapUpdates(itemSvc))
		})
		// Issues.
		r.Route("/issue", func(r chi.Router) {
			r.Get("/", handlerMgr.HandleReportIssue())
			r.With(htmx.RequireHTMX).Post("/", handlerMgr.HandleSubmitIssue(imgCache))
		})
		// Help/Documentation.
		r.Get("/docs", handlerMgr.DocumentationHandler("/docs"))

		r.Route("/settings", func(r chi.Router) {
			r.Route("/display", func(r chi.Router) {
				r.Post("/font", handlerMgr.HandleSaveFontSettings(userSvc))
			})
		})

		// User routes.
		r.Route("/user", func(r chi.Router) {
			r.Post(
				"/feedset",
				handlerMgr.HandleAddFeedset(feedSvc, userSvc, subscriptionSvc, httpClient, web.StaticContentFS),
			)
			// Import/export.
			r.Group(func(r chi.Router) {
				r.Use(handlers.AllSubscriptionsCtx(subscriptionSvc))
				r.Get("/import", handlerMgr.HandleImportSubscriptions(feedSvc, userSvc, subscriptionSvc, httpClient))
				r.With(htmx.RequireHTMX).
					Post("/import", handlerMgr.HandleImportSubscriptions(feedSvc, userSvc, subscriptionSvc, httpClient))
				r.Get("/export", handlerMgr.HandleExportSubscriptions(feedSvc))
				r.Post("/export", handlerMgr.HandleExportSubscriptions(feedSvc))
			})
			// Settings.
			r.Route("/settings", func(r chi.Router) {
				r.Get("/", handlerMgr.ShowSettings())
				r.With(htmx.RequireHTMX).Get("/display", handlerMgr.HandleShowDisplaySettings())
				r.With(htmx.RequireHTMX).Post("/display", handlerMgr.HandleSaveDisplaySettings(userSvc))
				r.With(htmx.RequireHTMX).Get("/account", handlerMgr.HandleShowAccountSettings())
				r.With(htmx.RequireHTMX).
					Post("/account", handlerMgr.HandleSaveAccountSettings(userSvc, imgCache))
				r.Group(func(r chi.Router) {
					r.Use(htmx.RequireHTMX)
					r.Use(handlers.AllSubscriptionsCtx(subscriptionSvc))
					r.Get("/subscriptions", handlerMgr.HandleShowSubscriptionsSettings())
					r.Post("/subscriptions", handlerMgr.HandleSaveSubscriptionsSettings(userSvc))
				})
				r.Get("/subscription", handlerMgr.HandleManageAccountSubscription())
				r.With(htmx.RequireHTMX).Post("/password", handlerMgr.HandleChangePassword())
				r.With(htmx.RequireHTMX).Post("/subscriptionemail", handlerMgr.HandleGenerateSubscriptionEmail(userSvc))
				r.With(htmx.RequireHTMX).Post("/fonts", handlerMgr.HandleSaveFontSettings(userSvc))
				r.With(htmx.RequireHTMX).Post("/theme", handlerMgr.HandleSaveThemeSettings(userSvc))
			})
			r.With(htmx.RequireHTMX).
				Post("/deactivate", handlerMgr.HandleDeactivateAccount(userSvc, authenticator))
		})

		// Moved routes.
		r.Get("/list/favorites", handlers.RedirectTo("/favorites", http.StatusMovedPermanently))
		r.Get("/posts", handlers.RedirectTo("/blog", http.StatusMovedPermanently))
		r.Get("/posts/*", handlers.RedirectParam("*", "blog/%s", http.StatusMovedPermanently))
		r.Get("/view/article/{item_id}", handlers.RedirectParam("item_id", "/articles/%s", http.StatusMovedPermanently))
	})

	svr := &http.Server{
		Protocols:         new(http.Protocols),
		Handler:           router,
		Addr:              net.JoinHostPort(cfg.Host, strconv.FormatUint(cfg.Port, 10)),
		ReadHeaderTimeout: cfg.ReadTimeout.Duration,
		ReadTimeout:       cfg.ReadTimeout.Duration,
		WriteTimeout:      cfg.WriteTimeout.Duration,
		IdleTimeout:       cfg.IdleTimeout.Duration,
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}
	svr.Protocols.SetUnencryptedHTTP2(true) // Enable H2C (HTTP/2 cleartext)
	svr.Protocols.SetHTTP1(true)            // Enable HTTP/1.1
	svr.Protocols.SetHTTP2(false)           // Explicitly disable encrypted HTTP/2 (HTTPS)

	// And we serve HTTP until the world ends.
	go func() {
		var err error
		if cfg.CertFile != "" && cfg.KeyFile != "" {
			logger.Debug("Using https.",
				slog.String("certificate file", cfg.CertFile),
				slog.String("key file", cfg.KeyFile),
			)
			err = svr.ListenAndServeTLS(cfg.CertFile, cfg.KeyFile)
		} else {
			logger.Debug("Using http.")
			err = svr.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			logger.Error("Could not listen.",
				slog.Any("error", err),
			)
		}
	}()

	logger.Info("Server started...",
		slog.String("address", svr.Addr),
		slog.String("version", appCfg.GetAppVersion()),
		slog.Time("start_time", time.Now().UTC()),
	)

	<-ctx.Done()

	// Create shutdown context with 30-second timeout
	shutdownCtx, cancel := context.WithTimeoutCause(
		context.Background(),
		gracefulShutdownTimeout,
		errors.New("graceful shutdown timeout"),
	)
	shutdownCtx = slogctx.NewCtx(shutdownCtx, logger)
	defer cancel()

	// Shutdown bulk indexer (if initialized).
	if err := bulk.Shutdown(shutdownCtx); err != nil {
		slogctx.FromCtx(shutdownCtx).Error("Bulk indexer failed to shutdown gracefully.",
			slog.Any("error", err),
		)
	}
	// Shutdown any Elasticsearch connection (if initialized).
	if err := elastic.Shutdown(shutdownCtx); err != nil {
		slogctx.FromCtx(shutdownCtx).Error("Elasticsearch failed to shutdown gracefully.",
			slog.Any("error", err),
		)
	}
	// Trigger graceful shutdown
	if err := svr.Shutdown(shutdownCtx); err != nil {
		slogctx.FromCtx(shutdownCtx).Error("Server failed to shutdown gracefully.",
			slog.Any("error", err),
		)
	}

	slogctx.FromCtx(shutdownCtx).Info("Server stopped",
		slog.Time("stop_time", time.Now().UTC()),
	)

	return nil
}
