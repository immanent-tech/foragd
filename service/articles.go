// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package service

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"time"

	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-syndication/sanitization"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/opensearch/doc"
	"github.com/immanent-tech/foragd/providers/opensearch/search"
	"github.com/immanent-tech/foragd/schema"
)

// GetArticles generates Article objects from the Items with the given IDs.
func (s *ItemService) GetArticles(
	ctx context.Context,
	itemIDs ...models.ItemID,
) (models.Articles, error) {
	items, err := s.GetItems(ctx, itemIDs...)
	if err != nil {
		return nil, fmt.Errorf("get items: %w", err)
	}
	if len(items) == 0 {
		return nil, models.ErrNotFound
	}
	articles, err := GenerateArticles(ctx, items)
	if err != nil {
		return nil, fmt.Errorf("generate articles: %w", err)
	}

	return articles, nil
}

// GetNextArticle returns the "next" article from the given article, based on the given article timestamp. The direction
// defines what the next article will be (previous or next). If a subscription is given, it will filter to that
// subscription only. Otherwise, the results are also filtered to the given view.
func (s *ItemService) GetNextArticle(
	ctx context.Context,
	currentID models.ItemID,
	subscriptionID models.SubscriptionID,
	view models.View,
	direction string,
	ts time.Time,
) (*models.Article, error) {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get user: %w", models.ErrCtxValueNotFound)
	}

	// filters are query clauses that filter the results.
	filters := make([]search.QueryOption, 0)
	// exclusions are query clauses that exclude some results.
	exclusions := make([]search.QueryOption, 0)
	exclusions = append(exclusions, search.TermQuery("item_id", currentID))

	// Define filters/exclusions based on subscription(s).
	allSubscriptions := models.SubscriptionsFromCtx(ctx)
	if allSubscriptions == nil {
		return nil, fmt.Errorf("get user subscriptions: %w", models.ErrCtxValueNotFound)
	}
	subscription := allSubscriptions.GetByID(subscriptionID)
	if subscription == nil {
		return nil, fmt.Errorf("get article subscription: %w", models.ErrNotFound)
	}
	filters = append(filters, search.TermQuery("feed_id", subscription.GetFeedID()))
	filters = append(filters,
		search.BoolQuery(
			ArticleFiltersQueryClause(user.GetSettings().GlobalFilters),
			search.WithBoolShould(BuildItemQueries(user, view, models.Subscriptions{subscription})...),
		),
	)

	// Define filters and sorting based on direction.
	var sort models.Sort
	switch direction {
	case "next":
		filters = append(filters,
			search.BoolQuery(
				search.WithBoolShould(
					search.SinceQuery("published", ts),
					search.SinceQuery("updated", ts),
				),
			),
		)
		sort = models.SortOldestFirst
	case "prev":
		filters = append(filters,
			search.BoolQuery(
				search.WithBoolShould(
					search.BeforeQuery("published", ts),
					search.BeforeQuery("updated", ts),
				),
			),
		)
		sort = models.SortNewestFirst
	}

	// Find the next item and generate an article.
	items, _, err := s.QueryItems(
		ctx,
		search.BoolQuery(
			search.WithBoolFilter(filters...),
			search.WithBoolMustNot(exclusions...),
		),
		1,
		&sort,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("search items: %w", err)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("search items: %w", models.ErrNotFound)
	}
	articles, err := GenerateArticles(ctx, items)
	if err != nil {
		return nil, fmt.Errorf("generate article: %w", err)
	}

	return articles[0], nil
}

