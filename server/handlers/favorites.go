// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	slogctx "github.com/veqryn/slog-context"
	"golang.org/x/sync/errgroup"

	"github.com/immanent-tech/go-base/pkg/htmx"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/service"
	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/element"
)

type Favorites struct {
	title    templates.PageTitle
	template templ.Component
}

// FullResponse renders a full page (headers, footers and list of subscriptions).
func (p *Favorites) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(
		templates.CreatePage(p.template,
			templates.WithPageTitle(p.title),
		)).ServeHTTP(res, req)
}

// PartialResponse will either render the list of subscriptions, the controls and update the title/dock/sidebar or, when
// paginating, just the list of subscriptions.
func (p *Favorites) PartialResponse(res http.ResponseWriter, req *http.Request) {
	res.Header().Set(htmx.HeaderPushURL, req.URL.String())
	templ.Handler(p.template, templ.WithFragments(templates.ContentFragment)).ServeHTTP(res, req)
	templ.Handler(templates.UpdateTitle(p.title)).ServeHTTP(res, req)
	templ.Handler(templates.SideBar(element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
	templ.Handler(templates.Dock(element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
}

// HandleListFavorites handles fetching the favorite subscriptions and articles of a user and showing them in a grid layout.
func HandleListFavorites() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		var (
			articles              models.Articles
			favoriteSubscriptions models.Subscriptions
		)

		user := models.UserFromCtx(req.Context())
		if user == nil {
			slogctx.FromCtx(req.Context()).Debug("Get user data failed.",
				slog.Any("error", models.ErrCtxValueNotFound))
			http.Redirect(res, req, "/login", http.StatusSeeOther)
			return
		}

		wg, jobCtx := errgroup.WithContext(req.Context())
		defer jobCtx.Done()

		// Get favorite articles.
		wg.Go(func() error {
			if len(user.ItemFavorites) > 0 {
				var err error
				articles, err = service.GetArticles(jobCtx, user.ItemFavorites...)
				if err != nil {
					return fmt.Errorf("get articles: %w", err)
				}
			}
			return nil
		})

		// Get favorite subscriptions.
		wg.Go(func() error {
			var err error
			allSubscriptions := models.SubscriptionsFromCtx(req.Context())
			if allSubscriptions == nil {
				return fmt.Errorf("get all subscriptions: %w", err)
			}
			favoriteSubscriptions = allSubscriptions.FilterByView(models.ViewFavorites)
			// Update subscription dynamic info.
			if err = service.UpdateSubscriptionDynamicInfo(req.Context(), favoriteSubscriptions); err != nil {
				return fmt.Errorf("update subscription dynamic info: %w", err)
			}
			service.GetLatestArticles(req.Context(), models.ViewAll, favoriteSubscriptions)
			return nil
		})

		if err := wg.Wait(); err != nil {
			HandleInternalError(
				http.StatusInternalServerError,
				fmt.Errorf("run data collection: %w", err),
			).ServeHTTP(res, req)
			return
		}

		// Render appropriate content.
		response := &models.ListFavoritesResponse{
			Subscriptions: favoriteSubscriptions,
			Articles:      articles,
		}
		page := &Favorites{
			title: templates.PageTitle{
				Summary:     "Favorites",
				Description: "All favorited Subscriptions and Articles",
			},
			template: templates.ListFavorites(response),
		}

		RenderInternalPage(page).ServeHTTP(res, req)
	}
}
