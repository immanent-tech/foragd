// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-base/pkg/htmx"

	"github.com/immanent-tech/go-base/validation"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/service"
	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/element"
	"github.com/immanent-tech/foragd/web/templates/partials"
)

// ArticleCtx retrieves the article matching the URL param and stores it in the context.
func ArticleCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		id := chi.URLParam(req, "articleID")
		articles, err := service.GetArticles(req.Context(), id)
		if err != nil {
			HandleInternalError(
				http.StatusUnprocessableEntity,
				fmt.Errorf("fetch article %s details: %w", id, err),
			).ServeHTTP(res, req)
			return
		}
		if len(articles) == 0 {
			HandleInternalError(
				http.StatusNotFound,
				fmt.Errorf("fetch article %s: not found", id),
			).ServeHTTP(res, req)
			return
		}
		ctx := models.ArticleToCtx(req.Context(), articles[0])
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

// ListArticles holds data for generating the articles list page.
type ListArticles struct {
	title    templates.PageTitle
	template templ.Component
}

// FullResponse renders a full page (headers, footers and list of articles).
func (p *ListArticles) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(
		templates.CreatePage(p.template,
			templates.WithPageTitle(p.title),
		)).ServeHTTP(res, req)
}

// PartialResponse will either render the list of articles, the controls and update the title/dock/sidebar or, when
// paginating, just the list of articles.
func (p *ListArticles) PartialResponse(res http.ResponseWriter, req *http.Request) {
	switch req.URL.Path {
	case "/list/articles":
		res.Header().Set(htmx.HeaderPushURL, req.URL.String())
		templ.Handler(p.template, templ.WithFragments(templates.ContentFragment)).ServeHTTP(res, req)
		templ.Handler(templates.UpdateTitle(p.title)).ServeHTTP(res, req)
		templ.Handler(templates.SideBar(element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
		templ.Handler(templates.Dock(element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
	case "/articles/paginate":
		templ.Handler(p.template, templ.WithFragments(templates.PaginateFragment)).ServeHTTP(res, req)
	}
}

// HandleListArticles handles fetching articles based on the given page filters and displaying them.
func HandleListArticles() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		user := models.UserFromCtx(req.Context())
		if user == nil {
			slogctx.FromCtx(req.Context()).Debug("Get user data failed.",
				slog.Any("error", models.ErrCtxValueNotFound))
			http.Redirect(res, req, "/login", http.StatusSeeOther)
			return
		}

		// Build request object.
		request := &models.ListRequest{
			Filters: *models.ListFiltersFromCtx(req.Context()),
		}
		if err := request.Validate(); err != nil {
			HandleInternalError(
				http.StatusUnprocessableEntity,
				fmt.Errorf("parse query values: %w", err),
			).ServeHTTP(res, req)
			return
		}

		var (
			articles     models.Articles
			subscription *models.Subscription
			err          error
		)

		// Get the subscription details if the list is for a specific subscription.
		var subscriptionID models.SubscriptionID
		if len(request.Filters.GetSubscriptions()) == 1 {
			subscriptionID = request.Filters.GetSubscriptions()[0]
		} else {
			extraRequestDetails, err := parseForm[*models.ListArticlesRequest](req)
			if err != nil {
				slogctx.Warn(req.Context(), "Could not parse list articles request.",
					slog.Any("error", err))
			} else if extraRequestDetails != nil {
				if extraRequestDetails.SubscriptionID != nil && *extraRequestDetails.SubscriptionID != "" {
					subscriptionID = *extraRequestDetails.SubscriptionID
				}
			}
		}

		if subscriptionID != "" {
			subscription, err = service.GetSubscription(
				req.Context(),
				subscriptionID,
			)
			if err != nil {
				HandleInternalError(
					http.StatusInternalServerError,
					fmt.Errorf("get subscription details: %w", err),
				).ServeHTTP(res, req)
				return
			}
			if user.GetSettings().ShowSubscriptionStats {
				if err := service.UpdateSubscriptionDynamicInfo(
					req.Context(),
					models.Subscriptions{subscription},
				); err != nil {
					slogctx.FromCtx(req.Context()).Warn("Could not generate subscription dynamic info.",
						slog.Any("error", err))
				}
			}
			request.Query = query.Bool(
				service.ArticleFiltersQueryClause(subscription.GetArticleFilters()),
			)
		}

		// Get articles matching filters.
		var next models.Pagination
		articles, next, err = service.FilterArticles(req.Context(), request)
		if err != nil && !errors.Is(err, models.ErrNotFound) {
			HandleInternalError(
				http.StatusInternalServerError,
				fmt.Errorf("filter articles: %w", err),
			).ServeHTTP(res, req)
			return
		}

		// Create response.
		response := &models.ListArticlesResponse{
			Subscription: subscription,
			Articles:     articles,
			Filters:      request.Filters,
		}
		// Update response filters.
		response.Filters.SearchAfter = next.SearchAfter
		response.Filters.UpTo = nil

		// If the list of articles is from a single subscription, update the page title to include the subscription
		// name.
		var titleSummary string
		if len(articles) > 0 && subscription != nil {
			titleSummary = subscription.GetTitle() + " | Articles"
		} else {
			titleSummary = "Articles"
		}
		title := templates.PageTitle{
			Summary:     titleSummary,
			Description: string(response.Filters.GetView()) + " | " + response.Filters.GetSort().String(),
		}

		// Choose rendering method based on method.
		switch req.Method {
		case http.MethodGet:
			// GET: render full page.
			RenderInternalPage(&ListArticles{
				title:    title,
				template: templates.ListArticles(response),
			}).ServeHTTP(res, req)
		case http.MethodPost:
			// POST: render cards only.
			RenderPartial(&ListArticles{
				title:    title,
				template: templates.ListArticles(response),
			}).ServeHTTP(res, req)
			// Update category filters.
			RenderPartial(&PartialTemplate{
				template: templates.UpdateListCategoryFilters(
					"/list/articles",
					response.Filters,
					response.Articles.GetCategoryCounts().GetCategories(),
				),
			}).ServeHTTP(res, req)
			// Update pagination control element.
			if response.Filters.SearchAfter != nil && len(response.Articles) == response.Filters.GetCount() {
				RenderPartial(&PartialTemplate{
					template: templates.ListPaginationControl(
						"/articles/paginate",
						&response.Filters,
						element.WithHXSwapOOB("true"),
					),
				}).ServeHTTP(res, req)
			}
		}
	}
}

// HandleListArticlesUpdates handles checking for any updates and notifying the user.
func HandleListArticlesUpdates() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		filters := models.ListFiltersFromCtx(req.Context())

		// Don't bother calculating updates if user is not viewing unread items.
		if filters.GetView() != models.ViewUnread {
			res.WriteHeader(http.StatusNoContent)
			return
		}

		// Retrieve the user object.
		user := models.UserFromCtx(req.Context())
		if user == nil {
			slogctx.FromCtx(req.Context()).Debug("Get user data failed.",
				slog.Any("error", models.ErrCtxValueNotFound))
			http.Redirect(res, req, "/login", http.StatusSeeOther)
			return
		}

		// Retreive subscription details.
		subscriptions, err := service.GetAllSubscriptions(req.Context())
		if err != nil && !errors.Is(err, models.ErrNotFound) {
			slogctx.FromCtx(req.Context()).Error("Failed to get user subscriptions.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusNoContent)
			return
		}
		// Update subscription dynamic info.
		if err = service.UpdateSubscriptionDynamicInfo(req.Context(), subscriptions); err != nil {
			slogctx.FromCtx(req.Context()).Warn("Unable to update subscription dynamic info.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusNoContent)
			return
		}
		// Filter subscriptions.
		subscriptions = subscriptions.
			FilterByView(filters.GetView()).
			FilterByIDs(filters.GetSubscriptions()...)
		if len(subscriptions) == 0 {
			res.WriteHeader(http.StatusNoContent)
			return
		}

		// Generate a query to find updates.
		updatesQuery := query.Bool(
			query.WithBoolQueryName("list_articles_updates_"+user.GetID()),
			query.Filter(
				// Published/updated within the last 5 minutes.
				query.Bool(
					query.Should(
						query.Since("published", time.Now().UTC().Add(-5*time.Minute)),
						query.Since("updated", time.Now().UTC().Add(-5*time.Minute)),
					),
				),
				// Must match any of the given feed IDs.
				query.Terms("feed_id", subscriptions.GetFeedIDs()),
				// Must match any of the given categories.
				query.Terms("categories.raw", filters.GetCategories()),
				// And should match one feed clause.
				query.Bool(
					query.Filter(service.BuildItemQueries(user, filters.GetView(), subscriptions)...),
				),
			),
		)

		// Count items matching.
		updateCount, err := service.CountItems(req.Context(), updatesQuery)
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Failed to get updates.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusNoContent)
			return
		}

		// If updates found, render a notification.
		if updateCount > 0 {
			var route string
			switch {
			case strings.Contains(req.Referer(), RouteHome):
				route = RouteHome
			default:
				route = "/list/articles"
			}
			RenderPartial(&PartialTemplate{template: partials.UpdatesToast(
				element.WithHXMethod(http.MethodGet, route),
				element.WithHXTarget(templates.ContentID.Target()),
				element.WithHXSwap("morph:innerHTML scroll:top transition:true"),
				element.WithHXPushURL(true),
				element.WithHXValues(filters),
			)}).ServeHTTP(res, req)
		} else {
			res.WriteHeader(http.StatusNoContent)
		}
	}
}

