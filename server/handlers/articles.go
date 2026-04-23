// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"codeberg.org/readeck/go-readability/v2"
	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"
	"github.com/goforj/godump"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/client"
	"github.com/immanent-tech/foragd/extractor"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/server/forms"
	"github.com/immanent-tech/foragd/server/session"
	"github.com/immanent-tech/foragd/validation"
	htmxext "github.com/immanent-tech/foragd/web/htmx"
	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/element"
)

// ListArticles holds data for generating the articles list page.
type ListArticles struct {
	title    string
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
		templ.Handler(templates.SideBar(templ.Attributes{"hx-swap-oob": "true"})).ServeHTTP(res, req)
		templ.Handler(templates.Dock(templ.Attributes{"hx-swap-oob": "true"})).ServeHTTP(res, req)
	case "/list/articles/paginate":
		templ.Handler(p.template, templ.WithFragments(templates.PaginateFragment)).ServeHTTP(res, req)
	}
}

// HandleListArticles handles fetching articles based on the given page filters and displaying them.
func HandleListArticles() http.HandlerFunc {
	return userContentHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		// Build request object.
		pagination := req.FormValue(models.ParamPagination)
		filters := getListArticleFilters(req)
		request := &models.ListRequest{
			Filters:    *filters,
			Pagination: &pagination,
		}
		if err := request.Valid(); err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("unable to list subscriptions: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
			}).ServeHTTP(res, req)
			return
		}

		// Redirect to include query parameters in address bar.
		if !strings.HasSuffix(req.URL.Path, "/paginate") {
			switch {
			case htmx.IsHTMX(req):
				res.Header().Set(htmx.HeaderReplaceUrl, req.URL.Path+"?"+request.Filters.QueryString())
			case len(req.URL.Query()) == 0:
				req.URL.RawQuery = request.Filters.QueryParams().Encode()
				http.Redirect(res, req, req.URL.String(), http.StatusSeeOther)
			}
		}

		var (
			articles     models.Articles
			subscription *models.Subscription
			err          error
		)

		// Get the subscription details if the list is for a specific subscription.
		if subscriptionID := req.FormValue("subscription_id"); subscriptionID != "" {
			subscription, err = models.GetSubscription(
				req.Context(),
				subscriptionID,
				models.GetSubscriptionsDynamicInfo(true),
			)
			if err != nil {
				HandleInternalError(&models.APIError{
					InternalError: fmt.Errorf("filter articles: %w", err),
					StatusCode:    http.StatusInternalServerError,
				}).ServeHTTP(res, req)
				return
			}
			request.Query = query.Bool(
				models.ArticleFiltersQueryClause(subscription),
			)
		}

		// Get articles matching filters.
		var next models.Pagination
		articles, next, err = models.FilterArticles(req.Context(), request)
		if err != nil && !errors.Is(err, models.ErrNotFound) {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("filter articles: %w", err),
				StatusCode:    http.StatusInternalServerError,
			}).ServeHTTP(res, req)
			return
		}
		request.Pagination = &next

		// If the list of articles is from a single subscription, update the page tile to include the subscription
		// name.
		var title string
		if len(articles) > 0 && subscription != nil {
			title = subscription.GetTitle() + " | Articles"
		} else {
			title = "Articles"
		}
		// Choose rendering method based on method (get = page, post = partial).
		switch req.Method {
		case http.MethodGet:
			RenderInternalPage(&ListArticles{
				title: title,
				template: templates.ListArticles(&models.ListArticlesResponse{
					Subscription: subscription,
					Articles:     articles,
					Filters:      request.Filters,
					Pagination:   *request.Pagination,
				}),
			}).ServeHTTP(res, req)
		case http.MethodPost:
			RenderPartial(&ListArticles{
				title: title,
				template: templates.ListArticles(&models.ListArticlesResponse{
					Subscription: subscription,
					Articles:     articles,
					Filters:      request.Filters,
					Pagination:   *request.Pagination,
				}),
			}).ServeHTTP(res, req)
		}
	}).ServeHTTP
}

