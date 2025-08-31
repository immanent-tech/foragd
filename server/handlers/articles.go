// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/go-chi/chi/v5"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-feed-me/models"
	"github.com/immanent-tech/go-feed-me/providers/elastic"
	"github.com/immanent-tech/go-feed-me/providers/elastic/aggregations"
	"github.com/immanent-tech/go-feed-me/providers/elastic/query"
	"github.com/immanent-tech/go-feed-me/server/forms"
	"github.com/immanent-tech/go-feed-me/server/session"
	"github.com/immanent-tech/go-feed-me/web/templates"
	"github.com/immanent-tech/go-feed-me/web/templates/layouts"
	"github.com/immanent-tech/go-feed-me/web/templates/partials"
	"github.com/immanent-tech/go-feed-me/web/templates/views"
)

// GetArticles handles showing a filtered collection of articles as cards.
func (a *API) GetArticles() http.HandlerFunc {
	return alice.New(
		routeLogger,
		decodeArticleFilters,
		saveArticleFilters,
	).ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		// Extract filters from request.
		filters := articleFiltersFromCtx(req.Context())
		slogctx.FromCtx(req.Context()).Debug("Showing articles.", slog.String("filters", filters.Query()))
		// Get articles matching filters.
		articles, pagination, err := a.filterArticles(req.Context(), &filters)
		if err != nil {
			render(RespBackendError(err)).ServeHTTP(res, req)
			return
		}
		// Render appropriate content.
		var template templ.Component
		if len(articles) == 0 {
			template = layouts.EmptyContent()
		} else {
			template = views.NewArticlesPage(articles, &filters, pagination).Content()
		}
		if !htmx.IsHTMX(req) || htmx.IsHistoryRestoreRequest(req) {
			template = templates.Page(
				"Go Feed Me - Articles",
				layouts.Drawer(template),
			)
		}
		render(
			models.NewResponse(models.WithResponseTemplate(template)),
		).ServeHTTP(res, req)
	}).ServeHTTP
}

func (a *API) GetArticleUpdates() http.HandlerFunc {
	return alice.New(
		routeLogger,
		decodeArticleFilters,
	).ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "text/event-stream")
		res.Header().Set("Cache-Control", "no-cache")
		res.Header().Set("Connection", "keep-alive")
		if f, ok := res.(http.Flusher); ok {
			f.Flush()
		} else {
			slogctx.FromCtx(req.Context()).Warn("Cannot flush update stream!")
			res.WriteHeader(http.StatusNoContent)
		}
		// Get filters and generate query.
		filters := articleFiltersFromCtx(req.Context())
		query, err := generateItemsQuery(req.Context(), &filters, filters.Subscriptions...)
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Cannot generate query for updates.",
				slog.Any("error", err))
			res.WriteHeader(http.StatusInternalServerError)
			return
		}

		var (
			currentCount int64
			prevCount    int64
		)
		prevCount, err = a.DataAPI().CountItems(req.Context(), query)
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Cannot get updates count.",
				slog.Any("error", err))
			res.WriteHeader(http.StatusInternalServerError)
			return
		}

		for {
			select {
			case <-req.Context().Done():
				slogctx.FromCtx(req.Context()).Debug("Stopping article updates.")
				res.Header().Set("Connection", "close")
				res.WriteHeader(http.StatusRequestTimeout)
				return
			default:
				slogctx.FromCtx(req.Context()).Debug("Checking for article updates.", slog.String("filters", filters.Query()))
				currentCount, err = a.DataAPI().CountItems(req.Context(), query)
				if err != nil {
					slogctx.FromCtx(req.Context()).Warn("Cannot get updates count.",
						slog.Any("error", err))
					continue
				}
				// Show updates toast if new items found.
				if currentCount > prevCount {
					slogctx.FromCtx(req.Context()).Debug("Article updates found.")
					var b bytes.Buffer
					template := bufio.NewWriter(&b)
					err := partials.UpdatesToast().Render(req.Context(), template)
					if err != nil {
						slogctx.FromCtx(req.Context()).Warn("Unable to render template.",
							slog.Any("error", err))
						continue
					}
					template.Flush()
					fmt.Fprintf(res, "data: %s\n\n", b.String())
					if f, ok := res.(http.Flusher); ok {
						f.Flush()
					}
				}
				prevCount = currentCount
				time.Sleep(defaultUpdateInterval)
			}
		}
	}).ServeHTTP
}