// FilterArticles returns a [models.Articles] slice (and a [models.Subscriptions] slice) of articles that match the
// given [models.ListFilters]. An additional array of [search.QueryOption] can be passed to further filter items matched.
func (s *ItemService) FilterArticles(
	ctx context.Context,
	filters *models.ListFilters,
	extraQueries ...search.QueryOption,
) (models.Articles, models.Subscriptions, models.Pagination, error) {
	user, subscriptions, err := getUserAndSubscriptions(ctx)
	if err != nil {
		return nil, nil, models.Pagination{}, models.NewAPIError(
			http.StatusInternalServerError,
			fmt.Errorf("get user and subscriptions: %w", err),
		)
	}

	if len(filters.GetSubscriptions()) > 0 {
		subscriptions = subscriptions.FilterByIDs(filters.GetSubscriptions()...)
	}

	var count int
	if filters.SearchAfter != nil {
		count = models.DefaultListResultsCount
	} else {
		count = filters.GetUpTo()
	}

	// Set sorting of results.
	sort := filters.GetSort()

	// Find items matching filters.
	items, pagination, err := s.QueryItems(
		ctx,
		generateArticlesQuery(user, subscriptions, filters, extraQueries...),
		count,
		&sort,
		filters.SearchAfter,
	)
	if err != nil {
		return nil, nil, models.Pagination{}, fmt.Errorf("could not retrieve filtered items: %w", err)
	}

	// Generate articles.
	articles, err := GenerateArticles(ctx, items)
	if err != nil {
		return nil, nil, models.Pagination{}, fmt.Errorf("could not generate articles from items: %w", err)
	}

	return articles, subscriptions, models.Pagination{SearchAfter: &pagination}, nil
}

// FilterGeoArticles executes FilterArticles with an additional filter clause to only match articles with geo data.
func (s *ItemService) FilterGeoArticles(
	ctx context.Context,
	filters *models.ListFilters,
) (models.Articles, models.Subscriptions, models.Pagination, error) {
	// Filter to items with geo data.
	return s.FilterArticles(
		ctx,
		filters,
		search.BoolQuery(
			search.WithBoolFilter(
				search.ExistsQuery("geo"),
			),
		),
	)
}

// CountArticles returns a count of items that match the given [models.ListFilters]. An additional array of
// [query.Option] can be passed to further filter items matched.
func (s *ItemService) CountArticles(
	ctx context.Context,
	filters *models.ListFilters,
	extraQueries ...search.QueryOption,
) (int64, error) {
	user, subscriptions, err := getUserAndSubscriptions(ctx)
	if err != nil {
		return 0, models.NewAPIError(
			http.StatusInternalServerError,
			fmt.Errorf("get user and subscriptions: %w", err),
		)
	}

	if len(filters.GetSubscriptions()) > 0 {
		subscriptions = subscriptions.FilterByIDs(filters.GetSubscriptions()...)
	}

	since := time.Now().UTC().Add(-5 * time.Minute)

	count, err := search.Count(ctx, s.osc, s.osc.IndexReadAlias(schema.ItemsIndex),
		search.WithCountQuery(
			generateArticlesQuery(
				user,
				subscriptions,
				filters,
				slices.Concat([]search.QueryOption{
					search.BoolQuery(
						search.WithBoolShould(
							search.SinceQuery("published", since),
							search.SinceQuery("updated", since),
						),
					),
				},
					extraQueries,
				)...,
			),
		),
	)

	if err != nil {
		return 0, fmt.Errorf("count items: %w", err)
	}

	return count, nil
}

// CountGeoArticles executes CountArticles with an additional filter clause to only match articles with geo data.
func (s *ItemService) CountGeoArticles(
	ctx context.Context,
	filters *models.ListFilters,
) (int64, error) {
	return s.CountArticles(ctx, filters,
		search.BoolQuery(
			search.WithBoolFilter(
				search.ExistsQuery("geo"),
			),
		))
}

// generateArticlesQuery creates the base query for finding articles matching the given [models.ListFilters]. An
// additional list of [query.Option] can be passed, which will be added to the filter clause of the overall bool query.
func generateArticlesQuery(
	user *models.User,
	subscriptions models.Subscriptions,
	filters *models.ListFilters,
	extraQueries ...search.QueryOption,
) search.QueryOption {
	// Build article query.
	clauses := []search.QueryOption{
		// Must match these feed IDs.
		search.TermsQuery("feed_id", subscriptions.GetFeedIDs()),
		// Must match these categories.
		search.TermsQuery("categories.raw", filters.GetCategories()),
		// Must have matching language.
		search.TermQuery("language", filters.GetLanguage()),
	}
	// Must match one of the subscription filter clauses.
	clauses = append(clauses, BuildItemQueries(user, filters.GetView(), subscriptions)...)
	if articleFilters := ArticleFiltersQueryClause(user.GetSettings().GlobalFilters); articleFilters != nil {
		clauses = append(clauses, search.BoolQuery(articleFilters))
	}
	if len(extraQueries) > 0 {
		clauses = append(clauses, extraQueries...)
	}
	return search.BoolQuery(
		search.WithBoolFilter(clauses...),
	)
}

