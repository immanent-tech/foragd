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
	"github.com/immanent-tech/foragd/providers/github"
	"github.com/immanent-tech/foragd/server/handlers"
	"github.com/immanent-tech/foragd/server/middlewares"
	"github.com/immanent-tech/foragd/server/session"
	"github.com/immanent-tech/foragd/web"
)

// Server represents the application server. It contains the underlying server object, the handlers, and embedded FS
// for static content.
type Server struct {
	*http.Server

	apis        *APIs
	environment string
}

type APIs struct {
	auth   *auth0.Authenticator
	github *github.Client
}

// NewServer sets up a new server.
func NewServer(ctx context.Context, env string) (Server, error) {
	svr := Server{
		environment: env,
		apis:        &APIs{},
	}
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
	// Set up auth0 api.
	authapi, err := auth0.New(ctx)
	if err != nil {
		return svr, fmt.Errorf("unable start server: %w", err)
	}
	svr.apis.auth = authapi
	// Set up github api.
	ghapi, err := github.NewClient(ctx)
	if err != nil {
		return svr, fmt.Errorf("unable start server: %w", err)
	}
	svr.apis.github = ghapi
	// Set up handlers api.
	api, err := svr.setupAPI(ctx)
	if err != nil {
		return svr, fmt.Errorf("unable to set up handlers api: %w", err)
	}
	// Set up routes.
	router := svr.setupRoutes(api)

	csrfRouter := nosurf.New(router)
	csrfRouter.SetFailureHandler(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		params := map[string]string{}
		for i, k := range chi.RouteContext(req.Context()).URLParams.Keys {
			params[k] = chi.RouteContext(req.Context()).URLParams.Values[i]
		}
		slogctx.FromCtx(req.Context()).Error("CSRF check failed",
			slog.String("method", req.Method),
			slog.String("host", req.Host),
			slog.String("path", req.URL.Path),
			slog.String("query", req.URL.RawQuery),
			slog.Any("params", params),
			slog.String("route", chi.RouteContext(req.Context()).RoutePattern()),
			slog.String("ip", req.RemoteAddr),
			slog.String("referer", req.Referer()),
			slog.String(slogchi.RequestIDKey, middleware.GetReqID(req.Context())),
		)
		res.WriteHeader(http.StatusBadRequest)
	}))

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

func (s *Server) setupRoutes(handler *handlers.API) *chi.Mux {
	rl := middlewares.NewRateLimiter()
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
				slogchi.IgnorePathContains("/content", "/favicon"),
			},
		}),
		middlewares.SetupCORS(s.environment),
		middlewares.SetupCSP(),
		middlewares.Etag,
		middleware.StripSlashes,
		middlewares.SaveCSRFToken,
		middleware.NoCache,
		middlewares.RateLimit(rl, s.environment),
	)

	// Routes.
	//
	// Liveness probe.
	router.HandleFunc("/livenessProbe", func(res http.ResponseWriter, req *http.Request) {
		fmt.Fprint(res, "I'm alive!")
	})

	// Static content.
	router.Group(func(r chi.Router) {
		r.Handle("/content/*", handlers.StaticFileServerHandler(http.FS(web.StaticContent)))
	})

	router.Get("/img-proxy/*", handlers.ImageProxy())

	// Error handling.
	router.NotFound(handlers.NotFound())

	// Front page.
	router.Get("/", handlers.Landing())
	// Privacy policy.
	router.Get("/policies/privacy", handlers.Document(web.StaticContent, "content/docs/privacy.md"))
	// Terms of Service.
	router.Get("/tos", handlers.Document(web.StaticContent, "content/docs/tos.md"))
	// Access routes.
	router.Group(func(r chi.Router) {
		r.Use(
			middlewares.SetupElastic(),
			session.Manager.LoadAndSave,
		)
		r.Get("/login/{provider}", handlers.Login(s.apis.auth))
		r.Get("/login/{provider}/callback", handlers.LoginCallback(s.apis.auth, handler.Elastic))
		r.Get("/logout", handlers.Logout(s.apis.auth))
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
		r.With(middlewares.RequireHTMX).Post("/search/filter/subscription", handlers.AddSubscriptionFilter())

		// Lists.
		r.Route("/list", func(r chi.Router) {
			r.Get("/{list}", handlers.ShowList(handler.Elastic))
			r.With(middlewares.RequireHTMX).Post("/{list}/paginate", handlers.ShowList(handler.Elastic))
			r.With(middlewares.RequireHTMX).Post("/{list}/mark", handlers.MarkList(handler.Elastic))
			r.Get("/updates", handlers.WatchList(handler.Elastic))
		})
		// Objects.
		r.Get("/view/{object}/{id}", handlers.ViewObject(handler.Elastic))
		r.With(middlewares.RequireHTMX).Post("/mark/{object}/{id}/{mark}", handlers.MarkObject(handler.Elastic))
		r.With(middlewares.RequireHTMX).Get("/view/{object}/{id}/similar", handlers.FindSimilar(handler.Elastic))
		r.With(middlewares.RequireHTMX).Get("/issue/{object}/{id}", handlers.GetObjectIssues(handler.Elastic))
		r.With(middlewares.RequireHTMX).Post("/issue/{object}/{id}", handlers.SubmitObjectIssues(handler.Elastic, s.apis.github))
		r.With(middlewares.RequireHTMX).Get("/remove/{object}/{id}", handlers.ConfirmRemoveObject(handler.Elastic))
		r.With(middlewares.RequireHTMX).Post("/remove/{object}/{id}", handlers.RemoveObject(handler.Elastic))
		// Subscription specific.
		r.Route("/edit/subscription/{id}", func(r chi.Router) {
			r.Get("/", handler.EditSubscription())
			r.With(middlewares.RequireHTMX).Post("/", handler.SaveSubscription())
			r.With(middlewares.RequireHTMX).Post("/category", handlers.AdjustSubscriptionCategories())
			r.With(middlewares.RequireHTMX).Delete("/category", handlers.AdjustSubscriptionCategories())
		})
		// General.
		r.With(middlewares.RequireHTMX).Get("/issue", handlers.GetPageIssues(handler))
		r.With(middlewares.RequireHTMX).Post("/issue", handlers.SubmitPageIssues(handler, s.apis.github))

		// User routes.
		r.Route("/user", func(r chi.Router) {
			// Subscription.
			r.Route("/subscription", func(r chi.Router) {
				// Add subscription.
				r.Get("/add", handler.AddSubscription())
				r.With(middlewares.RequireHTMX).Post("/add", handler.AddSubscription())
				// Edit subscription.
				r.Get("/edit/{subscription_id}", handler.EditSubscription())
				r.With(middlewares.RequireHTMX).Post("/edit/{subscription_id}", handler.SaveSubscription())
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
				r.Post("/add/search", handler.AddFavoriteSearch())
				r.Post("/remove/search", handler.RemoveFavoriteSearch())
			})
			// Settings.
			r.Route("/settings", func(r chi.Router) {
				r.Get("/", handler.GetSettings())
				r.With(middlewares.RequireHTMX).Post("/", handlers.SaveSettings(handler.Elastic))
				// r.Get("/subscriptions", handler.SubscriptionsSettings())
				r.With(middlewares.RequireHTMX).Post("/subscriptions", handler.GetSubscriptionsSettings())
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
