// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"golang.org/x/sync/errgroup"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/service"
	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/element"
)

type Favorites struct {
	template templ.Component
}

// FullResponse renders a full page (headers, footers and list of subscriptions).
func (p *Favorites) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(
		templates.CreatePage(p.template,
			templates.WithPageTitle("Favorites"),
		)).ServeHTTP(res, req)
}

// PartialResponse will either render the list of subscriptions, the controls and update the title/dock/sidebar or, when
// paginating, just the list of subscriptions.
func (p *Favorites) PartialResponse(res http.ResponseWriter, req *http.Request) {
	res.Header().Set(htmx.HeaderPushURL, req.URL.String())
	templ.Handler(p.template, templ.WithFragments(templates.ContentFragment)).ServeHTTP(res, req)
	templ.Handler(templates.UpdateTitle("Favorites")).ServeHTTP(res, req)
	templ.Handler(templates.SideBar(element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
	templ.Handler(templates.Dock(element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
}

// HandleListFavorites handles fetching the favorite subscriptions and articles of a user and showing them in a grid layout.
func HandleListFavorites() http.HandlerFunc {
	return userContentHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		var (
			articles      models.Articles
			subscriptions models.Subscriptions
			err           error
		)

		user := models.UserFromCtx(req.Context())
		if user == nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("get user data: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Could not list favorites",
					"This might be temporary, please try again.",
				),
			}).ServeHTTP(res, req)
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
					return fmt.Errorf("list favorites: get favorite articles: %w", err)
				}
			}
			return nil
		})

		// Get favorite subscriptions.
		wg.Go(func() error {
			var err error
			subscriptions, err = service.GetAllSubscriptions(jobCtx, user)
			if err != nil && !errors.Is(err, models.ErrNotFound) {
				return fmt.Errorf("list favorites: get favorite subscriptions: %w", err)
			}
			subscriptions = subscriptions.FilterByFavorites(true)
			return nil
		})

		if err := wg.Wait(); err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("run data collection: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Could not list favorites",
					"This might be temporary, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}

		// Render appropriate content.
		response := &models.ListFavoritesResponse{
			Subscriptions: subscriptions,
			Articles:      articles,
		}
		page := &Favorites{
			template: templates.ListFavorites(response),
		}

		RenderInternalPage(page).ServeHTTP(res, req)
	}).
		ServeHTTP
}
