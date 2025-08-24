// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/go-chi/chi/v5"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic"
	"github.com/joshuar/go-feed-me/providers/elastic/aggregations"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
	"github.com/joshuar/go-feed-me/server/forms"
	"github.com/joshuar/go-feed-me/server/session"
	"github.com/joshuar/go-feed-me/web/templates"
	"github.com/joshuar/go-feed-me/web/templates/layouts"
	"github.com/joshuar/go-feed-me/web/templates/partials"
	"github.com/joshuar/go-feed-me/web/templates/views"
)

// GetArticles handles showing a filtered collection of articles as cards.
func (a *API) GetArticles() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Set up handler chain.
		chain := alice.New(
			RouteLogger,
		)
		// Extract filters from request.
		filters, valid, err := forms.DecodeForm[*models.ArticleFilters](req)
		if err != nil || !valid {
			chain.Then(RenderResponse(RespInvalidInput(err))).ServeHTTP(res, req)
			return
		}
		// Save the filters to the session.
		chain = chain.Append(SavePageState(filters))
		// Get articles matching filters.
		articles, pagination, err := a.filterArticles(req.Context(), filters)
		if err != nil {
			chain.Then(RenderResponse(RespBackendError(err))).ServeHTTP(res, req)
			return
		}
		// Render appropriate content.
		var template templ.Component
		if len(articles) == 0 {
			template = layouts.EmptyContent()
		} else {
			template = views.NewArticlesPage(articles, filters, pagination).Content()
		}
		if !htmx.IsHTMX(req) || htmx.IsHistoryRestoreRequest(req) {
			template = templates.Page(
				"Go Feed Me - Articles",
				layouts.Drawer(template),
			)
		}
		chain.Then(RenderResponse(
			models.NewResponse(models.WithResponseTemplate(template)),
		)).ServeHTTP(res, req)
	}
}

// PaginateArticles handles showing the next set of articles.
func (a *API) PaginateArticles() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Set up handler chain.
		chain := alice.New(
			RouteLogger,
		)
		// Extract filters from request.
		filters, valid, err := forms.DecodeForm[*models.ArticleFilters](req)
		if err != nil || !valid {
			chain.Then(RenderResponse(RespInvalidInput(err))).ServeHTTP(res, req)
			return
		}
		// Get articles matching filters.
		articles, pagination, err := a.filterArticles(req.Context(), filters)
		if err != nil {
			chain.Then(RenderResponse(RespBackendError(err))).ServeHTTP(res, req)
			return
		}
		if len(articles) > 0 {
			// Render appropriate content.
			template := views.NewArticlesPage(articles, filters, pagination).List()
			chain.Then(RenderResponse(
				models.NewResponse(models.WithResponseTemplate(template)),
			)).ServeHTTP(res, req)
		} else {
			chain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
				res.WriteHeader(http.StatusNoContent)
			}).ServeHTTP(res, req)
		}
	}
}

// MarkArticle handles marking a articles as read or unread.
func (a *API) MarkArticle() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Set up handler chain.
		chain := alice.New(
			RouteLogger,
		)
		// Extract user data.
		user, found := models.UserFromCtx(req.Context())
		if !found {
			chain.Then(RenderResponse(RespForbidden())).ServeHTTP(res, req)
			return
		}
		// Construct the request from parameters.
		request := &models.MarkArticleRequest{
			SubscriptionID: chi.URLParam(req, "subscription"),
			Mark:           models.Mark(chi.URLParam(req, "mark")),
			ItemID:         chi.URLParam(req, "item"),
		}
		// Validate parameters.
		valid, err := request.Valid()
		if err != nil {
			chain.Then(RenderResponse(RespBackendError(err))).ServeHTTP(res, req)
			return
		}
		if !valid {
			chain.Then(RenderResponse(RespInvalidInput(err))).ServeHTTP(res, req)
			return
		}
		// Mark off items under subscription states.
		slogctx.FromCtx(req.Context()).Debug("Marking article.",
			slog.String("subscription_id", request.SubscriptionID),
			slog.String("item_id", request.ItemID),
			slog.String("mark", string(request.Mark)),
		)
		user.MarkItems(request.Mark, request.SubscriptionID, request.ItemID)
		// Update the user object.
		err = a.updateUser(req.Context(), map[string]any{
			"subscriptions": user.Subscriptions,
		})
		if err != nil {
			chain.Then(RenderResponse(RespBackendError(err))).ServeHTTP(res, req)
			return
		}

		s, err := a.getArticles(req.Context(), request.ItemID)
		if err != nil || len(s) == 0 || len(s) > 1 {
			chain.Then(RenderResponse(RespBackendError(err))).ServeHTTP(res, req)
			return
		}
		var resp *models.Response

		// Generate appropriate swap content based on target header.
		switch req.Header.Get(htmx.HeaderTarget) {
		case request.ItemID:
			slog.Debug("swapping card")
			// Swap target is card.
			filters := session.ArticleFiltersFromSession(req.Context())
			resp = models.NewResponse(
				models.WithResponseTemplate(partials.NewArticleContent(s[0]).Card(filters.GetView())),
			)
		case "mark_" + request.ItemID:
			slog.Debug("swapping link")
			// Swap target is link.
			resp = models.NewResponse(
				models.WithResponseTemplate(partials.NewArticleContent(s[0]).ToggleMark()),
			)
		}

		chain.Then(RenderResponse(resp)).ServeHTTP(res, req)
	}
}

