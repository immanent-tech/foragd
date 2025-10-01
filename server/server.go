// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/justinas/nosurf"
	slogchi "github.com/samber/slog-chi"
	slogctx "github.com/veqryn/slog-context"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/providers/auth0"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/server/handlers"
	"github.com/immanent-tech/foragd/server/middlewares"
	"github.com/immanent-tech/foragd/server/session"

	"github.com/didip/tollbooth/v8"
	"github.com/realclientip/realclientip-go"
)

// Server represents the application server. It contains the underlying server object, the handlers, and embedded FS
// for static content.
type Server struct {
	*http.Server

	static      embed.FS
	authAPI     *auth0.Authenticator
	environment string
}

// NewServer sets up a new server.
func NewServer(ctx context.Context, static embed.FS, env string) (Server, error) {
	var svr Server
	svr.static = static
	svr.environment = env
	// Load the server config.
	err := config.Load(serverConfigPrefix, serverConfigEnvPrefix, cfg)
	if err != nil {
		return svr, fmt.Errorf("unable to load server config: %w", err)
	}
	// If no secret is set, create a new secret.
	if cfg.Secret == "" {
		secret, err := randomBase16String(32)
		if err != nil {
			return svr, fmt.Errorf("unable to generate server secret: %w", err)
		}

		cfg.Secret = secret
	}
	// Set up authenticator
	authapi, err := auth0.New(ctx)
	if err != nil {
		return svr, fmt.Errorf("unable start server: %w", err)
	}
	svr.authAPI = authapi
	// Set up handlers api.
	api, err := svr.setupAPI(ctx)
	if err != nil {
		return svr, fmt.Errorf("unable to set up handlers api: %w", err)
	}
	// Set up routes.
	router := svr.setupRoutes(api, static)

	h2s := &http2.Server{}
	svr.Server = &http.Server{
		Handler:     h2c.NewHandler(nosurf.New(router), h2s),
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
	slogctx.FromCtx(ctx).Info("Starting server...",
		slog.String("address", s.Addr),
		slog.String("environment", s.environment))

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
			slog.Error("Error occurred when trying to shut down server.",
				slog.Any("error", err),
			)
		}
	}()

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
	elasticAPI, err := elastic.Connect(ctx, s.environment)
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

