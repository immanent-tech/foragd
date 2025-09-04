// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	gowebly "github.com/gowebly/helpers"
	slogchi "github.com/samber/slog-chi"

	"github.com/immanent-tech/go-feed-me/config"
	"github.com/immanent-tech/go-feed-me/server/handlers"
	"github.com/immanent-tech/go-feed-me/server/middlewares"
	"github.com/immanent-tech/go-feed-me/server/session"
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
	*http.Server

	static embed.FS
}

// NewServer sets up a new server.
func NewServer(ctx context.Context, static embed.FS) (Server, error) {
	var svr Server
	svr.static = static
	// Load the server config.
	err := config.Load(serverConfigPrefix, serverConfigEnvPrefix, ServerConfig)
	if err != nil {
		return svr, fmt.Errorf("unable to load server config: %w", err)
	}
	// If no secret is set, create a new secret.
	if ServerConfig.Secret == "" {
		secret, err := randomBase16String(32)
		if err != nil {
			return svr, fmt.Errorf("unable to generate server secret: %w", err)
		}

		ServerConfig.Secret = secret
	}
	// Set up handlers api.
	api, err := handlers.SetupAPI(ctx)
	if err != nil {
		return svr, fmt.Errorf("unable to set up handlers api: %w", err)
	}
	// Set up routes.
	router := setupRoutes(api, static)

	svr.Server = &http.Server{
		Handler:           router,
		Addr:              fmt.Sprintf(":%d", ServerConfig.Port),
		ReadTimeout:       0,
		WriteTimeout:      0,
		IdleTimeout:       0,
		ReadHeaderTimeout: ServerReadTimeout,
	}

	return svr, nil
}

func setupRoutes(handler *handlers.API, static embed.FS) *chi.Mux {
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
		r.Handle("/static/*", gowebly.StaticFileServerHandler(http.FS(static)))
	})
	// Error handling.
	router.NotFound(handlers.NotFound())

	// Front page.
	router.Get("/", handlers.Landing())
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
		r.Get("/signup", handler.ShowSignup())
		r.With(middlewares.RequireHTMX).Post("/signup", handler.ProcessSignup())
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
		r.With(middlewares.RequireHTMX).Get("/search/suggestions", handler.GetSearchSuggestions())
		r.Get("/search", handler.GetSearchResults())
		// Subscription routes.
		r.Route("/subscriptions", func(r chi.Router) {
			r.Get("/", handler.GetSubscriptions())
			r.With(middlewares.RequireHTMX).Post("/", handler.PaginateSubscriptions())
			r.With(middlewares.RequireHTMX).Post("/mark/{mark}", handler.MarkAllSubscriptions())
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
			r.With(middlewares.RequireHTMX).Post("/", handler.PaginateArticles())
			r.Get("/updates", handler.GetArticleUpdates())
		})
		r.Route("/subscription/{subscription}/article/{item}", func(r chi.Router) {
			r.Get("/", handler.ViewArticle())
			r.With(middlewares.RequireHTMX).Post("/mark/{mark}", handler.MarkArticle())
			r.With(middlewares.RequireHTMX).Post("/similar", handler.FindSimilarArticles())
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
				r.With(middlewares.RequireHTMX).Put("/edit/{subscription}", handler.SaveSubscription())
				// Remove subscription (unsubscribe).
				r.Get("/remove/{subscription}", handler.RemoveSubscription())
				r.With(middlewares.RequireHTMX).Post("/remove/{subscription}", handler.RemoveSubscription())
				// Category management for add/edit subscription.
				r.With(middlewares.RequireHTMX).Post("/category", handler.AdjustSubscriptionCategories())
				r.With(middlewares.RequireHTMX).Delete("/category", handler.AdjustSubscriptionCategories())
			})
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