// PaginateArticles handles showing the next set of articles.
func (a *API) PaginateArticles() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Set up handler chain.
		chain := alice.New(
			routeLogger,
		)
		// Extract filters from request.
		filters, valid, err := forms.DecodeForm[*models.ArticleFilters](req)
		if err != nil || !valid {
			chain.Then(render(RespInvalidInput(err))).ServeHTTP(res, req)
			return
		}
		// Get articles matching filters.
		articles, pagination, err := a.filterArticles(req.Context(), filters)
		if err != nil {
			chain.Then(render(RespBackendError(err))).ServeHTTP(res, req)
			return
		}
		if len(articles) > 0 {
			// Render appropriate content.
			template := views.NewArticlesPage(articles, filters, pagination).List()
			chain.Then(render(
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
			routeLogger,
		)
		// Extract user data.
		user, found := models.UserFromCtx(req.Context())
		if !found {
			chain.Then(render(RespForbidden())).ServeHTTP(res, req)
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
			chain.Then(render(RespBackendError(err))).ServeHTTP(res, req)
			return
		}
		if !valid {
			chain.Then(render(RespInvalidInput(err))).ServeHTTP(res, req)
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
			chain.Then(render(RespBackendError(err))).ServeHTTP(res, req)
			return
		}

		s, err := a.getArticles(req.Context(), request.ItemID)
		if err != nil || len(s) == 0 || len(s) > 1 {
			chain.Then(render(RespBackendError(err))).ServeHTTP(res, req)
			return
		}
		var resp *models.Response

		// Generate appropriate swap content based on target header.
		switch req.Header.Get(htmx.HeaderTarget) {
		case request.ItemID:
			// Swap target is card.
			filters := session.ArticleFiltersFromSession(req.Context())
			resp = models.NewResponse(
				models.WithResponseTemplate(partials.NewArticleContent(s[0]).Card(filters.GetView())),
			)
		case "mark_" + request.ItemID:
			// Swap target is link.
			resp = models.NewResponse(
				models.WithResponseTemplate(partials.NewArticleContent(s[0]).ToggleMark()),
			)
		}

		chain.Then(render(resp)).ServeHTTP(res, req)
	}
}

// ViewArticle handles viewing the content of an article.
func (a *API) ViewArticle() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		chain := alice.New(
			routeLogger,
		)
		// subscriptionID := chi.URLParam(req, "subscription")
		itemID := chi.URLParam(req, "item")
		articles, err := a.getArticles(req.Context(), itemID)
		if err != nil {
			chain.Then(render(RespBackendError(err))).ServeHTTP(res, req)
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
		chain.Then(render(
			models.NewResponse(models.WithResponseTemplate(template)),
		)).ServeHTTP(res, req)
	}
}

func generateItemsQuery(ctx context.Context, filters models.Filters, subscriptions ...models.SubscriptionID) (query.Option, error) {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, fmt.Errorf("generateItemsQuery: %w", ErrNoCtxData)
	}
	// Search through items matching any given feeds filters, excluding any read
	// items.
	meta := user.GetSubscriptionMetadata().FilterByIDs(subscriptions...)
	return query.Bool(
		query.BoolQueryName("get_items"),
		query.Filter(
			// Must match any of the given feed IDs.
			query.Terms("feed_id", meta.GetFeedIDs()...),
			// Must match any of the given categories.
			query.Terms("categories.raw", filters.GetCategories()...),
			// And should match one feed clause.
			query.Bool(
				query.Should(buildSubscriptionQueries(user, filters.GetView(), meta...)...),
			),
		),
	), nil
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

func decodeArticleFilters(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		filters, valid, err := forms.DecodeForm[*models.ArticleFilters](req)
		if err != nil || !valid {
			slogctx.FromCtx(req.Context()).Warn("Invalid article filters. Using defaults.",
				slog.Any("error", err),
			)
			ctx = articleFiltersToCtx(ctx, models.NewArticleFilters())
		} else {
			ctx = articleFiltersToCtx(ctx, *filters)
		}
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

// savePageState saves the current page state in the session.
func saveArticleFilters(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// Generate state.
		session.FiltersToSession(req.Context(), articleFiltersFromCtx(req.Context()))
		next.ServeHTTP(res, req)
	})
}
