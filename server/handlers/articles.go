// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"sync"

	"codeberg.org/readeck/go-readability/v2"
	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/server/forms"
	"github.com/immanent-tech/foragd/validation"
	htmxext "github.com/immanent-tech/foragd/web/htmx"
	"github.com/immanent-tech/foragd/web/templates"
)

// ListArticles handles fetching articles based on the given page filters and displaying them.
func ListArticles() http.HandlerFunc {
	return defaultHandlerChain.Append(parseFilters).
		ThenFunc(func(res http.ResponseWriter, req *http.Request) {
			list := func(res http.ResponseWriter, req *http.Request) error {
				// Build request object.
				request := &models.ListRequest{
					Filters:    *models.PageFiltersFromCtx(req.Context(), req.URL.Path),
					Pagination: req.FormValue(models.ParamPagination),
				}
				if err := request.Valid(); err != nil {
					return &models.APIError{
						InternalError: fmt.Errorf("unable to list subscriptions: %w", err),
						StatusCode:    http.StatusUnprocessableEntity,
					}
				}

				// Redirect to include query parameters in address bar.
				if len(req.URL.Query()) == 0 {
					if htmx.IsHTMX(req) {
						res.Header().Set(htmx.HeaderPushURL, req.URL.Path+"?"+request.Filters.QueryString())
					} else {
						http.Redirect(res, req, req.URL.Path+"?"+request.Filters.QueryString(), http.StatusSeeOther)
					}
				}

				var (
					articles     models.Articles
					subscription *models.Subscription
					err          error
					template     templ.Component
				)

				// Get articles matching filters.
				articles, request.Pagination, err = models.FilterArticles(req.Context(), request)
				if err != nil && !errors.Is(err, models.ErrNotFound) {
					return &models.APIError{
						InternalError: fmt.Errorf("filter articles: %w", err),
						StatusCode:    http.StatusInternalServerError,
					}
				}

				// Get the subscription details if the list is for a specific subscription.
				if len(articles.GetSubscriptionIDs()) == 1 {
					subscription, err = models.GetSubscription(
						req.Context(),
						articles.GetSubscriptionIDs()[0],
						models.GetSubscriptionsDynamicInfo(true),
					)
					if err != nil {
						return &models.APIError{
							InternalError: fmt.Errorf("filter articles: %w", err),
							StatusCode:    http.StatusInternalServerError,
						}
					}
				}

				// Generate response object.
				response := &models.ListArticlesResponse{
					Subscription: subscription,
					Articles:     articles,
					Filters:      request.Filters,
					Pagination:   request.Pagination,
				}

				// Render appropriate content.
				// If the list of articles is from a single subscription, update the page tile to include the subscription
				// name.
				var title string
				if len(articles) > 0 && response.Subscription != nil {
					title = subscription.GetTitle() + " | Articles"
				} else {
					title = "Articles"
				}
				template = templates.ListArticles(response)
				// Choose rendering method based on method (get = page, post = partial).
				switch req.Method {
				case http.MethodGet:
					renderPage(
						templates.NewPage(
							wrapContent(req, template),
							templates.WithPageTitle(title),
						),
					).ServeHTTP(res, req)
				case http.MethodPost:
					renderPartial(templates.NewPartial(template)).ServeHTTP(res, req)
				}
				return nil
			}
			switch req.Method {
			case http.MethodGet:
				showOnError(list).ServeHTTP(res, req)
			case http.MethodPost:
				notifyOnError(list).ServeHTTP(res, req)
			}
		}).ServeHTTP
}

