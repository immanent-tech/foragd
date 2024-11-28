// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/knadh/koanf/v2"
	"github.com/lxzan/gws"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/platforms/auth0"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic"
	"github.com/joshuar/go-feed-me/internal/platforms/postgres"
	"github.com/joshuar/go-feed-me/internal/server/middlewares"
	"github.com/joshuar/go-feed-me/internal/server/session"
)

const (
	requestIDKey = "request_id"

	envPrefix          = "GOFEEDME_"
	defaultServerPort  = 7000
	defaultEnvironment = EnvDevelopment
)

var (
	ConfigPath  = "./"
	ConfigFile  = "server.toml"
	ConfigPerms = 0o600
)

type API struct {
	user      *auth0.UserAPI
	elastic   *elastic.Client
	pg        *postgres.Client
	auth      *auth0.Authenticator
	websocket *gws.Upgrader
}

type Server struct {
	Config *koanf.Koanf
	API    *API
	Logger *slog.Logger
}

func NewServer(ctx context.Context) (Server, error) {
	var (
		err error
		svr Server
	)

	// Load the config.
	err = svr.loadConfig()
	if err != nil {
		return svr, fmt.Errorf("unable to load server config: %w", err)
	}

	// Set up the logger
	if loglevel := svr.Config.String("server.loglevel"); loglevel != "" {
		svr.Logger = logging.NewLogger(loglevel)
	} else {
		svr.Logger = logging.NewLogger("debug")
	}
	// Embed the logger into the context
	ctx = logging.ToContext(ctx, svr.Logger)

	// Embed the config into the context
	// ctx = config.ToContext(ctx, cfg)

	// Load the auth0UserAPI backend.
	auth0UserAPI, err := auth0.NewUserAPI(ctx, svr.Config)
	if err != nil {
		return svr, fmt.Errorf("failed to initialize the auth0 user backend API: %w", err)
	}

	// Load the Elastic backend
	elasticAPI, err := elastic.Connect(ctx, svr.Config)
	if err != nil {
		return svr, fmt.Errorf("failed to connect to the db backend: %w", err)
	}

	// Load the Postgres backend
	postgresAPI, err := postgres.Connect(ctx, svr.Config)
	if err != nil {
		return svr, fmt.Errorf("failed to connect to the db backend: %w", err)
	}

	// Load the authenticator backend.
	auth0API, err := auth0.NewAuthenticator(ctx, svr.Config)
	if err != nil {
		return svr, fmt.Errorf("failed to initialize the authenticator backend API: %w", err)
	}

	// websocket := handlers.NewWebsocketServer(&handlers.FeedItemWebsocketHandler{})

	// Set up the session manager.
	session.NewSessionManager(postgresAPI)

	// Add the API to the environment.
	svr.API = &API{
		user:    auth0UserAPI,
		elastic: elasticAPI,
		pg:      postgresAPI,
		auth:    auth0API,
		// websocket: websocket,
	}

	return svr, nil
}

func GenerateHandler(svr Server, router chi.Router) http.Handler {
	wrapper := ServerInterfaceWrapper{
		Handler: svr,
	}

	// Use chi middlewares.
	svr.Logger.Debug("Setting up middleware...")
	wrapper.HandlerMiddlewares = append(wrapper.HandlerMiddlewares,
		middleware.RequestID,
		middleware.RealIP,
		middlewares.Logger(svr.Logger, svr.GetLogLevel(), requestIDKey),
		middleware.Recoverer,
		middlewares.CORS(svr.GetEnvironment()),
		middlewares.CSP(svr.CSP()),
		session.LoadAndSave())

	svr.Logger.Debug("Setting up routes...")

	if svr.GetEnvironment() == "development" {
		router.Mount("/debug", middleware.Profiler())
	}

	// Login/Logout routes.
	router.Get("/", wrapper.GetIndex)
	router.Route("/login", func(loginRouter chi.Router) {
		loginRouter.Get("/{provider}", wrapper.GetLogin)
		loginRouter.Get("/{provider}/callback", wrapper.GetLoginCallback)
	})
	router.Get("/logout/{provider}", wrapper.GetLogout)

	// Sign up routes.
	router.Route("/signup", func(signupRouter chi.Router) {
		signupRouter.Get("/", wrapper.GetSignup)
		signupRouter.Post("/", wrapper.PostSignup)
		signupRouter.Post("/validate", wrapper.PostSignupValidate)
	})

	// /home routes.
	router.Route("/home", func(homeRouter chi.Router) {
		homeRouter.Get("/", wrapper.GetHome)
		homeRouter.Post("/search", wrapper.PostHomeSearch)
		homeRouter.Get("/settings", wrapper.GetHomeSettings)
	})
	// /subscription routes.
	router.Route("/subscription", func(r chi.Router) {
		r.Get("/add", wrapper.GetSubscriptionAdd)
		r.Post("/add", wrapper.PostSubscriptionAdd)
		r.Get("/edit", wrapper.GetSubscriptionEdit)
		r.Post("/edit", wrapper.PostSubscriptionEdit)
		r.Post("/validate", wrapper.PostSubscriptionValidate)
	})
	// /feed routes
	router.Route("/feed", func(r chi.Router) {
		r.Get("/{feedID}", wrapper.GetHomeFeed)
		r.Get("/{feedID}/item/{itemID}", wrapper.GetHomeFeedItem)
	})

	return router
}
