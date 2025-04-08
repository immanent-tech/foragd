// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"cmp"
	"maps"
	"slices"
	"time"

	"github.com/joshuar/go-feed-me/internal/id"
	"github.com/joshuar/go-feed-me/internal/validation"
	"github.com/joshuar/go-feed-me/pkg/feeds/types"
)

// Subscription should satisfy the base "Feed" types so it can be used in place of a Feed object. Effectively, a
// Subscription is a superset of a Feed plus some additional data.
var (
	_ types.ObjectCommon = (*Subscription)(nil)
	_ types.Source       = (*Subscription)(nil)
)

// Subscriptions is a list of subscriptions.
type Subscriptions []*Subscription

// UpdateUnreadCounts takes a map of feed unread counts and adds the count to
// each feed.
func (s Subscriptions) UpdateUnreadCounts(unreadCounts map[FeedID]int) {
	for subscription := range slices.Values(s) {
		if count, found := unreadCounts[subscription.GetFeedID()]; found {
			subscription.SetUnreadCount(count)
		}
	}
}

// ByFeed returns a map of Subscriptions by FeedID.
func (s Subscriptions) ByFeed() map[FeedID]*Subscription {
	return SliceToMap(s, func(v *Subscription) (FeedID, *Subscription) {
		return v.GetFeedID(), v
	})
}

// GetFeedIDs returns all the FeedIDs for the Subscriptions.
func (s Subscriptions) GetFeedIDs() []FeedID {
	feedIDs := make([]FeedID, 0, len(s))
	for subscription := range slices.Values(s) {
		feedIDs = append(feedIDs, subscription.GetFeedID())
	}
	return feedIDs
}

// GetCategories returns all the Categories for the Subscriptions.
func (s Subscriptions) GetCategories() []string {
	var categories []string
	for subscription := range slices.Values(s) {
		categories = append(categories, subscription.GetCategories()...)
	}
	slices.Sort(categories)
	return slices.Compact(categories)
}

// GetCategoryCounts returns a count of the occurrence of a Category across all
// the Subscriptions.
func (s Subscriptions) GetCategoryCounts() CategoryCounts {
	countsMap := make(map[Category]int)
	for subscription := range slices.Values(s) {
		for category := range slices.Values(subscription.GetCategories()) {
			countsMap[category]++
		}
	}
	var counts CategoryCounts
	for category, count := range maps.All(countsMap) {
		counts = append(counts, CategoryCount{Category: category, Count: count})
	}

	return counts
}

// FilterByFeedID will filter the list of subscriptions by the given
// FeedIDs. If no IDs are provided, the full list is returned.
func (s Subscriptions) FilterByFeedID(feedIDs ...FeedID) Subscriptions {
	if len(feedIDs) > 0 {
		return slices.Collect(FilterSlice(s, func(v *Subscription) bool {
			return slices.Contains(feedIDs, v.GetFeedID())
		}))
	}
	return s
}

// FilterByCategory will filter the list of subscriptions by the given
// Categories. If no categories are provided, the full list is returned.
func (s Subscriptions) FilterByCategory(categories ...Category) Subscriptions {
	if len(categories) > 0 {
		return slices.Collect(FilterSlice(s, func(v *Subscription) bool {
			var hasCategory bool
			for subscriptionCategory := range slices.Values(v.GetCategories()) {
				if slices.Contains(categories, subscriptionCategory) {
					hasCategory = true
				}
			}
			return hasCategory
		}))
	}
	return s
}

// FilterByUnread will filter subscriptions by those that have unread items.
func (s Subscriptions) FilterByUnread() Subscriptions {
	return slices.Collect(FilterSlice(s, func(v *Subscription) bool {
		return v.GetUnreadCount() > 0
	}))
}

// FilterByRead will filter subscriptions by those that are read.
func (s Subscriptions) FilterByRead() Subscriptions {
	return slices.Collect(FilterSlice(s, func(v *Subscription) bool {
		return v.GetUnreadCount() == 0
	}))
}

// Valid returns a boolean indicating if the Subscription contains valid data (true). If it contains invalid data
// (false) a non-nil error is also returned which contains validation issues.
func (s *Subscription) Valid() (bool, error) {
	if valid, err := validation.ValidateStruct(s); err != nil || !valid {
		return false, NewMessage("subscription is invalid", MessageStatusError, WithError(err))
	}
	if s.Feed == nil {
		return false, NewMessage("subscription is invalid", MessageStatusError)
	}
	return true, nil
}

// GetFeedID retrieves the FeedID associated with the subscription.
func (s *Subscription) GetFeedID() FeedID {
	return s.FeedID
}

// GetTitle returns first found of either the nickname of the subscription or the feed title, in that order.
func (s *Subscription) GetTitle() string {
	if s.UserNickname != "" {
		return s.UserNickname
	}
	return s.Feed.GetTitle()
}

// GetDescription retrieves the description (if any) of the feed.
func (s *Subscription) GetDescription() string {
	return s.Feed.GetDescription()
}

// GetCategories retrieves any custom categories set by the user of nil if
// unset.
func (s *Subscription) GetCategories() []Category {
	categories := slices.Concat(s.UserCategories, s.Feed.GetCategories())
	slices.Sort(categories)
	return slices.Compact(categories)
}