type SimilarArticles struct {
	title    templates.PageTitle
	template templ.Component
}

// FullResponse renders a full page (headers, footers and list of subscriptions).
func (h *SimilarArticles) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(
		templates.CreatePage(h.template,
			templates.WithPageTitle(h.title),
		)).ServeHTTP(res, req)
}

// PartialResponse will either render the list of subscriptions, the controls and update the title/dock/sidebar or, when
// paginating, just the list of subscriptions.
func (h *SimilarArticles) PartialResponse(res http.ResponseWriter, req *http.Request) {
	res.Header().Set(htmx.HeaderPushURL, req.URL.String())
	templ.Handler(h.template, templ.WithFragments(templates.ContentFragment)).ServeHTTP(res, req)
	templ.Handler(templates.UpdateTitle(h.title)).ServeHTTP(res, req)
	templ.Handler(templates.SideBar(element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
	templ.Handler(templates.Dock(element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
}

// HandleFindSimilarArticles handles finding articles similar to the given article and showing the results.
func HandleFindSimilarArticles() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// TODO: wrap id and count in a request object.
		const similarArticlesCount = 15
		// Extract request parameters.
		itemID := chi.URLParam(req, "item_id")
		if err := validation.Validate.Var(itemID, "required,startswith=item_"); err != nil {
			HandleInternalError(
				http.StatusUnprocessableEntity,
				fmt.Errorf("decode request: %w", err),
			).ServeHTTP(res, req)
			return
		}
		articles, err := service.FindSimilarArticles(req.Context(), similarArticlesCount, itemID)
		if err != nil && !errors.Is(err, models.ErrNotFound) {
			HandleInternalError(
				http.StatusInternalServerError,
				fmt.Errorf("find similar articles: %w", err),
			).ServeHTTP(res, req)
			return
		}
		// Show results.
		RenderInternalPage(&SimilarArticles{
			title: templates.PageTitle{
				Summary: "Similar Articles",
			},
			template: templates.SimilarArticles(articles),
		}).ServeHTTP(res, req)
	}
}

// ArticleContent contains the data to view article content.
type ArticleContent struct {
	title    templates.PageTitle
	template templ.Component
}

// FullResponse renders a full page (headers, footers and content).
func (t *ArticleContent) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(
		templates.CreatePage(t.template,
			templates.WithPageTitle(t.title),
		)).ServeHTTP(res, req)
}

// PartialResponse renders just the content and performs OOB swaps to update the title (if set) and sidebar/dock.
func (t *ArticleContent) PartialResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(t.template, templ.WithFragments(templates.ContentFragment)).ServeHTTP(res, req)
	templ.Handler(templates.SideBar(element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
	templ.Handler(templates.Dock(element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
	templ.Handler(templates.UpdateTitle(t.title)).ServeHTTP(res, req)
}

// HandleViewArticle handles showing an article's content.
func HandleViewArticle() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Extract request parameters.
		itemID := chi.URLParam(req, "item_id")
		if err := validation.Validate.Var(itemID, "required,startswith=item_"); err != nil {
			HandleInternalError(
				http.StatusUnprocessableEntity,
				fmt.Errorf("decode request: %w", err),
			).ServeHTTP(res, req)
			return
		}

		// Fetch article.
		articles, err := service.GetArticles(req.Context(), itemID)
		if err != nil {
			HandleInternalError(
				http.StatusUnprocessableEntity,
				fmt.Errorf("get article content: %w", err),
			).ServeHTTP(res, req)
			return
		}
		if len(articles) == 0 {
			res.WriteHeader(http.StatusNotFound)
			HandleNotFound().ServeHTTP(res, req)
			return
		}
		article := articles[0]

		// Get the "show_full_content" value and override the article value.
		request, err := parseForm[*models.ViewArticleRequest](req)
		if err != nil {
			HandleInternalError(
				http.StatusUnprocessableEntity,
				fmt.Errorf("parse request params: %w", err),
			).ServeHTTP(res, req)
			return
		}
		if request != nil && request.ShowFullContent != nil {
			article.ShowFullContent = *request.ShowFullContent
		}

		// Fetch and set remote content if required.
		if article.ShowFullContent {
			if err := service.GetArticleRemoteContent(req.Context(), article); err != nil {
				slogctx.FromCtx(req.Context()).Warn("Unable to get remote content for article.",
					slog.String("item_id", article.GetID()),
					slog.String("item_url", article.GetLink()),
					slog.Any("error", err),
				)
				if apiErr, ok := errors.AsType[*models.APIError](err); ok {
					res.WriteHeader(apiErr.HTTPStatus())
				}
				res.Header().Set(htmx.HeaderReswap, "none")
				res.Header().Set(htmx.HeaderReplaceUrl, "false")
				RenderPartial(&Notification{
					msg: models.NewErrorMessage(
						"Unable to fetch remote content",
						"An error occurred fetching the article remote content.",
					),
				}).ServeHTTP(res, req)
				article.ShowFullContent = false
				return
			}
		}

		// Render article content.
		RenderInternalPage(&ArticleContent{
			title: templates.PageTitle{
				Summary:     article.GetTitle(),
				Description: article.GetFeedTitle(),
			},
			template: templates.ArticleContent(&models.ShowArticleResponse{
				Article: *article,
				// Filters: filters,
			}),
		}).ServeHTTP(res, req)
	}
}

func HandleBrowseArticles(direction string) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		article := models.ArticleFromCtx(req.Context())
		if article == nil {
			HandleInternalError(
				http.StatusUnprocessableEntity,
				errors.New("no article in context"),
			).ServeHTTP(res, req)
			return
		}

		article, err := service.GetNextArticle(
			req.Context(),
			article.GetID(),
			article.GetSubscriptionID(),
			models.ViewUnread,
			direction,
			article.GetUpdatedDate().UTC(),
		)
		if err != nil && !errors.Is(err, elastic.ErrNotFound) {
			HandleInternalError(
				http.StatusInternalServerError,
				fmt.Errorf("get next article: %w", err),
			).ServeHTTP(res, req)
			return
		}
		if errors.Is(err, elastic.ErrNotFound) {
			// Likely reached the "end of the list".
			res.Header().Set(htmx.HeaderReswap, "none")
			RenderPartial(
				&Notification{
					msg: models.NewInfoMessage("No more articles!", ""),
				},
			).ServeHTTP(res, req)
			return
		}
		// Render article content.
		RenderInternalPage(&ArticleContent{
			title: templates.PageTitle{
				Summary:     article.GetTitle(),
				Description: article.GetFeedTitle(),
			},
			template: templates.ArticleContent(&models.ShowArticleResponse{
				Article: *article,
				// Filters: filters,
			}),
		}).ServeHTTP(res, req)
	}
}

