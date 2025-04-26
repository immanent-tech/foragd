// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/joshuar/go-feed-me/components/validation"
	"github.com/joshuar/go-feed-me/models/feeds/types"
)

// Subscription should satisfy the base "Feed" types so it can be used in place of a Feed object. Effectively, a
// Subscription is a superset of a Feed plus some additional data.
var (
	_ types.ObjectCommon = (*Subscription)(nil)
	_ types.Source       = (*Subscription)(nil)
)

// Subscriptions is a list of subscriptions.
type Subscriptions []*Subscription

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

// Filter will return subscriptions filtered (and sorted) by the given filters.
func (s Subscriptions) Filter(filters Filters) Subscriptions {
	s = s.FilterByFeedID(filters.Feeds...).
		FilterByCategory(filters.Categories...).
		FilterByView(filters.View)
	switch {
	case filters.Sort().SortBy == SortByUnreadCount:
		slices.SortFunc(s, CompareSubscriptionUnreadCount)
	default:
		slices.SortFunc(s, CompareSubscriptionUpdatedDate)
	}
	if filters.Sort().SortOrder == SortOrderDesc {
		slices.Reverse(s)
	}

	return s
}

// FilterByFeed will match the given feeds to the subscriptions and return subscriptions with matched feeds.
func (s Subscriptions) FilterByFeed(feeds Feeds) Subscriptions {
	return slices.Collect(FilterSlice(s, func(subscription *Subscription) bool {
		feed := feeds.FindByID(subscription.GetFeedID())
		if feed != nil {
			subscription.Feed = feed
			return true
		}
		return false
	}))
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

// FilterByView will filter subscriptions to those that match the view filter.
func (s Subscriptions) FilterByView(view View) Subscriptions {
	switch view {
	case ViewRead:
		return slices.Collect(FilterSlice(s, func(v *Subscription) bool {
			return !v.IsUnread()
		}))
	case ViewUnread:
		return slices.Collect(FilterSlice(s, func(v *Subscription) bool {
			return v.IsUnread()
		}))
	}
	return s
}

func (s Subscriptions) FindByURL(url string) *Subscription {
	idx := slices.IndexFunc(s, func(v *Subscription) bool { return v.GetSourceURL() == url })
	if idx == -1 {
		return nil
	}
	return s[idx]
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

func (s *Subscription) String() string {
	if s.GetTitle() != "" {
		return fmt.Sprintf("%s (%s)", s.GetTitle(), s.GetSourceURL())
	}
	return s.GetSourceURL()
}

// GetID retrieves the SubscriptionID.
func (s *Subscription) GetID() SubscriptionID {
	return s.SubscriptionID
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

// IsUnread returns a boolean indicating whether the subscription is considered unread.
func (s *Subscription) IsUnread() bool {
	return s.UnreadCount > 0 || len(s.GetUnreadItems()) > 0
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
	if s.MarkedRead.IsZero() {
		return s.GetMaxHistory()
	}
	return s.MarkedRead
}

// GetMaxHistory returns a timestamp in the past that is the maximum datetime for unread items to be viewed.
func (s *Subscription) GetMaxHistory() time.Time {
	return parseMaxHistory(s.MaxHistory)
}

// GetUnreadItems retrieves a list of ItemIDs for the subscription feed that
// user has explicitly marked as unread.
func (s *Subscription) GetUnreadItems() []ItemID {
	var unreadItems []ItemID //nolint:prealloc // unknown length
	for item := range FilterSlice(s.ItemStates, func(s ItemState) bool {
		return s.State == StateUnread
	}) {
		unreadItems = append(unreadItems, item.ItemID)
	}
	return unreadItems
}

// GetReadItems retrieves a list of ItemIDs for the subscription feed that
// user has explicitly marked as read.
func (s *Subscription) GetReadItems() []ItemID {
	var readItems []ItemID //nolint:prealloc // unknown length
	for item := range FilterSlice(s.ItemStates, func(s ItemState) bool {
		return s.State == StateRead
	}) {
		readItems = append(readItems, item.ItemID)
	}
	return readItems
}

// GetItemState retrieves the item state (read/unread/saved) from the
// subscription. By default it will return unread unless the user has explicitly
// marked or saved the item.
func (s *Subscription) GetItemState(itemID ItemID) State {
	if idx := slices.IndexFunc(s.GetUnreadItems(), func(v ItemID) bool { return v == itemID }); idx != -1 {
		return StateUnread
	}
	if idx := slices.IndexFunc(s.GetReadItems(), func(v ItemID) bool { return v == itemID }); idx != -1 {
		return StateRead
	}
	return StateUnread
}

// MarkRead will mark the subscription as read. This involves setting the MarkedRead field to the given value and
// removing any individual unread/read items.
func (s *Subscription) MarkRead(markedAt time.Time) {
	updated := time.Now().UTC()
	s.MarkedRead = markedAt
	s.ItemStates = nil
	s.UpdatedAt = &updated
}

// MarkItemsRead will mark the given items as read for the subscription.
func (s *Subscription) MarkItemsRead(items ...ItemID) {
	for itemID := range slices.Values(items) {
		idx := slices.IndexFunc(s.ItemStates, func(s ItemState) bool { return s.ItemID == itemID })
		if idx != -1 {
			s.ItemStates[idx].State = StateRead
		} else {
			s.ItemStates = append(s.ItemStates, ItemState{ItemID: itemID, State: StateRead})
		}
	}
}

// MarkItemsUnread will mark the given items as unread for the subscription.
func (s *Subscription) MarkItemsUnread(items ...ItemID) {
	for itemID := range slices.Values(items) {
		idx := slices.IndexFunc(s.ItemStates, func(s ItemState) bool { return s.ItemID == itemID })
		if idx != -1 {
			s.ItemStates[idx].State = StateUnread
		} else {
			s.ItemStates = append(s.ItemStates, ItemState{ItemID: itemID, State: StateUnread})
		}
	}
}

// CompareSubscriptionUnreadCount is a helper function for sorting Subscriptions by unread count, in ascending order. If
// descending order is required, slices.Reverse can be called after sorting the slice with this function.
func CompareSubscriptionUnreadCount(a, b *Subscription) int {
	return cmp.Compare(a.GetUnreadCount(), b.GetUnreadCount())
}

// CompareSubscriptionUpdatedDate is a helper function for sorting Subcriptions by updated date in ascending order. If
// descending order is required, slices.Reverse can be called after sorting the slice with this function.
func CompareSubscriptionUpdatedDate(a, b *Subscription) int {
	return a.GetUpdatedDate().Compare(b.GetUpdatedDate())
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

func (r *SubscriptionRequest) String() string {
	if r.Subscription != nil {
		return r.Subscription.String()
	}
	if r.UserNickname != "" {
		return fmt.Sprintf("%s (%s)", r.UserNickname, r.GetURL())
	}
	return r.GetURL()
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

func (r SubscriptionRequests) Subscriptions() Subscriptions {
	subscriptions := make(Subscriptions, 0, len(r))
	for request := range slices.Values(r.FilterWithSubscription()) {
		subscriptions = append(subscriptions, request.Subscription)
	}
	return subscriptions
}

// FindByURL will return the subscription request matching the given URL.
func (r SubscriptionRequests) FindByURL(url string) *SubscriptionRequest {
	idx := slices.IndexFunc(r, func(v *SubscriptionRequest) bool { return v.GetURL() == url })
	if idx == -1 {
		return nil
	}
	return r[idx]
}

// FilterNoResults will return all requests without a result.
func (r SubscriptionRequests) FilterNoResults() SubscriptionRequests {
	return slices.Collect(FilterSlice(r, func(v *SubscriptionRequest) bool {
		return v.Result == nil
	}))
}

// FilterWithResults will return all requests with a result.
func (r SubscriptionRequests) FilterWithResults() SubscriptionRequests {
	return slices.Collect(FilterSlice(r, func(v *SubscriptionRequest) bool {
		return v.Result != nil
	}))
}

// FilterByStatus will return all requests that have the given status.
func (r SubscriptionRequests) FilterByStatus(status MessageStatus) SubscriptionRequests {
	return slices.Collect(FilterSlice(r, func(v *SubscriptionRequest) bool {
		if v.Result != nil {
			return v.Result.Status == status
		}
		return false
	}))
}

// FilterNoSubscription will return all requests that have no subscription.
func (r SubscriptionRequests) FilterNoSubscription() SubscriptionRequests {
	return slices.Collect(FilterSlice(r, func(v *SubscriptionRequest) bool {
		return v.Subscription == nil
	}))
}

// FilterNoSubscription will return all requests that have no subscription.
func (r SubscriptionRequests) FilterWithSubscription() SubscriptionRequests {
	return slices.Collect(FilterSlice(r, func(v *SubscriptionRequest) bool {
		return v.Subscription != nil
	}))
}

// FilterValid will return all requests that have a valid subscription. In doing so, it will also add a result to any
// requests that have invalid subscription details.
func (r SubscriptionRequests) FilterValid() SubscriptionRequests {
	return slices.Collect(FilterSlice(r.FilterNoResults(), func(request *SubscriptionRequest) bool {
		if request.Subscription == nil {
			request.Result = NewMessage(
				"No subscription data for "+request.GetURL(),
				MessageStatusError)
			return false
		}
		if valid, err := request.Subscription.Valid(); !valid || err != nil {
			request.Result = NewMessage(
				fmt.Sprintf("Invalid details for %s (%s)",
					request.Subscription.GetTitle(),
					request.Subscription.GetSourceURL(),
				),
				MessageStatusError,
				WithDetails(err.Error()),
				WithError(err))
			return false
		}
		return true
	}))
}

// NewSubscriptionRequest creates a new SubscriptionRequest with the given URL.
func NewSubscriptionRequest(url string) *SubscriptionRequest {
	return &SubscriptionRequest{
		SubscriptionID: NewID(SubscriptionPFX),
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