// HandleListArticlesUpdates handles checking for any updates and notifying the user.
func HandleListArticlesUpdates() http.HandlerFunc {
	return userContentHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		// Parse and process filters.
		filters := getListArticleFilters(req)

		// Don't bother calculating updates if user is not viewing unread items.
		if filters.GetView() != models.ViewUnread {
			res.WriteHeader(http.StatusNoContent)
			return
		}

		// Build query.
		user := models.UserFromCtx(req.Context())
		if user == nil {
			slogctx.FromCtx(req.Context()).Error("Failed to get user data.",
				slog.Any("error", models.ErrCtxValueNotFound),
			)
			res.WriteHeader(http.StatusNoContent)
			return
		}
		subscriptions, err := models.GetSubscriptions(req.Context(),
			models.GetSubscriptionsByIDs(filters.GetSubscriptions()...),
		)
		switch {
		case err != nil:
			slogctx.FromCtx(req.Context()).Error("Failed to get user subscriptions.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusNoContent)
			return
		case len(subscriptions) == 0:
			slogctx.FromCtx(req.Context()).Error("No user subscriptions.")
			res.WriteHeader(http.StatusNoContent)
			return
		}
		updatesQuery := query.Bool(
			query.WithBoolQueryName("list_articles_updates_"+user.GetID()),
			query.Filter(
				// Published/updated within the last 5 minutes.
				query.Bool(
					query.Should(
						query.Since("updated", time.Now().UTC().Add(-5*time.Minute)),
						query.Since("published", time.Now().UTC().Add(-5*time.Minute)),
					),
				),
				// Must match any of the given feed IDs.
				query.Terms("feed_id", subscriptions.GetFeedIDs()...),
				// Must match any of the given categories.
				query.Terms("categories.raw", filters.GetCategories()...),
				// And should match one feed clause.
				query.Bool(
					query.Should(models.BuildItemQueries(user, filters.GetView(), subscriptions)...),
				),
			),
		)

		// Count items matching.
		updateCount, err := models.CountItems(req.Context(), updatesQuery)
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Failed to get updates.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusNoContent)
			return
		}

		// If updates found, render a notification.
		if updateCount > 0 {
			RenderPartial(&PartialTemplate{template: templates.UpdatesToast(
				element.WithHXOptions(
					element.WithHXMethod(http.MethodGet, "/list/articles"),
					element.WithHXTarget(templates.ContentID.Target()),
					element.WithHXSwap("innerHTML window:top transition:true"),
					element.WithHXPushURL(true),
					element.WithHXValues(filters.Values()),
				),
			)}).ServeHTTP(res, req)
		} else {
			res.WriteHeader(http.StatusNoContent)
		}
	}).ServeHTTP
}

type SimilarArticles struct {
	template templ.Component
}

// FullResponse renders a full page (headers, footers and list of subscriptions).
func (h *SimilarArticles) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(
		templates.CreatePage(h.template,
			templates.WithPageTitle("Similar Articles"),
		)).ServeHTTP(res, req)
}

// PartialResponse will either render the list of subscriptions, the controls and update the title/dock/sidebar or, when
// paginating, just the list of subscriptions.
func (h *SimilarArticles) PartialResponse(res http.ResponseWriter, req *http.Request) {
	res.Header().Set(htmx.HeaderPushURL, req.URL.String())
	templ.Handler(h.template, templ.WithFragments(templates.ContentFragment)).ServeHTTP(res, req)
	templ.Handler(templates.UpdateTitle("Similar Articles")).ServeHTTP(res, req)
	templ.Handler(templates.SideBar(templ.Attributes{"hx-swap-oob": "true"})).ServeHTTP(res, req)
	templ.Handler(templates.Dock(templ.Attributes{"hx-swap-oob": "true"})).ServeHTTP(res, req)
}