// HandleMarkArticle handles marking an article as read or unread.
func HandleMarkArticle(mark models.Mark) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Retrieve the article details.
		article := models.ArticleFromCtx(req.Context())
		if article == nil {
			HandleInternalError(http.StatusNotFound, fmt.Errorf("no article in context")).ServeHTTP(res, req)
			return
		}
		// Mark the article.
		if err := markArticles(
			req.Context(),
			mark,
			article.GetSubscriptionID(),
			article.GetID(),
		); err != nil {
			HandleInternalError(http.StatusInternalServerError, err).ServeHTTP(res, req)
			return
		}
		if currentURL, found := htmx.GetCurrentURL(req); found && strings.Contains(currentURL, "/list/articles") {
			filters := models.ListFiltersFromCtx(req.Context())
			// Remove the article card.
			if filters.GetView() != models.ViewAll {
				res.Header().Set(htmx.HeaderReswap, "delete transition:true swap:300ms")
				res.Header().Set(htmx.HeaderRetarget, htmx.ID(article.GetID()).Target())
				res.Header().Set(htmx.HeaderTrigger, "masonry:update")
				res.Header().Set(models.ActionHeader, "mark-article")
			}
		}
		res.WriteHeader(http.StatusOK)
	}
}

// HandleBulkMarkArticles handles marking multiple articles.
func HandleBulkMarkArticles(mark models.Mark) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Parse confirmation.
		request, err := parseForm[*models.BulkMarkArticlesRequest](req)
		if err != nil {
			HandleInternalError(http.StatusUnprocessableEntity, err).ServeHTTP(res, req)
			return
		}

		switch request.Confirmed {
		case false:
			// Show modal to confirm bulk mark articles.
			RenderPartial(&Modal{
				template: templates.BulkMarkArticlesModal(mark,
					element.WithHXSwap("none"),
				)}).ServeHTTP(res, req)
		case true:
			// For each subscription's articles shown, mark.
			for subscriptionID, itemIDs := range request.DisplayedArticles {
				if err = markArticles(req.Context(), mark, subscriptionID, itemIDs...); err != nil {
					HandleInternalError(http.StatusInternalServerError, err).ServeHTTP(res, req)
					return
				}
			}

			// Redirect/refresh as appropriate.
			if currentURL, found := htmx.GetCurrentURL(req); !found {
				htmx.LocationResponse(
					htmx.WithLocationPath("/home"),
					htmx.WithLocationTarget(templates.ContentID.Target()),
					htmx.WithLocationSwap("morph:innerHTML transition:true show:top"),
					htmx.WithLocationHeaders(map[string]string{
						models.ActionHeader: "mark-articles",
					}),
				).ServeHTTP(res, req)
				return
			} else {
				htmx.LocationResponse(
					htmx.WithLocationPath(currentURL),
					htmx.WithLocationTarget(templates.ContentID.Target()),
					htmx.WithLocationSwap("morph:innerHTML transition:true show:top"),
					htmx.WithLocationHeaders(map[string]string{
						models.ActionHeader: "mark-articles",
					}),
				).ServeHTTP(res, req)
				return
			}
		}

		res.WriteHeader(http.StatusOK)
	}
}

