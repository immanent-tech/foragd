// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/providers/elastic/results"
	"github.com/immanent-tech/foragd/service"
	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/element"
)

const (
	RouteHome Route = "/home"
)

// Home contains data for generating a user home page.
type Home struct {
	title string
	data  *templates.HomeData
}

// FullResponse renders a full page (headers, footers and data).
func (p *Home) FullResponse(res http.ResponseWriter, req *http.Request) {
	user := models.UserFromCtx(req.Context())
	if user == nil {
		HandleInternalError(req.URL.Path,
			&models.APIError{
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
		HandleInternalError(req.URL.Path,
			&models.APIError{
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
	templ.Handler(templates.SideBar(element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
	templ.Handler(templates.Dock(element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
}

// HandleHome handles displaying the user's home page.
func HandleHome() http.HandlerFunc {
	return internalPageHandlerChain.
		ThenFunc(func(res http.ResponseWriter, req *http.Request) {
			user := models.UserFromCtx(req.Context())
			if user == nil {
				HandleInternalError(req.URL.Path,
					&models.APIError{
						InternalError: fmt.Errorf("get user: %w", models.ErrCtxValueNotFound),
						StatusCode:    http.StatusInternalServerError,
						UserMessage: models.NewErrorMessage(
							"Could not display home page",
							"This might be temporary, please try again.",
						),
					}).ServeHTTP(res, req)
				return
			}

			start := time.Now()
			// Get subscriptions.
			subscriptions, err := service.GetAllSubscriptions(req.Context())
			if err != nil && !errors.Is(err, models.ErrNotFound) {
				HandleInternalError(req.URL.Path,
					&models.APIError{
						InternalError: fmt.Errorf("run data collection: %w", err),
						StatusCode:    http.StatusInternalServerError,
						UserMessage: models.NewErrorMessage(
							"Could not display home page",
							"This might be temporary, please try again.",
						),
					}).ServeHTTP(res, req)
				return
			}
			// Update subscription dynamic info.
			if err = service.UpdateSubscriptionDynamicInfo(req.Context(), subscriptions); err != nil {
				HandleInternalError(req.URL.Path,
					&models.APIError{
						InternalError: fmt.Errorf("run data collection: %w", err),
						StatusCode:    http.StatusInternalServerError,
						UserMessage: models.NewErrorMessage(
							"Could not display home page",
							"This might be temporary, please try again.",
						),
					}).ServeHTTP(res, req)
				return
			}
			// Filter by unread and sort by last update.
			subscriptions = subscriptions.
				FilterByView(models.ViewUnread).
				Sort(models.SortNewestFirst)
			if len(subscriptions) > 10 {
				subscriptions = subscriptions[:10]
			}
			slogctx.FromCtx(req.Context()).Debug("Retrieved user subscriptions.",
				slog.Duration("took", time.Since(start)))

			// Create an object to hold the data.
			data := &templates.HomeData{
				Subscriptions: subscriptions,
			}

			// Perform some aggregations for the subscriptions.
			if len(data.Subscriptions.GetFeedIDs()) > 0 {
				// Fetch aggregation data.
				termsField := "categories.raw"
				// Aggregation definition for fetching the top 10 item categories across all subscriptions.
				sampleField := "feed_id"
				defaultMaxDocsPerValue := 1
				shardSize := 200
				topCategoryHitsCount := 3
				lastestArticlesCount := 10
				maxDocCount := int64(3)

				// Perform the request.
				resp, err := elastic.Search[*models.Item](
					req.Context(),
					schema.ItemsIndexRO(),
					// Query is adapted from service.FilterArticles query to boost favorites and return unread only.
					query.Bool(
						query.Filter(
							query.Terms(
								"feed_id",
								data.Subscriptions.GetFeedIDs(),
								query.WithQueryName[*query.TermsQuery]("match-feed-id"),
							),
							query.Bool(
								query.Should(service.BuildItemQueries(user, models.ViewUnread, data.Subscriptions)...),
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
											Size: &lastestArticlesCount,
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
					slogctx.FromCtx(req.Context()).Warn("Unable to run aggregations.",
						slog.Any("error", err),
					)
					RenderInternalPage(&Home{
						title: "Home",
						data:  data,
					}).ServeHTTP(res, req)
					return
				}

				// slogctx.FromCtx(req.Context()).Debug("Performed home aggregations.",
				// 	slog.Duration("took", time.Since(start)))

				topCategoriesSamplerAgg, found, err := elastic.ExtractAggregation[*types.SamplerAggregate](
					resp.Aggregations,
					"top_categories_sample",
				)
				if !found || err != nil {
					slogctx.FromCtx(req.Context()).Warn("Unable to run extract aggregation.",
						slog.String("aggregation", "top_categories_sample"),
						slog.Any("error", err),
					)
					RenderInternalPage(&Home{
						title: "Home",
						data:  data,
					}).ServeHTTP(res, req)
					return
				}

				var wg sync.WaitGroup

				wg.Go(func() {
					// topCategoriesStart := time.Now()
					// Get the top categories.
					if topCategoriesAgg, foundTopCategoriesAgg, err := elastic.ExtractAggregation[*types.StringTermsAggregate](
						topCategoriesSamplerAgg.Aggregations,
						"top_categories",
					); foundTopCategoriesAgg &&
						err == nil {
						if categoryBuckets, ok := topCategoriesAgg.Buckets.([]types.StringTermsBucket); ok {
							// Generate categories from aggregation.
							for category := range slices.Values(categoryBuckets) {
								var value string
								if value, ok = category.Key.(string); !ok {
									continue
								}
								// // Get top articles.
								// topHitsAgg, found, err := elastic.ExtractAggregation[*types.TopHitsAggregate](
								// 	category.Aggregations,
								// 	"top_article_hits",
								// )
								// if !found || err != nil {
								// 	continue
								// }
								// var items models.Items
								// if items, _, err = results.ExtractSourceFromHits[*models.Item](
								// 	topHitsAgg.Hits.Hits,
								// ); err != nil {
								// 	continue
								// }
								// var articles models.Articles
								// if articles, err = service.GenerateArticles(req.Context(), items); err != nil {
								// 	continue
								// }
								newCategory := models.CategoryCount{Category: value, Count: int(category.DocCount)}
								data.TopCategories = append(data.TopCategories, newCategory)
							}
							// // Remove duplicate articles.
							// slices.SortFunc(data.TopArticles, func(a, b *models.Article) int {
							// 	return strings.Compare(a.GetID(), b.GetID())
							// })
						}
					}
					// slogctx.FromCtx(req.Context()).Debug("Extracted top categories.",
					// 	slog.Duration("took", time.Since(topCategoriesStart)))
				})

				wg.Go(func() {
					// latestArticlesStart := time.Now()
					// Get the latest articles.
					if latestArticlesSampleAgg, found, err := elastic.ExtractAggregation[*types.TopHitsAggregate](
						topCategoriesSamplerAgg.Aggregations,
						"latest_articles_sample",
					); found && err == nil {
						var items models.Items
						if items, _, err = results.ExtractSourceFromHits[*models.Item](
							latestArticlesSampleAgg.Hits.Hits,
						); err != nil {
							slogctx.FromCtx(req.Context()).Warn("Could not extract latest items from aggregation.",
								slog.Any("error", err),
							)
						}
						var articles models.Articles
						if articles, err = service.GenerateArticles(req.Context(), items); err != nil {
							slogctx.FromCtx(req.Context()).Warn("Could not generate articles from items.",
								slog.Any("error", err),
							)
						}
						data.LatestArticles = articles
					}
					// slogctx.FromCtx(req.Context()).Debug("Extracted latest articles.",
					// 	slog.Duration("took", time.Since(latestArticlesStart)))
				})

				wg.Go(func() {
					// rareAggsStart := time.Now()
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
					// slogctx.FromCtx(req.Context()).Debug("Extracted rare categories.",
					// 	slog.Duration("took", time.Since(rareAggsStart)))
				})

				wg.Wait()
			}

			slogctx.FromCtx(req.Context()).Debug("Extracted home aggregations data.",
				slog.Duration("took", time.Since(start)))

			RenderInternalPage(&Home{
				title: "Home",
				data:  data,
			}).ServeHTTP(res, req)
		}).
		ServeHTTP
}
