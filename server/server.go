// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"context"
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

	"github.com/immanent-tech/foragd/providers/resend"
	"github.com/immanent-tech/foragd/providers/stripe"
	"github.com/immanent-tech/foragd/server/handlers"
	"github.com/immanent-tech/foragd/server/middlewares"
	"github.com/immanent-tech/foragd/server/session"
	"github.com/immanent-tech/foragd/web"
)

const (
	gracefulShutdownTimeout = 30 * time.Second
)

// Start will start the server.
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

	// Set up routes.
	router := setupRoutes(ctx)

	h2s := &http2.Server{}
	svr := &http.Server{
		Handler:      h2c.NewHandler(router, h2s),
		Addr:         net.JoinHostPort(cfg.Host, strconv.FormatUint(cfg.Port, 10)),
		ReadTimeout:  cfg.ReadTimeout.Duration(),
		WriteTimeout: cfg.WriteTimeout.Duration(),
		IdleTimeout:  cfg.IdleTimeout.Duration(),
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}

	err = http2.ConfigureServer(svr, h2s)
	if err != nil {
		return fmt.Errorf("unable to configure server for H2C: %w", err)
	}

	logger.Info("Starting server...",
		slog.String("address", svr.Addr),
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
	logger.Info("Shutting down server...")
	if err := svr.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server failed to shutdown gracefully.",
			slog.Any("error", err),
		)
	}

	logger.Info("Server shutdown gracefully")

	return nil
}

