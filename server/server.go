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
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/justinas/nosurf"
	slogctx "github.com/veqryn/slog-context"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/immanent-tech/foragd/providers/stripe"
	"github.com/immanent-tech/foragd/server/handlers"
	"github.com/immanent-tech/foragd/server/middlewares"
	"github.com/immanent-tech/foragd/server/session"
	"github.com/immanent-tech/foragd/web"
)

const (
	gracefulShutdownTimeout = 5 * time.Second
)

// Server represents the application server.
type Server struct {
	*http.Server
}

// NewServer sets up a new server.
func NewServer(ctx context.Context) (Server, error) {
	svr := Server{}
	// Load the server config.
	if err := loadConfigOnce(); err != nil {
		return svr, fmt.Errorf("unable to load server config: %w", err)
	}

	var err error

	// Set up the session manager.
	err = session.NewSessionManager()
	if err != nil {
		return svr, fmt.Errorf("unable to set up session api: %w", err)
	}

	// Set up routes.
	router := svr.setupRoutes(ctx)

	csrfRouter := nosurf.New(router)
	csrfRouter.SetFailureHandler(handlers.CSRFError())
	csrfRouter.ExemptPath("/checkout/webhooks")

	h2s := &http2.Server{}
	svr.Server = &http.Server{
		Handler:     h2c.NewHandler(csrfRouter, h2s),
		Addr:        net.JoinHostPort(cfg.Host, strconv.FormatUint(cfg.Port, 10)),
		ReadTimeout: cfg.ReadTimeout.Duration(),
		// WriteTimeout: ServerConfig.WriteTimeout,
		IdleTimeout: cfg.IdleTimeout.Duration(),
	}

	err = http2.ConfigureServer(svr.Server, h2s)
	if err != nil {
		return svr, fmt.Errorf("unable to configure server for H2C: %w", err)
	}

	return svr, nil
}

// Start will start the server. It runs a background goroutine to safely shutdown the server when its context is
// cancelled.
func (s *Server) Start(ctx context.Context) error {
	slogctx.FromCtx(ctx).Info("Starting server...",
		slog.String("address", s.Addr),
		slog.Time("start_time", time.Now()),
	)

	// And we serve HTTP until the world ends.
	go func() {
		var err error
		if cfg.CertFile != "" && cfg.KeyFile != "" {
			slogctx.FromCtx(ctx).Info("Using https.",
				slog.String("certificate file", cfg.CertFile),
				slog.String("key file", cfg.KeyFile),
			)
			err = s.ListenAndServeTLS(cfg.CertFile, cfg.KeyFile)
		} else {
			slogctx.FromCtx(ctx).Info("Using http.")
			err = s.ListenAndServe()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slogctx.FromCtx(ctx).Error("Could not listen.",
				slog.Any("error", err),
			)
		}
	}()

	// Shutdown gracefully.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
	slogctx.FromCtx(ctx).Info("Stopping server...")
	ctx, cancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil {
		slog.Error("Server forced to shutdown", //nolint:sloglint // this is fine.
			slog.Any("error", err),
		)
	}

	return nil
}

