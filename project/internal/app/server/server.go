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
	protectedRoutes = []string{"/home", "/subscription"}
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
		middlewares.ElasticMiddleware(),
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
	router.Route("/user", func(signupRouter chi.Router) {
		signupRouter.Get("/new", wrapper.NewUser)
		signupRouter.Put("/new", wrapper.AddUser)
	})

	router.Post("/search", wrapper.Search)

	// /subscription routes.

	// /home navigation
	router.Route("/home", func(homeRouter chi.Router) {
		homeRouter.Get("/", SetCommonHomeFilters(wrapper.HandleHome))
		// Feeds:
		homeRouter.Get("/feeds", SetCommonHomeFilters(wrapper.HandleShowFeeds))
		homeRouter.Post("/feeds", SetCommonHomeFilters(wrapper.HandleMarkFeeds))
		// Items:
		homeRouter.Get("/items", SetCommonHomeFilters(wrapper.HandleShowItems))
		homeRouter.Post("/items", SetCommonHomeFilters(wrapper.HandleMarkItems))
		// Item:
		homeRouter.Get("/{feed}/{item}", SetCommonHomeFilters(wrapper.HandleShowItem))
		homeRouter.Post("/{feed}/{item}/{mark}", SetCommonHomeFilters(wrapper.HandleMarkItem))
		homeRouter.Put("/{feed}/{item}", SetCommonHomeFilters(wrapper.HandleSaveItem))
		homeRouter.Delete("/{feed}/{item}", SetCommonHomeFilters(wrapper.HandleUnsaveItem))
	})

	router.Route("/subscription", func(subscription chi.Router) {
		subscription.Get("/new", wrapper.NewSubscription)
		subscription.Put("/new", wrapper.AddSubscription)
		// Existing subscription management:
		subscription.Get("/{subscription}", wrapper.ShowSubscription)
		subscription.Put("/{subscription}", wrapper.SaveSubscription)
		subscription.Delete("/{subscription}", wrapper.RemoveSubscription)
		// Subscription category management:
		subscription.Put("/category", wrapper.AddSubscriptionCategory)
		subscription.Delete("/category", wrapper.RemoveSubscriptionCategory)
	})

	return router
}
