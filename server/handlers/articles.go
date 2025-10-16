// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-shiori/go-readability"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/validation"
)

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
