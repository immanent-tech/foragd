/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"golang.org/x/sync/errgroup"

	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/templates"
)

type Favorites struct {
	*Page
}

// HandleListFavorites handles fetching the favorite subscriptions and articles of a user and showing them in a grid layout.
func (m *Manager) HandleListFavorites(subSvc SubscriptionsService, itemSvc ItemService) http.HandlerFunc {
	page := &Favorites{
		Page: m.NewPage("/favorites",
			WithSummary("Favorites"),
			WithDescription("View your favorite subscriptions and articles"),
			WithNavigation(templates.NavFavorites),
		),
	}
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
				articles, err = itemSvc.GetArticles(jobCtx, user.ItemFavorites...)
				if err != nil {
					if apiErr, ok := errors.AsType[*models.APIError](
						err,
					); ok &&
						apiErr.StatusCode == http.StatusNotFound {
						return nil
					}
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
			if len(favoriteSubscriptions) > 0 {
				subSvc.GetLatestArticles(req.Context(), models.ViewAll, favoriteSubscriptions)
			}
			return nil
		})

		if err := wg.Wait(); err != nil {
			m.HandleInternalError(
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
		page.template = templates.ListFavorites(response)
		page.Render().ServeHTTP(res, req)
	}
}