func markArticles(
	ctx context.Context,
	mark models.Mark,
	subscriptionID models.SubscriptionID,
	itemIDs ...models.ItemID,
) error {
	subscription, err := service.GetSubscription(ctx, subscriptionID)
	if err != nil {
		return models.NewAPIError(
			http.StatusInternalServerError,
			fmt.Errorf("get subscriptions: %w", err),
			models.WithUserErrorSummary("Backend request failed!"),
			models.WithUserErrorDescription("This might be a temporary error, please try again."),
		)
	}
	subscription.MarkItems(mark, itemIDs...)
	if err = service.UpdateSubscriptions(ctx, subscription); err != nil {
		return models.NewAPIError(
			http.StatusInternalServerError,
			fmt.Errorf("update subscription: %w", err),
			models.WithUserErrorSummary("Backend request failed!"),
			models.WithUserErrorDescription("This might be a temporary error, please try again."),
		)
	}
	return nil
}

// HandleFavoriteArticle handles toggling an article favorite.
func HandleFavoriteArticle() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Retrieve the article details.
		article := models.ArticleFromCtx(req.Context())
		if article == nil {
			HandleInternalError(http.StatusNotFound, fmt.Errorf("no article in context")).ServeHTTP(res, req)
			return
		}

		user := models.UserFromCtx(req.Context())
		if user == nil {
			slogctx.FromCtx(req.Context()).Debug("Get user data failed.",
				slog.Any("error", models.ErrCtxValueNotFound))
			http.Redirect(res, req, "/login", http.StatusSeeOther)
			return
		}

		var favorite bool
		if slices.Contains(user.ItemFavorites, article.GetID()) {
			favorite = false
		} else {
			favorite = true
		}

		if err := updateFavoriteArticle(req.Context(), user, article.GetID(), favorite); err != nil {
			HandleInternalError(http.StatusInternalServerError, err).ServeHTTP(res, req)
			return
		}

		// Update toggle.
		article.Favorite = favorite
		RenderPartial(&PartialTemplate{
			template: templates.ArticleFavoriteToggle(article,
				element.WithHXSwapOOB("true"),
			),
		}).ServeHTTP(res, req)

		res.WriteHeader(http.StatusOK)
	}
}

