// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package partials

import (
	"github.com/joshuar/go-templ-daisyui/display/article"

	"github.com/joshuar/go-feed-me/internal/models"
)

type Article struct {
	*article.Props
}

func NewArticle(item models.APIItem) (*Article, error) {
	props := &Article{}
	props.Props = article.Build(showItemArticle(item))

	return props, nil
}
