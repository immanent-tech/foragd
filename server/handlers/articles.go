// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
	"github.com/joshuar/go-feed-me/server/forms"
	"github.com/joshuar/go-feed-me/web/templates/content"
	"github.com/joshuar/go-feed-me/web/views"
)

// GetArticles handles showing a filtered collection of articles as cards.
func GetArticles(api models.DocumentsAPI) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Extract filters from request.
		filters, valid, err := forms.DecodeForm[*models.ArticleFilters](req)
		if err != nil || !valid {
			RenderError(res, req, models.NewResponse(http.StatusBadRequest, fmt.Errorf("parameters are invalid: %w", err)))
			return
		}
		// Get articles matching filters.
		articles, pagination, resp := filterArticles(req.Context(), api, filters)
		if resp != nil {
			ProcessResponse(res, req, resp)
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
func PaginateArticles(api models.DocumentsAPI) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Extract filters from request.
		filters, valid, err := forms.DecodeForm[*models.ArticleFilters](req)
		if err != nil || !valid {
			RenderError(res, req, models.NewResponse(http.StatusBadRequest, fmt.Errorf("parameters are invalid: %w", err)))
			return
		}
		// Get articles matching filters.
		articles, pagination, resp := filterArticles(req.Context(), api, filters)
		if resp != nil {
			ProcessResponse(res, req, resp)
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
func MarkArticles(api models.DocumentsAPI) http.HandlerFunc {
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

		// Retrieve user.
		user, found := models.UserFromCtx(req.Context())
		if !found {
			RenderError(res, req, models.RespErrUnauthorized())
			return
		}
		states := user.GetAllSubscriptionStates()
		// Mark off items under subscription state in user data.
		for subscription, items := range marks {
			switch models.Mark(mark) {
			case models.MarkRead:
				states[subscription].MarkItemsRead(items...)
			case models.MarkUnread:
				states[subscription].MarkItemsUnread(items...)
			}
		}
		// Update the user object.
		if err := api.UpdateUser(req.Context(), map[string]any{
			"subscriptions": slices.Collect(maps.Values(states)),
		}); err != nil {
			RenderError(res, req, models.NewResponse(http.StatusInternalServerError, fmt.Errorf("could not process mark request: %w", err)))
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
func ViewArticle(api models.DocumentsAPI) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// subscriptionID := chi.URLParam(req, "subscription")
		itemID := chi.URLParam(req, "item")
		articles, resp := getArticles(req.Context(), api, itemID)
		if resp != nil || len(articles) == 0 {
			ProcessResponse(res, req, resp)
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

func filterArticles(ctx context.Context, api models.DocumentsAPI, filters *models.ArticleFilters) (models.Articles, models.Pagination, *models.Response) {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, "", models.RespErrUnauthorized()
	}

	subscriptionStates := user.FilterSubscriptionStatesByID(filters.Subscriptions...)
	feedIDs := make([]models.FeedID, 0, len(subscriptionStates))
	for _, state := range subscriptionStates {
		feedIDs = append(feedIDs, state.GetFeedID())
	}

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
				query.Should(models.BuildSubscriptionQueries(user, filters.View, slices.Collect(maps.Values(subscriptionStates))...)...),
			),
		),
	)
	sort := filters.GetSort()

	// Find items matching filters.
	items, pagination, err := api.SearchItems(ctx, query, filters.GetCount(), &sort, filters.Pagination)
	if err != nil {
		return nil, "", models.RespErrBackend(err)
	}
	// Retrieve subscription customisations for feed subscriptions.
	states := user.FilterSubscriptionStatesByFeed(items.GetFeedIDs()...)
	customisations, err := api.GetSubscriptionCustomisations(ctx, models.GetIDsFromStates(states)...)
	if err != nil {
		return nil, "", models.RespErrBackend(err)
	}
	// Create articles from the items.
	articles := make(models.Articles, 0, len(items))
	for item := range slices.Values(items) {
		state := states[item.GetFeedID()]
		customisation := customisations.GetCustomisation(state.GetID())
		article, err := models.GenerateArticle(item, state.GetItemState(item.GetID()), state.GetID(), customisation)
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

func getArticles(ctx context.Context, api models.DocumentsAPI, itemIDs ...models.ItemID) (models.Articles, *models.Response) {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, models.RespErrUnauthorized()
	}

	// Search through items matching any given feeds filters, excluding any read
	// items.
	query := query.Bool(
		query.Filter(
			// Must match any of the given item IDs,
			query.Terms("item_id", itemIDs...),
		),
	)

	items, _, err := api.SearchItems(ctx, query, len(itemIDs), nil, nil)
	if err != nil {
		return nil, models.RespErrBackend(err)
	}

	// Retrieve subscription customisations for feed subscriptions.
	states := user.FilterSubscriptionStatesByFeed(items.GetFeedIDs()...)
	customisations, err := api.GetSubscriptionCustomisations(ctx, models.GetIDsFromStates(states)...)
	if err != nil {
		return nil, models.RespErrBackend(err)
	}
	// Create articles from the items.
	articles := make(models.Articles, 0, len(items))
	for item := range slices.Values(items) {
		state := states[item.GetFeedID()]
		customisation := customisations.GetCustomisation(state.GetID())
		article, err := models.GenerateArticle(item, state.GetItemState(item.GetID()), state.GetID(), customisation)
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
