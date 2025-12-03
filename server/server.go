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

// Server represents the application server. It contains the underlying server object, the handlers, and embedded FS
// for static content.
type Server struct {
	*http.Server
}

// NewServer sets up a new server.
func NewServer(ctx context.Context) (Server, error) {
	svr := Server{}
	// Load the server config.
	err := LoadConfigOnce()
	if err != nil {
		return svr, fmt.Errorf("unable to load server config: %w", err)
	}
	// Set up handlers api.
	api, err := svr.setupAPI(ctx)
	if err != nil {
		return svr, fmt.Errorf("unable to set up handlers api: %w", err)
	}
	// Set up routes.
	router := svr.setupRoutes(api)

	csrfRouter := nosurf.New(router)
	csrfRouter.SetFailureHandler(handlers.CSRFError())
	csrfRouter.ExemptPath("/checkout/webhooks")

	h2s := &http2.Server{}
	svr.Server = &http.Server{
		Handler:     h2c.NewHandler(csrfRouter, h2s),
		Addr:        net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		ReadTimeout: cfg.ReadTimeout,
		// WriteTimeout: ServerConfig.WriteTimeout,
		IdleTimeout: cfg.IdleTimeout,
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

		err := s.Shutdown(context.Background())
		// Can't do much here except for logging any errors
		if err != nil {
			slogctx.FromCtx(ctx).Error("Error occurred when trying to shut down server.",
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

// SetupAPI creates the object containing the various backend APIs needed by handlers.
func (s *Server) setupAPI(ctx context.Context) (*handlers.API, error) {
	// Load the Elastic backend
	elasticAPI, err := elastic.Connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to set up elastic api: %w", err)
	}
	// Set up the session manager.
	err = session.NewSessionManager(ctx, elasticAPI)
	if err != nil {
		return nil, fmt.Errorf("unable to set up session api: %w", err)
	}
	return &handlers.API{
		Elastic: elasticAPI,
	}, nil
}

//nolint:funlen
func (s *Server) setupRoutes(handler *handlers.API) *chi.Mux {
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
		middlewares.SetupCSP(),
		middlewares.Etag,
		middleware.StripSlashes,
		middlewares.SaveCSRFToken,
		middlewares.RateLimit(rateLimiter),
		middlewares.SetupImgProxy(cfg.ImgProxy.Key, cfg.ImgProxy.Salt),
	)

	// Static content.
	router.Handle("/content/*", handlers.StaticFileHandler(http.FS(web.StaticContent)))
	// Docs.
	router.Get("/docs/*", handlers.DocsHandler(web.StaticContent))

	// Image proxy.
	router.Get(
		"/img-proxy/*",
		handlers.ImageProxy(cfg.ImgProxy.BaseURL),
	)

	// Error handling.
	router.NotFound(handlers.NotFound())

	// Front page.
	router.Get("/", handlers.Landing())

	router.Group(func(r chi.Router) {
		r.Use(
			middlewares.SetupElastic(),
			session.Manager.LoadAndSave,
		)
		// Access routes.
		r.Get("/signup", handlers.Login())
		r.Get("/login", handlers.Login())
		r.Get("/login/callback", handlers.LoginCallback(handler.Elastic))
		r.Get("/logout", handlers.Logout())
	})

	// Handle incoming webhook requests from Stripe.
	router.With(middlewares.SetupElastic()).Post("/checkout/webhooks", stripe.HandleWebhook(handler.Elastic))

	// Authenticated routes.
	router.Group(func(r chi.Router) {
		r.Use(
			middlewares.SetupHTMX,
			middlewares.SetupElastic(),
			session.Manager.LoadAndSave,
			middlewares.RequireUserAuth(handler.DataAPI()),
			// middleware.NoCache,
		)
		// Payment routes (Stripe).
		r.Route("/checkout", func(r chi.Router) {
			r.Get("/choose-plan", handlers.UserChooseSubscriptionPlan())
			r.Post("/", handlers.UserSubscriptionPlanCheckout())
			r.Get("/success", handlers.UserAccountSuccess())
			r.Get("/cancel", handlers.Landing())
		})
		r.Get("/home", handler.Home())
		r.Get("/home/updates", handlers.WatchHome(handler.Elastic))
		r.With(middlewares.RequireHTMX).Post("/search/suggestions", handlers.GetSearchSuggestions(handler.Elastic))
		r.With(middlewares.RequireHTMX).Post("/search", handlers.GetSearchResults(handler.Elastic))
		r.With(middlewares.RequireHTMX).Post("/search/paginate", handlers.GetSearchResults(handler.Elastic))
		r.With(middlewares.RequireHTMX).
			Post("/search/subscription/suggestions", handlers.GetSubscriptionFilterSuggestions(handler.Elastic))
		r.With(middlewares.RequireHTMX).Post("/search/subscription", handlers.AddSubscriptionFilter())
		r.Get("/search", handlers.GetSearchResults(handler.Elastic))
		r.Get("/search/updates", handlers.WatchSearchResults(handler.Elastic))

		// Objects.
		r.Get("/view/{object}/{id}", handlers.ViewObject(handler.Elastic))
		r.With(middlewares.RequireHTMX).Get("/view/{object}/{id}/similar", handlers.FindSimilar(handler.Elastic))
		// r.With(middlewares.RequireHTMX).Get("/view/{object}/{id}/share", handlers.ShareObject(handler.Elastic))
		r.With(middlewares.RequireHTMX).Get("/issue/{object}/{id}", handlers.GetObjectIssues())
		r.With(middlewares.RequireHTMX).Post("/issue/{object}/{id}", handlers.SubmitObjectIssues())
		// Subscription specific.
		r.Route("/list/subscriptions", func(r chi.Router) {
			r.Get("/", handlers.ListSubscriptions(handler.Elastic))
			r.With(middlewares.RequireHTMX).Post("/", handlers.ListSubscriptions(handler.Elastic))
			r.With(middlewares.RequireHTMX).Post("/paginate", handlers.PaginateSubscriptions(handler.Elastic))
			r.With(middlewares.RequireHTMX).Post("/mark/{mark}", handlers.MarkSubscriptions(handler.Elastic))
			r.Get("/updates", handlers.WatchList(handler.Elastic))
		})
		r.With(middlewares.RequireHTMX).
			Post("/mark/subscription/{subscription_id}/{mark}", handlers.MarkSubscription(handler.Elastic))
		r.With(middlewares.RequireHTMX).
			Post("/remove/subscription/{subscription_id}", handlers.RemoveSubscription(handler.Elastic))
		r.With(middlewares.RequireHTMX).
			Delete("/remove/subscription/{subscription_id}", handlers.RemoveSubscription(handler.Elastic))
		r.Route("/edit/subscription/{subscription_id}", func(r chi.Router) {
			r.Get("/", handlers.EditSubscription(handler.Elastic))
			r.With(middlewares.RequireHTMX).Post("/", handlers.SaveSubscription(handler.Elastic))
			r.With(middlewares.RequireHTMX).Post("/category", handlers.AdjustSubscriptionCategories())
			r.With(middlewares.RequireHTMX).Delete("/category", handlers.AdjustSubscriptionCategories())
		})
		// Article specific.
		r.Route("/list/articles", func(r chi.Router) {
			r.Get("/", handlers.ListArticles(handler.Elastic))
			r.With(middlewares.RequireHTMX).Post("/", handlers.ListArticles(handler.Elastic))
			r.With(middlewares.RequireHTMX).Post("/paginate", handlers.PaginateArticles(handler.Elastic))
			r.With(middlewares.RequireHTMX).Post("/mark/{mark}", handlers.MarkArticles(handler.Elastic))
			r.Get("/updates", handlers.WatchList(handler.Elastic))
		})
		r.With(middlewares.RequireHTMX).Post("/mark/article/{item_id}/{mark}", handlers.MarkArticle(handler.Elastic))
		// General.
		r.With(middlewares.RequireHTMX).Get("/issue", handlers.GetPageIssues())
		r.With(middlewares.RequireHTMX).Post("/issue", handlers.SubmitPageIssues())
		// Favorite specific.
		r.Route("/list/favorites", func(r chi.Router) {
			r.Get("/", handlers.ListFavorites(handler.Elastic))
		})

		// User routes.
		r.Route("/user", func(r chi.Router) {
			r.Get("/account-issue", handlers.UserAccountIssue())
			// Subscription.
			r.Route("/subscription", func(r chi.Router) {
				// Add feed subscription.
				r.Get("/add/feed", handlers.AddFeedSubscription(handler.Elastic))
				r.With(middlewares.RequireHTMX).Post("/add/feed", handlers.AddFeedSubscription(handler.Elastic))
				// Add search subscription.
				r.Get("/add/search", handlers.AddSearchSubscription(handler.Elastic))
				r.With(middlewares.RequireHTMX).Post("/add/search", handlers.AddSearchSubscription(handler.Elastic))
				// Add group subscription.
				r.Get("/add/group", handlers.AddGroupSubscription(handler.Elastic))
				r.With(middlewares.RequireHTMX).Post("/add/group", handlers.AddGroupSubscription(handler.Elastic))
				// Category management for add/edit subscription.
				r.With(middlewares.RequireHTMX).Post("/category", handlers.AdjustSubscriptionCategories())
				r.With(middlewares.RequireHTMX).Delete("/category", handlers.AdjustSubscriptionCategories())
			})
			r.Post("/feedset", handlers.AddFeedset(handler.Elastic, web.StaticContent))
			// Import/export.
			r.Get("/import", handler.ImportSubscriptions())
			r.With(middlewares.RequireHTMX).Post("/import", handler.ImportSubscriptions())
			r.Get("/export", handler.ExportSubscriptions())
			r.Get("/export/opml", handler.ExportSubscriptions())
			// Favorites.
			r.Route("/favorite", func(r chi.Router) {
				r.Post("/add/subscription/{subscription_id}", handler.AddFavoriteSubscription())
				r.Post("/remove/subscription/{subscription_id}", handler.RemoveFavoriteSubscription())
				r.Post("/add/article/{item_id}", handler.AddFavoriteArticle())
				r.Post("/remove/article/{item_id}", handler.RemoveFavoriteArticle())
				// r.Post("/add/search", handler.AddFavoriteSearch())
				// r.Post("/remove/search", handler.RemoveFavoriteSearch())
			})
			// Settings.
			r.Route("/settings", func(r chi.Router) {
				r.Get("/", handler.ShowSettings())
				r.With(middlewares.RequireHTMX).Get("/display", handlers.ShowDisplaySettings())
				r.With(middlewares.RequireHTMX).Post("/display", handlers.SaveDisplaySettings(handler.Elastic))
				r.With(middlewares.RequireHTMX).Get("/account", handlers.ShowAccountSettings())
				r.With(middlewares.RequireHTMX).Post("/account", handlers.SaveAccountSettings(handler.Elastic))
				r.Get("/subscription", handlers.UserManageAccountSubscription())
				r.With(middlewares.RequireHTMX).Post("/password", handlers.ChangePassword())
				r.Route("/theme", func(r chi.Router) {
					r.With(middlewares.RequireHTMX).Put("/{theme}", handler.SetTheme())
				})
			})
			r.With(middlewares.RequireHTMX).Get("/delete", handlers.DeleteUser(handler.Elastic))
			r.With(middlewares.RequireHTMX).Post("/delete", handlers.DeleteUser(handler.Elastic))
		})
	})

	return router
}
