// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"slices"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic/aggregations"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
	"github.com/joshuar/go-feed-me/web/templates"
	"github.com/joshuar/go-feed-me/web/templates/content"
	"github.com/joshuar/go-feed-me/web/views"
)

// Home handles displaying the user's home page.
func (a *API) Home() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		ctx = context.WithValue(ctx, titleCtxKey, "Go Feed Me Home")
		data, resp := a.getHomePageData(ctx)
		if resp != nil {
			RenderError(res, req, resp)
			return
		}

		chain := alice.New(
			RouteLogger,
			SavePageState(nil),
		)

		// Display content based on request.
		switch {
		case htmx.IsHTMX(req) && !htmx.IsHistoryRestoreRequest(req):
			ctx := templateToCtx(req.Context(), data)
			// Partial update. Only render fragments.
			chain.Then(RenderTemplateFragments("content")).ServeHTTP(res, req.WithContext(ctx))
		default:
			ctx := templateToCtx(req.Context(), templates.NewPage("Go Feed Me Home", templates.WithBodyContent(data)))
			// Full page render.
			chain.Then(RenderTemplate()).ServeHTTP(res, req.WithContext(ctx))
		}
	}
}

// getHomePageData retrieves the data required to construct the home page content.
//
//nolint:funlen
func (a *API) getHomePageData(ctx context.Context) (templ.Component, *models.Response) {
	// Retrieve user object.
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, models.RespErrUnauthorized()
	}
	// User has no subscriptions, show empty page
	if len(user.GetSubscriptionsByID()) == 0 {
		return views.EmptyContent(), nil
	}

	data := &views.HomePage{}

	// Get subscriptions.
	subscriptions, err := a.getSubscriptions(ctx)
	if err != nil {
		return data, models.RespErrBackend(err)
	}
	data.Subscriptions = subscriptions

	// Query definition for fetching unread items for all subscriptions.
	query := query.Bool(
		query.BoolQueryName("item_filters"),
		query.Filter(
			// Must match any of the given feed IDs.
			query.Terms("feed_id", subscriptions.GetStates().GetFeedIDs()...),
			// And should match one feed clause.
			query.Bool(
				query.Should(buildSubscriptionQueries(user, models.ViewUnread)...),
			),
		),
	)

	var aggs []aggregations.Aggregation
	TermsField := "categories.raw"

	// Aggregation definition for fetching the top 10 item categories across all subscriptions.
	SampleField := "feed_id"
	DefaultMaxDocsPerValue := 10
	ShardSize := 1000
	aggs = append(aggs,
		aggregations.Aggregation{
			Name: "top_categories_sample",
			Definition: types.Aggregations{
				DiversifiedSampler: &types.DiversifiedSamplerAggregation{
					Field:           &SampleField,
					MaxDocsPerValue: &DefaultMaxDocsPerValue,
					ShardSize:       &ShardSize,
				},
				Aggregations: map[string]types.Aggregations{
					"top_categories": {
						Terms: &types.TermsAggregation{
							Field: &TermsField,
						},
					},
				},
			},
		},
	)
	// Aggregation definition for fetching the rare item categories across all subscriptions.
	MaxDocCount := int64(5)
	aggs = append(aggs,
		aggregations.Aggregation{
			Name: "rare_categories",
			Definition: types.Aggregations{
				RareTerms: &types.RareTermsAggregation{
					Field:       &TermsField,
					MaxDocCount: &MaxDocCount,
				},
			},
		},
	)

	// Aggregation definition for fetching a set of random items from all subscriptions.
	ItemIDField := "item_id"
	aggs = append(aggs,
		aggregations.Aggregation{
			Name: "random_items",
			Definition: types.Aggregations{
				RandomSampler: &types.RandomSamplerAggregation{
					Probability: 0.1,
				},
				Aggregations: map[string]types.Aggregations{
					"items": {
						Terms: &types.TermsAggregation{
							Field: &ItemIDField,
						},
					},
				},
			},
		},
	)

	// Perform the request.
	aggsResult, resp := a.DataAPI().ItemsAggregation(ctx, query, aggs...)
	if resp != nil {
		return nil, resp
	}
	// Extract aggregations.
	aggregations := aggsResult.Aggregations
	// Use aggregations to generate data.
	data.TopCategories = generateTopCategoriesLinks(ctx, aggregations)
	data.RareCategories = generateRareCategoriesLinks(ctx, aggregations)
	data.RandomArticles = getRandomArticles(ctx, a, aggregations)

	return data, nil
}

