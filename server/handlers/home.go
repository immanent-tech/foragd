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

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/providers/elastic/results"
	"github.com/immanent-tech/foragd/web/templates"
)

// Home contains data for generating a user home page.
type Home struct {
	title string
	data  *models.HomeResponse
}

// FullResponse renders a full page (headers, footers and data).
func (p *Home) FullResponse(res http.ResponseWriter, req *http.Request) {
	user := models.UserFromCtx(req.Context())
	if user == nil {
		HandleInternalError(&models.APIError{
			InternalError: fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound),
			StatusCode:    http.StatusInternalServerError,
			UserMessage: models.NewErrorMessage(
				"Cannot show home.",
				"The backend produced an error. This might be temporary, please try again.",
			),
		}).ServeHTTP(res, req)
		return
	}

	switch user.GetSettings().ShowOnboarding {
	case true:
		templ.Handler(
			templates.CreatePage(templates.NewUserHome(),
				templates.WithPageTitle(p.title),
			)).ServeHTTP(res, req)
	case false:
		templ.Handler(
			templates.CreatePage(templates.UserHome(p.data),
				templates.WithPageTitle(p.title),
			)).ServeHTTP(res, req)
	}
}

// PartialResponse will render just the data.
func (p *Home) PartialResponse(res http.ResponseWriter, req *http.Request) {
	user := models.UserFromCtx(req.Context())
	if user == nil {
		HandleInternalError(&models.APIError{
			InternalError: fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound),
			StatusCode:    http.StatusInternalServerError,
			UserMessage: models.NewErrorMessage(
				"Cannot show home.",
				"The backend produced an error. This might be temporary, please try again.",
			),
		}).ServeHTTP(res, req)
		return
	}

	res.Header().Set(htmx.HeaderPushURL, req.URL.String())

	switch user.GetSettings().ShowOnboarding {
	case true:
		templ.Handler(
			templates.NewUserHome(),
			templ.WithFragments(templates.ContentFragment),
		).ServeHTTP(res, req)
	case false:
		templ.Handler(
			templates.UserHome(p.data),
			templ.WithFragments(templates.ContentFragment),
		).ServeHTTP(res, req)
	}
	// Update title, dock/sidebar.
	templ.Handler(templates.UpdateTitle(p.title)).ServeHTTP(res, req)
	templ.Handler(templates.SideBar(templ.Attributes{"hx-swap-oob": "true"})).ServeHTTP(res, req)
	templ.Handler(templates.Dock(templ.Attributes{"hx-swap-oob": "true"})).ServeHTTP(res, req)
}

// HandleHome handles displaying the user's home page.
func HandleHome() http.HandlerFunc {
	return userContentHandlerChain.
		ThenFunc(func(res http.ResponseWriter, req *http.Request) {
			data, err := getHomePageData(req.Context())
			if err != nil && !errors.Is(err, models.ErrNotFound) {
				HandleInternalError(&models.APIError{
					InternalError: fmt.Errorf("run data collection: %w", err),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Could not display home page",
						"This might be temporary, please try again.",
					),
				}).ServeHTTP(res, req)
			}
			page := &Home{
				title: "Home",
				data:  data,
			}

			RenderInternalPage(page).ServeHTTP(res, req)
		}).
		ServeHTTP
}

// getHomePageData retrieves the data required to construct the homepage content.
func getHomePageData(ctx context.Context) (*models.HomeResponse, error) {
	data := &models.HomeResponse{
		TopCategories: make(map[models.CategoryCount]models.Articles),
	}

	// Retrieve unread subscriptions and articles.
	subscriptions, articles, err := getHomePageObjects(ctx)
	if err != nil {
		return data, fmt.Errorf("unable to retrieve subscriptions and/or articles: %w", err)
	}

	data.Subscriptions = subscriptions
	data.LatestArticles = articles

	if len(data.Subscriptions) > 0 {
		// Retrieve statistics.
		if err := performHomePageAggs(ctx, data); err != nil {
			return data, fmt.Errorf("unable to perform aggregations: %w", err)
		}
	}

	return data, nil
}

