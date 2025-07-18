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

// Server represents the application server. It contains the underlying server object, the handlers, and embedded FS
// for static content.
type Server struct {
	server *http.Server
	static embed.FS
	api    *handlers.API
}

var ErrStartServer = errors.New("start server failed")

// NewServer sets up a new server.
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

//nolint:funlen // it doesn't make sense to split up handler definitions.
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
	// Error handling.
	router.NotFound(handlers.NotFound())
	router.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte("method is not valid"))
	})

	// Front page.
	router.Get("/", handlers.Index())
	// Access routes.
	router.Get("/login", handlers.LoginSelect())
	router.Group(func(r chi.Router) {
		r.Use(
			session.Manager.LoadAndSave,
		)
		r.Get("/login/{provider}", handler.Login())
		r.Get("/login/{provider}/callback", handler.LoginCallback())
		r.Get("/logout", handlers.Logout())
	})
	// Signup routes.
	router.Group(func(r chi.Router) {
		r.Use(
			middlewares.SetupElastic(),
		)
		r.Get("/signup", handlers.SignupSetup())
		r.Post("/signup/{provider}", handler.Signup())
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
		r.With(middlewares.RequireHTMX).Post("/search", handler.GetSearchSuggestions())
		r.With(middlewares.RequireHTMX).Post("/search/results", handler.GetSearchResults())
		// Subscription routes.
		r.Get("/subscriptions", handler.GetSubscriptions())
		r.With(middlewares.RequireHTMX).Post("/subscriptions", handler.GetSubscriptions())
		r.With(middlewares.RequireHTMX).Post("/subscriptions/mark/{mark}", handler.MarkSubscriptions())
		r.With(middlewares.RequireHTMX).Post("/subscriptions/remove", handler.RemoveSubscriptions())
		// Article routes.
		r.Get("/articles", handler.GetArticles())
		r.With(middlewares.RequireHTMX).Post("/articles", handler.GetArticles())
		r.With(middlewares.RequireHTMX).Post("/articles/mark/{mark}", handler.MarkArticles())
		// Article route.
		r.Get("/view/{subscription}/{item}", handler.ViewArticle())
		// User routes.
		r.Route("/user", func(r chi.Router) {
			// Subscription.
			r.Route("/subscription", func(r chi.Router) {
				r.Get("/new", handlers.NewSubscription())
				r.Post("/new", handler.AddSubscription())
				r.Get("/edit/{subscription}", handler.EditSubscription())
				r.Put("/edit/{subscription}", handler.SaveSubscription())
			})
			// Import/export.
			r.Get("/import", handler.ImportSubscriptions())
			r.With(middlewares.RequireHTMX).Put("/import", handler.ImportSubscriptions())
			r.With(middlewares.RequireHTMX).Post("/import", handler.ImportSubscriptions())
			// Favourites.
			r.Route("/favourite", func(r chi.Router) {
				r.Put("/subscription/{subscription}", handler.AddFavouriteSubscription())
				r.Delete("/subscription/{subscription}", handler.RemoveFavouriteSubscription())
			})
			// Settings.
			r.Route("/settings", func(r chi.Router) {
				r.Get("/", handler.GetSettings())
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
