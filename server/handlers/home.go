// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-base/pkg/htmx"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/element"
)

// Home contains data for generating a user home page.
type Home struct {
	title templates.PageTitle
	data  *templates.HomeData
	svc   pageServices
}

// FullResponse renders a full page (headers, footers and data).
func (p *Home) FullResponse(res http.ResponseWriter, req *http.Request) {
	user := models.UserFromCtx(req.Context())
	if user == nil {
		slogctx.FromCtx(req.Context()).Debug("Get user data failed.",
			slog.Any("error", models.ErrCtxValueNotFound))
		http.Redirect(res, req, "/login", http.StatusSeeOther)
		return
	}

	switch user.GetSettings().ShowOnboarding {
	case true:
		templ.Handler(
			templates.CreatePage(
				p.svc.appCfg,
				p.svc.sessionMgr,
				templates.NewUserHome(),
				templates.WithPageTitle(p.title),
			)).ServeHTTP(res, req)
	case false:
		templ.Handler(
			templates.CreatePage(
				p.svc.appCfg,
				p.svc.sessionMgr,
				templates.UserHome(p.data),
				templates.WithPageTitle(p.title),
			)).ServeHTTP(res, req)
	}
}

// PartialResponse will render just the data.
func (p *Home) PartialResponse(res http.ResponseWriter, req *http.Request) {
	user := models.UserFromCtx(req.Context())
	if user == nil {
		slogctx.FromCtx(req.Context()).Debug("Get user data failed.",
			slog.Any("error", models.ErrCtxValueNotFound))
		http.Redirect(res, req, "/login", http.StatusSeeOther)
		return
	}

	res.Header().Set(htmx.HeaderPushURL, req.URL.String())

	switch user.GetSettings().ShowOnboarding {
	case true:
		templ.Handler(
			templates.NewUserHome(),
			templ.WithFragments(templates.ContentFragment),
		).ServeHTTP(res, req)
	case false:
		templ.Handler(
			templates.UserHome(p.data),
			templ.WithFragments(templates.ContentFragment),
		).ServeHTTP(res, req)
	}
	// Update title, dock/sidebar.
	templ.Handler(templates.UpdateTitle(p.title)).ServeHTTP(res, req)
	templ.Handler(templates.SideBar(templates.NavHome, element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
	templ.Handler(templates.Dock(templates.NavHome, element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
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
	return func(res http.ResponseWriter, req *http.Request) {
		title := templates.PageTitle{
			Summary:     "Your Home",
			Description: "Latest Articles and Subscription Updates",
		}
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
		data := &templates.HomeData{
			Subscriptions:  subscriptions,
			LatestArticles: latestArticles,
			TopCategories:  topCategories,
			RareCategories: rareCategories,
		}

		RenderInternalPage(&Home{
			title: title,
			data:  data,
			svc:   m.NewPageServices(),
		}).ServeHTTP(res, req)
	}
}