//nolint:funlen
func setupRoutes(ctx context.Context) *chi.Mux {
	rateLimiter := middlewares.NewRateLimiter()

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
		middlewares.RateLimit(rateLimiter),
		middlewares.SetupImgProxy(cfg.ImgProxy.Key, cfg.ImgProxy.Salt),
		middleware.Compress(defaultCompressionLevel, compressMimetypes...),
		middleware.StripSlashes,
		middlewares.Etag,
	)

	// Public facing routes.
	// Error handling.
	router.NotFound(handlers.HandleNotFound())
	// Image proxy.
	router.Get("/img-proxy/*", handlers.ImageProxy(cfg.ImgProxy.BaseURL))
	// Robots.
	router.Handle("/robots.txt", handlers.RobotsHandler())
	// Static content.
	router.Handle("/content/*", handlers.StaticFileHandler(http.FS(web.StaticContentFS)))
	// Avatars
	router.Get("/img/avatar/*", handlers.LoadCachedImage)
	// User custom subscription images
	router.Get("/img/subscription/*", handlers.LoadCachedImage)
	// User uploaded screenshots
	router.Get("/img/screenshots/*", handlers.LoadCachedImage)
	// Landing.
	router.Get("/", handlers.HandleLandingPage())
	// About.
	router.Get("/about", handlers.About())
	// Feed Viewer.
	router.Get("/viewer", handlers.HandleViewer())
	router.With(middlewares.RequireHTMX).Post("/viewer", handlers.HandleViewer())
	// Help documentation.
	router.Get("/help", handlers.DocumentationHandler())
	// Policy documentation (i.e., terms of service, privacy).
	router.Get("/policies/*", handlers.PolicyDocsHandler())
	// Posts/Blog.
	router.Get("/posts", handlers.PostsHandler())
	router.Get("/posts/*", handlers.PostsHandler())

	// Sign-up/Login routes.
	router.Group(func(r chi.Router) {
		r.Use(
			middlewares.Etag,
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

	// Handle incoming webhook requests from Stripe.
	router.Post("/checkout/webhooks", stripe.HandleWebhook)

	// Handle incoming webhook requests from Resend
	router.Post("/mail/webhooks", resend.HandleWebhook)

	// Authenticated routes.
	router.Group(func(r chi.Router) {
		r.Use(
			middlewares.SetupHTMX,
			session.LoadAndSave,
			middlewares.RequireUserAuth,
			middlewares.RefreshTokenIfNeeded,
			middlewares.SetCacheControl,
			// middleware.NoCache,
		)
		// Payment routes (Stripe).
		r.Route("/checkout", func(r chi.Router) {
			r.Get("/choose-plan", handlers.UserChooseSubscriptionPlan())
			r.Post("/", handlers.UserSubscriptionPlanCheckout())
			r.Get("/success", handlers.UserAccountSuccess())
			r.Get("/cancel", handlers.HandleLandingPage())
		})
		r.Get("/home", handlers.Home())
		r.Get("/home/updates", handlers.WatchHome())
		// Searching.
		r.With(middlewares.RequireHTMX).Post("/search/suggestions", handlers.GetSearchSuggestions())
		r.With(middlewares.RequireHTMX).Post("/search", handlers.GetSearchResults())
		r.With(middlewares.RequireHTMX).Post("/search/paginate", handlers.GetSearchResults())
		r.With(middlewares.RequireHTMX).
			Post("/search/subscription/suggestions", handlers.GetSubscriptionFilterSuggestions())
		r.With(middlewares.RequireHTMX).Post("/search/subscription", handlers.AddSubscriptionFilter())
		r.Get("/search", handlers.GetSearchResults())
		r.Get("/search/updates", handlers.WatchSearchResults())
		// Issues.
		r.With(middlewares.RequireHTMX).Get("/issue/{object}/{id}", handlers.GetObjectIssues())
		r.With(middlewares.RequireHTMX).Post("/issue/{object}/{id}", handlers.SubmitObjectIssues())
		// Subscription specific.
		r.Route("/list/subscriptions", func(r chi.Router) {
			r.Get("/", handlers.HandleListSubscriptions())
			r.With(middlewares.RequireHTMX).Post("/", handlers.HandleListSubscriptions())
			r.With(middlewares.RequireHTMX).Post("/paginate", handlers.HandleListSubscriptions())
			r.With(middlewares.RequireHTMX).Post("/mark/{mark}", handlers.MarkSubscriptions())
			r.Get("/updates", handlers.WatchList())
			r.With(middlewares.RequireHTMX).Get("/categories", handlers.ListCategories())
		})
		r.With(middlewares.RequireHTMX).
			Post("/mark/subscription/{subscription_id}", handlers.MarkSubscription())
		r.With(middlewares.RequireHTMX).
			Post("/favorite/subscription/{subscription_id}", handlers.FavoriteSubscription())
		r.With(middlewares.RequireHTMX).
			Post("/remove/subscription/{subscription_id}", handlers.RemoveSubscription())
		r.With(middlewares.RequireHTMX).
			Delete("/remove/subscription/{subscription_id}", handlers.RemoveSubscription())
		r.Route("/edit/subscription/{subscription_id}", func(r chi.Router) {
			r.Get("/", handlers.EditSubscription())
			r.With(middlewares.RequireHTMX).Post("/", handlers.SaveSubscription())
			r.With(middlewares.RequireHTMX).Post("/category", handlers.AdjustSubscriptionCategories())
			r.With(middlewares.RequireHTMX).Delete("/category", handlers.AdjustSubscriptionCategories())
		})
		// Article specific.
		r.Route("/list/articles", func(r chi.Router) {
			r.Get("/", handlers.ListArticles())
			r.Post("/", handlers.ListArticles())
			// r.With(middlewares.RequireHTMX).Post("/", handlers.HandleListArticles())
			r.With(middlewares.RequireHTMX).Post("/paginate", handlers.PaginateArticles())
			r.With(middlewares.RequireHTMX).Post("/mark/{mark}", handlers.MarkArticles())
			r.Get("/updates", handlers.WatchList())
			r.With(middlewares.RequireHTMX).Get("/categories", handlers.ListCategories())
		})
		r.With(middlewares.RequireHTMX).Post("/mark/article/{item_id}", handlers.MarkArticle())
		r.With(middlewares.RequireHTMX).Post("/favorite/article/{item_id}", handlers.FavoriteArticle())
		r.With(middlewares.RequireHTMX).Post("/share/article/{item_id}", handlers.ShareArticle())
		r.Get("/view/article/{item_id}", handlers.ViewArticle())
		r.Get("/view/article/{item_id}/similar", handlers.FindSimilarArticles())
		// General.
		r.With(middlewares.RequireHTMX).Get("/issue", handlers.GetPageIssues())
		r.With(middlewares.RequireHTMX).Post("/issue", handlers.SubmitPageIssues())
		// Favorite specific.
		r.Route("/list/favorites", func(r chi.Router) {
			r.Get("/", handlers.ListFavorites())
		})

		// User routes.
		r.Route("/user", func(r chi.Router) {
			r.Get("/account-issue", handlers.UserAccountIssue())
			// Subscription.
			r.Route("/subscription", func(r chi.Router) {
				// Add feed subscription.
				r.Get("/add/feed", handlers.AddFeedSubscription())
				r.With(middlewares.RequireHTMX).Post("/add/feed", handlers.AddFeedSubscription())
				// Add search subscription.
				r.Get("/add/search", handlers.AddSearchSubscription())
				r.With(middlewares.RequireHTMX).Post("/add/search", handlers.AddSearchSubscription())
				// Add group subscription.
				r.Get("/add/group", handlers.AddGroupSubscription())
				r.With(middlewares.RequireHTMX).Post("/add/group", handlers.AddGroupSubscription())
				// Category management for add/edit subscription.
				r.With(middlewares.RequireHTMX).Post("/category", handlers.AdjustSubscriptionCategories())
				r.With(middlewares.RequireHTMX).Delete("/category", handlers.AdjustSubscriptionCategories())
			})
			r.Post("/feedset", handlers.AddFeedset(web.StaticContentFS))
			// Import/export.
			r.Get("/import", handlers.ImportSubscriptions())
			r.With(middlewares.RequireHTMX).Post("/import", handlers.ImportSubscriptions())
			r.Get("/export", handlers.ExportSubscriptions())
			r.Post("/export", handlers.ExportSubscriptions())
			// Settings.
			r.Route("/settings", func(r chi.Router) {
				r.Get("/", handlers.ShowSettings())
				r.With(middlewares.RequireHTMX).Get("/display", handlers.ShowDisplaySettings())
				r.With(middlewares.RequireHTMX).Post("/display", handlers.SaveDisplaySettings())
				r.With(middlewares.RequireHTMX).Get("/account", handlers.ShowAccountSettings())
				r.With(middlewares.RequireHTMX).Post("/account", handlers.SaveAccountSettings())
				r.Get("/subscription", handlers.UserManageAccountSubscription())
				r.With(middlewares.RequireHTMX).Post("/password", handlers.ChangePassword())
				r.Route("/theme", func(r chi.Router) {
					r.With(middlewares.RequireHTMX).Put("/{theme}", handlers.SetTheme())
				})
				r.With(middlewares.RequireHTMX).Post("/subscriptionemail", handlers.GenerateSubscriptionEmail())
			})
			r.With(middlewares.RequireHTMX).Get("/deactivate", handlers.UserDeactivateAccount())
			r.With(middlewares.RequireHTMX).Post("/deactivate", handlers.UserDeactivateAccount())
			r.With(middlewares.RequireHTMX).Post("/deactivate/cancel", handlers.UserCancelDeactivation())
		})
		r.Get("/logout", handlers.Logout)
	})

	return router
}
