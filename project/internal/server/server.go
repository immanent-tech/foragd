// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
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
)

type API struct {
	user      *auth0.UserAPI
	elastic   *elastic.Client
	pg        *postgres.Client
	auth      *auth0.Authenticator
	websocket *gws.Upgrader
}

type Server struct {
	API    *API
	Logger *slog.Logger
}

func NewServer(ctx context.Context) (Server, error) {
	var svr Server

	// Load the config.
	if err := loadConfig(); err != nil {
		return svr, fmt.Errorf("unable to load server config: %w", err)
	}

	// Set up the logger
	svr.Logger = logging.NewLogger(config.LogLevel)
	// Embed the logger into the context
	ctx = logging.ToContext(ctx, svr.Logger)

	// Embed the config into the context
	// ctx = config.ToContext(ctx, cfg)

	// Load the auth0UserAPI backend.
	auth0UserAPI, err := auth0.NewUserAPI(ctx)
	if err != nil {
		return svr, fmt.Errorf("failed to initialize the auth0 user backend API: %w", err)
	}

	// Load the Elastic backend
	elasticAPI, err := elastic.Connect(ctx, config.Environment)
	if err != nil {
		return svr, fmt.Errorf("failed to connect to the db backend: %w", err)
	}

	// Load the Postgres backend
	postgresAPI, err := postgres.Connect(ctx)
	if err != nil {
		return svr, fmt.Errorf("failed to connect to the db backend: %w", err)
	}

	// Load the authenticator backend.
	auth0API, err := auth0.NewAuthenticator(ctx, "http://localhost:"+strconv.Itoa(config.Port))
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

var (
	htmxOnlyRoutes  = []string{"/home/settings", "/signup", "/subscription"}
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
		middlewares.Logger(svr.Logger, config.LogLevel, requestIDKey),
		middleware.Recoverer,
		middlewares.CORS(config.Environment),
		middlewares.CSP(config.CSP),
		middlewares.RequireAuthentication(protectedRoutes, svr.API.pg),
		middlewares.RequireHTMX(htmxOnlyRoutes),
		session.LoadAndSave())

	svr.Logger.Debug("Setting up routes...")

	if config.Environment == "development" {
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

	router.Post("/search", wrapper.Search)

	// /subscription routes.
	router.Route("/subscription", func(subscriptionRouter chi.Router) {
		subscriptionRouter.Get("/add", wrapper.GetSubscriptionAdd)
		subscriptionRouter.Post("/add", wrapper.PostSubscriptionAdd)
		subscriptionRouter.Get("/edit", wrapper.GetSubscriptionEdit)
		subscriptionRouter.Post("/edit", wrapper.PostSubscriptionEdit)
		subscriptionRouter.Post("/validate", wrapper.PostSubscriptionValidate)
	})

	// /home navigation
	router.Route("/home", func(homeRouter chi.Router) {
		homeRouter.Get("/show/{list}", wrapper.ShowList)
		homeRouter.Post("/mark/{list}/{action}", wrapper.MarkList)
		homeRouter.Get("/show/article/{feed}/{item}", wrapper.ShowArticle)
		homeRouter.Post("/mark/article/{action}/{feed}/{item}", wrapper.MarkArticle)
		homeRouter.Get("/settings", wrapper.GetHomeSettings)
	})

	return router
}
