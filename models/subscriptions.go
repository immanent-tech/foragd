// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
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

type Subscription struct {
	*SubscriptionDetails
	*Feed
}

// Subscriptions is a list of subscriptions.
type Subscriptions []*Subscription

// ConvertFeedsToSubscriptions will take the given list of feeds and return the user subscriptions they match.
func ConvertFeedsToSubscriptions(user *User, feeds ...*Feed) Subscriptions {
	subscriptions := make(Subscriptions, 0, len(feeds))

	for feed := range slices.Values(feeds) {
		details := user.GetSubscriptionByFeedID(feed.GetID())
		if details != nil {
			subscriptions = append(subscriptions,
				&Subscription{
					Feed:                feed,
					SubscriptionDetails: details,
				},
			)
		}
	}

	return subscriptions
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

// Sort will sort the subscription list based on the given sort option.
func (s Subscriptions) Sort(sort Sort) Subscriptions {
	switch {
	case sort.SortBy == SortByUnreadCount:
		slices.SortFunc(s, func(a, b *Subscription) int {
			return cmp.Or(
				CompareSubscriptionUnreadCount(a, b),
				CompareSubscriptionUpdatedDate(a, b),
				strings.Compare(a.GetName(), b.GetName()),
			)
		})
	default:
		slices.SortFunc(s, func(a, b *Subscription) int {
			return cmp.Or(
				CompareSubscriptionUpdatedDate(a, b),
				strings.Compare(a.GetName(), b.GetName()),
			)
		})
	}
	if sort.SortOrder == SortOrderDesc {
		slices.Reverse(s)
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

// Valid returns a boolean indicating if the Subscription contains valid data (true). If it contains invalid data
// (false) a non-nil error is also returned which contains validation issues.
func (s *Subscription) Valid() (bool, error) {
	if valid, err := validation.ValidateStruct(s); err != nil || !valid {
		return false, fmt.Errorf("subscription is invalid: %w", err)
	}
	if s.Feed == nil {
		return false, errors.New("no feed associated with subscription")
	}
	return true, nil
}

func (s *Subscription) String() string {
	if s.GetTitle() != "" {
		return fmt.Sprintf("%s (%s)", s.GetTitle(), s.GetSourceURL())
	}
	return s.GetSourceURL()
}

func (s *Subscription) GetID() SubscriptionID {
	return s.SubscriptionDetails.GetID()
}

// GetTitle returns first found of either the nickname of the subscription or the feed title, in that order.
func (s *Subscription) GetTitle() string {
	if s.UserNickname != "" {
		return s.UserNickname
	}
	return s.Feed.GetTitle()
}

// GetCategories retrieves any custom categories set by the user of nil if
// unset.
func (s *Subscription) GetCategories() []Category {
	categories := slices.Concat(s.UserCategories, s.Feed.GetCategories())
	slices.Sort(categories)
	return slices.Compact(categories)
}

// GetName retrieves any custom name set by the user or an empty string if unset.
func (s *Subscription) GetName() string {
	if s.UserNickname != "" {
		return s.UserNickname
	}
	return s.Feed.GetTitle()
}

// CompareSubscriptionUnreadCount is a helper function for sorting Subscriptions by unread count, in ascending order. If
// descending order is required, slices.Reverse can be called after sorting the slice with this function.
func CompareSubscriptionUnreadCount(a, b *Subscription) int {
	if a.GetUnreadCount() == b.GetUnreadCount() {
		return CompareSubscriptionUpdatedDate(a, b)
	}
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
		return false, fmt.Errorf("subscription is invalid: %w", err)
	}
	return true, nil
}

// Sanitise will sanitise the input values of the SubscriptionRequest.
func (r *SubscriptionRequest) Sanitise() error {
	r.URL = validation.SanitizeString(r.URL)
	r.UserNickname = validation.SanitizeString(r.UserNickname)
	categories := make([]Category, 0, len(r.UserCategories))
	for category := range slices.Values(r.UserCategories) {
		category = validation.SanitizeString(category)
		categories = append(categories, category)
	}
	r.UserCategories = categories
	return nil
}

func (r *SubscriptionRequest) String() string {
	if r.Details != nil {
		if r.UserNickname != "" {
			return fmt.Sprintf("%s (%s)", r.UserNickname, r.GetURL())
		}
	}
	return r.GetURL()
}

func (r *SubscriptionRequest) GetURL() string {
	return strings.TrimSpace(r.URL)
}

func (r *SubscriptionRequest) GenerateDetails(feed *Feed) {
	r.Details = &SubscriptionDetails{
		CreatedAt:      time.Now().UTC(),
		SubscriptionID: NewID(SubscriptionPFX),
		UserCategories: r.UserCategories,
		UserNickname:   r.UserNickname,
		MarkedRead:     UnixEpoch,
		MaxHistory:     MaxHistory(defaultMaxHistory),
	}
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
func (r SubscriptionRequests) FilterByStatus(status UserMessageStatus) SubscriptionRequests {
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
		return v.Details == nil
	}))
}

// FilterNoSubscription will return all requests that have no subscription.
func (r SubscriptionRequests) FilterWithSubscription() SubscriptionRequests {
	return slices.Collect(FilterSlice(r, func(v *SubscriptionRequest) bool {
		return v.Details != nil
	}))
}

