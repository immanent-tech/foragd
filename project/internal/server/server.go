// Copyright (C) 2024 Joshua Rich <joshua.rich@gmail.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/knadh/koanf/v2"

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
	user    *auth0.UserAPI
	elastic *elastic.Client
	pg      *postgres.Client
	auth    *auth0.Authenticator
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

	// Set up the session manager.
	session.NewSessionManager(postgresAPI)

	// Add the API to the environment.
	svr.API = &API{
		user:    auth0UserAPI,
		elastic: elasticAPI,
		pg:      postgresAPI,
		auth:    auth0API,
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

	// Login/Logout routes.
	router.Get("/", wrapper.Index)
	router.Route("/login", func(r chi.Router) {
		r.Get("/{provider}", wrapper.UserLogin)
		r.Get("/{provider}/callback", wrapper.UserLoginCallback)
	})
	router.Get("/logout/{provider}", wrapper.UserLogout)

	// Sign up routes.
	router.Route("/signup", func(r chi.Router) {
		r.Get("/", wrapper.Signup)
		r.Post("/", wrapper.ProcessSignup)
		r.Post("/validate", wrapper.ValidateSignup)
	})

	// User home routes.
	router.Route("/home", func(r chi.Router) {
		r.Get("/", wrapper.UserHome)
		r.Post("/search", wrapper.UserSearch)
		r.Get("/settings", wrapper.UserSettings)
		r.Route("/add", func(r chi.Router) {
			r.Get("/", wrapper.AddItem)
			r.Post("/", wrapper.ProcessAddItem)
			r.Post("/validate", wrapper.ValidateAddItem)
		})
	})

	return router
}