// PaginateArticles handles a request to list more articles.
func PaginateArticles() http.HandlerFunc {
	return defaultHandlerChain.Append(parseFilters).
		ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
			// Build request object.
			request := &models.ListRequest{
				Filters:    *models.PageFiltersFromCtx(req.Context(), req.URL.Path),
				Pagination: req.FormValue(models.ParamPagination),
			}
			if err := request.Valid(); err != nil {
				return &models.APIError{
					InternalError: fmt.Errorf("unable to list articles: %w", err),
					StatusCode:    http.StatusUnprocessableEntity,
					UserMessage: models.NewErrorMessage(
						"Could not list more articles",
						"This might be temporary, please try again.",
					),
				}
			}

			// Get articles matching filters.
			var (
				articles models.Articles
				err      error
			)
			articles, request.Pagination, err = models.FilterArticles(req.Context(), request)
			if err != nil && !errors.Is(err, elastic.ErrNotFound) {
				return &models.APIError{
					InternalError: fmt.Errorf("unable to list articles: %w", err),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Could not list more articles",
						"This might be temporary, please try again.",
					),
				}
			}

			// If there are articles to show, render the articles. Else, return StatusNoContent.
			if len(articles) > 0 {
				// Generate response object.
				response := &models.ListArticlesResponse{
					Articles:   articles,
					Filters:    request.Filters,
					Pagination: request.Pagination,
				}
				renderPartial(templates.NewPartial(templates.PaginateArticles(response))).ServeHTTP(res, req)
			} else {
				res.WriteHeader(http.StatusNoContent)
				return nil
			}
			return nil
		})).
		ServeHTTP
}

// FindSimilarArticles handles finding articles similar to the given article and showing the results.
func FindSimilarArticles() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
		// TODO: wrap id and count in a request object.
		const similarArticlesCount = 15
		// Extract request parameters.
		itemID := chi.URLParam(req, models.ParamItemID)
		if err := validation.Validate.Var(itemID, "required,startswith=item_"); err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("decode request: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to find similar",
					"There was a problem with the request. Please try again.",
				),
			}
		}
		articles, err := models.FindSimilarArticles(req.Context(), similarArticlesCount, itemID)
		if err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("find similar articles: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to find similar",
					"There was a problem with the request. Please try again.",
				),
			}
		}
		// Show results.
		var template templ.Component
		if len(articles) > 0 {
			template = templates.SimilarArticles(articles)
		} else {
			template = templates.NoSearchResults()
		}
		renderPage(
			templates.NewPage(
				wrapContent(req, template),
				templates.WithPageTitle("Similar Articles"),
			),
		).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// ViewArticle handles showing an article's content.
func ViewArticle() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(showOnError(func(res http.ResponseWriter, req *http.Request) error {
		// Extract request parameters.
		itemID := chi.URLParam(req, models.ParamItemID)
		if err := validation.Validate.Var(itemID, "required,startswith=item_"); err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("decode request: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to find similar",
					"There was a problem with the request. Please try again.",
				),
			}
		}
		articles, err := models.GetArticles(req.Context(), itemID)
		if err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("get article content: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to view object",
					"There was a problem with the request. Please try again.",
				),
			}
		}
		article := articles[0]
		// Get the "show_full_content" value and override the article value.
		if fullContent, err := strconv.ParseBool(req.FormValue(models.ParamFullArticleContent)); err != nil ||
			!fullContent {
			article.ShowFullContent = false
		} else if fullContent {
			article.ShowFullContent = fullContent
		}
		var remoteContentErrMsg templ.Component
		// Fetch and set remote content if required.
		if article.ShowFullContent {
			if content, err := extractArticleFromURL(article.GetLink()); err != nil {
				// Couldn't fetch remote article content, show an error message.
				remoteContentErrMsg = templates.Notification(
					models.NewErrorMessage("Unable to fetch article remote content", ""), 0,
				)
				article.ShowFullContent = false
			} else {
				if content == article.Content {
					// Remote article content is the same as feed content, show an info message.
					remoteContentErrMsg = templates.Notification(
						models.NewInfoMessage("No remote content available", "Page returned existing content."), templates.DefaultNotificationTimeout,
					)
				}
				article.Content = content
			}
		}
		// Render appropriate content.
		var template templ.Component
		if remoteContentErrMsg != nil {
			template = templ.Join(templates.NewArticleView(article).Content(), remoteContentErrMsg)
		} else {
			template = templates.NewArticleView(article).Content()
		}
		renderPage(
			templates.NewPage(
				wrapContent(req, template),
				templates.WithPageTitle(article.GetTitle()+" | "+article.GetFeedTitle()),
			),
		).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// MarkArticle handles marking an article as read/unread and updates the UI accordingly.
