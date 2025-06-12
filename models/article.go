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

// GetSubscriptionIDs retrieves a slice of the SubscriptionID for each article.
func (a Articles) GetSubscriptionIDs() []SubscriptionID {
	subIDs := make([]SubscriptionID, 0, len(a))
	for article := range slices.Values(a) {
		subIDs = append(subIDs, article.SubscriptionID)
	}
	return slices.Compact(subIDs)
}

// HasState returns a boolean indicating whether this article has a user state.
func (a *Article) HasState() bool {
	return a.State != ""
}

// IsUnread returns a boolean indicating whether this article is unread by the user.
func (a *Article) IsUnread() bool {
	if !a.HasState() {
		return true
	}
	return a.State == StateUnread
}

// ArticleFromItem generates an article object from the given item object.
func ArticleFromItem(item *Item) *Article {
	return &Article{
		FeedID: item.GetFeedID(),
		ItemID: item.GetID(),
		Item:   item,
	}
}

// ConvertItemsToArticles creates a list of article objects from the given items, populating them with appropriate user data.
func ConvertItemsToArticles(user *User, items ...*Item) Articles {
	articles := make(Articles, 0, len(items))
	subscriptions := user.GetAllSubscriptions()
	for item := range slices.Values(items) {
		article := ArticleFromItem(item)
		idx := slices.IndexFunc(subscriptions, func(sub *SubscriptionDetails) bool {
			return sub.GetFeedID() == item.GetFeedID()
		})
		if idx == -1 {
			continue
		}
		article.State = subscriptions[idx].GetItemState(item.GetID())
		article.SubscriptionID = subscriptions[idx].GetID()
		// Overwrite the feed title in the item to the subscription nickname, if set.
		if subscriptions[idx].UserNickname != "" {
			article.Item.FeedTitle = subscriptions[idx].UserNickname
		}
		articles = append(articles, article)
	}
	return articles
}
