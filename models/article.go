// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"slices"
)

// Articles is a slices of individual articles.
type Articles []*Article

// GetItems retrieves all the items for the articles.
func (a Articles) GetItems() Items {
	items := make(Items, 0, len(a))
	for article := range slices.Values(a) {
		items = append(items, article.Item)
	}
	return items
}

func (a Articles) GetSubscriptionIDs() []SubscriptionID {
	subIDs := make([]SubscriptionID, 0, len(a))
	for article := range slices.Values(a) {
		subIDs = append(subIDs, article.SubscriptionID)
	}
	return slices.Compact(subIDs)
}

func (a *Article) HasState() bool {
	return a.State != ""
}

func (a *Article) IsUnread() bool {
	if !a.HasState() {
		return true
	}
	return a.State == StateUnread
}

func (a *Article) GetUserState() State {
	if a.HasState() {
		return a.State
	}
	return StateUnread
}

func ArticleFromItem(item *Item) *Article {
	return &Article{
		FeedID: item.GetFeedID(),
		ItemID: item.GetID(),
		Item:   item,
	}
}

// GenerateArticles creates a list of article objects from the given items, populating them with appropriate user data.
func GenerateArticles(user *User, items ...*Item) Articles {
	articles := make(Articles, 0, len(items))
	subscriptionsByFeed := user.GetSubscriptions().ByFeed()
	for item := range slices.Values(items) {
		article := ArticleFromItem(item)
		article.State = subscriptionsByFeed[item.GetFeedID()].GetItemState(item.GetID())
		article.SubscriptionID = subscriptionsByFeed[item.GetFeedID()].GetID()
		articles = append(articles, article)
	}
	return articles
}
