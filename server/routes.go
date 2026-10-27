/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/immanent-tech/go-base/config"
	"github.com/immanent-tech/go-base/pkg/htmx"
	"github.com/immanent-tech/go-base/server/handlers/assets"
	"github.com/immanent-tech/go-base/server/handlers/redirect"
	"github.com/immanent-tech/go-base/server/middlewares/etag"
	"github.com/immanent-tech/go-base/server/middlewares/security"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/google/android"
	"github.com/immanent-tech/foragd/server/cache"
	"github.com/immanent-tech/foragd/server/handlers"
	"github.com/immanent-tech/foragd/server/imgproxy"
	"github.com/immanent-tech/foragd/server/middlewares"
	"github.com/immanent-tech/foragd/service"
	"github.com/immanent-tech/foragd/web"
)

//nolint:funlen
func buildRouter(services *services, appCfg *config.AppConfig) *chi.Mux {
	handlerMgr := handlers.Manager{
		AppConfig:   appCfg,
		SessionMgr:  services.session,
		Breadcrumbs: services.breadcrumbs,
	}

	// Set up a new chi router.
	router := chi.NewRouter()

	// Health check endpoints (for GCP).
	router.Use(readiness)
	router.Use(middleware.Heartbeat("/health-check"))

	// Standard middleware stack.
	router.Use(
		middleware.RequestID,
		middlewares.Logger,
		middlewares.Recoverer,
		middleware.StripSlashes,
		security.SetupCORS,
		security.CrossOriginProtection,
		security.ContentSecurityPolicy,
		security.GeneralSecurity(
			[]string{"camera=()", "microphone=()", "geolocation=self", "usb=()", "bluetooth=()", "web-share=self"},
		),
		security.PreventCSRF(
			security.WithCSRFInsecureBypassPattern("/checkout/webhooks"),
			security.WithCSRFInsecureBypassPattern("/mail/webhooks"),
		),
		// middlewares.RateLimit,
		etag.Etag,
		middlewares.SetClient,
		middlewares.Otel,
		middleware.Compress(cfg.CompressionLevel, cfg.CompressionMimetypes...),
		services.session.LoadAndSave,
		htmx.SetupHTMX,
	)

	// Error handling.
	router.NotFound(handlerMgr.HandleNotFound())
	// sitemap.xml.
	router.Handle("/sitemap.xml", handlerMgr.HandleSitemap())
	// Static content.
	router.Handle("/robots.txt", assets.ServeFiles(web.Files, "files"))
	router.Handle("/manifest.json", assets.ServeFiles(web.Files, "files"))
	router.Handle("/favicon.ico", assets.ServeFiles(web.Files, "files/images"))
	router.Handle("/.well-known/*", assets.ServeFiles(web.Files, "files/.well-known"))
	router.Handle("/assets/*", assets.ServeFiles(web.Files, ""))
	router.Handle("/files/assets/*", assets.ServeFiles(web.Files, "files/dist/assets"))
	router.Handle("/files/misc/*", assets.ServeFiles(web.Files, "files/misc"))
	router.Handle("/images/*", assets.ServeFiles(web.Files, "files/images"))

	// Image proxy.
	router.Get("/img-proxy/*", imgproxy.HandleImage(services.imageCache))
	router.Get("/img/{imgType}/*", cache.HandleImage(services.imageCache))

	// Handle incoming webhooks from Resend
	router.Post("/mail/webhooks", handlers.HandleResendWebhook(services.emailVerifier, services.emailReceiver))
	// Handle incoming webhooks from Paddle.
	router.Post("/webhooks/paddle", handlers.HandlePaddleWebhook(services.users, services.emailSender))
	// Handle incoming Google Play Real Time Developer Notifications.
	router.Post("/webhooks/googleplay", android.HandleRTDN(appCfg, services.users))

	// External Pages.
	router.Group(func(r chi.Router) {
		// Landing and features.
		r.Get("/", handlerMgr.HandleLanding())
		r.Route("/features", func(r chi.Router) {
			r.Get("/", handlerMgr.HandleFeatures())
			r.Get("/collect", handlerMgr.HandleFeaturesCollect())
			r.Get("/curate", handlerMgr.HandleFeaturesCurate())
			r.Get("/consume", handlerMgr.HandleFeaturesConsume())
		})
		// Comparison pages.
		r.Get("/compare/{service}", handlerMgr.HandleComparison())
		// About.
		r.Get("/about", handlerMgr.HandleAbout())
		// Contact.
		r.Route("/contact", func(r chi.Router) {
			r.Get("/", handlerMgr.HandleContact())
			r.With(htmx.RequireHTMX).Post("/", handlerMgr.HandleSubmitContact(services.emailSender))
		})
		r.Route("/forget-me", func(r chi.Router) {
			r.Get("/", handlerMgr.HandleForgetMe())
			r.With(htmx.RequireHTMX).Post("/", handlerMgr.HandleSubmitContact(services.emailSender))
		})
		// Feed Viewer.
		r.Route("/viewer", func(r chi.Router) {
			r.Get("/", handlerMgr.HandleViewer(services.feeds))
			r.Get("/url/*", handlerMgr.HandleViewer(services.feeds))
			r.With(htmx.RequireHTMX).Post("/", handlerMgr.HandleViewer(services.feeds))
		})
		// Feed Linter.
		r.Route("/linter", func(r chi.Router) {
			r.Get("/", handlerMgr.HandleLinter(services.httpClient))
			r.With(htmx.RequireHTMX).Post("/", handlerMgr.HandleLinter(services.httpClient))
		})
		// Help documentation.
		r.Get("/docs", handlerMgr.DocumentationHandler("/docs"))
		r.Get("/help", handlerMgr.DocumentationHandler("help"))
		// Policy documentation (i.e., terms of service, privacy).
		r.Get("/policies/*", handlerMgr.PolicyDocsHandler())
		// Blog.
		r.Route("/blog", func(r chi.Router) {
			r.Get("/", handlerMgr.HandlePosts())
			r.Get("/*", handlerMgr.HandlePosts())
		})
		// Changelog.
		r.Route("/changelog", func(r chi.Router) {
			r.Get("/", handlerMgr.HandleChangelog())
			r.Get("/feed", handlerMgr.HandleChangelogFeed())
		})
		// Posts RSS feed.
		r.Get("/feed", handlerMgr.HandlePostsFeed())
		// Sign-up/Login routes.
		r.Group(func(r chi.Router) {
			r.Get("/signup", handlerMgr.HandleLogin(services.auth))
			r.Route("/login", func(r chi.Router) {
				r.Get("/", handlerMgr.HandleLogin(services.auth))
				r.Get(
					"/callback",
					handlerMgr.HandleLoginCallback(
						services.users,
						services.authMgr,
						services.auth,
						services.emailSender,
					),
				)
				r.Get("/error", handlerMgr.HandleLoginError)
			})
			r.Get("/logout", handlerMgr.HandleLogout(services.auth))
			r.Get("/account-issue", handlerMgr.HandleAccountIssue())
		})
		// Web payment routes.
		r.Group(func(r chi.Router) {
			r.Use(
				middlewares.ExtractUserFromSession(services.users, services.auth, services.session),
			)
			r.Route("/checkout", func(r chi.Router) {
				r.Get("/", handlerMgr.HandleChooseSubscription())
				r.Post("/", handlerMgr.HandlePurchaseSubscription(services.users))
				r.Get("/success", handlerMgr.HandlePurchaseSubscriptionSuccess())
			})
		})
		// User routes that don't required authentication.
		r.Route("/unsubscribe/{token}", func(r chi.Router) {
			r.Get("/", handlerMgr.HandleUserUnsubscribe(services.users))
			r.Post("/", handlerMgr.HandleUserUnsubscribe(services.users))
		})
	})

	// Authenticated routes.
	router.Group(func(r chi.Router) {
		r.Use(
			services.breadcrumbs.Recorder,
			middlewares.ExtractUserFromSession(services.users, services.auth, services.session),
			middlewares.RequireValidUser,
			handlerMgr.ValidateSubscriptionLimits(services.users, services.subscriptions, services.emailSender),
			middlewares.NoCache,
			handlerMgr.CustomisationCtx,
		)
		// Manual login refresh.
		r.Get("/login/refresh", handlerMgr.HandleRefreshToken(services.auth))
		r.With(handlerMgr.AllSubscriptionsCtx(services.subscriptions)).
			Get("/home", handlerMgr.HandleHome(&service.Home{}))
		// Searching.
		r.Route("/search", func(r chi.Router) {
			r.Use(middlewares.CanonicalizeSearchParams(services.session))
			r.Use(handlerMgr.AllSubscriptionsCtx(services.subscriptions))
			r.Get("/", handlerMgr.HandleSearchResults(services.items))
			r.With(htmx.RequireHTMX).
				Post("/suggestions", handlerMgr.HandleSearchSuggestions(services.subscriptions, services.items))
			r.With(htmx.RequireHTMX).Post("/paginate", handlerMgr.HandleSearchResults(services.items))
			r.With(htmx.RequireHTMX).
				Post("/subscription/suggestions", handlerMgr.GetSubscriptionFilterSuggestions(services.subscriptions))
			r.With(htmx.RequireHTMX).Post("/subscription", handlerMgr.AddSubscriptionFilter())
			r.Post("/updates", handlerMgr.HandleSearchUpdates(services.items))
		})
		r.Route("/action", func(r chi.Router) {
			r.With(htmx.RequireHTMX).
				Post("/subscription/suggestions", handlerMgr.GetSubscriptionActionSuggestions(services.subscriptions))
		})
		r.Route("/discover", func(r chi.Router) {
			r.Use(handlerMgr.CheckUserLimits)
			r.Get("/", handlerMgr.HandleDiscover())
			r.With(htmx.RequireHTMX).Post("/suggest", handlerMgr.HandleDiscoverSuggestions(services.feeds))
		})
		// Subscription specific.
		r.Route("/subscriptions", func(r chi.Router) {
			r.Use(middlewares.CanonicalizeListFilters(services.session))
			r.Use(handlerMgr.AllSubscriptionsCtx(services.subscriptions))
			r.Get("/", handlerMgr.HandleListSubscriptions(services.subscriptions))
			r.Group(func(r chi.Router) {
				r.Use(htmx.RequireHTMX)
				r.Post("/categories", handlerMgr.ListCategories(services.subscriptions, services.items))
				r.Post("/languages", handlerMgr.ListLanguages(services.subscriptions, services.items))
				r.Post("/updates", handlerMgr.HandleListSubscriptionsUpdates(services.items))
				r.Post("/paginate", handlerMgr.HandleListSubscriptions(services.subscriptions))
				r.Post("/read", handlerMgr.HandleBulkMarkSubscriptions(services.subscriptions, models.MarkRead))
				r.Post(
					"/unread",
					handlerMgr.HandleBulkMarkSubscriptions(services.subscriptions, models.MarkUnread),
				)
				r.Post("/remove", handlerMgr.HandleBulkRemoveSubscriptions(services.subscriptions))
			})
			r.Route("/add", func(r chi.Router) {
				r.Get("/", handlerMgr.HandleAddSubscription())
				r.Group(func(r chi.Router) {
					r.Use(htmx.RequireHTMX)
					r.Post("/suggestions", handlerMgr.HandleSuggestFeeds(services.feeds))
					r.Post(
						"/feed",
						handlerMgr.HandleAddNewFeedSubscription(services.subscriptions, services.users, services.feeds),
					)
					// Add search subscription.
					r.Get("/search", handlerMgr.HandleAddSearchSubscription(services.subscriptions, services.users))
					r.Post("/search", handlerMgr.HandleAddSearchSubscription(services.subscriptions, services.users))
					// Add group subscription.
					r.Get("/group", handlerMgr.HandleAddGroupSubscription(services.subscriptions, services.users))
					r.Post("/group", handlerMgr.HandleAddGroupSubscription(services.subscriptions, services.users))
					r.Post("/feedset", handlerMgr.HandleAddFeedset(services.feeds, services.subscriptions))
					r.Post("/category", handlerMgr.HandleSubscriptionCategories())
				})
			})
			r.Route("/group", func(r chi.Router) {
				r.With(htmx.RequireHTMX).Post("/add", handlerMgr.HandleAddSubscriptionToGroup())
			})
			r.Route("/{subscriptionID}", func(r chi.Router) {
				r.Use(handlerMgr.SubscriptionCtx(services.subscriptions))
				// 	// r.Get("/", handleSubscriptionDetail) // its own article feed: ?sort=&status=&page=
				r.Route("/edit", func(r chi.Router) {
					r.Get("/", handlerMgr.HandleEditSubscription(services.subscriptions))
					r.Post("/", handlerMgr.HandleSaveSubscription(services.imageCache, services.subscriptions))
				})
				r.Group(func(r chi.Router) {
					r.Use(htmx.RequireHTMX)
					r.Post(
						"/read",
						handlerMgr.HandleMarkSubscription(
							services.subscriptions,
							models.MarkRead,
						),
					)
					r.Post(
						"/unread",
						handlerMgr.HandleMarkSubscription(
							services.subscriptions,
							models.MarkUnread,
						),
					)
					r.Post("/favorite", handlerMgr.HandleFavoriteSubscription(services.subscriptions))
					r.Get("/remove", handlerMgr.HandleRemoveSubscription(services.subscriptions))
				})
			})
		})
		r.Route("/subscription", func(r chi.Router) {
			r.Use(handlerMgr.AllSubscriptionsCtx(services.subscriptions))
			// Group subscription management.
			// Search subscription management.
			r.Route("/search", func(r chi.Router) {
				r.With(htmx.RequireHTMX).
					Post("/suggest", handlerMgr.HandleSuggestSubscriptionForSearch(services.subscriptions))
				r.With(htmx.RequireHTMX).Post("/add", handlerMgr.HandleAddSubscriptionToSearch())
			})
		})

		// Article specific.
		r.Route("/articles", func(r chi.Router) {
			r.Use(middlewares.CanonicalizeListFilters(services.session))
			r.Use(handlerMgr.AllSubscriptionsCtx(services.subscriptions))
			r.Get("/", handlerMgr.HandleListArticles(services.items))
			r.Group(func(r chi.Router) {
				r.Use(htmx.RequireHTMX)
				r.Post("/categories", handlerMgr.ListCategories(services.subscriptions, services.items))
				r.Post("/languages", handlerMgr.ListLanguages(services.subscriptions, services.items))
				r.Post("/updates", handlerMgr.HandleListArticlesUpdates(services.items))
				r.Post("/paginate", handlerMgr.HandleListArticles(services.items))
				r.Post("/read", handlerMgr.HandleBulkMarkArticles(services.subscriptions, models.MarkRead))
				r.Post("/unread", handlerMgr.HandleBulkMarkArticles(services.subscriptions, models.MarkRead))
			})
			r.Route("/{articleID}", func(r chi.Router) {
				r.Use(handlerMgr.ArticleCtx(services.items))
				r.Get(
					"/",
					handlerMgr.HandleViewArticle(services.items),
				)
				r.Get("/similar", handlerMgr.HandleFindSimilarArticles(services.items))
				r.Group(func(r chi.Router) {
					r.Use(htmx.RequireHTMX)
					r.Get("/next", handlerMgr.HandleBrowseArticles(services.subscriptions, services.items, "next"))
					r.Get("/prev", handlerMgr.HandleBrowseArticles(services.subscriptions, services.items, "prev"))
					r.Post("/read", handlerMgr.HandleMarkArticle(services.subscriptions, models.MarkRead))
					r.Post("/unread", handlerMgr.HandleMarkArticle(services.subscriptions, models.MarkUnread))
					r.Post("/favorite", handlerMgr.HandleFavoriteArticle(services.items, services.users))
					r.Post("/share", handlerMgr.HandleShareArticle())
				})
			})
		})
		// Favorites.
		r.Route("/favorites", func(r chi.Router) {
			r.Use(handlerMgr.AllSubscriptionsCtx(services.subscriptions))
			r.Get("/", handlerMgr.HandleListFavorites(services.subscriptions, services.items))
		})
		// Map
		r.Route("/map", func(r chi.Router) {
			r.Use(middlewares.CanonicalizeListFilters(services.session))
			r.Use(handlerMgr.AllSubscriptionsCtx(services.subscriptions))
			r.Get("/", handlerMgr.HandleMap(services.items))
			r.With(htmx.RequireHTMX).Post("/updates", handlerMgr.HandleMapUpdates(services.items))
		})
		// Issues.
		r.Route("/issue", func(r chi.Router) {
			r.Get("/", handlerMgr.HandleReportIssue())
			r.With(htmx.RequireHTMX).Post("/", handlerMgr.HandleSubmitIssue(services.imageCache, services.emailSender))
		})
		// Help/Documentation.
		r.Get("/docs", handlerMgr.DocumentationHandler("/docs"))
		// Settings.
		r.Route("/settings", func(r chi.Router) {
			r.Get("/", handlerMgr.ShowSettings())
			r.Route("/display", func(r chi.Router) {
				r.Use(htmx.RequireHTMX)
				r.Get("/", handlerMgr.HandleShowDisplaySettings())
				r.Post("/", handlerMgr.HandleSaveDisplaySettings(services.users))
				r.Post("/font", handlerMgr.HandleSaveFontSettings(services.users))
				r.Post("/theme", handlerMgr.HandleSaveThemeSettings(services.users))
			})
			r.Route("/account", func(r chi.Router) {
				r.Use(htmx.RequireHTMX)
				r.Get("/", handlerMgr.HandleShowAccountSettings())
				r.Post("/", handlerMgr.HandleSaveAccountSettings(services.users, services.authMgr, services.imageCache))
				r.Post("/password", handlerMgr.HandleChangePassword(services.authMgr))
				r.Post("/newsletters", handlerMgr.HandleGenerateSubscriptionEmail(services.users))
				r.Post(
					"/deactivate",
					handlerMgr.HandleDeactivateAccount(
						services.users,
						services.authMgr,
						services.auth,
						services.emailSender,
					),
				)
			})
			r.Route("/subscriptions", func(r chi.Router) {
				r.Use(htmx.RequireHTMX)
				r.Use(handlerMgr.AllSubscriptionsCtx(services.subscriptions))
				r.Get("/", handlerMgr.HandleShowSubscriptionsSettings())
				r.Post("/", handlerMgr.HandleSaveSubscriptionsSettings(services.users))
			})
			// TODO: check if still used?
			r.Get("/subscription", handlerMgr.HandleManageAccountSubscription())
		})
		// Import.
		r.Route("/import", func(r chi.Router) {
			r.Get("/", handlerMgr.HandleSetupImport(services.subscriptions, services.users, services.emailSender))
			r.With(htmx.RequireHTMX).Post("/", handlerMgr.HandleStartImport(services.imports))
			r.Route("/status", func(r chi.Router) {
				r.Use(handlerMgr.AllSubscriptionsCtx(services.subscriptions))
				r.Get("/", handlerMgr.HandleImportStatus(services.imports, services.subscriptions))
				r.Get("/{jobID}", handlerMgr.HandleImportStatus(services.imports, services.subscriptions))
			})
		})
		// Export.
		r.Route("/export", func(r chi.Router) {
			r.Use(handlerMgr.AllSubscriptionsCtx(services.subscriptions))
			r.Get("/", handlerMgr.HandleExportSubscriptions(services.feeds))
			r.Post("/", handlerMgr.HandleExportSubscriptions(services.feeds))
		})

		// Moved routes.
		addLegacyRoutes(r)
	})

	return router
}