// FilterValid will return all requests that have a valid subscription. In doing so, it will also add a result to any
// requests that have invalid subscription details.
func (r SubscriptionRequests) FilterValid() SubscriptionRequests {
	return slices.Collect(FilterSlice(r.FilterNoResults(), func(request *SubscriptionRequest) bool {
		if request.Details == nil {
			request.Result = &UserMessage{
				Status:  UserMessageStatusError,
				Summary: "No subscription data for " + request.GetURL(),
			}
			return false
		}
		if valid, err := request.Details.Valid(); !valid || err != nil {
			details := err.Error()
			request.Result = &UserMessage{
				Status: UserMessageStatusError,
				Summary: fmt.Sprintf("Invalid details for %s (%s)",
					request.Details.GetTitle(),
					request.GetURL(),
				),
				Details: &details,
			}
			return false
		}
		return true
	}))
}

func (s *SubscriptionDetails) GetTitle() string {
	return s.UserNickname
}

func (s *SubscriptionDetails) GetCategories() []Category {
	return s.UserCategories
}

func (s *SubscriptionDetails) GetID() SubscriptionID {
	return s.SubscriptionID
}

func (s *SubscriptionDetails) GetFeedID() FeedID {
	return s.FeedID
}

// GetMarkedRead retrieves the timestamp when the user last marked the
// subscription feed as read.
func (s *SubscriptionDetails) GetMarkedRead() time.Time {
	maxHistory, err := time.ParseDuration(s.MaxHistory)
	if err != nil {
		maxHistory = defaultMaxHistory
	}
	if s.MarkedRead.IsZero() {
		return time.Now().Add(-maxHistory)
	}
	return s.MarkedRead
}

// MarkRead will mark the subscription as read. This involves setting the MarkedRead field to the given value and
// removing any individual unread/read items.
func (s *SubscriptionDetails) MarkRead(markedAt time.Time) {
	updated := time.Now().UTC()
	s.MarkedRead = markedAt
	s.ItemStates = nil
	s.UpdatedAt = &updated
}

// GetMaxHistory returns a timestamp in the past that is the maximum datetime for unread items to be viewed.
func (s *SubscriptionDetails) GetMaxHistory() time.Time {
	return parseMaxHistory(s.MaxHistory)
}

func (s *SubscriptionDetails) GetUnreadCount() int {
	return s.UnreadCount
}

func (s *SubscriptionDetails) SetUnreadCount(count int) {
	s.UnreadCount = count
}

// IsUnread returns a boolean indicating whether the subscription is considered unread.
func (s *SubscriptionDetails) IsUnread() bool {
	return s.UnreadCount > 0 || len(s.GetUnreadItems()) > 0
}

// GetUnreadItems retrieves a list of ItemIDs for the subscription feed that
// user has explicitly marked as unread.
func (s *SubscriptionDetails) GetUnreadItems() []ItemID {
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
func (s *SubscriptionDetails) GetReadItems() []ItemID {
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
func (s *SubscriptionDetails) GetItemState(itemID ItemID) State {
	if idx := slices.IndexFunc(s.GetUnreadItems(), func(v ItemID) bool { return v == itemID }); idx != -1 {
		return StateUnread
	}
	if idx := slices.IndexFunc(s.GetReadItems(), func(v ItemID) bool { return v == itemID }); idx != -1 {
		return StateRead
	}
	return StateUnread
}

// MarkItemsRead will mark the given items as read for the subscription.
func (s *SubscriptionDetails) MarkItemsRead(items ...ItemID) {
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
func (s *SubscriptionDetails) MarkItemsUnread(items ...ItemID) {
	for itemID := range slices.Values(items) {
		idx := slices.IndexFunc(s.ItemStates, func(s ItemState) bool { return s.ItemID == itemID })
		if idx != -1 {
			s.ItemStates[idx].State = StateUnread
		} else {
			s.ItemStates = append(s.ItemStates, ItemState{ItemID: itemID, State: StateUnread})
		}
	}
}

// Valid returns a boolean indicating if the Subscription contains valid data (true). If it contains invalid data
// (false) a non-nil error is also returned which contains validation issues.
func (s *SubscriptionDetails) Valid() (bool, error) {
	if valid, err := validation.ValidateStruct(s); err != nil || !valid {
		return false, fmt.Errorf("subscription is invalid: %w", err)
	}
	return true, nil
}

// Sanitise will sanitise the user input for a SubscriptionCustomisation.
func (s *SubscriptionDetails) Sanitise() error {
	s.UserNickname = validation.SanitizeString(s.UserNickname)
	categories := make([]Category, 0, len(s.UserCategories))
	for category := range slices.Values(s.UserCategories) {
		category = validation.SanitizeString(category)
		categories = append(categories, category)
	}
	s.UserCategories = categories
	return nil
}

type FeedSource interface {
	GetFeedID() FeedID
}

func GetFeedIDs[T FeedSource](objects []T) []FeedID {
	ids := make([]FeedID, 0, len(objects))
	for details := range slices.Values(objects) {
		ids = append(ids, details.GetFeedID())
	}
	return ids
}

// Valid returns a boolean indicating if the Subscription contains valid data (true). If it contains invalid data
// (false) a non-nil error is also returned which contains validation issues.
func (s *SubscriptionCustomisation) Valid() (bool, error) {
	if valid, err := validation.ValidateStruct(s); err != nil || !valid {
		return false, fmt.Errorf("subscription is invalid: %w", err)
	}
	return true, nil
}

// Sanitise will sanitise the user input for a SubscriptionCustomisation.
func (s *SubscriptionCustomisation) Sanitise() error {
	s.UserNickname = validation.SanitizeString(s.UserNickname)
	categories := make([]Category, 0, len(s.UserCategories))
	for category := range slices.Values(s.UserCategories) {
		category = validation.SanitizeString(category)
		categories = append(categories, category)
	}
	s.UserCategories = categories
	return nil
}