// FindSimilarArticles performs a "more like this" search to find other Articles that are similar to the Items with the
// given IDs.
func (s *ItemService) FindSimilarArticles(
	ctx context.Context,
	count int,
	itemIDs ...models.ItemID,
) (models.Articles, error) {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get user details: %w", models.ErrCtxValueNotFound)
	}
	allSubscriptions := models.SubscriptionsFromCtx(ctx)
	if allSubscriptions == nil {
		return nil, fmt.Errorf("get user subscriptions: %w", models.ErrCtxValueNotFound)
	}

	// Build query
	similarQuery := search.BoolQuery(
		search.WithBoolFilter(
			search.BoolQuery(
				search.WithBoolShould(BuildItemQueries(user, models.ViewUnread, allSubscriptions)...),
			),
		),
		search.WithBoolMust(
			search.MoreLikeThisQuery(
				[]string{"title", "description", "content", "categories.raw", "author"},
				search.WithMoreLikeThisQueryLikeDocIDs(itemIDs...),
				search.WithMoreLikeThisQueryMinTermFreq(1),
				search.WithMoreLikeThisQueryMinDocFreq(2),
				search.WithMoreLikeThisQueryMinWordLen(3),
				search.WithMoreLikeThisQueryMaxQueryTerms(25),
			),
		),
	)
	// Query for similar articles.
	sort := models.SortMostRelevant
	items, _, err := s.QueryItems(ctx, similarQuery, count, &sort, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to find similar articles: %w", err)
	}
	// Generate article data.
	articles, err := GenerateArticles(ctx, items)
	if err != nil {
		return nil, fmt.Errorf("unable to find similar articles: %w", err)
	}
	return articles, nil
}

// ArchiveArticle will index the given article content to the article archive for permanent storage.
func (s *ItemService) ArchiveArticle(ctx context.Context, article *models.ArticleArchive) error {
	if err := doc.Create(ctx, s.osc.Doc, s.osc.IndexWriteAlias(schema.FavoritesIndex), article); err != nil {
		return fmt.Errorf("archive article: %w", err)
	}
	return nil
}

// UnarchiveArticle will delete an article from the archive.
func (s *ItemService) UnarchiveArticle(
	ctx context.Context,
	userID models.UserID,
	itemID models.ItemID,
) error {
	// Set up the query to match the user's favorited article.
	if err := doc.DeleteByQuery(ctx, s.osc, s.osc.IndexWriteAlias(schema.ItemsIndex), doc.WithDeleteByQuery(
		search.BoolQuery(search.WithBoolFilter(
			search.TermQuery("user_id", userID),
			search.TermQuery("item_id", itemID),
		)),
	)); err != nil {
		return fmt.Errorf("unarchive article: %w", err)
	}

	return nil
}