// GetAuthors returns any feed authors.
func (s *Subscription) GetAuthors() []string {
	return s.Feed.GetAuthors()
}

// GetContributors returns any feed contributors.
func (s *Subscription) GetContributors() []string {
	return s.Feed.GetContributors()
}

// GetImage returns any image of the feed.
func (s *Subscription) GetImage() *types.Image {
	return s.Feed.GetImage()
}

// GetLanguage returns the feed language.
func (s *Subscription) GetLanguage() string {
	return s.Feed.GetLanguage()
}

// GetRights returns any copyright or rights notes associated with the feed.
func (s *Subscription) GetRights() string {
	return s.Feed.GetRights()
}

// GetLink retrieves the link to the webpage source of the feed.
func (s *Subscription) GetLink() string {
	return s.Feed.GetLink()
}

// GetSourceURL retrieves the link to the source feed.
func (s *Subscription) GetSourceURL() string {
	return s.Feed.GetSourceURL()
}

// GetPublishedDate retrieves the last published date of the feed.
func (s *Subscription) GetPublishedDate() time.Time {
	return s.Feed.GetPublishedDate()
}

// GetUpdatedDate retrieves the last updated date of the feed.
func (s *Subscription) GetUpdatedDate() time.Time {
	return s.Feed.GetUpdatedDate()
}

// SetUnreadCount sets the count of unread items for the subscription feed.
func (s *Subscription) SetUnreadCount(count int) {
	s.UnreadCount = count
}

// GetUnreadCount retrieves the count of unread items for the subscription feed.
func (s *Subscription) GetUnreadCount() int {
	return s.UnreadCount
}

// GetName retrieves any custom name set by the user or an empty string if unset.
func (s *Subscription) GetName() string {
	if s.UserNickname != "" {
		return s.UserNickname
	}
	return s.Feed.GetTitle()
}

// GetMarkedRead retrieves the timestamp when the user last marked the
// subscription feed as read.
func (s *Subscription) GetMarkedRead() time.Time {
	return s.MarkedRead
}

// GetUnreadItems retrieves a list of ItemIDs for the subscription feed that
// user has explicitly marked as unread.
func (s *Subscription) GetUnreadItems() []ItemID {
	return s.UnreadItems
}

// GetReadItems retrieves a list of ItemIDs for the subscription feed that
// user has explicitly marked as read.
func (s *Subscription) GetReadItems() []ItemID {
	return s.ReadItems
}

// GetItemState retrieves the item state (read/unread/saved) from the
// subscription. By default it will return unread unless the user has explicitly
// marked or saved the item.
func (s *Subscription) GetItemState(itemID ItemID) State {
	if idx := slices.IndexFunc(s.UnreadItems, func(v ItemID) bool { return v == itemID }); idx != -1 {
		return StateUnread
	}
	if idx := slices.IndexFunc(s.ReadItems, func(v ItemID) bool { return v == itemID }); idx != -1 {
		return StateRead
	}
	return StateUnread
}

// MarkRead will mark the subscription as read. This involves setting the MarkedRead field to the given value and
// removing any individual unread/read items.
func (s *Subscription) MarkRead(markedAt time.Time) {
	updated := time.Now().UTC()
	s.MarkedRead = markedAt
	s.UnreadItems = nil
	s.ReadItems = nil
	s.UpdatedAt = &updated
}

func (s *Subscription) IsUnread() bool {
	return s.GetUnreadCount() > 0
}

// CompareSubscriptionUnreadCount is a helper function for sorting Subscriptions by unread count, in
// ascending order. If descending order is required, slices.Reverse can be
// called after sorting the slice with this function.
func CompareSubscriptionUnreadCount(a, b *Subscription) int {
	return cmp.Compare(a.GetUnreadCount(), b.GetUnreadCount())
}

// Valid returns a boolean indicating whether the SubscriptionRequest is valid,
// and any validation errors if applicable.
func (r *SubscriptionRequest) Valid() (bool, error) {
	valid, err := validation.ValidateStruct(r)
	if !valid || err != nil {
		return false, NewMessage("Details are invalid", MessageStatusError, WithError(err))
	}
	return true, nil
}

func (r *SubscriptionRequest) GetURL() string {
	return r.URL
}

func (r *SubscriptionRequest) GetID() string {
	return r.SubscriptionID
}

// SubscriptionRequests is a list of subscription requests.
type SubscriptionRequests []*SubscriptionRequest

// URLs extracts and returns a list of URLs from the requests.
func (r SubscriptionRequests) URLs() []URL {
	urls := make([]URL, 0, len(r))
	for request := range slices.Values(r) {
		if url := request.GetURL(); url != "" {
			urls = append(urls, url)
		}
	}
	return urls
}

func NewSubscriptionRequest(url string) *SubscriptionRequest {
	return &SubscriptionRequest{
		SubscriptionID: id.NewID(id.Subscription),
		URL:            url,
	}
}

// NewSubscription creates a Subscription from the request and feed details.
func NewSubscription(request *SubscriptionRequest, feed *Feed) *Subscription {
	return &Subscription{
		CreatedAt:      time.Now().UTC(),
		SubscriptionID: request.SubscriptionID,
		UserCategories: request.UserCategories,
		UserNickname:   request.UserNickname,
		MarkedRead:     UnixEpoch,
		FeedID:         feed.GetID(),
		Feed:           feed,
	}
}
