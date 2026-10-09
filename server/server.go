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
	"sync/atomic"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	slogctx "github.com/veqryn/slog-context"
	"golang.org/x/sync/errgroup"

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
	gerror "github.com/immanent-tech/foragd/providers/google/error"
	"github.com/immanent-tech/foragd/providers/resend"
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
	// readinessDrainDelay is the time to keep serving (while reporting not-ready) so the load balancer can stop routing
	// to us. Set to 0 for local development. Consider making this configurable.
	readinessDrainDelay = 5 * time.Second
	// gracefulShutdownTimeout is the time allowed for in-flight requests to finish and dependencies to flush.
	gracefulShutdownTimeout = 30 * time.Second
	// otelShutdownTimeout is the time allowed for OpenTelemetry to flush its final batch.
	otelShutdownTimeout = 5 * time.Second
)

var (
	// ready reports whether the server should receive new traffic. Flipped to false on shutdown.
	ready atomic.Bool
	// shuttingDown is closed when the server begins shutting down. Long-lived handlers (SSE, long-poll) should select
	// on ShuttingDown() so they don't stall svr.Shutdown.
	shuttingDown = make(chan struct{})
)

// ShuttingDown returns a channel that is closed when the server starts shutting down.
func ShuttingDown() <-chan struct{} { return shuttingDown }