// getHomePageObjects retrieves all unread subscriptions and the latest (max 10) unread articles.
func getHomePageObjects(ctx context.Context) (models.Subscriptions, models.Articles, error) {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return nil, nil, fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound)
	}

	var (
		subscriptions models.Subscriptions
		articles      models.Articles
		err           error
	)

	// Fetch unread subscriptions.
	subscriptions, err = user.GetSubscriptions(ctx, models.WithDynamicInfo(true))
	if err != nil {
		return nil, nil, fmt.Errorf("get subscriptions: %w", err)
	}
	subscriptions = subscriptions.FilterByView(models.ViewUnread).Sort(models.SortNewestFirst)

	// // Fetch articles for unread subscriptions.
	// if len(subscriptions) > 0 {
	// 	articleQuery := query.Bool(
	// 		query.Filter(
	// 			query.Terms("feed_id", subscriptions.GetFeedIDs()...),
	// 			query.Terms("categories.raw", filters.GetCategories()...),
	// 			query.Bool(
	// 				query.Should(models.BuildItemQueries(user, filters.GetView(), subscriptions)...),
	// 			),
	// 		),
	// 	)

	// 	sort := filters.GetSort()

	// 	// Find items matching filters.
	// 	items, _, err := models.SearchItems(ctx, articleQuery, filters.GetCount(), &sort, "")
	// 	if err != nil {
	// 		return nil, nil, fmt.Errorf("get items: %w", err)
	// 	}

	// 	// Generate articles.
	// 	articles, err = models.GenerateArticles(ctx, items)
	// 	if err != nil {
	// 		return nil, nil, fmt.Errorf("generate articles: %w", err)
	// 	}
	// }

	return subscriptions, articles, nil
}

