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

	"github.com/joshuar/go-feed-me/cmd/server/middlewares"
	"github.com/joshuar/go-feed-me/internal/config"
	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/platforms/auth0"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic/bulk"
	"github.com/joshuar/go-feed-me/internal/session"
	"github.com/joshuar/go-feed-me/internal/session/store"
)

const (
	requestIDKey = "request_id"
)

type DataAPI interface {
	// User methods:
	AddUser(ctx context.Context, userID models.UserID) error
	GetUser(ctx context.Context) (*models.User, error)
	// Subscription methods:
	GetSubscriptions(ctx context.Context, filters *models.Filters) (models.Subscriptions, error)
	MarkSubscriptions(ctx context.Context, mark models.Mark, feedIDs ...models.FeedID) error
	AddSubscriptions(ctx context.Context, subscriptions models.Subscriptions) error
	// Feeds methods:
	GetFeedsByURL(ctx context.Context, urls ...models.URL) (models.Feeds, error)
	AddFeeds(ctx context.Context, feeds ...*models.Feed) (*bulk.Response, error)
	// Item methods:
	GetItem(ctx context.Context, feedID models.FeedID, itemID models.ItemID) (*models.Item, bool, error)
	GetItems(ctx context.Context, filters *models.Filters) (models.Items, models.Pagination, error)
	MarkItems(ctx context.Context, mark models.Mark, itemIDs ...models.ItemID) error
}

type API struct {
	user    *auth0.UserAPI
	elastic DataAPI
	auth    *auth0.Authenticator
}

type Server struct {
	API *API
	Log *slog.Logger
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
	svr.Log = logging.FromContext(ctx)
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
	auth0API, err := auth0.NewAuthenticator(ctx, "https://localhost:"+strconv.Itoa(serverConfig.Port))
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
	svr.Log.Debug("Setting up middleware...")
	wrapper.HandlerMiddlewares = append(wrapper.HandlerMiddlewares,
		middleware.RequestID,
		middleware.RealIP,
		middlewares.Logger(svr.Log, config.LogLevel(), requestIDKey),
		middleware.Recoverer,
		middlewares.CORS(config.Environment()),
		middlewares.CSP(serverConfig.CSP),
		middlewares.ElasticMiddleware(),
		middlewares.RequireAuthentication(protectedRoutes, svr.DataAPI()),
		middlewares.RequireHTMX(htmxOnlyRoutes),
		session.LoadAndSave())

	svr.Log.Debug("Setting up routes...")

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
	router.Get("/signup", wrapper.SignUp)
	router.Post("/signup", wrapper.ProcessSignUp)

	router.Post("/search", wrapper.Search)

	// router.Group(func(r chi.Router) {
	// 	for m := range slices.Values(wrapper.HandlerMiddlewares) {
	// 		r.Use(m)
	// 	}
	// r.Use(middlewares.ElasticMiddleware())
	// r.Use(middlewares.RequireAuthentication(protectedRoutes, svr.DataAPI()))
	// r.Use(session.LoadAndSave())
	// /home navigation
	router.Route("/home", func(homeRouter chi.Router) {
		homeRouter.Get("/", wrapper.HandleHome)
		homeRouter.Get("/notifications", wrapper.HandleHomeNotifications)
		// Feeds:
		homeRouter.Get("/feeds", middlewares.CheckRequiredFilters(wrapper.HandleShowFeeds))
		homeRouter.Post("/feeds", wrapper.HandleMarkFeeds)
		// Items:
		homeRouter.Get("/items", middlewares.CheckRequiredFilters(wrapper.HandleShowItems))
		homeRouter.Post("/items", wrapper.HandleMarkItems)
		// Item:
		homeRouter.Get("/{feed}/{item}", wrapper.HandleShowItem)
		homeRouter.Post("/{feed}/{item}/{mark}", wrapper.HandleMarkItem)
		homeRouter.Put("/{feed}/{item}", wrapper.HandleSaveItem)
		homeRouter.Delete("/{feed}/{item}", wrapper.HandleUnsaveItem)
	})
	// })

	// // /home navigation
	// router.Route("/home", func(homeRouter chi.Router) {
	// 	homeRouter.Get("/", wrapper.HandleHome)
	// 	homeRouter.Get("/notifications", wrapper.HandleHomeNotifications)
	// 	// Feeds:
	// 	homeRouter.Get("/feeds", middlewares.CheckRequiredFilters(wrapper.HandleShowFeeds))
	// 	homeRouter.Post("/feeds", wrapper.HandleMarkFeeds)
	// 	// Items:
	// 	homeRouter.Get("/items", middlewares.CheckRequiredFilters(wrapper.HandleShowItems))
	// 	homeRouter.Post("/items", wrapper.HandleMarkItems)
	// 	// Item:
	// 	homeRouter.Get("/{feed}/{item}", wrapper.HandleShowItem)
	// 	homeRouter.Post("/{feed}/{item}/{mark}", wrapper.HandleMarkItem)
	// 	homeRouter.Put("/{feed}/{item}", wrapper.HandleSaveItem)
	// 	homeRouter.Delete("/{feed}/{item}", wrapper.HandleUnsaveItem)
	// })

	router.Route("/subscription", func(subscription chi.Router) {
		subscription.Get("/new", wrapper.NewSubscription)
		subscription.Put("/add", wrapper.AddSubscription)
		subscription.Get("/import", wrapper.StartImport)
		subscription.Put("/import", wrapper.SetImportMethod)
		subscription.Post("/import", wrapper.ProcessImport)
		// Existing subscription management:
		subscription.Get("/edit/{subscription}", wrapper.ShowSubscription)
		subscription.Put("/edit/{subscription}", wrapper.SaveSubscription)
		subscription.Delete("/edit/{subscription}", wrapper.RemoveSubscription)
		subscription.Put("/edit/category", wrapper.AddSubscriptionCategory)
		subscription.Delete("/edit/category", wrapper.DelSubscriptionCategory)
	})

	return router
}
