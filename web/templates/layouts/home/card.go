// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import (
	"context"
	"log/slog"

	"github.com/a-h/templ"
	"github.com/joshuar/go-templ-daisyui/display/card"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/web/templates/action"
	"github.com/joshuar/go-feed-me/web/templates/partials"
	"github.com/joshuar/go-feed-me/web/templates/partials/appbar"
)

type FeedCard struct {
	Card
	UnreadCount int
}

type ItemCard struct {
	Card
}

type Article struct {
	models.SourceWithContent
}

type cardActions []templ.Component

// Card is a display component that shows a DaisyUI Card for the given data.
type Card struct {
	id          string
	menuActions cardActions
	models.Source
	*card.Props
}

// AddPagination adds htmx attributes for triggering pagination to a card.
func (c *Card) addPagination(ctx context.Context, pagination models.Pagination, path string) {
	filters := models.FiltersFromCtx(ctx)
	link := models.NewPageView(path, &filters)
	c.AddAttributes(link.AsAction(
		action.WithAttributes(templ.Attributes{
			"hx-trigger":     "intersect once",
			"hx-swap":        "afterend",
			"hx-replace-url": "false",
			"hx-indicator":   "#content-loading",
		}),
		action.WithParam(models.ParamPagination, pagination),
	).Attributes())
}

func BuildFeedsLayout(ctx context.Context, pagination models.Pagination, subscriptions models.Subscriptions) *HomeLayout {
	cards := GenerateFeedCards(ctx, subscriptions, pagination)
	// Generate backlink.
	backlink := models.GetBacklink(ctx, models.SessionFromCtx(ctx), models.FeedsRoute)
	return &HomeLayout{
		Title:   "Feeds",
		Content: cards,
		FooterContent: []templ.Component{
			partials.UpdateBacklink(backlink),
			UpdateFilters(models.FeedsRoute, subscriptions.GetCategoryCounts()),
			UpdateSorting(models.FeedsRoute),
			UpdateActions(
				AddSubscriptionAction(),
				ImportAction(),
				MarkAllFeedsAction(ctx),
			),
		},
		Header: appbar.AppBar().Show(),
	}
}

func GenerateFeedCards(ctx context.Context, subscriptions models.Subscriptions, pagination models.Pagination) []templ.Component {
	cards := make([]templ.Component, 0, len(subscriptions))
	for idx, subscription := range subscriptions {
		card := newFeedCard(ctx, subscription)
		if idx == len(subscriptions)-1 && len(subscriptions) == models.FiltersFromCtx(ctx).Count {
			card.addPagination(ctx, pagination, models.FeedsRoute)
		}

		cards = append(cards, card.Show())
	}
	return cards
}

func BuildItemsLayout(ctx context.Context, pagination models.Pagination, items models.Items) *HomeLayout {
	cards := GenerateItemCards(ctx, items, pagination)
	// Generate backlink.
	backlink := models.GetBacklink(ctx, models.SessionFromCtx(ctx), models.ItemsRoute)
	// Return the home items layout.
	return &HomeLayout{
		Title:   "Items",
		Content: cards,
		FooterContent: []templ.Component{
			partials.UpdateBacklink(backlink),
			UpdateFilters(models.ItemsRoute, items.GetCategoryCounts()),
			UpdateSorting(models.ItemsRoute),
			UpdateActions(
				AddSubscriptionAction(),
				ImportAction(),
				MarkAllItemsAction(ctx, items.GetFeedIDs()),
			),
		},
		Header: appbar.AppBar().Show(),
	}
}

func GenerateItemCards(ctx context.Context, items models.Items, pagination models.Pagination) []templ.Component {
	cards := make([]templ.Component, 0, len(items))
	for idx, item := range items {
		slogctx.FromCtx(ctx).Debug("displaying item", slog.String("item_id", item.GetID()), slog.Bool("state", item.IsUnread()))
		// Create a card for this item.
		itemCard := newItemCard(ctx, item)
		// Add a pagination action to the last item.
		if idx == len(items)-1 && len(items) == models.FiltersFromCtx(ctx).Count {
			itemCard.addPagination(ctx, pagination, models.ItemsRoute)
		}
		// Append the card to the list of cards.
		cards = append(cards, itemCard.Show())
	}
	return cards
}

func BuildArticleLayout(ctx context.Context, item *models.Item) *HomeLayout {
	backlink := models.GetBacklink(ctx, models.SessionFromCtx(ctx), "/home/"+item.GetFeedID()+"/"+item.GetID())
	return &HomeLayout{
		Title:   item.GetTitle(),
		Header:  appbar.AppBar().Show(),
		Content: []templ.Component{GenerateArticle(item)},
		FooterContent: []templ.Component{
			partials.UpdateBacklink(backlink),
		},
	}
}

func GenerateArticle(item *models.Item) templ.Component {
	article := &Article{SourceWithContent: item}
	return article.Show()
}