// HandleShareArticle handles sharing an article.
func HandleShareArticle() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Retrieve the article details.
		article := models.ArticleFromCtx(req.Context())
		if article == nil {
			HandleInternalError(http.StatusNotFound, fmt.Errorf("no article in context")).ServeHTTP(res, req)
			return
		}
		// Render share modal.
		RenderPartial(&Modal{
			template: templates.ShareArticleModal(article),
		}).ServeHTTP(res, req)
	}
}

// archiveArticle will index the given article content to the article archive for permanent storage.
func archiveArticle(ctx context.Context, article *models.ArticleArchive) error {
	if err := elastic.CreateDoc(ctx, schema.FavoritesIndexRW(), article.ItemID, article); err != nil {
		return fmt.Errorf("archive article: %w", err)
	}
	return nil
}

// unarchiveArticle will delete an article from the archive.
func unarchiveArticle(ctx context.Context, userID models.UserID, itemID models.ItemID) error {
	// Set up the query to match the user's favorited article.
	query := query.Bool(
		query.Filter(
			query.Term("user_id", userID),
			query.Term("item_id", itemID),
		),
	)
	if err := elastic.DeleteDocs(ctx, schema.FavoritesIndexRW(), query); err != nil {
		return fmt.Errorf("unarchive article: %w", err)
	}
	return nil
}