func MarkArticle() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
		request, valid, err := forms.DecodeForm[*models.MarkArticleRequest](req)
		if err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to mark article",
					"This might be a temporary issue, please try again.",
				),
			}
		}
		if !valid {
			return &models.APIError{
				InternalError: fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to mark article",
					"This might be a temporary issue, please try again.",
				),
			}
		}

		// Mark the article.
		if err := markArticles(req.Context(), request.Mark, request.SubscriptionID, request.ItemID); err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("unable to mark article: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to mark article",
					"This might be a temporary issue, please try again.",
				),
			}
		}

		res.WriteHeader(http.StatusOK)
		return nil
	})).ServeHTTP
}

// MarkArticles handles marking multiple articles as read/unread and updating the UI appropriately.
func MarkArticles() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
		// Decode request parameters.
		request, valid, err := forms.DecodeForm[*models.MarkArticlesRequest](req)
		if err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("decode mark articles request: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to mark articles.",
					"This might be a temporary error, please try again.",
				),
			}
		}
		if !valid {
			return &models.APIError{
				InternalError: fmt.Errorf("validate mark articles request: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to mark articles.",
					"This might be a temporary error, please try again.",
				),
			}
		}

		// Mark Articles.
		for subscriptionID, itemIDs := range request.DisplayedArticles {
			if err = markArticles(req.Context(), request.Mark, subscriptionID, itemIDs...); err != nil {
				return &models.APIError{
					InternalError: fmt.Errorf("mark subscriptions: %w", err),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Unable to mark articles.",
						"This might be a temporary error, please try again.",
					),
				}
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
			return &models.APIError{
				InternalError: fmt.Errorf("mark subscriptions: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to mark articles.",
					"This might be a temporary error, please try again.",
				),
			}
		}

		res.WriteHeader(http.StatusOK)
		return nil
	})).ServeHTTP
}

// FavoriteArticle handles adding an article favorite.
func FavoriteArticle() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
		request, valid, err := forms.DecodeForm[*models.FavoriteArticleRequest](req)
		if err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to favorite article",
					"This might be a temporary issue, please try again.",
				),
			}
		}
		if !valid {
			return &models.APIError{
				InternalError: fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to favorite article",
					"This might be a temporary issue, please try again.",
				),
			}
		}

		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("get user data: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to add favorite article",
					"This might be a temporary error, please try again.",
				),
			}
		}

		var favorite bool
		if slices.Contains(user.ItemFavorites, request.ItemID) {
			favorite = false
		} else {
			favorite = true
		}

		if err := updateFavoriteArticle(req.Context(), user, request.ItemID, favorite); err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("update favorite article: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable favorite article",
					"This might be a temporary error, please try again.",
				),
			}
		}

		res.WriteHeader(http.StatusOK)
		return nil
	})).ServeHTTP
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
	remote, err := readability.FromURL(url, models.DefaultHTTPRequestTimeout)
	if err != nil {
		return "", fmt.Errorf("extract article from url %s: %w", url, err)
	}

	articleBufPtr, ok := articleBufPool.Get().(*bytes.Buffer)
	if !ok {
		return "", errors.New("unable to allocate article content buffer")
	}
	articleBuf := *articleBufPtr
	defer func() {
		articleBufPtr.Reset()
		imgBufPool.Put(articleBufPtr)
	}()

	if err := remote.RenderHTML(&articleBuf); err != nil {
		return "", fmt.Errorf("render article html: %w", err)
	}
	content := validation.SanitizeString(articleBuf.String())
	return content, nil
}

// archiveArticle will index the given article content to the article archive for permanent storage.
func archiveArticle(ctx context.Context, article *models.ArticleArchive) error {
	if err := elastic.CreateDoc(ctx, models.FavoriteArticlesIndexRW, article.ItemID, article); err != nil {
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
	if err := elastic.DeleteDocs(ctx, models.FavoriteArticlesIndexRW, query); err != nil {
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
