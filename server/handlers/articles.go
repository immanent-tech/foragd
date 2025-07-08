// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/go-chi/chi/v5"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic/aggregations"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
	"github.com/joshuar/go-feed-me/server/forms"
	"github.com/joshuar/go-feed-me/web/templates/content"
	"github.com/joshuar/go-feed-me/web/views"
)

// GetArticles handles showing a filtered collection of articles as cards.
func (a *API) GetArticles() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Extract filters from request.
		filters, valid, err := forms.DecodeForm[*models.ArticleFilters](req)
		if err != nil || !valid {
			RenderError(res, req, models.NewResponse(http.StatusBadRequest, fmt.Errorf("parameters are invalid: %w", err)))
			return
		}
		// Get articles matching filters.
		articles, pagination, err := a.filterArticles(req.Context(), filters)
		if err != nil {
			RenderError(res, req, models.RespErrBackend(err))
			return
		}
		// Generate articles page.
		articlesPage := views.NewArticlesPage(articles, filters, pagination)
		ctx := templateToCtx(req.Context(), articlesPage)
		// Set up handler chain.
		chain := alice.New(
			RouteLogger,
			SavePageState(filters),
		)
		// Render fragments/page.
		switch htmx.IsHTMX(req) {
		case true:
			chain.Then(RenderTemplateFragments("content-header", "content", "content-footer")).ServeHTTP(res, req.WithContext(ctx))
		case false:
			chain.Then(RenderTemplate()).ServeHTTP(res, req.WithContext(ctx))
		}
	}
}

// PaginateArticles handles showing the next set of articles in a filtered collection.
func (a *API) PaginateArticles() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Extract filters from request.
		filters, valid, err := forms.DecodeForm[*models.ArticleFilters](req)
		if err != nil || !valid {
			RenderError(res, req, models.NewResponse(http.StatusBadRequest, fmt.Errorf("parameters are invalid: %w", err)))
			return
		}
		// Get articles matching filters.
		articles, pagination, resp := a.filterArticles(req.Context(), filters)
		if resp != nil {
			RenderError(res, req, models.RespErrBackend(err))
			return
		}
		// Generate article cards.
		cards := make([]templ.Component, 0, len(articles))
		for article := range slices.Values(articles) {
			cards = append(cards, views.NewArticleContent(article).ShowAsCard())
		}
		// Add pagination element if pagination is required.
		if pagination != "" && len(cards) == filters.GetCount() {
			// Add pagination htmx props to last article.
			cards = append(cards, content.PaginationControl(req.Context(), "/articles", pagination))
		}
		ctx := templateToCtx(req.Context(), templ.Join(cards...))
		// Set up handler chain and render cards.
		alice.New(
			RouteLogger,
			SavePageState(filters),
		).Then(RenderTemplate()).ServeHTTP(res, req.WithContext(ctx))
	}
}

// MarkArticles handles marking a articles as read or unread.
func (a *API) MarkArticles() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Get request details.
		request, valid, err := forms.DecodeForm[*models.MarkArticlesRequest](req)
		if err != nil || !valid {
			RenderError(res, req, models.NewResponse(http.StatusBadRequest, err))
			return
		}
		// Parse out items into respective subscriptions.
		marks := make(map[models.SubscriptionID][]models.ItemID)
		for a := range slices.Values(request.Articles) {
			data := strings.Split(a, "__")
			marks[data[0]] = append(marks[data[0]], data[1])
		}
		// Get mark.
		mark := chi.URLParam(req, "mark")
		// Get subscription states containing items.
		states, err := a.getSubscriptionStates(req.Context(), slices.Collect(maps.Keys(marks))...)
		if err != nil {
			RenderError(res, req, models.RespErrUnauthorized())
			return
		}
		// Mark off items under subscription states.
		for subscriptionID, items := range marks {
			state := states.GetByID(subscriptionID)
			if state == nil {
				slogctx.FromCtx(req.Context()).Warn("No subscription matches given item.",
					slog.Any("item_ids", items),
				)
			}
			switch models.Mark(mark) {
			case models.MarkRead:
				state.MarkItemsRead(items...)
			case models.MarkUnread:
				state.MarkItemsUnread(items...)
			}
		}
		// Update the states.
		if err := a.updateSubscriptionStates(req.Context(), states); err != nil {
			RenderError(res, req, models.RespErrBackend(err))
			return
		}

		alice.New(
			RouteLogger,
			SetupRedirect(request.Redirect),
			TriggerStateUpdates,
		).Then(RenderTemplate()).ServeHTTP(res, req)
	}
}

