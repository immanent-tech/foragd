// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"fmt"
	"net/http"

	"github.com/a-h/templ"
	"golang.org/x/sync/errgroup"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/templates"
)

// ListFavorites handles fetching the favorite subscriptions and articles of a user and showing them in a grid layout.
func ListFavorites() http.HandlerFunc {
	return defaultHandlerChain.Append(parseFilters, setCacheControl).
		ThenFunc(showOnError(func(res http.ResponseWriter, req *http.Request) error {
			var (
				articles      models.Articles
				subscriptions models.Subscriptions
				template      templ.Component
				err           error
			)

			ctx := templates.PageTitleToCtx(req.Context(), "Favorites")

			user, err := models.UserFromCtx(ctx)
			if err != nil {
				return &models.APIError{
					InternalError: fmt.Errorf("get user data: %w", err),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Could not list favorites",
						"This might be temporary, please try again.",
					),
				}
			}

			wg, jobCtx := errgroup.WithContext(req.Context())
			defer jobCtx.Done()

			// Get favorite articles.
			wg.Go(func() error {
				if len(user.ItemFavorites) > 0 {
					var err error
					articles, err = models.GetArticles(jobCtx, user.ItemFavorites...)
					if err != nil {
						return fmt.Errorf("list favorites: get favorite articles: %w", err)
					}
				}
				return nil
			})

			// Get favorite subscriptions.
			wg.Go(func() error {
				var err error
				subscriptions, err = models.GetSubscriptions(jobCtx,
					models.GetSubscriptionsByFavorite(true),
					models.GetSubscriptionsDynamicInfo(true),
				)
				if err != nil && models.HTTPStatus(err) != http.StatusNotFound {
					return fmt.Errorf("list favorites: get favorite subscriptions: %w", err)
				}
				return nil
			})

			if err := wg.Wait(); err != nil {
				return &models.APIError{
					InternalError: fmt.Errorf("run data collection: %w", err),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Could not list favorites",
						"This might be temporary, please try again.",
					),
				}
			}

			// Render appropriate content.
			template = templates.FavoritesGrid(subscriptions, articles)

			// Choose rendering method based on method (get = page, post = partial).
			switch req.Method {
			case http.MethodGet:
				renderPage(wrapContent(req.WithContext(ctx), template)).ServeHTTP(res, req.WithContext(ctx))
			case http.MethodPost:
				renderPartial(template).ServeHTTP(res, req.WithContext(ctx))
			}
			return nil
		})).
		ServeHTTP
}
