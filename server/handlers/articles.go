// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"
	"github.com/go-shiori/go-readability"
	"github.com/justinas/alice"
	"github.com/justinas/nosurf"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/providers/github"
	"github.com/immanent-tech/foragd/server/forms"
	"github.com/immanent-tech/foragd/validation"
	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/layouts"
	"github.com/immanent-tech/foragd/web/templates/partials"
)

// FindSimilarArticles handles finding other articles that are "similar" to a set of given articles.
func (a *API) FindSimilarArticles() http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		itemID := chi.URLParam(req, models.ParamItemID)
		// Build the More Like This query.
		// TODO: tweak values and fields for optimum results matching...
		var (
			minTermFreq   = 1
			maxQueryTerms = 12
		)
		mlt := query.NewMoreLikeThisQuery("similar_to_" + itemID)
		mlt.LikeDocs(itemID)
		mlt.Fields = []string{"title", "categories.raw", "author"}
		mlt.MinTermFreq = &minTermFreq
		mlt.MaxQueryTerms = &maxQueryTerms
		// Query for similar articles.
		items, _, err := a.DataAPI().SearchItems(req.Context(), mlt.ToQueryOption(), 10, nil, nil)
		if err != nil {
			renderPartial(partials.Notification(
				models.NewErrorMessage(
					"Unable to find similar articles",
					"The backend reported an issue. Please try again.",
				), 0,
			))
			return models.NewAPIError(
				fmt.Errorf("unable to find similar articles subscription: %w", err),
				http.StatusUnprocessableEntity)
		}
		// Generate article data.
		articles, err := models.GenerateArticles(req.Context(), items)
		if err != nil {
			renderPartial(partials.Notification(
				models.NewErrorMessage(
					"Unable to find similar articles",
					"The backend reported an issue. Please try again.",
				), 0,
			))
			return models.NewAPIError(
				fmt.Errorf("unable to find similar articles subscription: %w", err),
				http.StatusUnprocessableEntity)
		}
		// Show results.
		template := layouts.SimilarArticles(articles)
		ctx := models.CSRFTokenToCtx(req.Context(), nosurf.Token(req))
		renderPage(template, templates.GeneratePageTitle("Similar Articles")).ServeHTTP(res, req.WithContext(ctx))
		return nil
	})).ServeHTTP
}

// GetArticleIssues handles presenting a form for the user to submit details about subscription issues.
func GetArticleIssues(api *API) http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Get the current URL on which the issue is being reported.
		currentURL, found := htmx.GetCurrentURL(req)
		if !found {
			slogctx.FromCtx(req.Context()).Warn("No HX-Current-URL header found.")
		}
		// Get the item ID.
		id := chi.URLParam(req, models.ParamItemID)
		i, err := models.GetArticles(req.Context(), api.Elastic, id)
		if err != nil || len(i) == 0 {
			msg := models.NewErrorMessage(
				"Unable to create report form.",
				"The backend had issues generating the report form. Please try again.",
			)
			renderPartial(partials.ServerErrorNotification(msg)).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		template := layouts.ReportArticleIssue(i[0], &models.ArticleIssue{PageUrl: currentURL})
		renderPartial(template).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// SubmitSubscriptionIssues handles processing the user submitted subscription issues form.
func SubmitArticleIssues(esapi *API, ghapi *github.Client) http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Validate the subscription issue request.
		request, valid, err := forms.DecodeForm[*models.ArticleIssue](req)
		if err != nil || !valid {
			msg := models.NewErrorMessage(
				"Unable to submit issue.",
				"The backend had issues submitting the report. Please try again.",
			)
			renderPartial(partials.ServerErrorNotification(msg)).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		// Get subscription details.
		id := chi.URLParam(req, models.ParamItemID)
		i, err := models.GetArticles(req.Context(), esapi.Elastic, id)
		if err != nil || len(i) == 0 {
			msg := models.NewErrorMessage(
				"Unable to get subscription details.",
				"The backend had issues processing the data. Please try again.",
			)
			renderPartial(partials.ServerErrorNotification(msg)).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		// Create the issue in Github.
		err = ghapi.CreateArticleIssue(req.Context(), i[0], request)
		if err != nil {
			msg := models.NewErrorMessage(
				"Unable to submit issue.",
				"The backend had issues submitting the report. Please try again.",
			)
			template := templ.Join(
				layouts.ReportArticleIssue(i[0], request),
				partials.ServerErrorNotification(msg),
			)
			renderPartial(template).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		// Force refresh of page.
		msg := models.NewErrorMessage(
			"Thanks for reporting the issue!",
			"We will look into it and implement fixes as appropriate.",
		)
		renderPartial(partials.IssueReportedConfirmation(msg)).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// archiveArticle will index an article into the item archive to avoid deletion.
func (a *API) archiveArticle(ctx context.Context, article *models.Article) error {
	index, err := elastic.ItemsArchiveWriteIndexFromCtx(ctx)
	if err != nil {
		return fmt.Errorf("unable to archive article: %w", err)
	}
	user, err := models.UserFromCtx(ctx)
	if err != nil {
		return fmt.Errorf("unable to archive article: %w", err)
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
	index, err := elastic.ItemsArchiveWriteIndexFromCtx(ctx)
	if err != nil {
		return fmt.Errorf("unable to removed archived article: %w", err)
	}
	user, err := models.UserFromCtx(ctx)
	if err != nil {
		return fmt.Errorf("unable to remove archived article: %w", err)
	}
	// Set up the query to match the user's favorited article.
	query := query.Bool(
		query.Filter(
			query.Term("user_id", user.GetID()),
			query.Term("item_id", id),
		),
	)
	err = elastic.DeleteDocs(ctx, a.DataAPI().GetAPI(), index, query)
	if err != nil {
		return fmt.Errorf("unable to remove archived article: %w", err)
	}
	return nil
}

func fetchArticleRemoteContent(url string) (string, error) {
	remote, err := readability.FromURL(url, 30*time.Second)
	if err != nil {
		return "", fmt.Errorf("failed to parse content for %s, %w", url, err)
	}
	content := validation.SanitizeString(remote.Content)
	return content, nil
}

func generateItemsQuery(ctx context.Context, filters models.Filters, subscriptions ...models.SubscriptionID) (query.Option, error) {
	user, err := models.UserFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("generateItemsQuery: %w", err)
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