//nolint:funlen
func (s *Server) setupRoutes(ctx context.Context) *chi.Mux {
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
		middlewares.SaveCSRFToken,
		middlewares.RateLimit(rateLimiter),
		middlewares.SetupImgProxy(cfg.ImgProxy.Key, cfg.ImgProxy.Salt),
		middleware.Compress(defaultCompressionLevel, compressMimetypes...),
		middleware.StripSlashes,
	)

	// Error handling.
	router.NotFound(handlers.NotFound())
	// Static content.
	router.Handle("/content/*", handlers.StaticFileHandler(http.FS(web.StaticContentFS)))
	router.Handle("/robots.txt", handlers.RobotsHandler())
	// Policy documents (i.e., terms of service, privacy).
	router.Get("/policies/*", handlers.PolicyDocsHandler())
	// Image proxy.
	router.Get("/img-proxy/*", handlers.ImageProxy(cfg.ImgProxy.BaseURL))
	// Avatars
	router.Get("/img/avatar/*", handlers.LoadCachedImage("avatar"))
	// User custom subscription images
	router.Get("/img/subscription/*", handlers.LoadCachedImage("subscription_thumbnail"))
	// User uploaded screenshots
	router.Get("/img/screenshots/*", handlers.LoadCachedImage("screenshot"))

	// Front page.
	router.Get("/", handlers.Landing())
	// Sign-up/Login.
	router.Group(func(r chi.Router) {
		r.Use(
			middlewares.Etag,
			session.LoadAndSave,
		)
		if !cfg.BlockSignup {
			r.Get("/signup", handlers.Login)
		} else {
			slogctx.FromCtx(ctx).Warn("Signups have been BLOCKED by configuration.")
		}
		if !cfg.BlockLogin {
			r.Get("/login", handlers.Login)
			r.Get("/login/callback", handlers.LoginCallback)
		} else {
			slogctx.FromCtx(ctx).Warn("Logins have been BLOCKED by configuration.")
		}
		r.Get("/logout", handlers.Logout)
	})

	// Handle incoming webhook requests from Stripe.
	router.Post("/checkout/webhooks", stripe.HandleWebhook)

	// Authenticated routes.
	router.Group(func(r chi.Router) {
		r.Use(
			middlewares.Etag,
			middlewares.CrossOriginProtection,
			middlewares.SetupHTMX,
			session.LoadAndSave,
			middlewares.RequireUserAuth,
			// middleware.NoCache,
		)
		// Payment routes (Stripe).
		r.Route("/checkout", func(r chi.Router) {
			r.Get("/choose-plan", handlers.UserChooseSubscriptionPlan())
			r.Post("/", handlers.UserSubscriptionPlanCheckout())
			r.Get("/success", handlers.UserAccountSuccess())
			r.Get("/cancel", handlers.Landing())
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
			r.Get("/", handlers.ListSubscriptions())
			r.With(middlewares.RequireHTMX).Post("/", handlers.ListSubscriptions())
			r.With(middlewares.RequireHTMX).Post("/paginate", handlers.PaginateSubscriptions())
			r.With(middlewares.RequireHTMX).Post("/mark/{mark}", handlers.MarkSubscriptions())
			r.Get("/updates", handlers.WatchList())
		})
		r.With(middlewares.RequireHTMX).
			Post("/mark/subscription/{subscription_id}/{mark}", handlers.MarkSubscription())
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
			r.With(middlewares.RequireHTMX).Post("/", handlers.ListArticles())
			r.With(middlewares.RequireHTMX).Post("/paginate", handlers.PaginateArticles())
			r.With(middlewares.RequireHTMX).Post("/mark/{mark}", handlers.MarkArticles())
			r.Get("/updates", handlers.WatchList())
		})
		r.With(middlewares.RequireHTMX).Post("/mark/article/{item_id}/{mark}", handlers.MarkArticle())
		r.Get("/view/article/{item_id}", handlers.ViewArticle())
		r.Get("/view/article/{item_id}/similar", handlers.FindSimilarArticles())
		// General.
		r.With(middlewares.RequireHTMX).Get("/issue", handlers.GetPageIssues())
		r.With(middlewares.RequireHTMX).Post("/issue", handlers.SubmitPageIssues())
		// Favorite specific.
		r.Route("/list/favorites", func(r chi.Router) {
			r.Get("/", handlers.ListFavorites())
		})

		// Help documentation.
		r.Get("/help/*", handlers.DocumentationHandler())

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
			r.Get("/export/opml", handlers.ExportSubscriptions())
			// Favorites.
			r.Route("/favorite", func(r chi.Router) {
				r.Post("/add/subscription/{subscription_id}", handlers.AddFavoriteSubscription())
				r.Post("/remove/subscription/{subscription_id}", handlers.RemoveFavoriteSubscription())
				r.Post("/add/article/{item_id}", handlers.AddFavoriteArticle())
				r.Post("/remove/article/{item_id}", handlers.RemoveFavoriteArticle())
				// r.Post("/add/search", handler.AddFavoriteSearch())
				// r.Post("/remove/search", handler.RemoveFavoriteSearch())
			})
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
			})
			r.With(middlewares.RequireHTMX).Get("/deactivate", handlers.UserDeactivateAccount())
			r.With(middlewares.RequireHTMX).Post("/deactivate", handlers.UserDeactivateAccount())
			r.With(middlewares.RequireHTMX).Post("/deactivate/cancel", handlers.UserCancelDeactivation())
		})
	})

	return router
}