// readiness is a middleware that answers /readyz before any other middleware runs.
func readiness(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/readyz" {
			if !ready.Load() {
				http.Error(w, "shutting down", http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
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

// Start will start the server.
//
//nolint:funlen
func Start() error {
	logger := logging.New()
	startTime := time.Now()

	ctx, cancelFunc := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancelFunc()

	ctx = slogctx.NewCtx(ctx, logger)

	//  Load the app config and server config first: other components may depend on them.
	appCfg, err := config.LoadAppConfig()
	if err != nil {
		return fmt.Errorf("load app config: %w", err)
	}
	if err := loadConfigOnce(); err != nil {
		return fmt.Errorf("unable to load server config: %w", err)
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
	subscriptionSvcF := loadAsync(g, "subscription service", service.LoadSubscriptionService)
	feedSvcF := loadAsync(g, "feed service", service.LoadFeedService)
	userSvcF := loadAsync(g, "user service", service.LoadUserService)
	itemSvcF := loadAsync(g, "items service", service.LoadItemService)
	importSvcF := loadAsync(g, "import service", service.NewImportService)
	resendVerifierF := loadAsync(g, "resend verifier", resend.NewVerifier)
	emailSenderF := loadAsync(g, "email sender", resend.NewSender)
	sessionManagerF := loadAsync(g, "session manager", session.Load)
	// Authenticator, used to exchange authentication with the backend service.
	authF := loadAsync(g, "authenticator", auth0.LoadAuthenticator)
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
		return err
	}

	imgCache := imgCacheF()
	subscriptionSvc := subscriptionSvcF()
	feedSvc := feedSvcF()
	userSvc := userSvcF()
	itemSvc := itemSvcF()
	importSvc := importSvcF()
	resendVerifier := resendVerifierF()
	emailSender := emailSenderF()
	sessionManager := sessionManagerF()
	auth := authF()
	authMgr := authMgrF()
	otelShutdown := otelShutdownF()

	// Flush OpenTelemetry last, after everything else has stopped producing telemetry.
	// Runs on every exit path, and the error is propagated via the named return.
	defer func() {
		c, cancel := context.WithTimeout(context.WithoutCancel(ctx), otelShutdownTimeout)
		defer cancel()
		err = errors.Join(err, otelShutdown(c))
	}()

	// Load the http client.
	httpClient := client.New().SetHeader(
		"User-Agent",
		appCfg.GetAppName()+"/"+appCfg.GetAppVersion()+" (+https://foragd.app/policies/bot)",
	)

	resendProcesser := resend.NewReceiver(
		subscriptionSvc,
		userSvc,
		itemSvc,
		service.NewEmailSubscription,
	)

	breadcrumbs := breadcrumbs.New(sessionManager)

	handlerMgr := handlers.Manager{
		AppConfig:   appCfg,
		SessionMgr:  sessionManager,
		Breadcrumbs: breadcrumbs,
	}

	// Set up a new chi router.
	router := chi.NewRouter()

	// Health check endpoints (for GCP).
	router.Use(readiness)
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
	router.Handle("/robots.txt", assets.ServeFiles(web.Files, "files"))
	router.Handle("/manifest.json", assets.ServeFiles(web.Files, "files"))
	router.Handle("/favicon.ico", assets.ServeFiles(web.Files, "files/images"))
	router.Handle("/.well-known/*", assets.ServeFiles(web.Files, "files/.well-known"))
	router.Handle("/assets/*", assets.ServeFiles(web.Files, ""))
	router.Handle("/files/fonts/*", assets.ServeFiles(web.Files, "files/dist/fonts"))
	router.Handle("/files/misc/*", assets.ServeFiles(web.Files, "files/misc"))
	router.Handle("/images/*", assets.ServeFiles(web.Files, "files/images"))

	// Image proxy.
	router.Get("/img-proxy/*", imgproxy.HandleImage(imgCache))
	router.Get("/img/{imgType}/*", cache.HandleImage(imgCache))

	// Handle incoming webhooks from Resend
	router.Post("/mail/webhooks", handlers.HandleResendWebhook(resendVerifier, resendProcesser))
	// Handle incoming webhooks from Paddle.
	router.Post("/webhooks/paddle", handlers.HandlePaddleWebhook(userSvc, emailSender))
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
			r.With(htmx.RequireHTMX).Post("/", handlerMgr.HandleSubmitContact(emailSender))
		})
		r.Route("/forget-me", func(r chi.Router) {
			r.Get("/", handlerMgr.HandleForgetMe())
			r.With(htmx.RequireHTMX).Post("/", handlerMgr.HandleSubmitContact(emailSender))
		})
		// Feed Viewer.
		r.Route("/viewer", func(r chi.Router) {
			r.Get("/", handlerMgr.HandleViewer(feedSvc))
			r.Get("/url/*", handlerMgr.HandleViewer(feedSvc))
			r.With(htmx.RequireHTMX).Post("/", handlerMgr.HandleViewer(feedSvc))
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
			r.Get("/signup", handlerMgr.HandleLogin(auth))
			r.Route("/login", func(r chi.Router) {
				r.Get("/", handlerMgr.HandleLogin(auth))
				r.Get("/callback", handlerMgr.HandleLoginCallback(userSvc, authMgr, auth, emailSender))
				r.Get("/error", handlerMgr.HandleLoginError)
			})
			r.Get("/logout", handlerMgr.HandleLogout(auth))
			r.Get("/account-issue", handlerMgr.HandleAccountIssue())
		})
		// Web payment routes.
		r.Group(func(r chi.Router) {
			r.Use(
				middlewares.ExtractUserFromSession(userSvc, auth, sessionManager),
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
			middlewares.ExtractUserFromSession(userSvc, auth, sessionManager),
			middlewares.RequireValidUser,
			handlerMgr.ValidateSubscriptionLimits(userSvc, subscriptionSvc, emailSender),
			middlewares.NoCache,
			handlerMgr.CustomisationCtx,
		)
		// Manual login refresh.
		r.Get("/login/refresh", handlerMgr.HandleRefreshToken(auth))
		r.With(handlerMgr.AllSubscriptionsCtx(subscriptionSvc)).
			Get("/home", handlerMgr.HandleHome(&service.Home{}))
		// Searching.
		r.Route("/search", func(r chi.Router) {
			r.Use(middlewares.CanonicalizeSearchParams(sessionManager))
			r.Use(handlerMgr.AllSubscriptionsCtx(subscriptionSvc))
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
			r.Use(handlerMgr.CheckUserLimits)
			r.Get("/", handlerMgr.HandleDiscover())
			r.With(htmx.RequireHTMX).Post("/suggest", handlerMgr.HandleDiscoverSuggestions(feedSvc))
		})
		// Subscription specific.
		r.Route("/subscriptions", func(r chi.Router) {
			r.Use(middlewares.CanonicalizeListFilters(sessionManager))
			r.Use(handlerMgr.AllSubscriptionsCtx(subscriptionSvc))
			r.Get("/", handlerMgr.HandleListSubscriptions(subscriptionSvc))
			r.With(htmx.RequireHTMX).Get("/categories", handlerMgr.ListCategories(subscriptionSvc, itemSvc))
			// r.Get("/", handlers.HandleListSubscriptions()) // ?sort=&status=&category=&page=&per_page=
			r.Group(func(r chi.Router) {
				r.Use(htmx.RequireHTMX)
				r.Post("/updates", handlerMgr.HandleListSubscriptionsUpdates(itemSvc))
				r.Post("/paginate", handlerMgr.HandleListSubscriptions(subscriptionSvc))
				r.Post("/read", handlerMgr.HandleBulkMarkSubscriptions(subscriptionSvc, models.MarkRead))
				r.Post(
					"/unread",
					handlerMgr.HandleBulkMarkSubscriptions(subscriptionSvc, models.MarkUnread),
				)
				r.Post("/remove", handlerMgr.HandleBulkRemoveSubscriptions(subscriptionSvc))
				r.Route("/add", func(r chi.Router) {
					r.Post("/feedset", handlerMgr.HandleAddFeedset(feedSvc, subscriptionSvc))
				})
			})
			r.Route("/{subscriptionID}", func(r chi.Router) {
				r.Use(handlerMgr.SubscriptionCtx(subscriptionSvc))
				// 	// r.Get("/", handleSubscriptionDetail) // its own article feed: ?sort=&status=&page=
				r.Route("/edit", func(r chi.Router) {
					r.Get("/", handlerMgr.HandleEditSubscription(subscriptionSvc))
					r.Post("/", handlerMgr.HandleSaveSubscription(imgCache, subscriptionSvc))
				})
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
					r.Get("/remove", handlerMgr.HandleRemoveSubscription(subscriptionSvc))
				})
			})
		})
		r.Route("/subscription", func(r chi.Router) {
			r.Use(handlerMgr.AllSubscriptionsCtx(subscriptionSvc))
			r.Route("/add", func(r chi.Router) {
				r.Get("/", handlerMgr.HandleAddSubscription())
				r.With(htmx.RequireHTMX).Post("/suggestions", handlerMgr.HandleSuggestFeeds(feedSvc))
				r.With(htmx.RequireHTMX).
					Post("/feed", handlerMgr.HandleAddNewFeedSubscription(subscriptionSvc, userSvc, feedSvc))
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
		r.Route("/articles", func(r chi.Router) {
			r.Use(middlewares.CanonicalizeListFilters(sessionManager))
			r.Use(handlerMgr.AllSubscriptionsCtx(subscriptionSvc))
			r.Get("/", handlerMgr.HandleListArticles(itemSvc))
			r.With(htmx.RequireHTMX).Get("/categories", handlerMgr.ListCategories(subscriptionSvc, itemSvc))
			r.Group(func(r chi.Router) {
				r.Use(htmx.RequireHTMX)
				r.Post("/updates", handlerMgr.HandleListArticlesUpdates(itemSvc))
				r.Post("/paginate", handlerMgr.HandleListArticles(itemSvc))
				r.Post("/read", handlerMgr.HandleBulkMarkArticles(subscriptionSvc, models.MarkRead))
				r.Post("/unread", handlerMgr.HandleBulkMarkArticles(subscriptionSvc, models.MarkRead))
			})
			r.Route("/{articleID}", func(r chi.Router) {
				r.Use(handlerMgr.ArticleCtx(itemSvc))
				r.Get(
					"/",
					handlerMgr.HandleViewArticle(itemSvc),
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
			r.Use(handlerMgr.AllSubscriptionsCtx(subscriptionSvc))
			r.Get("/", handlerMgr.HandleListFavorites(subscriptionSvc, itemSvc))
		})
		// Map
		r.Route("/map", func(r chi.Router) {
			r.Use(middlewares.CanonicalizeListFilters(sessionManager))
			r.Use(handlerMgr.AllSubscriptionsCtx(subscriptionSvc))
			r.Get("/", handlerMgr.HandleMap(itemSvc))
			r.With(htmx.RequireHTMX).Post("/updates", handlerMgr.HandleMapUpdates(itemSvc))
		})
		// Issues.
		r.Route("/issue", func(r chi.Router) {
			r.Get("/", handlerMgr.HandleReportIssue())
			r.With(htmx.RequireHTMX).Post("/", handlerMgr.HandleSubmitIssue(imgCache, emailSender))
		})
		// Help/Documentation.
		r.Get("/docs", handlerMgr.DocumentationHandler("/docs"))
		// Settings.
		r.Route("/settings", func(r chi.Router) {
			r.Get("/", handlerMgr.ShowSettings())
			r.Route("/display", func(r chi.Router) {
				r.Use(htmx.RequireHTMX)
				r.Get("/", handlerMgr.HandleShowDisplaySettings())
				r.Post("/", handlerMgr.HandleSaveDisplaySettings(userSvc))
				r.Post("/font", handlerMgr.HandleSaveFontSettings(userSvc))
				r.Post("/theme", handlerMgr.HandleSaveThemeSettings(userSvc))
			})
			r.Route("/account", func(r chi.Router) {
				r.Use(htmx.RequireHTMX)
				r.Get("/", handlerMgr.HandleShowAccountSettings())
				r.Post("/", handlerMgr.HandleSaveAccountSettings(userSvc, authMgr, imgCache))
				r.Post("/password", handlerMgr.HandleChangePassword(authMgr))
				r.Post("/newsletters", handlerMgr.HandleGenerateSubscriptionEmail(userSvc))
				r.Post("/deactivate", handlerMgr.HandleDeactivateAccount(userSvc, authMgr, auth, emailSender))
			})
			r.Route("/subscriptions", func(r chi.Router) {
				r.Use(htmx.RequireHTMX)
				r.Use(handlerMgr.AllSubscriptionsCtx(subscriptionSvc))
				r.Get("/", handlerMgr.HandleShowSubscriptionsSettings())
				r.Post("/", handlerMgr.HandleSaveSubscriptionsSettings(userSvc))
			})
			// TODO: check if still used?
			r.Get("/subscription", handlerMgr.HandleManageAccountSubscription())
		})
		// Import.
		r.Route("/import", func(r chi.Router) {
			r.Get("/", handlerMgr.HandleSetupImport(subscriptionSvc, userSvc, emailSender))
			r.With(htmx.RequireHTMX).Post("/", handlerMgr.HandleStartImport(importSvc))
			r.Route("/status", func(r chi.Router) {
				r.Use(handlerMgr.AllSubscriptionsCtx(subscriptionSvc))
				r.Get("/", handlerMgr.HandleImportStatus(importSvc, subscriptionSvc))
				r.Get("/{jobID}", handlerMgr.HandleImportStatus(importSvc, subscriptionSvc))
			})
		})
		// Export.
		r.Route("/export", func(r chi.Router) {
			r.Use(handlerMgr.AllSubscriptionsCtx(subscriptionSvc))
			r.Get("/", handlerMgr.HandleExportSubscriptions(feedSvc))
			r.Post("/", handlerMgr.HandleExportSubscriptions(feedSvc))
		})

		// Moved routes.
		r.Get("/list/favorites", handlers.RedirectTo("/favorites", http.StatusMovedPermanently))
		r.Get("/list/subscriptions", handlers.RedirectTo("/subscriptions", http.StatusMovedPermanently))
		r.Get("/list/articles", handlers.RedirectTo("/articles", http.StatusMovedPermanently))
		r.Get("/posts", handlers.RedirectTo("/blog", http.StatusMovedPermanently))
		r.Get("/posts/*", handlers.RedirectParam("*", "blog/%s", http.StatusMovedPermanently))
		r.Get("/view/article/{item_id}", handlers.RedirectParam("item_id", "/articles/%s", http.StatusMovedPermanently))
		r.Get("/user/import", handlers.RedirectTo("/import", http.StatusMovedPermanently))
		r.Get("/user/export", handlers.RedirectTo("/export", http.StatusMovedPermanently))
		r.Get("/user/settings", handlers.RedirectTo("/settings", http.StatusMovedPermanently))
	})

	// Requests must NOT inherit the signal context, otherwise SIGTERM cancels every in-flight request
	// immediately and svr.Shutdown has nothing left to drain. WithoutCancel keeps the logger/values.
	baseCtx := context.WithoutCancel(ctx)

	svr := &http.Server{
		Protocols:         new(http.Protocols),
		Handler:           router,
		Addr:              net.JoinHostPort(cfg.Host, strconv.FormatUint(cfg.Port, 10)),
		ReadHeaderTimeout: cfg.ReadTimeout.Duration,
		ReadTimeout:       cfg.ReadTimeout.Duration,
		WriteTimeout:      cfg.WriteTimeout.Duration,
		IdleTimeout:       cfg.IdleTimeout.Duration,
		BaseContext: func(_ net.Listener) context.Context {
			return baseCtx
		},
	}
	svr.Protocols.SetUnencryptedHTTP2(true) // Enable H2C (HTTP/2 cleartext)
	svr.Protocols.SetHTTP1(true)            // Enable HTTP/1.1
	svr.Protocols.SetHTTP2(false)           // Explicitly disable encrypted HTTP/2 (HTTPS)
	// Lets long-lived handlers (SSE, long-poll) exit via ShuttingDown() instead of stalling Shutdown.
	svr.RegisterOnShutdown(func() { close(shuttingDown) })

	// Bind synchronously so bind errors (port in use, etc.) are returned immediately and the
	// "Server started" message is accurate.
	ln, err := net.Listen("tcp", svr.Addr)
	if err != nil {
		// Dependencies are already initialised, so release them on this path too.
		return errors.Join(fmt.Errorf("listen on %s: %w", svr.Addr, err), shutdown(svr, logger))
	}

	// And we serve HTTP until the world ends.
	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)
		var serveErr error
		if cfg.CertFile != "" && cfg.KeyFile != "" {
			logger.Debug("Using https.",
				slog.String("certificate file", cfg.CertFile),
				slog.String("key file", cfg.KeyFile),
			)
			serveErr = svr.ServeTLS(ln, cfg.CertFile, cfg.KeyFile)
		} else {
			logger.Debug("Using http.")
			serveErr = svr.Serve(ln)
		}
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errCh <- serveErr
		}
	}()

	ready.Store(true)
	logger.Info("Server started...",
		slog.String("address", svr.Addr),
		slog.String("version", appCfg.GetAppVersion()),
		slog.Time("start_time", time.Now().UTC()),
		slog.Duration("startup_took", time.Since(startTime)),
	)

	var serveErr error
	select {
	case serveErr = <-errCh:
		if serveErr != nil {
			serveErr = fmt.Errorf("server failed: %w", serveErr)
		}
	case <-ctx.Done():
		slog.Info("Shutting down server")
	}

	// Stop catching signals so a second Ctrl-C force-kills the process.
	cancelFunc()

	// Tell the load balancer to stop routing to us, and keep serving while it catches up.
	ready.Store(false)
	if serveErr == nil && readinessDrainDelay > 0 {
		time.Sleep(readinessDrainDelay)
	}

	// Cleanup always runs, whether we got here from a signal or a serve failure.
	return errors.Join(serveErr, shutdown(svr, logger))
}

// shutdown stops the server first (so no new requests arrive and in-flight ones finish), and only then
// tears down dependencies, in the reverse order they are used. OpenTelemetry is flushed last by the
// deferred call in Start.
func shutdown(svr *http.Server, logger *slog.Logger) error {
	// The shutdown context must not derive from the (already cancelled) signal context.
	shutdownCtx, cancel := context.WithTimeoutCause(
		context.Background(),
		gracefulShutdownTimeout,
		errors.New("graceful shutdown timeout"),
	)
	shutdownCtx = slogctx.NewCtx(shutdownCtx, logger)
	defer cancel()

	var errs []error

	// 1. Stop accepting connections and wait for in-flight requests to complete.
	if err := svr.Shutdown(shutdownCtx); err != nil {
		errs = append(errs, fmt.Errorf("server graceful shutdown: %w", err))
		// Timed out: force-close remaining connections so the process can exit.
		if closeErr := svr.Close(); closeErr != nil {
			errs = append(errs, fmt.Errorf("server close: %w", closeErr))
		}
	}

	// 2. TODO: stop background work (import jobs, Resend processing) and wait for it to finish,
	// e.g. importSvc.Shutdown(shutdownCtx) backed by a sync.WaitGroup.

	// 3. Flush and close the bulk indexer (if initialized).
	if err := bulk.Shutdown(shutdownCtx); err != nil {
		slogctx.FromCtx(shutdownCtx).Error("Bulk indexer failed to shutdown gracefully.",
			slog.Any("error", err),
		)
		errs = append(errs, fmt.Errorf("bulk indexer shutdown: %w", err))
	}
	// 4. Then close any Elasticsearch connection (if initialized).
	if err := elastic.Shutdown(shutdownCtx); err != nil {
		slogctx.FromCtx(shutdownCtx).Error("Elasticsearch failed to shutdown gracefully.",
			slog.Any("error", err),
		)
		errs = append(errs, fmt.Errorf("elasticsearch shutdown: %w", err))
	}

	// 5. TODO: close caches / session store if they expose a Close method.

	// 6. Close error client.
	gerror.CloseClient()

	slogctx.FromCtx(shutdownCtx).Info("Server stopped",
		slog.Time("stop_time", time.Now().UTC()),
	)

	return errors.Join(errs...)
}
