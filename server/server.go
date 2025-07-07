// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	gowebly "github.com/gowebly/helpers"
	slogchi "github.com/samber/slog-chi"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/components/auth"
	"github.com/joshuar/go-feed-me/components/config"
	"github.com/joshuar/go-feed-me/components/session"
	"github.com/joshuar/go-feed-me/providers/auth0"
	"github.com/joshuar/go-feed-me/providers/elastic"
	"github.com/joshuar/go-feed-me/server/handlers"
	"github.com/joshuar/go-feed-me/server/middlewares"
)

const (
	// ServerReadTimeout is the default read timeout for the server.
	ServerReadTimeout = 5 * time.Second
	// ServerWriteTimeout is the default write timeout for the server.
	ServerWriteTimeout = 10 * time.Second
)

const (
	RequestIDKey = "request_id"
)

type API struct {
	user    *auth0.UserAPI
	elastic *elastic.API
	auth    *auth.Authenticator
}

type Server struct {
	API    *API
	Log    *slog.Logger
	server *http.Server
	static embed.FS
}

// Ensures we statisfy the ServerInterface interface.
// var _ ServerInterface = (*Server)(nil)

var ErrStartServer = errors.New("start server failed")

func NewServer(ctx context.Context, static embed.FS) (Server, error) {
	var svr Server
	svr.static = static
	// Load the server config.
	if err := config.Load(serverConfigPrefix, serverConfigEnvPrefix, ServerConfig); err != nil {
		return svr, fmt.Errorf("unable to load server config: %w", err)
	}
	// If no secret is set, create a new secret.
	if ServerConfig.Secret == "" {
		secret, err := randomBase16String(32)
		if err != nil {
			return svr, fmt.Errorf("%w: %w", ErrLoadConfig, err)
		}

		ServerConfig.Secret = secret
	}
	// Set up the logger
	svr.Log = slogctx.FromCtx(ctx)
	// Load the auth0UserAPI backend.
	auth0UserAPI, err := auth0.NewUserAPI(ctx)
	if err != nil {
		return svr, errors.Join(ErrStartServer, err)
	}
	// Load the Elastic backend
	elasticAPI, err := elastic.Connect(ctx)
	if err != nil {
		return svr, errors.Join(ErrStartServer, err)
	}
	// Set up the session manager.
	if err := session.NewSessionManager(ctx, elasticAPI, auth.SessionName); err != nil {
		return svr, errors.Join(ErrStartServer, err)
	}
	// Set up authentication manager.
	authAPI, err := auth.NewAuthenticator(ctx)
	if err != nil {
		return svr, errors.Join(ErrStartServer, err)
	}

	// Add the API to the environment.
	svr.API = &API{
		user:    auth0UserAPI,
		elastic: elasticAPI,
		auth:    authAPI,
	}

	svr.setupRoutes()

	return svr, nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *Server) ListenAndServeTLS(cert, key string) error {
	return s.server.ListenAndServeTLS(cert, key)
}

func (s *Server) setupRoutes() {
	// Set up a new chi router.
	router := chi.NewRouter()
	router.Use(
		middleware.RequestID,
		middleware.Recoverer,
		slogchi.NewWithConfig(slog.Default(), slogchi.Config{WithRequestID: true}),
		middlewares.SetupCORS(config.Environment()),
		// middlewares.CSP(server.ServerConfig.CSP),
		middlewares.Etag,
		middleware.StripSlashes,
	)

	// Routes.
	//
	// Static content.
	router.Group(func(r chi.Router) {
		r.Handle("/static/*", gowebly.StaticFileServerHandler(http.FS(s.static)))
	})
	// Front page.
	router.Get("/", handlers.Index())
	// Access routes.
	router.Group(func(r chi.Router) {
		r.Use(
			session.Manager.LoadAndSave,
		)
		r.Get("/login/{provider}", handlers.Login(s.AuthAPI()))
		r.Get("/login/{provider}/callback", handlers.LoginCallback(s.AuthAPI()))
		r.Get("/logout", handlers.Logout())
	})

	// Authenticated routes.
	router.Group(func(r chi.Router) {
		r.Use(
			middlewares.SetupHTMX,
			middlewares.SetupElastic(),
			session.Manager.LoadAndSave,
			handlers.RequireUserAuth(s.DataAPI(), s.AuthAPI()),
		)
		r.Get("/home", handlers.Home(s.DataAPI()))
		r.Post("/search", handlers.GetSearchSuggestions(s.DataAPI()))
		// Subscription routes.
		r.Get("/subscriptions", handlers.GetSubscriptions(s.DataAPI()))
		r.With(middlewares.RequireHTMX).Post("/subscriptions", handlers.GetSubscriptions(s.DataAPI()))
		r.With(middlewares.RequireHTMX).Post("/subscriptions/mark/{mark}", handlers.MarkSubscriptions(s.DataAPI()))
		r.With(middlewares.RequireHTMX).Post("/subscriptions/remove", handlers.RemoveSubscriptions(s.DataAPI()))
		// Article routes.
		r.Get("/articles", handlers.GetArticles(s.DataAPI()))
		r.With(middlewares.RequireHTMX).Post("/articles", handlers.PaginateArticles(s.DataAPI()))
		r.With(middlewares.RequireHTMX).Post("/articles/mark/{mark}", handlers.MarkArticles(s.DataAPI()))
		// Article route.
		r.Get("/view/{subscription}/{item}", handlers.ViewArticle(s.DataAPI()))
		// User routes.
		r.Route("/user", func(r chi.Router) {
			r.Route("/subscription", func(r chi.Router) {
				r.Get("/new", handlers.NewSubscription())
				r.Get("/edit/{subscription}", handlers.EditSubscription(s.DataAPI()))
				r.Put("/edit/{subscription}", handlers.SaveSubscription(s.DataAPI()))
			})
			r.Route("/settings", func(r chi.Router) {
				r.Get("/", handlers.GetSettings())
				r.Route("/theme", func(r chi.Router) {
					r.With(middlewares.RequireHTMX).Put("/{theme}", handlers.SetTheme(s.DataAPI()))
				})
			})
		})
	})

	s.server = &http.Server{
		Handler:           router,
		Addr:              fmt.Sprintf(":%d", Port()),
		ReadHeaderTimeout: ServerReadTimeout,
	}
}