func addLegacyRoutes(r chi.Router) {
	r.Get("/list/favorites", redirect.To("/favorites", http.StatusMovedPermanently))
	r.Get("/list/subscriptions", redirect.To("/subscriptions", http.StatusMovedPermanently))
	r.Get("/list/articles", redirect.To("/articles", http.StatusMovedPermanently))
	r.Get("/posts", redirect.To("/blog", http.StatusMovedPermanently))
	r.Get("/posts/*", redirect.Param("*", "blog/%s", http.StatusMovedPermanently))
	r.Get("/view/article/{item_id}", redirect.Param("item_id", "/articles/%s", http.StatusMovedPermanently))
	r.Get("/user/import", redirect.To("/import", http.StatusMovedPermanently))
	r.Get("/user/export", redirect.To("/export", http.StatusMovedPermanently))
	r.Get("/user/settings", redirect.To("/settings", http.StatusMovedPermanently))
	r.Get("/subscription/add", redirect.To("/subscriptions/add", http.StatusMovedPermanently))
	r.Get("/feedly", redirect.To("/compare/feedly", http.StatusMovedPermanently))
	r.Get("/inoreader", redirect.To("/compare/inoreader", http.StatusMovedPermanently))
	r.Get("/innoreader", redirect.To("/compare/inoreader", http.StatusMovedPermanently))
	r.Get("/newsblur", redirect.To("/compare/newsblur", http.StatusMovedPermanently))
	r.Get("/freshrss", redirect.To("/compare/freshrss", http.StatusMovedPermanently))
	r.Get("/security.txt", redirect.To("/.well-known/security.txt", http.StatusMovedPermanently))
}
