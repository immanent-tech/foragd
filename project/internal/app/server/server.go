// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/joshuar/go-feed-me/internal/app/server/middlewares"
	"github.com/joshuar/go-feed-me/internal/app/server/session"
	"github.com/joshuar/go-feed-me/internal/config"
	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/platforms/auth0"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic"
	store "github.com/joshuar/go-feed-me/internal/platforms/elastic/implementations/session"
)

const (
	requestIDKey = "request_id"
)

type API struct {
	user    *auth0.UserAPI
	elastic *elastic.Client
	auth    *auth0.Authenticator
}

type Server struct {
	API    *API
	Logger *slog.Logger
}

// Ensures we statisfy the ServerInterface interface.
var _ ServerInterface = (*Server)(nil)

var ErrStartServer = errors.New("start server failed")

func NewServer(ctx context.Context) (Server, error) {
	var svr Server
	// Load the server config.
	if err := config.Load(serverConfigPrefix, serverConfigEnvPrefix, serverConfig); err != nil {
		return svr, fmt.Errorf("unable to load server config: %w", err)
	}
	// If no secret is set, create a new secret.
	if serverConfig.Secret == "" {
		secret, err := randomBase16String(32)
		if err != nil {
			return svr, fmt.Errorf("%w: %w", ErrLoadConfig, err)
		}

		serverConfig.Secret = secret
	}
	// Set up the logger
	svr.Logger = logging.FromContext(ctx)
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
	// Load the authenticator backend.
	auth0API, err := auth0.NewAuthenticator(ctx, "http://localhost:"+strconv.Itoa(serverConfig.Port))
	if err != nil {
		return svr, errors.Join(ErrStartServer, err)
	}
	// Load the session store.
	sessionStore, err := store.NewSessionStore(ctx, elasticAPI)
	if err != nil {
		return svr, errors.Join(ErrStartServer, err)
	}

	// Set up the session manager.
	session.NewSessionManager(sessionStore)
	// Add the API to the environment.
	svr.API = &API{
		user:    auth0UserAPI,
		elastic: elasticAPI,
		auth:    auth0API,
	}

	return svr, nil
}

var (
	htmxOnlyRoutes  = []string{"/home/settings", "/subscription"}
	protectedRoutes = []string{"/home"}
)

func GenerateHandler(svr Server, router chi.Router) http.Handler {
	wrapper := ServerInterfaceWrapper{
		Handler: svr,
	}

	// Use chi middlewares.
	svr.Logger.Debug("Setting up middleware...")
	wrapper.HandlerMiddlewares = append(wrapper.HandlerMiddlewares,
		middleware.RequestID,
		middleware.RealIP,
		middlewares.Logger(svr.Logger, config.LogLevel(), requestIDKey),
		middleware.Recoverer,
		middlewares.CORS(config.Environment()),
		middlewares.CSP(serverConfig.CSP),
		middlewares.RequireAuthentication(protectedRoutes, svr.API.elastic),
		middlewares.RequireHTMX(htmxOnlyRoutes),
		session.LoadAndSave())

	svr.Logger.Debug("Setting up routes...")

	if config.Environment() == "development" {
		router.Mount("/debug", middleware.Profiler())
	}

	// Login/Logout routes.
	router.Get("/", wrapper.Index)
	router.Route("/login", func(loginRouter chi.Router) {
		loginRouter.Get("/{provider}", wrapper.Login)
		loginRouter.Get("/{provider}/callback", wrapper.LoginCallback)
	})
	router.Get("/logout/{provider}", wrapper.Logout)

	// Sign up routes.
	router.Route("/signup", func(signupRouter chi.Router) {
		signupRouter.Get("/", wrapper.Signup)
		signupRouter.Post("/", wrapper.ProcessSignup)
	})

	router.Post("/search", wrapper.Search)

	// /subscription routes.

	// /home navigation
	router.Route("/home", func(homeRouter chi.Router) {
		homeRouter.Get("/show/{list}", wrapper.ShowList)
		homeRouter.Post("/mark/{list}/{action}", wrapper.MarkList)
		homeRouter.Get("/show/article/{feed}/{item}", wrapper.ShowArticle)
		homeRouter.Post("/mark/article/{action}/{feed}/{item}", wrapper.MarkArticle)
		homeRouter.Get("/settings", wrapper.GetHomeSettings)
		homeRouter.Route("/subscription", func(subscriptionRouter chi.Router) {
			subscriptionRouter.Get("/add", wrapper.AddSubscription)
			subscriptionRouter.Post("/add", wrapper.ProcessAddSubscription)
			subscriptionRouter.Get("/edit", wrapper.GetSubscriptionEdit)
			subscriptionRouter.Post("/edit", wrapper.PostSubscriptionEdit)
		})
	})

	return router
}