// performHomePageAggs performs aggregations to get statistics and samples of the top unread subscriptions and articles.
func performHomePageAggs(ctx context.Context, data *models.HomeResponse) error {
	// Don't run aggregations if there are no feed subscriptions.
	if len(data.Subscriptions.GetFeedIDs()) == 0 {
		return nil
	}

	// Fetch aggregation data.
	termsField := "categories.raw"
	// Aggregation definition for fetching the top 10 item categories across all subscriptions.
	sampleField := "feed_id"
	defaultMaxDocsPerValue := 1
	shardSize := 200
	topCategoryHitsCount := 3
	topSampleHitsCount := 6
	maxDocCount := int64(3)

	// Perform the request.
	resp, err := elastic.Search[*models.Item](ctx,
		schema.ItemsIndexRO(),
		query.Bool(
			query.Filter(
				query.Terms("feed_id", data.Subscriptions.GetFeedIDs()),
				query.Bool(
					query.Should(
						// Boost items that are from a favorite subscription.
						query.Terms(
							"feed_id",
							data.Subscriptions.FilterByFavorites(true).GetFeedIDs(),
							query.WithQueryName[*query.TermsQuery]("boost-favorites"),
							query.WithQueryBoost[*query.TermsQuery](2.0),
						),
					),
					// On the home page, only show articles which have an image (looks nicer).
					query.Must(
						query.Exists("image.url"),
					),
					query.MustNot(
						query.Term("image.url", ""),
					),
				),
			),
		),
		elastic.WithAggregations(
			elastic.Aggs{
				// top_categories_sample: diversified sampler to ensure top categories not dominated by single overwhelming
				// source.
				"top_categories_sample": types.Aggregations{
					DiversifiedSampler: &types.DiversifiedSamplerAggregation{
						Field:           &sampleField,
						MaxDocsPerValue: &defaultMaxDocsPerValue,
						ShardSize:       &shardSize,
					},
					Aggregations: map[string]types.Aggregations{
						// top_categories: the top categories across all subscriptions.
						"top_categories": {
							Terms: &types.TermsAggregation{
								Field:   &termsField,
								Exclude: slices.Concat(models.CommonCategoryFilters, []string{""}),
							},
							Aggregations: map[string]types.Aggregations{
								"top_article_hits": {
									TopHits: &types.TopHitsAggregation{
										Size: &topCategoryHitsCount,
									},
								},
								// top_articles: the top scoring article for each top category.
								// "top_articles": {
								// 	Filter: query.Build(query.Bool(
								// 		query.MustNot(
								// 			query.Terms("item_id", data.LatestArticles.GetIDs()...),
								// 		),
								// 	)),
								// 	Aggregations: map[string]types.Aggregations{
								// 		"top_article_hits": {
								// 			TopHits: &types.TopHitsAggregation{
								// 				Size: &topHitsCount,
								// 			},
								// 		},
								// 	},
								// },
							},
						},
						"latest_articles_sample": {
							TopHits: &types.TopHitsAggregation{
								Size: &topSampleHitsCount,
							},
						},
					},
				},
				// Aggregation definition for fetching the rare item categories across all subscriptions.
				"rare_categories": types.Aggregations{
					RareTerms: &types.RareTermsAggregation{
						Field:       &termsField,
						MaxDocCount: &maxDocCount,
						Exclude:     models.CommonCategoryFilters,
					},
				},
			},
		),
		elastic.WithSize(0),
		elastic.WithDocSorting(),
	)
	if err != nil {
		return fmt.Errorf("unable to calculate aggregations: %w", err)
	}

	topCategoriesSamplerAgg, found, err := elastic.ExtractAggregation[*types.SamplerAggregate](
		resp.Aggregations,
		"top_categories_sample",
	)
	if !found || err != nil {
		return fmt.Errorf("unable to find required aggregation: %w", err)
	}

	// Get the top categories.
	if topCategoriesAgg, found, err := elastic.ExtractAggregation[*types.StringTermsAggregate](
		topCategoriesSamplerAgg.Aggregations,
		"top_categories",
	); found && err == nil {
		if categoryBuckets, ok := topCategoriesAgg.Buckets.([]types.StringTermsBucket); ok {
			// Generate categories from aggregation.
			for category := range slices.Values(categoryBuckets) {
				var value string
				if value, ok = category.Key.(string); !ok {
					continue
				}
				// Get top articles.
				topHitsAgg, found, err := elastic.ExtractAggregation[*types.TopHitsAggregate](
					category.Aggregations,
					"top_article_hits",
				)
				if !found || err != nil {
					continue
				}
				var items models.Items
				if items, _, err = results.ExtractSourceFromHits[models.Item](topHitsAgg.Hits.Hits); err != nil {
					continue
				}
				var articles models.Articles
				if articles, err = models.GenerateArticles(ctx, items); err != nil {
					continue
				}
				newCategory := models.CategoryCount{Category: value, Count: int(category.DocCount)}
				data.TopCategories[newCategory] = articles
			}
			// // Remove duplicate articles.
			// slices.SortFunc(data.TopArticles, func(a, b *models.Article) int {
			// 	return strings.Compare(a.GetID(), b.GetID())
			// })
		}
	}

	// Get the latest articles.
	if latestArticlesSampleAgg, found, err := elastic.ExtractAggregation[*types.TopHitsAggregate](
		topCategoriesSamplerAgg.Aggregations,
		"latest_articles_sample",
	); found && err == nil {
		var items models.Items
		if items, _, err = results.ExtractSourceFromHits[models.Item](
			latestArticlesSampleAgg.Hits.Hits,
		); err != nil {
			slogctx.FromCtx(ctx).Warn("Could not extract latest items from aggregation.",
				slog.Any("error", err),
			)
		}
		var articles models.Articles
		if articles, err = models.GenerateArticles(ctx, items); err != nil {
			slogctx.FromCtx(ctx).Warn("Could not generate articles from items.",
				slog.Any("error", err),
			)
		}
		data.LatestArticles = articles
	}

	// Get the rare categories.
	if rareCategoriesAgg, found, err := elastic.ExtractAggregation[*types.StringRareTermsAggregate](
		resp.Aggregations,
		"rare_categories",
	); found && err == nil {
		// Generate category counts from buckets.
		if rareCategoryBuckets, ok := rareCategoriesAgg.Buckets.([]types.StringRareTermsBucket); ok {
			data.RareCategories = make(models.CategoryCounts, 0)
			for category := range slices.Values(rareCategoryBuckets) {
				data.RareCategories = append(
					data.RareCategories,
					models.CategoryCount{Category: category.Key, Count: int(category.DocCount)},
				)
			}
			data.RareCategories.Sort()
			if len(data.RareCategories) > 10 {
				data.RareCategories = data.RareCategories[:10]
			}
		}
	}

	return nil
}
