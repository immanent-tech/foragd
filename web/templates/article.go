// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package templates

import (
	"net/http"

	"github.com/a-h/templ"

	"github.com/immanent-tech/go-base/pkg/htmx"

	"github.com/immanent-tech/foragd/models"
)

type Article struct {
	*models.Article
}

func (a *Article) markValueID() htmx.ID {
	return htmx.ID(a.GetID() + "-mark-value")
}

// markAttributes are the htmx attributes for marking an article.
func (a *Article) markAttributes(view models.View) templ.Attributes {
	return htmx.NewAttributes(
		htmx.WithHXMethod(http.MethodPost, "/mark/article/"+a.GetID()),
		htmx.WithHXSwap("none"),
		htmx.WithHXTrigger("click consume"),
		htmx.WithHXInclude("[id='"+a.markValueID().String()+"']"),
		htmx.WithHXVals(map[string]string{
			"subscription_id": a.GetSubscriptionID(),
			"item_id":         a.GetID(),
			"view":            string(view),
		}),
	).GetAttributes()
}

// favoriteAttributes are the htmx attributes for favoriting an article.
func (a *Article) favoriteAttributes() templ.Attributes {
	return htmx.NewAttributes(
		htmx.WithHXMethod(http.MethodPost, "/favorite/article/"+a.GetID()),
		htmx.WithHXSwap("none"),
		htmx.WithHXTrigger("click consume"),
		htmx.WithHXVals(map[string]any{
			"subscription_id": a.GetSubscriptionID(),
			"item_id":         a.GetID(),
		}),
	).GetAttributes()
}

// findSimilarAttributes are the htmx attributes for finding similar articles to this one.
func (a *Article) findSimilarAttributes() templ.Attributes {
	return htmx.NewAttributes(
		htmx.WithHXMethod(http.MethodGet, "/view/article/"+a.GetID()+"/similar"),
		htmx.WithHXTarget(ContentID.Target()),
		htmx.WithHXSwap("morph:innerHTML transition:true"),
		htmx.WithHXTrigger("click consume"),
		htmx.WithHXVals(map[string]any{"from": "/list/articles"}),
	).GetAttributes()
}

// reportIssueAttributes are the htmx attributes for reporting an issue related to this article.
func (a *Article) reportIssueAttributes() templ.Attributes {
	return htmx.NewAttributes(
		htmx.WithHXMethod(http.MethodGet, "/issue"),
		htmx.WithHXTarget(ContentID.Target()),
		htmx.WithHXReplaceURL(true),
		htmx.WithHXVals(map[string]any{
			"object_id": a.GetID(),
		}),
		htmx.WithHXTrigger("click consume"),
	).GetAttributes()
}