// HandleFindSimilarArticles handles finding articles similar to the given article and showing the results.
func HandleFindSimilarArticles() http.HandlerFunc {
	return userContentHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		// TODO: wrap id and count in a request object.
		const similarArticlesCount = 15
		// Extract request parameters.
		itemID := chi.URLParam(req, models.ParamItemID)
		if err := validation.Validate.Var(itemID, "required,startswith=item_"); err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("decode request: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to find similar",
					"There was a problem with the request. Please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}
		articles, err := models.FindSimilarArticles(req.Context(), similarArticlesCount, itemID)
		if err != nil && !errors.Is(err, models.ErrNotFound) {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("find similar articles: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to find similar",
					"There was a problem with the request. Please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}
		// Show results.
		RenderInternalPage(&SimilarArticles{
			template: templates.SimilarArticles(articles),
		}).ServeHTTP(res, req)
	}).ServeHTTP
}

// ArticleContent contains the data to view article content.
type ArticleContent struct {
	title    string
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
	res.Header().Set(htmx.HeaderPushURL, req.URL.String())
	templ.Handler(t.template, templ.WithFragments(templates.ContentFragment)).ServeHTTP(res, req)
	templ.Handler(templates.SideBar(templ.Attributes{"hx-swap-oob": "true"})).ServeHTTP(res, req)
	templ.Handler(templates.Dock(templ.Attributes{"hx-swap-oob": "true"})).ServeHTTP(res, req)
	templ.Handler(templates.UpdateTitle(t.title)).ServeHTTP(res, req)
}

// HandleViewArticle handles showing an article's content.
func HandleViewArticle() http.HandlerFunc {
	return userContentHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		// Extract request parameters.
		itemID := chi.URLParam(req, models.ParamItemID)
		if err := validation.Validate.Var(itemID, "required,startswith=item_"); err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("decode request: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to find similar",
					"There was a problem with the request. Please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}

		// Fetch article.
		articles, err := models.GetArticles(req.Context(), itemID)
		if err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("get article content: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to view object",
					"There was a problem with the request. Please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}
		article := articles[0]

		// Get the "show_full_content" value and override the article value.
		if fullContent, err := strconv.ParseBool(req.FormValue(models.ParamFullArticleContent)); err != nil ||
			!fullContent {
			article.ShowFullContent = false
		} else if fullContent {
			article.ShowFullContent = fullContent
		}

		// Fetch and set remote content if required.
		if article.ShowFullContent {
			// content, err := extractArticleFromURL(article.GetLink())
			content, err := getFullContent(req.Context(), article.GetLink())
			switch {
			case err != nil:
				// Couldn't fetch remote article content, show an error message.
				slogctx.FromCtx(req.Context()).Warn("Unable to fetch remote content for article.",
					slog.String("item_id", article.GetID()),
					slog.String("item_url", article.GetLink()),
					slog.Any("error", err),
				)
				res.Header().Set(htmx.HeaderReswap, "none")
				res.Header().Set(htmx.HeaderReplaceUrl, "false")
				RenderPartial(&Notification{
					msg: models.NewErrorMessage(
						"Cannot display remote content",
						"Cannot fetch article content from remote.",
					),
				}).ServeHTTP(res, req)
				article.ShowFullContent = false
				return
			case content == "":
				// Remote content is same as feed content.
				slogctx.FromCtx(req.Context()).Warn("No remote content returned for article.",
					slog.String("item_id", article.GetID()),
					slog.String("item_url", article.GetLink()),
				)
				res.Header().Set(htmx.HeaderReswap, "none")
				res.Header().Set(htmx.HeaderReplaceUrl, "false")
				RenderPartial(&Notification{
					msg: models.NewErrorMessage("No remote content", "Remote article content is missing."),
				}).ServeHTTP(res, req)
				article.ShowFullContent = false
				return
			default:
				// Got remote content.
				article.Content = &content
			}
		}

		// Render article content.
		RenderInternalPage(&ArticleContent{
			title:    article.GetTitle() + " | " + article.GetFeedTitle(),
			template: templates.ArticleContent(article),
		}).ServeHTTP(res, req)
	}).ServeHTTP
}

// ExtractArticleFromURL fetches the text content of the given URL and attempts to extract the main article content from
// it.
func getFullContent(ctx context.Context, originalURL string) (string, error) {
	extractorURL, err := extractor.GenerateExtractorURL(originalURL, "html")
	if err != nil {
		return "", fmt.Errorf("get full content: %w", err)
	}

	var resp models.ExtractorResponse
	var respErr models.ExtractorErrorResponse

	httpClient := client.LoadHTTPClient()
	rawResp, err := httpClient.R().
		SetHeader("Accept", "application/json").
		SetContext(ctx).
		SetResult(&resp).
		SetError(&respErr).
		Get(extractorURL)
	if err != nil {
		return "", fmt.Errorf("get url: %w", err)
	}
	if rawResp.IsError() {
		godump.Dump(respErr)
		return "", fmt.Errorf("%s: %s", rawResp.Status(), respErr.Detail)
	}

	if resp.Content != nil {
		content := validation.SanitizeString(*resp.Content)
		return content, nil
	}

	return "", nil
}

// MarkArticle handles marking an article as read/unread and updates the UI accordingly.
func MarkArticle() http.HandlerFunc {
	return userContentHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		request, valid, err := forms.DecodeForm[*models.MarkArticleRequest](req)
		if err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to mark article",
					"This might be a temporary issue, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}
		if !valid {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to mark article",
					"This might be a temporary issue, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}

		// Mark the article.
		if err := markArticles(req.Context(), request.Mark, request.SubscriptionID, request.ItemID); err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("unable to mark article: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to mark article",
					"This might be a temporary issue, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}

		res.WriteHeader(http.StatusOK)
	}).ServeHTTP
}

// MarkArticles handles marking multiple articles as read/unread and updating the UI appropriately.
func MarkArticles() http.HandlerFunc {
	return userContentHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		// Decode request parameters.
		request, valid, err := forms.DecodeForm[*models.MarkArticlesRequest](req)
		if err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("decode mark articles request: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to mark articles.",
					"This might be a temporary error, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}
		if !valid {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("validate mark articles request: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to mark articles.",
					"This might be a temporary error, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}

		// Mark Articles.
		for subscriptionID, itemIDs := range request.DisplayedArticles {
			if err = markArticles(req.Context(), request.Mark, subscriptionID, itemIDs...); err != nil {
				HandleInternalError(&models.APIError{
					InternalError: fmt.Errorf("mark subscriptions: %w", err),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Unable to mark articles.",
						"This might be a temporary error, please try again.",
					),
				}).ServeHTTP(res, req)
				return
			}
		}

		if currentURL, found := htmx.GetCurrentURL(req); !found {
			err = setRedirect(res, htmxext.HXLocationRequest{
				Path:   "/home",
				Target: templates.ContentID.Target(),
				Swap:   "innerHTML show:window:top transition:true",
			})
		} else {
			err = setRedirect(res, htmxext.HXLocationRequest{
				Path:   currentURL,
				Target: templates.ContentID.Target(),
				Swap:   "innerHTML show:window:top transition:true",
			})
		}
		if err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("mark subscriptions: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to mark articles.",
					"This might be a temporary error, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}

		// res.Header().Set(htmx.HeaderRefresh, "true")

		res.WriteHeader(http.StatusOK)
	}).ServeHTTP
}

// FavoriteArticle handles adding an article favorite.
func FavoriteArticle() http.HandlerFunc {
	return userContentHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		request, valid, err := forms.DecodeForm[*models.FavoriteArticleRequest](req)
		if err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to favorite article",
					"This might be a temporary issue, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}
		if !valid {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to favorite article",
					"This might be a temporary issue, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}

		user := models.UserFromCtx(req.Context())
		if user == nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("get user data: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to add favorite article",
					"This might be a temporary error, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}

		var favorite bool
		if slices.Contains(user.ItemFavorites, request.ItemID) {
			favorite = false
		} else {
			favorite = true
		}

		if err := updateFavoriteArticle(req.Context(), user, request.ItemID, favorite); err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("update favorite article: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable favorite article",
					"This might be a temporary error, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}

		res.WriteHeader(http.StatusOK)
	}).ServeHTTP
}

// ShareArticle handles sharing an article.
func ShareArticle() http.HandlerFunc {
	return userContentHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		request, _, err := forms.DecodeForm[*models.ShareArticleRequest](req)
		if err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to favorite article",
					"This might be a temporary issue, please try again.",
				),
			}).ServeHTTP(res, req)
		}

		RenderPartial(&Modal{
			template: templates.ShareArticleModal(request),
		}).ServeHTTP(res, req)
	}).ServeHTTP
}