func (s *Server) setupRoutes(handler *handlers.API, static embed.FS) *chi.Mux {
	// Set up rate-limiting.
	strat, err := realclientip.NewRightmostNonPrivateStrategy("X-Forwarded-For")
	if err != nil {
		panic("realclientip.NewRightmostNonPrivateStrategy returned error (bad input)")
	}
	lmt := tollbooth.NewLimiter(1, nil)

	// Set up a new chi router.
	router := chi.NewRouter()
	router.Use(
		middleware.RequestID,
		middleware.Recoverer,
		slogchi.NewWithConfig(slog.Default(), slogchi.Config{
			ClientErrorLevel: slog.LevelWarn,
			ServerErrorLevel: slog.LevelError,
			WithRequestID:    true,
			Filters: []slogchi.Filter{
				slogchi.IgnorePathContains("/web/content"),
			},
		}),
		middlewares.SetupCORS(s.environment),
		middlewares.SetupCSP(),
		middlewares.Etag,
		middleware.StripSlashes,
		middlewares.SaveCSRFToken,
		middleware.NoCache,
		middlewares.RateLimiter(strat, lmt, s.environment),
	)

	// Routes.
	//
	// Static content.
	router.Group(func(r chi.Router) {
		r.Handle("/web/content/*", handlers.StaticFileServerHandler(http.FS(static)))
	})

	router.Get("/img-proxy/*", handlers.ImageProxy())

	// Error handling.
	router.NotFound(handlers.NotFound())

	// Front page.
	router.Get("/", handlers.Landing())
	// Access routes.
	router.Get("/login", handlers.LoginSelect())
	router.Group(func(r chi.Router) {
		r.Use(
			middlewares.SetupElastic(),
			session.Manager.LoadAndSave,
		)
		r.Get("/login/{provider}", handlers.Login(s.authAPI))
		r.Get("/login/{provider}/callback", handlers.LoginCallback(s.authAPI, handler.Elastic))
		r.Get("/logout", handlers.Logout(s.authAPI))
	})

	// Authenticated routes.
	router.Group(func(r chi.Router) {
		r.Use(
			middlewares.SetupHTMX,
			middlewares.SetupElastic(),
			session.Manager.LoadAndSave,
			middlewares.RequireUserAuth(handler.DataAPI()),
		)
		r.Get("/home", handler.Home())
		r.With(middlewares.RequireHTMX).Post("/search/suggestions", handler.GetSearchSuggestions())
		r.With(middlewares.RequireHTMX).Post("/search", handler.GetSearchResults())
		r.Get("/search", handler.GetSearchResults())
		// Subscription routes.
		r.Route("/subscriptions", func(r chi.Router) {
			r.Get("/", handler.GetSubscriptions())
			r.With(middlewares.RequireHTMX).Put("/", handler.MarkAllSubscriptions())
			r.With(middlewares.RequireHTMX).Post("/", handler.PaginateSubscriptions())
			r.Get("/updates", handler.GetSubscriptionUpdates())
		})
		// Subscription route.
		r.Route("/subscription/{subscription}", func(r chi.Router) {
			// r.Get("/", handler.GetSubscriptionArticles())
			r.With(middlewares.RequireHTMX).Post("/mark/{mark}", handler.MarkSubscription())
		})
		// Article routes.
		r.Route("/articles", func(r chi.Router) {
			r.Get("/", handler.GetArticles())
			r.With(middlewares.RequireHTMX).Put("/", handler.MarkAllArticles())
			r.With(middlewares.RequireHTMX).Post("/", handler.PaginateArticles())
			r.Get("/updates", handler.GetArticleUpdates())
		})
		r.Route("/subscription/{subscription}/article/{item}", func(r chi.Router) {
			r.Get("/", handler.ViewArticle())
			r.With(middlewares.RequireHTMX).Post("/", handler.ViewArticle())
			r.With(middlewares.RequireHTMX).Post("/mark/{mark}", handler.MarkArticle())
			r.Get("/similar", handler.FindSimilarArticles())
		})
		// User routes.
		r.Route("/user", func(r chi.Router) {
			// Subscription.
			r.Route("/subscription", func(r chi.Router) {
				// Add subscription.
				r.Get("/add", handler.AddSubscription())
				r.With(middlewares.RequireHTMX).Post("/add", handler.AddSubscription())
				// Edit subscription.
				r.Get("/edit/{subscription}", handler.EditSubscription())
				r.With(middlewares.RequireHTMX).Post("/edit/{subscription}", handler.SaveSubscription())
				// Remove subscription (unsubscribe).
				r.Get("/remove/{subscription}", handler.GetRemoveSubscriptionConfirmation())
				r.With(middlewares.RequireHTMX).Post("/remove/{subscription}", handler.ProcessRemoveSubscription())
				// Category management for add/edit subscription.
				r.With(middlewares.RequireHTMX).Post("/category", handler.AdjustSubscriptionCategories())
				r.With(middlewares.RequireHTMX).Delete("/category", handler.AdjustSubscriptionCategories())
			})
			r.Post("/feedset", handlers.AddFeedset(handler.Elastic, static))
			// Import/export.
			r.Get("/import", handler.ImportSubscriptions())
			r.With(middlewares.RequireHTMX).Post("/import", handler.ImportSubscriptions())
			r.Get("/export", handler.ExportSubscriptions())
			r.Get("/export/opml", handler.ExportSubscriptions())
			// Favorites.
			r.Route("/favorite", func(r chi.Router) {
				r.Put("/subscription/{subscription}", handler.AddFavoriteSubscription())
				r.Delete("/subscription/{subscription}", handler.RemoveFavoriteSubscription())
				r.Put("/article/{item}", handler.AddFavoriteArticle())
				r.Delete("/article/{item}", handler.RemoveFavoriteArticle())
				r.Put("/search", handler.AddFavoriteSearch())
				r.Delete("/search", handler.RemoveFavoriteSearch())
			})
			// Settings.
			r.Route("/settings", func(r chi.Router) {
				r.Get("/", handler.GetSettings())
				// r.Get("/subscriptions", handler.SubscriptionsSettings())
				r.With(middlewares.RequireHTMX).Post("/subscriptions", handler.SubscriptionsSettings())
				// r.Get("/account", handler.AccountSettings())
				// r.With(middlewares.RequireHTMX).Post("/account", handler.AccountSettings())
				// r.Get("/app", handler.AppSettings())
				r.Route("/theme", func(r chi.Router) {
					r.With(middlewares.RequireHTMX).Put("/{theme}", handler.SetTheme())
				})
			})
			r.With(middlewares.RequireHTMX).Get("/delete", handler.DeleteUser())
		})
	})

	return router
}
