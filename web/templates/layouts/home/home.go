// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import (
	"net/http"
	"net/url"

	"github.com/a-h/templ"

	"github.com/joshuar/go-templ-daisyui/attributes"

	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/web/templates"
)

// ContentID is the id attribute for the main content area.
var ContentID = attributes.ID("content")

// viewAttributes are the common htmx attributes for view actions.
var viewAttributes = templ.Attributes{
	"hx-target":   ContentID.Target(),
	"hx-push-url": "true",
	"hx-swap":     "morph:innerHTML",
}

// markAttributes are the common htmx attributes for mark actions.
var markAttributes = templ.Attributes{
	"_":           "on click halt the event's bubbling",
	"hx-push-url": "false",
	"hx-target":   ContentID.Target(),
}

func buildMarkFeedAction(feedID models.FeedID, mark models.Mark) *templates.Action {
	return templates.BuildAction(models.FeedsRoute,
		templates.WithAttributes(markAttributes),
		templates.WithMethod(http.MethodPost),
		templates.WithQueryParam("mark", string(mark)),
		templates.WithQueryParams(url.Values{models.ParamFeeds: []string{feedID}}),
	)
}

func buildMarkItemAction(feedID models.FeedID, itemID models.ItemID, mark models.Mark) *templates.Action {
	return templates.BuildAction(models.ItemsRoute,
		templates.WithAttributes(markAttributes),
		templates.WithMethod(http.MethodPost),
		templates.WithQueryParam("mark", string(mark)),
		templates.WithQueryParams(url.Values{
			models.ParamFeedID: []string{feedID},
			models.ParamItems:  []string{itemID},
		},
		),
	)
}
