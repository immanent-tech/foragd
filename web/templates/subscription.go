// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package templates

import (
	"net/http"

	"github.com/a-h/templ"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/htmx"
	"github.com/immanent-tech/foragd/web/templates/partials"
)

type Subscription struct {
	*models.Subscription
}

func (s *Subscription) viewAttributes() templ.Attributes {
	var (
		path string
		vals any
	)
	switch s.GetSubscriptionType() {
	case models.SubscriptionTypeFeed, models.SubscriptionTypeEmail:
		filters := models.NewListDisplayFilters()
		filters.Subscriptions = []string{s.GetID()}
		if s.GetStats().IsUnread() {
			filters.View = models.ViewUnread
		} else {
			filters.View = models.ViewRead
		}
		values := filters.Values()
		values[models.ParamSubscriptionID] = s.GetID()
		vals = values
		path = "/list/articles"
	case models.SubscriptionTypeGroup:
		filters := models.NewListDisplayFilters()
		filters.Subscriptions = s.GroupData.Subscriptions
		if s.GetStats().IsUnread() {
			filters.View = models.ViewUnread
		} else {
			filters.View = models.ViewRead
		}
		values := filters.Values()
		values[models.ParamSubscriptionID] = s.GetID()
		vals = values
		path = "/list/articles"
	case models.SubscriptionTypeSearch:
		s.SearchData.Search.SubscriptionID = new(s.GetID())
		vals = s.SearchData.Search
		path = "/search"
	}
	return htmx.NewAttributes(
		htmx.WithHXMethod(http.MethodGet, path),
		htmx.WithHXTarget(ContentID.Target()),
		htmx.WithHXSwap("innerHTML scroll:#content:top transition:true"),
		htmx.WithHXPushURL(),
		htmx.WithHXTrigger("click consume"),
		htmx.WithHXVals(vals),
	).GetAttributes()
}

func (s *Subscription) markAttributes(view models.View) templ.Attributes {
	// var mark models.Mark
	// if s.GetStats().IsUnread() {
	// 	mark = models.MarkRead
	// } else {
	// 	mark = models.MarkUnread
	// }
	return htmx.NewAttributes(
		htmx.WithHXMethod(http.MethodPost, "/mark/subscription/"+s.GetID()),
		htmx.WithHXSwap("none"),
		htmx.WithHXInclude("[id='"+s.GetID()+"-mark-action']"),
		htmx.WithHXVals(map[string]string{
			"subscription_id": s.GetID(),
			"view":            string(view),
			// "mark":            string(mark),
		}),
	).GetAttributes()
}

func (s *Subscription) favoriteAttributes() templ.Attributes {
	return htmx.NewAttributes(
		htmx.WithHXMethod(http.MethodPost, "/favorite/subscription/"+s.GetID()),
		htmx.WithHXSwap("none"),
		htmx.WithHXVals(map[string]string{
			"subscription_id": s.GetID(),
		}),
	).GetAttributes()
}

func (s *Subscription) editAttributes(path string) templ.Attributes {
	return htmx.NewAttributes(
		htmx.WithHXMethod(http.MethodGet, "/subscription/edit/"+s.GetID()),
		htmx.WithHXTarget(ContentID.Target()),
		htmx.WithHXSwap("innerHTML scroll:#content:top transition:true"),
		htmx.WithHXTrigger("click consume"),
		htmx.WithHXVals(map[string]string{"from": path}),
	).GetAttributes()
}

func (s *Subscription) unsubscribeAttributes() templ.Attributes {
	return htmx.NewAttributes(
		htmx.WithHXMethod(http.MethodPost, "/remove/subscription/"+s.GetID()),
		htmx.WithHXTarget(partials.ModalContainerID.Target()),
		htmx.WithHXVals(map[string]string{
			"nickname":  s.GetTitle(),
			"confirmed": "false",
		}),
		htmx.WithHXTrigger("click consume"),
	).GetAttributes()
}

func (s *Subscription) reportIssueAttributes() templ.Attributes {
	return htmx.NewAttributes(
		htmx.WithHXMethod(http.MethodGet, "/issue"),
		htmx.WithHXTarget(ContentID.Target()),
		htmx.WithHXReplaceURL(),
		htmx.WithHXVals(map[string]any{
			"object_id": s.GetID(),
		}),
		htmx.WithHXTrigger("click consume"),
	).GetAttributes()
}
