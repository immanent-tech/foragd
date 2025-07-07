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

	"github.com/joshuar/go-feed-me/config"
	"github.com/joshuar/go-feed-me/server/handlers"
	"github.com/joshuar/go-feed-me/server/middlewares"
	"github.com/joshuar/go-feed-me/server/session"
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

type Server struct {
	server *http.Server
	static embed.FS
	api    *handlers.API
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

	api, err := handlers.SetupAPI(ctx)
	if err != nil {
		return svr, fmt.Errorf("%w: %w", ErrStartServer, err)
	}

	svr.setupRoutes(api)

	return svr, nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *Server) ListenAndServeTLS(cert, key string) error {
	return s.server.ListenAndServeTLS(cert, key)
}

func (s *Server) setupRoutes(handler *handlers.API) {
	// Set up a new chi router.
	router := chi.NewRouter()
	router.Use(
		middleware.RequestID,
		middleware.Recoverer,
		slogchi.NewWithConfig(slog.Default(), slogchi.Config{
			ClientErrorLevel: slog.LevelWarn,
			ServerErrorLevel: slog.LevelError,
			WithRequestID:    true,
		}),
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
		r.Get("/login/{provider}", handler.Login())
		r.Get("/login/{provider}/callback", handler.LoginCallback())
		r.Get("/logout", handlers.Logout())
	})

	// Authenticated routes.
	router.Group(func(r chi.Router) {
		r.Use(
			middlewares.SetupHTMX,
			middlewares.SetupElastic(),
			session.Manager.LoadAndSave,
			middlewares.RequireUserAuth(handler.DataAPI(), handler.AuthAPI()),
		)
		r.Get("/home", handler.Home())
		r.Post("/search", handler.GetSearchSuggestions())
		r.Post("/search/results", handler.GetSearchResults())
		// Subscription routes.
		r.Get("/subscriptions", handler.GetSubscriptions())
		r.With(middlewares.RequireHTMX).Post("/subscriptions", handler.GetSubscriptions())
		r.With(middlewares.RequireHTMX).Post("/subscriptions/mark/{mark}", handler.MarkSubscriptions())
		r.With(middlewares.RequireHTMX).Post("/subscriptions/remove", handler.RemoveSubscriptions())
		// Article routes.
		r.Get("/articles", handler.GetArticles())
		r.With(middlewares.RequireHTMX).Post("/articles", handler.PaginateArticles())
		r.With(middlewares.RequireHTMX).Post("/articles/mark/{mark}", handler.MarkArticles())
		// Article route.
		r.Get("/view/{subscription}/{item}", handler.ViewArticle())
		// User routes.
		r.Route("/user", func(r chi.Router) {
			r.Route("/subscription", func(r chi.Router) {
				r.Get("/new", handlers.NewSubscription())
				r.Get("/edit/{subscription}", handler.EditSubscription())
				r.Put("/edit/{subscription}", handler.SaveSubscription())
			})
			r.Route("/favourite", func(r chi.Router) {
				r.Put("/subscription/{subscription}", handler.AddFavouriteSubscription())
			})
			r.Route("/settings", func(r chi.Router) {
				r.Get("/", handlers.GetSettings())
				r.Route("/theme", func(r chi.Router) {
					r.With(middlewares.RequireHTMX).Put("/{theme}", handler.SetTheme())
				})
			})
		})
	})

	s.server = &http.Server{
		Handler:           router,
		Addr:              fmt.Sprintf(":%d", ServerConfig.Port),
		ReadHeaderTimeout: ServerReadTimeout,
	}
}
