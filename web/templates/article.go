// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package templates

import (
	"fmt"
	"net/http"

	"github.com/a-h/templ"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/htmx"
	"github.com/immanent-tech/foragd/web/templates/partials"
)

type Article struct {
	*models.Article
}

// markAttributes are the htmx attributes for marking an article.
func (a *Article) markAttributes(view models.View) templ.Attributes {
	var mark models.Mark
	if a.IsUnread() {
		mark = models.MarkRead
	} else {
		mark = models.MarkUnread
	}
	return htmx.NewAttributes(
		htmx.WithHXMethod(http.MethodPost, "/mark/article/"+a.GetID()),
		htmx.WithHXSwap("none"),
		htmx.WithHXTrigger("click consume"),
		htmx.WithHXVals(map[string]string{
			"subscription_id": a.GetSubscriptionID(),
			"item_id":         a.GetID(),
			"view":            string(view),
			"mark":            string(mark),
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

// showContentAttributes are the htmx attributes for showing either the feed or remote article content.
func (a *Article) showContentAttributes(remote bool) templ.Attributes {
	return htmx.NewAttributes(
		htmx.WithHXMethod(http.MethodPost, "/view/article/"+a.GetID()),
		htmx.WithHXTarget(ContentID.Target()),
		htmx.WithHXSwap("innerHTML transition:true"),
		htmx.WithHXTrigger("click consume"),
		htmx.WithHXVals(map[string]any{models.ParamFullArticleContent: remote}),
	).GetAttributes()
}

// viewOriginalAttributes are the attributes for viewing the original (remote) article.
func (a *Article) viewOriginalAttributes() templ.Attributes {
	return templ.Attributes{
		"href":   a.GetLink(),
		"target": "_blank",
		"rel":    "noopener",
		"_":      "on click halt the event's bubbling",
	}
}

// findSimilarAttribuets are the htmx attributes for finding similar articles to this one.
func (a *Article) findSimilarAttributes() templ.Attributes {
	return htmx.NewAttributes(
		htmx.WithHXMethod(http.MethodGet, "/view/article/"+a.GetID()+"/similar"),
		htmx.WithHXTarget(ContentID.Target()),
		htmx.WithHXSwap("innerHTML transition:true"),
		htmx.WithHXTrigger("click consume"),
	).GetAttributes()
}

// mobileShareAttributes are the attributes for sharing an article via the navigator API on mobile.
func (a *Article) mobileShareAttributes() templ.Attributes {
	return templ.Attributes{
		"_": "on click " + fmt.Sprintf(
			"navigator.share({title: %q, url: %q})",
			a.GetTitle(),
			a.GetLink(),
		) + "then halt the event's bubbling",
	}
}

// desktopShareAttributes are the htmx attributes for sharing an article via a modal on desktop.
func (a *Article) desktopShareAttributes() templ.Attributes {
	return htmx.NewAttributes(
		htmx.WithHXMethod(http.MethodPost, "/share/article/"+a.GetID()),
		htmx.WithHXTarget(partials.ModalContainerID.Target()),
		htmx.WithHXTrigger("click consume"),
		htmx.WithHXVals(map[string]string{
			"item_id": a.GetID(),
			"title":   a.GetTitle(),
			"link":    a.GetLink(),
		}),
	).GetAttributes()
}

// reportIssueAttributes are the htmx attributes for reporting an issue related to this article.
func (a *Article) reportIssueAttributes() templ.Attributes {
	return htmx.NewAttributes(
		htmx.WithHXMethod(http.MethodGet, "/issue"),
		htmx.WithHXTarget(ContentID.Target()),
		htmx.WithHXReplaceURL(),
		htmx.WithHXVals(map[string]any{
			"object_id": a.GetID(),
		}),
		htmx.WithHXTrigger("click consume"),
	).GetAttributes()
}
