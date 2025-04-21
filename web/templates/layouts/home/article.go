// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import (
	"net/http"

	"github.com/a-h/templ"

	"github.com/joshuar/go-feed-me/models"
)

// Article is a display component that shows an article for the given data.
type Article struct {
	models.SourceWithContent
}

func BuildArticle(req *http.Request, item *models.Item) templ.Component {
	article := &Article{
		SourceWithContent: item,
	}
	return article.Render(req)
}
