// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package partials

import (
	"github.com/a-h/templ"
	"github.com/joshuar/go-templ-daisyui/display/article"

	"github.com/joshuar/go-feed-me/internal/models"
)

type Article struct {
	*article.Props
}

func NewArticle(item models.APIItem) (*Article, error) {
	props := &Article{}
	props.Props = article.Build(
		showItemArticle(item),
		article.WithExtraAttributes(templ.Attributes{
			"hx-post":     "/home/" + item.GetFeedID() + "/" + item.GetID() + "/read",
			"hx-swap":     "none",
			"hx-push-url": "false",
			"hx-trigger":  "load delay:2s",
		}),
	)

	return props, nil
}
