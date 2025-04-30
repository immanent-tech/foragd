// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import (
	"net/http"

	"github.com/a-h/templ"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/web/templates"
)

// Article is a display component that shows an article for the given data.
type Article struct {
	models.SourceWithContent
}

func BuildArticleLayout(req *http.Request, back templ.Component, item *models.Item) templates.Layout {
	article := &Article{SourceWithContent: item}
	return &HomeLayout{
		title:   item.GetTitle(),
		content: []templ.Component{article.Show()},
		footer:  BuildArticleFooter(item, back).Show(),
	}
}