// ViewArticle handles viewing the content of an article.
func (a *API) ViewArticle() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// subscriptionID := chi.URLParam(req, "subscription")
		itemID := chi.URLParam(req, "item")
		articles, err := a.getArticles(req.Context(), itemID)
		if err != nil {
			ProcessResponse(res, req, models.RespErrBackend(err))
			return
		}
		articleLayout := views.NewArticleContent(articles[0]).ShowContent()

		ctx := req.Context()
		ctx = templateToCtx(ctx, articleLayout)
		ctx = context.WithValue(ctx, titleCtxKey, articles[0].GetTitle())
		alice.New(
			RouteLogger,
		).Then(RenderTemplate()).ServeHTTP(res, req.WithContext(ctx))
	}
}

func (a *API) filterArticles(ctx context.Context, filters *models.ArticleFilters) (models.Articles, models.Pagination, error) {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, "", models.ErrUserCtx
	}
	subscriptionStates, err := a.getSubscriptionStates(ctx, filters.Subscriptions...)
	if err != nil {
		return nil, "", fmt.Errorf("filterArticles: %w", err)
	}
	feedIDs := subscriptionStates.GetFeedIDs()

	// Search through items matching any given feeds filters, excluding any read
	// items.
	query := query.Bool(
		query.BoolQueryName("get_items"),
		query.Filter(
			// Must match any of the given feed IDs.
			query.Terms("feed_id", feedIDs...),
			// Must match any of the given categories.
			query.Terms("categories.raw", filters.Categories...),
			// And should match one feed clause.
			query.Bool(
				query.Should(buildSubscriptionQueries(user, filters.View, subscriptionStates...)...),
			),
		),
	)
	sort := filters.GetSort()

	// Find items matching filters.
	items, pagination, err := a.DataAPI().SearchItems(ctx, query, filters.GetCount(), &sort, filters.Pagination)
	if err != nil {
		return nil, "", models.RespErrBackend(err)
	}
	// Retrieve subscription customisations for feed subscriptions.
	subscriptionStates = subscriptionStates.FilterByFeedIDs(items.GetFeedIDs()...)
	// Create articles from the items.
	articles := make(models.Articles, 0, len(items))
	for item := range slices.Values(items) {
		state := subscriptionStates.GetByFeedID(item.GetFeedID())
		article, err := models.GenerateArticle(item, state)
		if err != nil {
			slogctx.FromCtx(ctx).Warn("Could not generate article from data.",
				slog.Any("error", err),
			)
			continue
		}
		articles = append(articles, article)

	}

	return articles, pagination, nil
}

func (a *API) getArticles(ctx context.Context, itemIDs ...models.ItemID) (models.Articles, error) {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, models.ErrUserCtx
	}

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

	// Retrieve subscription customisations for feed subscriptions.
	states, err := a.getSubscriptionStates(ctx, slices.Collect(maps.Values(user.GetSubscriptionsByFeedID(items.GetFeedIDs()...)))...)
	if err != nil {
		return nil, fmt.Errorf("getArticles: %w", err)
	}
	if len(states) == 0 {
		return nil, errors.New("no items")
	}

	// Create articles from the items.
	articles := make(models.Articles, 0, len(items))
	for item := range slices.Values(items) {
		state := states.GetByFeedID(item.GetFeedID())
		article, err := models.GenerateArticle(item, state)
		if err != nil {
			slogctx.FromCtx(ctx).Warn("Could not generate article from data.",
				slog.Any("error", err),
			)
			continue
		}
		articles = append(articles, article)
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
		return nil, models.NewResponse(http.StatusInternalServerError, fmt.Errorf("could not extract category counts: %w", err))
	}

	return topCategories.BucketNames(), nil
}