func markArticles(
	ctx context.Context,
	mark models.Mark,
	subscriptionID models.SubscriptionID,
	itemIDs ...models.ItemID,
) error {
	subscription, err := models.GetSubscription(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("get subscriptions: %w", err)
	}
	subscription.MarkItems(mark, itemIDs...)

	_, err = models.UpdateSubscriptions(ctx, subscription)
	if err != nil {
		return fmt.Errorf("update subscription data: %w", err)
	}

	return nil
}

var articleBufPool = sync.Pool{
	New: func() any {
		var buf bytes.Buffer
		return &buf
	},
}

// ExtractArticleFromURL fetches the text content of the given URL and attempts to extract the main article content from
// it.
func extractArticleFromURL(url string) (string, error) {
	remote, err := readability.FromURL(url, client.DefaultHTTPRequestTimeout)
	if err != nil {
		return "", fmt.Errorf("extract article from url %s: %w", url, err)
	}

	buf, ok := articleBufPool.Get().(*bytes.Buffer)
	if !ok {
		return "", errors.New("unable to allocate article content buffer")
	}
	defer func() {
		buf.Reset()
		bufPool.Put(buf)
	}()

	if err := remote.RenderHTML(buf); err != nil {
		return "", fmt.Errorf("render article html: %w", err)
	}
	content := validation.SanitizeString(buf.String())
	return content, nil
}