// GenerateArticles takes a slice of items and creates articles from them, grabbing the necessary data from the user
// object.
func GenerateArticles(ctx context.Context, items models.Items) (models.Articles, error) {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get user details: %w", models.ErrCtxValueNotFound)
	}

	// Get the subscriptions associated with the items.
	allSubscriptions := models.SubscriptionsFromCtx(ctx)
	if allSubscriptions == nil {
		return nil, fmt.Errorf("get user subscriptions: %w", models.ErrCtxValueNotFound)
	}
	subscriptions := allSubscriptions.FilterByFeedIDs(items.GetFeedIDs()...)
	if len(subscriptions) == 0 {
		return nil, fmt.Errorf("get subscriptions for items: %w", models.ErrNotFound)
	}

	// Create articles from the items.
	articles := make(models.Articles, 0, len(items))
	for item := range slices.Values(items) {
		subscription := subscriptions.GetByFeedID(item.GetFeedID())
		if subscription == nil {
			slogctx.FromCtx(ctx).WarnContext(ctx, "Could not match item to subscription.",
				slog.String("item_id", item.GetID()),
				slog.String("feed_id", item.GetFeedID()),
			)
			continue
		}
		article := &models.Article{
			Item:         *item,
			Subscription: *subscription,
			State:        subscription.GetItemState(item.GetID()),
		}
		// If there is favorite data, mark article as a favorite.
		if slices.Contains(user.ItemFavorites, item.GetID()) {
			article.State.Favorite = true
		}
		// Add any appropriate feed customisation data.
		article.Item.FeedTitle = subscription.GetTitle()
		// 	Update read status.
		if item.GetTimestamp().Before(subscription.GetMarkedReadAt()) {
			article.State.MarkRead(subscription.GetMarkedReadAt())
		}
		// Toggle showing remote article content.
		article.ShowFullContent = subscription.Settings.ShowFullArticleContent
		// Toggle marking read on view.
		article.MarkArticleReadOnView = user.GetSettings().MarkArticleReadOnView
		// Validate the article.
		if err := article.Validate(); err != nil {
			slogctx.FromCtx(ctx).WarnContext(ctx, "Could not generate article from data.",
				slog.Any("error", err),
				slog.String("item_id", item.GetID()),
			)
			continue
		}
		articles = append(articles, article)
	}
	return articles, nil
}

// GetArticleRemoteContent populates the article content with the item source.
func (s *ItemService) GetArticleRemoteContent(
	ctx context.Context,
	article *models.Article,
) error {
	// Get the complete item HTML source, either from the cache or fetch fresh.
	sourceURL, err := url.Parse(article.GetLink())
	if err != nil {
		return models.NewAPIError(http.StatusUnprocessableEntity, fmt.Errorf("parse article URL: %w", err))
	}

	itemPageBuf, err := s.getItemContent(ctx, article.GetID(), sourceURL)
	if err != nil {
		return models.NewAPIError(http.StatusInternalServerError, fmt.Errorf("get item content: %w", err))
	}

	if itemPageBuf.Len() != 0 {
		// Extract opengraph and readability data from item HTML source.
		_, readabilityData, err := extractMetadataFromHTML(sourceURL, itemPageBuf.Bytes())
		if err != nil {
			if s.osc != nil {
				logGeneralError(ctx, s.indexer, err, article.GetLink(), article.Item.GetFeedID())
			} else {
				logGeneralError(ctx, nil, err, article.GetLink(), article.Item.GetFeedID())
			}
		}
		if readabilityData != nil {
			// Extract article content using readability.
			var articleBuf bytes.Buffer
			if err := readabilityData.RenderHTML(&articleBuf); err != nil {
				return models.NewAPIError(http.StatusInternalServerError, fmt.Errorf("render article HTML: %w", err))
			}

			// Set the article content to the extracted content.
			article.Content = new(sanitization.SanitizeHTML(articleBuf.String()))
		}
	}

	return nil
}

func ArticleFiltersQueryClause(filters *models.ArticleFilters) search.BoolQueryOption {
	if filters == nil {
		return nil
	}
	if filters.IsEmpty() {
		return nil
	}
	clauses := make([]search.QueryOption, 0)
	if filters.Text != nil && *filters.Text != "" {
		clauses = append(clauses,
			search.SimpleQueryStringQuery(
				*filters.Text,
				[]string{"title", "description", "content"},
				search.WithSimpleQueryStringQueryOperator(search.OperatorAnd),
			),
		)
	}
	if filters.Authors != nil && *filters.Authors != "" {
		clauses = append(clauses,
			search.SimpleQueryStringQuery(
				*filters.Authors,
				[]string{"authors", "contributors"},
				search.WithSimpleQueryStringQueryOperator(search.OperatorAnd),
			),
		)
	}
	if filters.Categories != nil && *filters.Categories != "" {
		clauses = append(clauses,
			search.SimpleQueryStringQuery(
				*filters.Categories,
				[]string{"categories"},
				search.WithSimpleQueryStringQueryOperator(search.OperatorAnd),
			),
		)
	}
	return search.WithBoolMust(clauses...)
}
