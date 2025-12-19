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
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/justinas/nosurf"
	slogctx "github.com/veqryn/slog-context"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/stripe"
	"github.com/immanent-tech/foragd/server/handlers"
	"github.com/immanent-tech/foragd/server/middlewares"
	"github.com/immanent-tech/foragd/server/session"
	"github.com/immanent-tech/foragd/web"
)

// Server represents the application server.
type Server struct {
	*http.Server
}

// APIs contains backend APIs used by the server or handlers/middlewares.
type APIs struct {
	elastic *elastic.API
}

var apis APIs

// NewServer sets up a new server.
func NewServer(ctx context.Context) (Server, error) {
	svr := Server{}
	// Load the server config.
	if err := loadConfigOnce(); err != nil {
		return svr, fmt.Errorf("unable to load server config: %w", err)
	}

	var err error

	// Load the Elastic backend
	apis.elastic, err = elastic.NewConnection()
	if err != nil {
		return svr, fmt.Errorf("unable to set up elastic api: %w", err)
	}
	// Set up the session manager.
	err = session.NewSessionManager(apis.elastic)
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
	var wg sync.WaitGroup

	wg.Add(1)
	// Listen for shutdown events and process them.
	go func() {
		wg.Done()

		stop := make(chan os.Signal, 1)

		signal.Notify(stop, os.Interrupt)
		<-stop

		if err := s.Shutdown(context.Background()); err != nil {
			// Can't do much here except for logging any errors
			slogctx.FromCtx(context.Background()).Error("Error occurred when trying to shut down server.",
				slog.Any("error", err),
			)
		}
	}()

	slogctx.FromCtx(ctx).Info("Starting server...",
		slog.String("address", s.Addr),
		slog.Time("start_time", time.Now()),
	)

	// And we serve HTTP until the world ends.
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
	if errors.Is(err, http.ErrServerClosed) { // graceful shutdown
		slogctx.FromCtx(ctx).Info("commencing server shutdown...")
		wg.Wait()
	} else if err != nil {
		return fmt.Errorf("error shutting down server: %w", err)
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
		middleware.Recoverer,
		middlewares.Logger(),
		middlewares.SetupCORS(),
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
	router.Get("/img/avatar/*", handlers.Avatar())

	// Front page.
	router.Get("/", handlers.Landing())
	// Sign-up/Login.
	router.Group(func(r chi.Router) {
		r.Use(
			middlewares.Etag,
			middlewares.CrossOriginProtection,
			session.LoadAndSave,
		)
		if !cfg.BlockSignup {
			r.Get("/signup", handlers.Login())
		} else {
			slogctx.FromCtx(ctx).Warn("Signups have been BLOCKED by configuration.")
		}
		if !cfg.BlockLogin {
			r.Get("/login", handlers.Login())
			r.Get("/login/callback", handlers.LoginCallback(apis.elastic))
		} else {
			slogctx.FromCtx(ctx).Warn("Logins have been BLOCKED by configuration.")
		}
		r.Get("/logout", handlers.Logout())
	})

	// Handle incoming webhook requests from Stripe.
	router.Post("/checkout/webhooks", stripe.HandleWebhook(apis.elastic))

	// Authenticated routes.
	router.Group(func(r chi.Router) {
		r.Use(
			middlewares.Etag,
			middlewares.CrossOriginProtection,
			middlewares.SetupHTMX,
			session.LoadAndSave,
			middlewares.RequireUserAuth(apis.elastic),
			// middleware.NoCache,
		)
		// Payment routes (Stripe).
		r.Route("/checkout", func(r chi.Router) {
			r.Get("/choose-plan", handlers.UserChooseSubscriptionPlan())
			r.Post("/", handlers.UserSubscriptionPlanCheckout())
			r.Get("/success", handlers.UserAccountSuccess())
			r.Get("/cancel", handlers.Landing())
		})
		r.Get("/home", handlers.Home(apis.elastic))
		r.Get("/home/updates", handlers.WatchHome(apis.elastic))
		r.With(middlewares.RequireHTMX).Post("/search/suggestions", handlers.GetSearchSuggestions(apis.elastic))
		r.With(middlewares.RequireHTMX).Post("/search", handlers.GetSearchResults(apis.elastic))
		r.With(middlewares.RequireHTMX).Post("/search/paginate", handlers.GetSearchResults(apis.elastic))
		r.With(middlewares.RequireHTMX).
			Post("/search/subscription/suggestions", handlers.GetSubscriptionFilterSuggestions(apis.elastic))
		r.With(middlewares.RequireHTMX).Post("/search/subscription", handlers.AddSubscriptionFilter())
		r.Get("/search", handlers.GetSearchResults(apis.elastic))
		r.Get("/search/updates", handlers.WatchSearchResults(apis.elastic))

		// Objects.
		r.Get("/view/{object}/{id}", handlers.ViewObject(apis.elastic))
		r.With(middlewares.RequireHTMX).Get("/view/{object}/{id}/similar", handlers.FindSimilar(apis.elastic))
		// r.With(middlewares.RequireHTMX).Get("/view/{object}/{id}/share", handlers.ShareObject(apis.elastic))
		r.With(middlewares.RequireHTMX).Get("/issue/{object}/{id}", handlers.GetObjectIssues())
		r.With(middlewares.RequireHTMX).Post("/issue/{object}/{id}", handlers.SubmitObjectIssues())
		// Subscription specific.
		r.Route("/list/subscriptions", func(r chi.Router) {
			r.Get("/", handlers.ListSubscriptions(apis.elastic))
			r.With(middlewares.RequireHTMX).Post("/", handlers.ListSubscriptions(apis.elastic))
			r.With(middlewares.RequireHTMX).Post("/paginate", handlers.PaginateSubscriptions(apis.elastic))
			r.With(middlewares.RequireHTMX).Post("/mark/{mark}", handlers.MarkSubscriptions(apis.elastic))
			r.Get("/updates", handlers.WatchList(apis.elastic))
		})
		r.With(middlewares.RequireHTMX).
			Post("/mark/subscription/{subscription_id}/{mark}", handlers.MarkSubscription(apis.elastic))
		r.With(middlewares.RequireHTMX).
			Post("/remove/subscription/{subscription_id}", handlers.RemoveSubscription(apis.elastic))
		r.With(middlewares.RequireHTMX).
			Delete("/remove/subscription/{subscription_id}", handlers.RemoveSubscription(apis.elastic))
		r.Route("/edit/subscription/{subscription_id}", func(r chi.Router) {
			r.Get("/", handlers.EditSubscription(apis.elastic))
			r.With(middlewares.RequireHTMX).Post("/", handlers.SaveSubscription(apis.elastic))
			r.With(middlewares.RequireHTMX).Post("/category", handlers.AdjustSubscriptionCategories())
			r.With(middlewares.RequireHTMX).Delete("/category", handlers.AdjustSubscriptionCategories())
		})
		// Article specific.
		r.Route("/list/articles", func(r chi.Router) {
			r.Get("/", handlers.ListArticles(apis.elastic))
			r.With(middlewares.RequireHTMX).Post("/", handlers.ListArticles(apis.elastic))
			r.With(middlewares.RequireHTMX).Post("/paginate", handlers.PaginateArticles(apis.elastic))
			r.With(middlewares.RequireHTMX).Post("/mark/{mark}", handlers.MarkArticles(apis.elastic))
			r.Get("/updates", handlers.WatchList(apis.elastic))
		})
		r.With(middlewares.RequireHTMX).Post("/mark/article/{item_id}/{mark}", handlers.MarkArticle(apis.elastic))
		// General.
		r.With(middlewares.RequireHTMX).Get("/issue", handlers.GetPageIssues())
		r.With(middlewares.RequireHTMX).Post("/issue", handlers.SubmitPageIssues())
		// Favorite specific.
		r.Route("/list/favorites", func(r chi.Router) {
			r.Get("/", handlers.ListFavorites(apis.elastic))
		})

		// Help documentation.
		r.Get("/help/*", handlers.DocumentationHandler())

		// User routes.
		r.Route("/user", func(r chi.Router) {
			r.Get("/account-issue", handlers.UserAccountIssue())
			// Subscription.
			r.Route("/subscription", func(r chi.Router) {
				// Add feed subscription.
				r.Get("/add/feed", handlers.AddFeedSubscription(apis.elastic))
				r.With(middlewares.RequireHTMX).Post("/add/feed", handlers.AddFeedSubscription(apis.elastic))
				// Add search subscription.
				r.Get("/add/search", handlers.AddSearchSubscription(apis.elastic))
				r.With(middlewares.RequireHTMX).Post("/add/search", handlers.AddSearchSubscription(apis.elastic))
				// Add group subscription.
				r.Get("/add/group", handlers.AddGroupSubscription(apis.elastic))
				r.With(middlewares.RequireHTMX).Post("/add/group", handlers.AddGroupSubscription(apis.elastic))
				// Category management for add/edit subscription.
				r.With(middlewares.RequireHTMX).Post("/category", handlers.AdjustSubscriptionCategories())
				r.With(middlewares.RequireHTMX).Delete("/category", handlers.AdjustSubscriptionCategories())
			})
			r.Post("/feedset", handlers.AddFeedset(apis.elastic, web.StaticContentFS))
			// Import/export.
			r.Get("/import", handlers.ImportSubscriptions(apis.elastic))
			r.With(middlewares.RequireHTMX).Post("/import", handlers.ImportSubscriptions(apis.elastic))
			r.Get("/export", handlers.ExportSubscriptions(apis.elastic))
			r.Get("/export/opml", handlers.ExportSubscriptions(apis.elastic))
			// Favorites.
			r.Route("/favorite", func(r chi.Router) {
				r.Post("/add/subscription/{subscription_id}", handlers.AddFavoriteSubscription(apis.elastic))
				r.Post("/remove/subscription/{subscription_id}", handlers.RemoveFavoriteSubscription(apis.elastic))
				r.Post("/add/article/{item_id}", handlers.AddFavoriteArticle(apis.elastic))
				r.Post("/remove/article/{item_id}", handlers.RemoveFavoriteArticle(apis.elastic))
				// r.Post("/add/search", handler.AddFavoriteSearch())
				// r.Post("/remove/search", handler.RemoveFavoriteSearch())
			})
			// Settings.
			r.Route("/settings", func(r chi.Router) {
				r.Get("/", handlers.ShowSettings())
				r.With(middlewares.RequireHTMX).Get("/display", handlers.ShowDisplaySettings())
				r.With(middlewares.RequireHTMX).Post("/display", handlers.SaveDisplaySettings(apis.elastic))
				r.With(middlewares.RequireHTMX).Get("/account", handlers.ShowAccountSettings())
				r.With(middlewares.RequireHTMX).Post("/account", handlers.SaveAccountSettings(apis.elastic))
				r.Get("/subscription", handlers.UserManageAccountSubscription())
				r.With(middlewares.RequireHTMX).Post("/password", handlers.ChangePassword())
				r.Route("/theme", func(r chi.Router) {
					r.With(middlewares.RequireHTMX).Put("/{theme}", handlers.SetTheme(apis.elastic))
				})
			})
			r.With(middlewares.RequireHTMX).Get("/deactivate", handlers.UserDeactivateAccount())
			r.With(middlewares.RequireHTMX).Post("/deactivate", handlers.UserDeactivateAccount())
			r.With(middlewares.RequireHTMX).Post("/deactivate/cancel", handlers.UserCancelDeactivation())
		})
	})

	return router
}