// archiveArticle will index the given article content to the article archive for permanent storage.
func archiveArticle(ctx context.Context, article *models.ArticleArchive) error {
	if err := elastic.CreateDoc(ctx, schema.FavoritesIndexRW, article.ItemID, article); err != nil {
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
	if err := elastic.DeleteDocs(ctx, schema.FavoritesIndexRW, query); err != nil {
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
		articles, err := models.GetArticles(ctx, id)
		if err != nil {
			return fmt.Errorf("unable to add favorite article: %w", err)
		}
		if len(articles) != 1 {
			return models.ErrInvalidAPIResult
		}
		article := articles[0]
		// Archive the article.
		archive, err := models.NewArchivedArticle(user.GetID(), article.GetSubscriptionID(), &article.Item)
		if err != nil {
			return fmt.Errorf("unable to add favorite article: %w", err)
		}
		err = archiveArticle(ctx, archive)
		if err != nil {
			return fmt.Errorf("unable to add favorite article: %w", err)
		}
		// Update the list of favorites items in the user object
		user.ItemFavorites = append(user.ItemFavorites, id)
		err = models.UpdateUser(ctx, user.GetID(), map[string]any{
			"item_favorites": user.ItemFavorites,
		})
		if err != nil {
			return fmt.Errorf("unable to add favorite article: %w", err)
		}
	case false:
		err := unarchiveArticle(ctx, user.GetID(), id)
		if err != nil {
			return fmt.Errorf("unable to remove favorite article: %w", err)
		}
		newFavorites := slices.DeleteFunc(user.ItemFavorites, func(e models.ItemID) bool {
			return e == id
		})
		err = models.UpdateUser(ctx, user.GetID(), map[string]any{
			"item_favorites": newFavorites,
		})
		if err != nil {
			return fmt.Errorf("unable to remove favorite article: %w", err)
		}
	}
	return nil
}

func getListArticleFilters(req *http.Request) *models.ListFilters {
	// Parse and process filters.
	filters, valid, err := forms.DecodeForm[*models.ListFilters](req)
	switch {
	case err != nil:
		slogctx.FromCtx(req.Context()).Warn("Unable to decode article filters. Using filters from session.",
			slog.Any("error", err),
			slog.Any("filters", filters),
		)
		// Try to restore filters from session.
		filters = session.GetListArticleFiltersFromSession(req.Context())
	case !valid:
		slogctx.FromCtx(req.Context()).Warn("Invalid subscription filters. Creating new filters.")
		session.StoreListArticleFiltersInSession(req.Context(), models.NewListDisplayFilters())
	default:
		session.StoreListArticleFiltersInSession(req.Context(), *filters)
	}

	return filters
}