// ViewArticle handles viewing the content of an article.
func (a *API) ViewArticle() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		chain := alice.New(
			RouteLogger,
		)
		// subscriptionID := chi.URLParam(req, "subscription")
		itemID := chi.URLParam(req, "item")
		articles, err := a.getArticles(req.Context(), itemID)
		if err != nil {
			chain.Then(RenderResponse(RespBackendError(err))).ServeHTTP(res, req)
			return
		}
		// Render appropriate content.
		var template templ.Component
		page := views.NewArticlePage(articles[0])
		switch {
		case htmx.IsHTMX(req) && !htmx.IsHistoryRestoreRequest(req):
			template = page.Content()
		default:
			template = templates.Page(
				articles[0].GetTitle()+" - Go Feed Me",
				layouts.Drawer(page.Content()),
			)
		}
		chain.Then(RenderResponse(
			models.NewResponse(models.WithResponseTemplate(template)),
		)).ServeHTTP(res, req)
	}
}

func (a *API) filterArticles(ctx context.Context, filters *models.ArticleFilters) (models.Articles, models.Pagination, error) {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, "", models.ErrUserCtx
	}

	// Search through items matching any given feeds filters, excluding any read
	// items.
	subscriptions := user.GetSubscriptionMetadata().FilterByIDs(filters.Subscriptions...)
	query := query.Bool(
		query.BoolQueryName("get_items"),
		query.Filter(
			// Must match any of the given feed IDs.
			query.Terms("feed_id", subscriptions.GetFeedIDs()...),
			// Must match any of the given categories.
			query.Terms("categories.raw", filters.Categories...),
			// And should match one feed clause.
			query.Bool(
				query.Should(buildSubscriptionQueries(user, filters.View, subscriptions...)...),
			),
		),
	)
	sort := filters.GetSort()

	// Find items matching filters.
	items, pagination, err := a.DataAPI().SearchItems(ctx, query, filters.GetCount(), &sort, &filters.Pagination)
	if err != nil {
		return nil, "", models.RespErrBackend(err)
	}
	// Generate articles.
	articles, err := models.GenerateArticles(ctx, items)
	if err != nil {
		return nil, "", models.RespErrBackend(err)
	}

	return articles, pagination, nil
}

func (a *API) getArticles(ctx context.Context, itemIDs ...models.ItemID) (models.Articles, error) {
	// Search through items matching any given feeds filters, excluding any read
	// items.
	query := query.Bool(
		query.Filter(
			// Must match any of the given item IDs,
			query.Terms("item_id", itemIDs...),
		),
	)
	items, _, err := a.DataAPI().SearchItems(ctx, query, len(itemIDs), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("getArticles: %w", err)
	}
	articles, err := models.GenerateArticles(ctx, items)
	if err != nil {
		return nil, fmt.Errorf("getArticles: %w", err)
	}

	return articles, nil
}

func (a *API) getItemTopCategories(ctx context.Context, feeds ...models.FeedID) ([]models.Category, *models.Response) {
	query := query.Bool(
		query.Filter(
			// Must match any of the given feed IDs.
			query.Terms("feed_id", feeds...),
		),
	)
	aggsResult, resp := a.DataAPI().ItemsAggregation(ctx, query, aggregations.NewTermsAggregation("TopCategories", "categories.raw", 10))
	if resp != nil {
		return nil, resp
	}
	var (
		topCategories aggregations.TermsAggregationResults
		err           error
	)
	topCategories.StringTermsAggregate, err = aggregations.ExtractAggregation[*types.StringTermsAggregate](aggsResult.Aggregations, "TopCategories")
	if err != nil {
		return nil, RespBackendError(fmt.Errorf("could not extract category counts: %w", err))
	}

	return topCategories.BucketNames(), nil
}

// archiveArticle will index an article into the item archive to avoid deletion.
func (a *API) archiveArticle(ctx context.Context, article *models.Article) error {
	index := elastic.ArchiveIndexFromCtx(ctx)
	if index == "" {
		return fmt.Errorf("unable to archive article: %w", ErrNoCtxData)
	}
	user, found := models.UserFromCtx(ctx)
	if !found {
		return fmt.Errorf("unable to archive article: %w", ErrNoCtxData)
	}
	archive, err := models.NewArchivedArticle(user.GetID(), article.GetSubscriptionID(), &article.Item)
	if err != nil {
		return fmt.Errorf("unable to archive article: %w", err)
	}
	err = elastic.CreateDoc(ctx, a.DataAPI().GetAPI(), index, archive.ItemID, archive)
	if err != nil {
		return fmt.Errorf("unable to archive article: %w", err)
	}
	return nil
}

// unarchiveArticle will delete an article from the archive.
func (a *API) unarchiveArticle(ctx context.Context, id models.ItemID) error {
	index := elastic.ArchiveIndexFromCtx(ctx)
	if index == "" {
		return fmt.Errorf("unable to removed archived article: %w", ErrNoCtxData)
	}
	user, found := models.UserFromCtx(ctx)
	if !found {
		return fmt.Errorf("unable to remove archived article: %w", ErrNoCtxData)
	}
	// Set up the query to match the user's favorited article.
	query := query.Bool(
		query.Filter(
			query.Term("user_id", user.GetID()),
			query.Term("item_id", id),
		),
	)
	err := elastic.DeleteDocs(ctx, a.DataAPI().GetAPI(), index, query)
	if err != nil {
		return fmt.Errorf("unable to remove archived article: %w", err)
	}
	return nil
}
