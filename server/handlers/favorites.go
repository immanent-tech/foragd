// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"fmt"
	"net/http"

	"github.com/a-h/templ"
	"golang.org/x/sync/errgroup"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/web/templates"
)

// ListFavorites handles fetching the favorite subscriptions and articles of a user and showing them in a grid layout.
func ListFavorites(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.Append(parseFilters, setCacheControl).
		ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
			var (
				articles      models.Articles
				subscriptions models.Subscriptions
				template      templ.Component
				wg            errgroup.Group
				err           error
			)

			ctx := templates.PageTitleToCtx(req.Context(), "Favorites")

			user := models.UserFromCtx(ctx)

			if user == nil {
				msg := models.NewErrorMessage(
					"Server could not complete request!",
					"This might be temporary, please try again.",
				)
				switch req.Method {
				case http.MethodGet:
					renderPage(
						wrapContent(req.WithContext(ctx), templates.ErrorPage(msg)),
					).ServeHTTP(res, req.WithContext(ctx))
				case http.MethodPost:
					template = templates.ServerErrorNotification(msg)
					renderPartial(template).ServeHTTP(res, req.WithContext(ctx))
				}
				return models.NewAPIError(
					fmt.Errorf("could not fetch user info from context: %w", err),
					http.StatusInternalServerError,
				)
			}

			// Get favorite articles.
			wg.Go(func() error {
				if len(user.ItemFavorites) > 0 {
					var err error
					articles, err = api.GetArticles(ctx, user.ItemFavorites...)
					if err != nil {
						return fmt.Errorf("list favorites: get favorite articles: %w", err)
					}
				}
				return nil
			})

			// Get favorite subscriptions.
			wg.Go(func() error {
				var err error
				subscriptions, err = api.GetSubscriptions(ctx,
					elastic.GetSubscriptionsByFavorite(true),
					elastic.GetSubscriptionsDynamicInfo(true),
				)
				if err != nil && models.HTTPStatus(err) != http.StatusNotFound {
					return fmt.Errorf("list favorites: get favorite subscriptions: %w", err)
				}
				return nil
			})

			if err := wg.Wait(); err != nil {
				msg := models.NewErrorMessage(
					"Server could not complete request!",
					"This might be temporary, please try again.",
				)
				switch req.Method {
				case http.MethodGet:
					renderPage(
						wrapContent(req.WithContext(ctx), templates.ErrorPage(msg)),
					).ServeHTTP(res, req.WithContext(ctx))
				case http.MethodPost:
					template = templates.ServerErrorNotification(msg)
					renderPartial(template).ServeHTTP(res, req.WithContext(ctx))
				}
				return models.NewAPIError(
					fmt.Errorf("could not fetch user info from context: %w", err),
					http.StatusInternalServerError,
				)
			}

			// Render appropriate content.
			if len(subscriptions) > 0 || len(articles) > 0 {
				template = templates.FavoritesGrid(subscriptions, articles)
			} else {
				template = templates.EmptyContent()
			}

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
