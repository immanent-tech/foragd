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

	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/go-chi/chi/v5"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic/aggregations"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
	"github.com/joshuar/go-feed-me/server/forms"
	"github.com/joshuar/go-feed-me/web/views"
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

// MarkArticles handles marking a articles as read or unread.
func (a *API) MarkArticles() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Set up handler chain.
		chain := alice.New(
			RouteLogger,
			TriggerStateUpdates,
		)
		// Get request details.
		request, valid, err := forms.DecodeForm[*models.MarkArticlesRequest](req)
		if err != nil || !valid {
			chain.Then(RenderResponse(RespInvalidInput(err))).ServeHTTP(res, req)
			return
		}
		chain = chain.Append(
			SetupRedirect(request.Redirect),
		)
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
			chain.Then(RenderResponse(RespBackendError(err))).ServeHTTP(res, req)
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
		err = a.updateSubscriptionStates(req.Context(), states)
		if err != nil {
			chain.Then(RenderResponse(RespBackendError(err))).ServeHTTP(res, req)
			return
		}

		chain.Then(RenderResponse(nil)).ServeHTTP(res, req)
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
			models.WithResponseTemplate(views.NewArticleContent(articles[0]).ShowContent()),
		)
		ctx := context.WithValue(req.Context(), titleCtxKey, articles[0].GetTitle())
		chain.Then(RenderResponse(resp)).ServeHTTP(res, req.WithContext(ctx))
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
		return nil, RespBackendError(fmt.Errorf("could not extract category counts: %w", err))
	}

	return topCategories.BucketNames(), nil
}