func generateTopCategoriesLinks(ctx context.Context, aggs aggregations.AggregationResults) templ.Component {
	if aggs == nil {
		return nil
	}
	// Get the top categories.
	topCategoriesSamplerAgg, err := aggregations.ExtractAggregation[*types.SamplerAggregate](aggs, "top_categories_sample")
	if err != nil {
		slogctx.FromCtx(ctx).Warn("Unable to show top categories.", slog.Any("error", err))
	}
	topCategoriesAgg, err := aggregations.ExtractAggregation[*types.StringTermsAggregate](topCategoriesSamplerAgg.Aggregations, "top_categories")
	if err != nil {
		slogctx.FromCtx(ctx).Warn("Unable to show top categories.", slog.Any("error", err))
	}

	// Generate categories from aggregation.
	var topCategories content.Categories
	for category := range slices.Values(topCategoriesAgg.Buckets.([]types.StringTermsBucket)) {
		topCategories = append(topCategories, content.NewCategoryContent(ctx, category.Key.(string), int(category.DocCount), "/articles"))
	}
	topCategories.Sort()
	// Display list of categories.
	var categoryList []templ.Component
	for category := range slices.Values(topCategories) {
		categoryList = append(categoryList, category.Badge())
	}

	return templ.Join(categoryList...)
}

func generateRareCategoriesLinks(ctx context.Context, aggs aggregations.AggregationResults) templ.Component {
	if aggs == nil {
		return nil
	}
	// Get the rare categories.
	rareCategoriesAgg, err := aggregations.ExtractAggregation[*types.StringRareTermsAggregate](aggs, "rare_categories")
	if err != nil {
		slogctx.FromCtx(ctx).Warn("Unable to show rare categories.", slog.Any("error", err))
	}
	// Generate category counts from buckets.
	var rareCategoryCounts content.Categories
	for category := range slices.Values(rareCategoriesAgg.Buckets.([]types.StringRareTermsBucket)) {
		rareCategoryCounts = append(rareCategoryCounts, content.NewCategoryContent(ctx, category.Key, int(category.DocCount), "/articles"))
	}
	rareCategoryCounts.Sort()

	count := 10
	if len(rareCategoryCounts) < 10 {
		count = len(rareCategoryCounts)
	}
	var categoryList []templ.Component
	for idx := range count {
		categoryList = append(categoryList, rareCategoryCounts[idx].Badge())
	}

	return templ.Join(categoryList...)
}

// getHomePageArticles retrieves a list of articles to display on the home page along with other content.
func getRandomArticles(ctx context.Context, api *API, aggs aggregations.AggregationResults) []templ.Component {
	if aggs == nil {
		return nil
	}
	// Get the rare categories aggregation.
	randomItemsAgg, err := aggregations.ExtractAggregation[map[string]any](aggs, "random_items")
	if err != nil {
		return nil
	}
	// itemsAgg, err := aggregations.ExtractAggregation[*types.StringTermsAggregate](randomItemsAgg, "sterms#items")
	itemsAgg, ok := randomItemsAgg["sterms#items"].(map[string]any)
	if !ok {
		return nil
	}
	// if err != nil {
	// 	return nil, fmt.Errorf("could not get random items: %w", err)
	// }
	buckets, ok := itemsAgg["buckets"].([]any)
	if !ok {
		return nil
	}

	itemIDs := make([]models.ItemID, 0, len(buckets))
	for bucket := range slices.Values(buckets) {
		value, ok := bucket.(map[string]any)
		if !ok {
			continue
		}
		key, ok := value["key"].(string)
		if !ok {
			continue
		}
		itemIDs = append(itemIDs, key)
	}

	articles, resp := api.getArticles(ctx, itemIDs...)
	if resp != nil {
		return nil
	}

	cards := make([]templ.Component, 0, len(articles))
	for article := range slices.Values(articles) {
		cards = append(cards, views.NewArticleContent(article).ShowAsCard())
	}
	return cards
}