// updateFavoriteArticle changes the favorite status of an article. For adding a favorite article, the content is stored
// in a separate and the user object is updated with a link to the content. For removing a favorite, the stored content
// is removed and user object updated appropriately.
func updateFavoriteArticle(
	ctx context.Context,
	user *models.User,
	id models.ItemID,
	favorite bool,
) error {
	switch favorite {
	case true:
		// Don't do anything if article is already a favorite.
		if slices.Contains(user.ItemFavorites, id) {
			return models.ErrUserAlreadyFavorited
		}
		// Get the article details.
		articles, err := service.GetArticles(ctx, id)
		if err != nil {
			return models.NewAPIError(http.StatusInternalServerError,
				fmt.Errorf("get favorite articles: %w", err),
				models.WithUserErrorSummary("Backend error"),
				models.WithUserErrorDescription("This might be a temporary problem. Please try again"),
			)
		}
		if len(articles) != 1 {
			return models.NewAPIError(http.StatusInternalServerError,
				models.ErrInvalidAPIResult,
				models.WithUserErrorSummary("Backend error"),
				models.WithUserErrorDescription("This might be a temporary problem. Please try again"),
			)
		}
		article := articles[0]
		// Archive the article.
		archive, err := models.NewArchivedArticle(user.GetID(), article.GetSubscriptionID(), &article.Item)
		if err != nil {
			return models.NewAPIError(http.StatusInternalServerError,
				fmt.Errorf("new archived article: %w", err),
				models.WithUserErrorSummary("Backend error"),
				models.WithUserErrorDescription("This might be a temporary problem. Please try again"),
			)
		}
		err = archiveArticle(ctx, archive)
		if err != nil {
			return models.NewAPIError(http.StatusInternalServerError,
				fmt.Errorf("archive article: %w", err),
				models.WithUserErrorSummary("Backend error"),
				models.WithUserErrorDescription("This might be a temporary problem. Please try again"),
			)
		}
		// Update the list of favorites items in the user object
		user.ItemFavorites = append(user.ItemFavorites, id)
		err = service.UpdateUser(ctx, user, map[string]any{
			"item_favorites": user.ItemFavorites,
		})
		if err != nil {
			return models.NewAPIError(http.StatusInternalServerError,
				fmt.Errorf("update user: %w", err),
				models.WithUserErrorSummary("Backend error"),
				models.WithUserErrorDescription("This might be a temporary problem. Please try again"),
			)
		}
	case false:
		err := unarchiveArticle(ctx, user.GetID(), id)
		if err != nil {
			return models.NewAPIError(http.StatusInternalServerError,
				fmt.Errorf("unarchive article: %w", err),
				models.WithUserErrorSummary("Backend error"),
				models.WithUserErrorDescription("This might be a temporary problem. Please try again"),
			)
		}
		newFavorites := slices.DeleteFunc(user.ItemFavorites, func(e models.ItemID) bool {
			return e == id
		})
		err = service.UpdateUser(ctx, user, map[string]any{
			"item_favorites": newFavorites,
		})
		if err != nil {
			return models.NewAPIError(http.StatusInternalServerError,
				fmt.Errorf("update user: %w", err),
				models.WithUserErrorSummary("Backend error"),
				models.WithUserErrorDescription("This might be a temporary problem. Please try again"),
			)
		}
	}
	return nil
}
