// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/go-chi/chi/v5"
	"github.com/justinas/alice"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic"
	"github.com/joshuar/go-feed-me/providers/elastic/aggregations"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
	"github.com/joshuar/go-feed-me/server/forms"
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
		// Generate articles page.
		resp := models.NewResponse(
			models.WithResponseTemplate(views.NewArticlesPage(articles, filters, pagination).Template(req)),
		)

		chain.Then(RenderResponse(resp)).ServeHTTP(res, req)
	}
}

// MarkArticle handles marking a articles as read or unread.
func (a *API) MarkArticle() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Set up handler chain.
		chain := alice.New(
			RouteLogger,
			// TriggerStateUpdates,
		)
		// Extract user data.
		user, found := models.UserFromCtx(req.Context())
		if !found {
			chain.Then(RenderResponse(RespForbidden())).ServeHTTP(res, req)
			return
		}
		// Construct the request from parameters.
		request := &models.MarkArticlesRequest{
			SubscriptionID: chi.URLParam(req, "subscription"),
			Mark:           models.Mark(chi.URLParam(req, "mark")),
			Articles:       []models.ItemID{chi.URLParam(req, "item")},
			View:           models.View(req.FormValue("view")),
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
		user.MarkItems(request.Mark, request.SubscriptionID, request.Articles...)
		// Update the user object.
		err = a.updateUser(req.Context(), map[string]any{
			"subscriptions": user.Subscriptions,
		})
		if err != nil {
			chain.Then(RenderResponse(RespBackendError(err))).ServeHTTP(res, req)
			return
		}

		var resp *models.Response
		// If the view is "all" send back the updated subscription card.
		if request.View == models.ViewAll {
			s, err := a.getArticles(req.Context(), request.Articles...)
			if err != nil || len(s) == 0 || len(s) > 1 {
				chain.Then(RenderResponse(RespBackendError(err))).ServeHTTP(res, req)
				return
			}
			card := partials.NewArticleContent(s[0])
			resp = models.NewResponse(
				models.WithResponseTemplate(card.ShowAsItem()),
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
		resp := models.NewResponse(
			models.WithResponseTemplate(views.NewArticlePage(articles[0]).Template(req)),
		)
		chain.Then(RenderResponse(resp)).ServeHTTP(res, req)
	}
}

// ShareArticle handles sharing an article to external sources.
func (a *API) ShareArticle() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Set up handler chain.
		chain := alice.New(
			RouteLogger,
		)
		itemID := chi.URLParam(req, "item")
		articles, err := a.getArticles(req.Context(), itemID)
		if err != nil {
			chain.Then(RenderResponse(RespBackendError(err))).ServeHTTP(res, req)
			return
		}
		var resp *models.Response
		switch req.Method {
		case http.MethodGet:
			// Generate articles page.
			resp = models.NewResponse(
				models.WithResponseTemplate(partials.ShareArticleModal(articles[0])),
			)
		}

		chain.Then(RenderResponse(resp)).ServeHTTP(res, req)
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
