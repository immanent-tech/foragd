/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package handlers

import (
	"context"
	"log/slog"
	"net/http"

	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/templates"
)

// Home contains data for generating a user home page.
type Home struct {
	*Page

	data *templates.HomeData
}

type HomePageService interface {
	AggregateSubscriptions(
		ctx context.Context,
		user *models.User,
		subscriptions models.Subscriptions,
	) (models.CategoryCounts, models.CategoryCounts, models.Articles, error)
}

// HandleHome handles displaying the user's home page.
func (m *Manager) HandleHome(homepageSvc HomePageService) http.HandlerFunc {
	page := &Home{
		Page: m.NewPage("/home",
			WithSummary("Your Home"),
			WithDescription("Latest Articles and Subscription Updates"),
			WithNavigation(templates.NavHome),
		),
	}
	return func(res http.ResponseWriter, req *http.Request) {
		user := models.UserFromCtx(req.Context())
		if user == nil {
			slogctx.FromCtx(req.Context()).Debug("Get user data failed.",
				slog.Any("error", models.ErrCtxValueNotFound))
			http.Redirect(res, req, "/login", http.StatusSeeOther)
			return
		}

		// Get and filter subscriptions.
		allSubscriptions := models.SubscriptionsFromCtx(req.Context())
		subscriptions := allSubscriptions.
			ExcludeGrouped(user.GetSettings().HideGrouped).
			FilterByView(models.ViewUnread).
			Sort(models.SortNewestFirst)
		if len(subscriptions) > 10 {
			subscriptions = subscriptions[:10]
		}

		topCategories, rareCategories, latestArticles, err := homepageSvc.AggregateSubscriptions(
			req.Context(),
			user,
			subscriptions,
		)
		if err != nil {
			m.HandleInternalError(http.StatusInternalServerError, err)
			return
		}

		// Create an object to hold the data.
		page.data = &templates.HomeData{
			Subscriptions:  subscriptions,
			LatestArticles: latestArticles,
			TopCategories:  topCategories,
			RareCategories: rareCategories,
		}

		switch user.GetSettings().ShowOnboarding {
		case true:
			// Show new user home.
			page.template = templates.NewUserHome()
		case false:
			// Show regular home.
			page.template = templates.UserHome(page.data)
		}

		page.Handle().ServeHTTP(res, req)
	}
}
