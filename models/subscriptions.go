// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/immanent-tech/go-syndication/types"

	"github.com/immanent-tech/foragd/validation"
)

var ErrInvalidSubscriptionState = errors.New("invalid subscription state")

// NewFeedSubscription creates a new subscription for a feed from the request and feed details.
func NewFeedSubscription(feed *Feed, request *AddFeedSubscriptionRequest, favorite bool) (*Subscription, error) {
	ts := time.Now().UTC()
	// Create state based on feed and user data.
	subscription := &Subscription{
		Metadata: SubscriptionMetadata{
			SubscriptionID: NewID(SubscriptionPFX),
			UpdatedAt:      ts,
			CreatedAt:      ts,
			Customisation: SubscriptionCustomisation{
				Nickname:   feed.GetTitle(),
				Categories: feed.GetCategories(),
			},
			Favorite: favorite,
		},
		Type: SubscriptionTypeFeed,
	}
	// Add any user customisations.
	if request != nil {
		if request.Nickname != "" {
			subscription.Metadata.Customisation.Nickname = request.Nickname
		}
		if len(request.Categories) > 0 {
			subscription.Metadata.Customisation.Categories = request.Categories
		}
	}
	// Encode the type-specific data.
	err := subscription.Data.FromFeedSubscription(FeedSubscription{FeedID: feed.GetID()})
	if err != nil {
		return nil, fmt.Errorf("unable to create subscription: %w", err)
	}
	return subscription, nil
}

// Valid returns a boolean indicating if the Subscription contains valid data (true). If it contains invalid data
// (false) a non-nil error is also returned which contains validation issues.
func (s *FeedSubscription) Valid() (bool, error) {
	valid, err := validation.ValidateStruct(s)
	if err != nil || !valid {
		return false, fmt.Errorf("subscription is invalid: %w", err)
	}
	return true, nil
}

// GetID returns the subscription ID.
func (s *FeedSubscription) GetID() string {
	return s.Metadata.SubscriptionID
}

// GetFeedID returns the feed ID.
func (s *FeedSubscription) GetFeedID() FeedID {
	return s.FeedID
}

// GetTitle returns the title of the subscription. Either the user's nickname or original feed title.
func (s *FeedSubscription) GetTitle() string {
	if s.Metadata.Customisation.Nickname != "" {
		return s.Metadata.Customisation.Nickname
	}
	return s.Feed.GetTitle()
}

// GetLink returns the feed source URL.
func (s *FeedSubscription) GetLink() string {
	return s.Feed.URL
}

// GetDescription returns any description contained in the feed content.
func (s *FeedSubscription) GetDescription() string {
	return s.Feed.GetDescription()
}

// GetCategories returns the categories of the subscription. It is the combined list of any user-assigned categories and
// the categories in the feed content.
func (s *FeedSubscription) GetCategories(maxCount int) Categories {
	var all []Category
	if s.Metadata.Customisation.Categories != nil {
		all = slices.Compact(slices.Concat(s.Metadata.Customisation.Categories, s.Feed.GetCategories()))
	} else {
		all = s.Feed.GetCategories()
	}
	if maxCount != 0 {
		if len(all) > maxCount {
			return all[:maxCount]
		} else {
			return all
		}
	}
	return all
}

// GetAuthors returns the list of authors (if any) of the feed.
func (s *FeedSubscription) GetAuthors() []string {
	return s.Feed.GetAuthors()
}

// GetUpdatedDate returns the timestamp when items for the feed were last fetched.
func (s *FeedSubscription) GetUpdatedDate() time.Time {
	return s.Feed.LastFetched
}

// GetImage retrieves the image that represents the subscription, or nil if no image is available.
func (s *FeedSubscription) GetImage() *types.ImageInfo {
	if s.Feed.GetImage() != nil && s.Feed.GetImage().GetURL() != "" {
		return s.Feed.GetImage()
	}
	return nil
}

func (s *FeedSubscription) GetStats() *SubscriptionStats {
	return &s.Stats
}

// // SetUnreadCount sets the unread count of the subscription to the given value.
// func (s *FeedSubscription) SetUnreadCount(count int) {
// 	s.Stats.UnreadCount = count
// }

// IsFavorite returns a boolean indicating whether the subscription has been favorited.
func (s *FeedSubscription) IsFavorite() bool {
	return s.Metadata.Favorite
}

// Type returns the type of the object, in this case, "subscription".
func (s *FeedSubscription) GetObjectType() ObjectType {
	return ObjectTypeSubscription
}

// // ViewURL returns the app URL for viewing articles in the subscription.
// func (s *FeedSubscription) ViewURL() string {
// 	return "/list/articles"
// }

// // MarkURL returns the app URL for marking the subscription.
// func (s *FeedSubscription) MarkURL() string {
// 	if s.GetStats().IsUnread() {
// 		return "/mark/subscription/" + s.GetID() + "/read"
// 	}
// 	return "/mark/subscription/" + s.GetID() + "/unread"
// }

// // IssueURL returns an app URL for reporting issues with the subscription.
// func (s *FeedSubscription) IssueURL() string {
// 	return "/issue/subscription/" + s.GetID()
// }

// FeedSubscriptions is a slice of Subscription objects.
type FeedSubscriptions []*FeedSubscription

// // FilterByIDs returns a new slice containing the subscriptions with the given ids only.
// func (s FeedSubscriptions) FilterByIDs(ids ...SubscriptionID) FeedSubscriptions {
// 	return slices.Collect(
// 		FilterSlice(s, func(e *FeedSubscription) bool {
// 			return slices.Contains(ids, e.GetID())
// 		}),
// 	)
// }

// FilterByFeedIDs returns a new slice containing the subscriptions with the given feed ids only. If no ids are
// provided, it returns the unfiltered slice.
func (s FeedSubscriptions) FilterByFeedIDs(ids ...FeedID) FeedSubscriptions {
	if len(ids) == 0 {
		return s
	}
	return slices.Collect(
		FilterSlice(s, func(e *FeedSubscription) bool {
			return slices.Contains(ids, e.GetFeedID())
		}),
	)
}

// FilterByCategories returns a new slice containing the subscriptions which have a category matching the given
// categories.
func (s FeedSubscriptions) FilterByCategories(categories ...Category) FeedSubscriptions {
	if len(categories) == 0 {
		return s
	}
	return slices.Collect(FilterSlice(s, func(subscription *FeedSubscription) bool {
		for category := range slices.Values(categories) {
			return slices.Contains(subscription.GetCategories(0), category)
		}
		return false
	}))
}

// FilterByView returns a slice containing the subscription which match the given view state.
func (s FeedSubscriptions) FilterByView(view View) FeedSubscriptions {
	switch view {
	case ViewRead:
		return slices.Collect(FilterSlice(s, func(subscription *FeedSubscription) bool {
			return !subscription.GetStats().IsUnread()
		}))
	case ViewUnread:
		return slices.Collect(FilterSlice(s, func(subscription *FeedSubscription) bool {
			return subscription.GetStats().IsUnread()
		}))
	default:
		return s
	}
}

// Search performs a case-insensitive substring search for the given text in the title and categories customisations for
// the subscriptions, returning a slice of those subscriptions that match.
func (s FeedSubscriptions) Search(text string) FeedSubscriptions {
	return slices.Collect(
		FilterSlice(s, func(e *FeedSubscription) bool {
			return strings.Contains(strings.ToLower(e.Metadata.Customisation.Nickname), strings.ToLower(text)) ||
				slices.ContainsFunc(e.Metadata.Customisation.Categories, func(e Category) bool {
					return strings.Contains(strings.ToLower(e), strings.ToLower(text))
				})
		}),
	)
}

// GetFeedIDs returns the feed ids for all subscription states in the slice.
func (s FeedSubscriptions) GetFeedIDs() []FeedID {
	ids := make([]FeedID, 0, len(s))
	for state := range slices.Values(s) {
		ids = append(ids, state.GetFeedID())
	}
	return ids
}

// GetIDs returns the subscription ids for all subscription states in the slice.
func (s FeedSubscriptions) GetIDs() []SubscriptionID {
	ids := make([]SubscriptionID, 0, len(s))
	for state := range slices.Values(s) {
		ids = append(ids, state.GetID())
	}
	return ids
}

// GetByID retrieves a state by the subscription id from the slice.
func (s FeedSubscriptions) GetByID(id SubscriptionID) *FeedSubscription {
	if idx := slices.IndexFunc(s, func(e *FeedSubscription) bool {
		return e.GetID() == id
	}); idx != -1 {
		return s[idx]
	}
	return nil
}

// GetByFeedID retrieves a state by the FeedID from the slice.
func (s FeedSubscriptions) GetByFeedID(id FeedID) *FeedSubscription {
	if idx := slices.IndexFunc(s, func(e *FeedSubscription) bool {
		return e.GetFeedID() == id
	}); idx != -1 {
		return s[idx]
	}
	return nil
}

// SortByTitle sorts the slice of subscriptions by their title.
func (s FeedSubscriptions) SortByTitle() FeedSubscriptions {
	sort.Slice(s, func(i, j int) bool {
		return s[i].Metadata.Customisation.Nickname < s[j].Metadata.Customisation.Nickname
	})
	return s
}

// Sort will sort the slice of subscriptions by the given sort option. Favorite subscriptions are always sorted before
// other subscriptions, and the sort option is used as a tiebreaker.
func (s FeedSubscriptions) Sort(sort *Sort) FeedSubscriptions {
	if sort == nil {
		sort = &Sort{
			SortBy:    SortByUnreadCount,
			SortOrder: SortOrderDesc,
		}
	}
	switch sort.SortBy {
	case SortByLastUpdated:
		slices.SortFunc(s, func(subscriptionA, subscriptionB *FeedSubscription) int {
			switch {
			case subscriptionA.IsFavorite() && !subscriptionB.IsFavorite():
				return 1
			case !subscriptionA.IsFavorite() && subscriptionB.IsFavorite():
				return -1
			default:
				return subscriptionA.GetUpdatedDate().Compare(subscriptionB.GetUpdatedDate())
			}
		})
	case SortByUnreadCount:
		slices.SortFunc(s, func(subscriptionA, subscriptionB *FeedSubscription) int {
			switch {
			case subscriptionA.IsFavorite() && !subscriptionB.IsFavorite():
				return 1
			case !subscriptionA.IsFavorite() && subscriptionB.IsFavorite():
				return -1
			default:
				cmpValue := cmp.Compare(subscriptionA.GetStats().UnreadTotal(), subscriptionB.GetStats().UnreadTotal())
				if cmpValue == 0 {
					return subscriptionA.GetUpdatedDate().Compare(subscriptionB.GetUpdatedDate())
				}
				return cmpValue
			}
		})
	}
	if sort.SortOrder == SortOrderDesc {
		slices.Reverse(s)
	}
	return s
}

// Paginate will paginate through a slice of subscriptions, returning a new slice of subscriptions and the next
// pagination value (if any).
func (s FeedSubscriptions) Paginate(pagination Pagination, count int) (FeedSubscriptions, Pagination) {
	var from, to int
	if pagination != "" {
		value, err := strconv.Atoi(pagination)
		if err == nil {
			from = value
		}
	}
	to = min(from+count, len(s))
	pagination = strconv.Itoa(to)
	return s[from:to], pagination
}

// GetTotalUnreadCount calculates the total unread articles across all subscriptions in the slice.
func (s FeedSubscriptions) GetTotalUnreadCount() int {
	var unread int
	for subscription := range slices.Values(s) {
		unread += subscription.GetStats().UnreadTotal()
	}
	return unread
}

// GetCategoryCounts returns a count of the occurrence of a Category across all
// the Subscriptions.
func (s FeedSubscriptions) GetCategoryCounts() CategoryCounts {
	countsMap := make(map[Category]int)
	for object := range slices.Values(s) {
		for category := range slices.Values(object.GetCategories(0)) {
			countsMap[category]++
		}
	}
	var counts CategoryCounts
	for category, count := range maps.All(countsMap) {
		counts = append(counts, CategoryCount{Category: category, Count: count})
	}

	return counts
}

// GetID returns the subscription ID.
func (s *SearchSubscription) GetID() string {
	return s.Metadata.SubscriptionID
}

// SearchSubscriptions is a slice of SearchSubscription subscriptions.
type SearchSubscriptions []*SearchSubscription

// GetIDs returns the subscription ids for all subscription states in the slice.
func (s SearchSubscriptions) GetIDs() []SubscriptionID {
	ids := make([]SubscriptionID, 0, len(s))
	for state := range slices.Values(s) {
		ids = append(ids, state.GetID())
	}
	return ids
}

// Valid returns a boolean indicating if the Subscription contains valid data (true). If it contains invalid data
// (false) a non-nil error is also returned which contains validation issues.
func (s *Subscription) Valid() (bool, error) {
	valid, err := validation.ValidateStruct(s)
	if err != nil || !valid {
		return false, fmt.Errorf("subscription is invalid: %w", err)
	}
	return true, nil
}

func (s *Subscription) GetID() SubscriptionID {
	return s.Metadata.SubscriptionID
}

func (s *Subscription) GetType() SubscriptionType {
	return s.Type
}

func (s *Subscription) GetTitle() string {
	return s.Metadata.Customisation.Nickname
}

type Subscriptions []*Subscription

// GetIDs returns the subscription ids for all subscription states in the slice.
func (s Subscriptions) GetIDs() []SubscriptionID {
	ids := make([]SubscriptionID, 0, len(s))
	for state := range slices.Values(s) {
		ids = append(ids, state.GetID())
	}
	return ids
}

// GetByID retrieves a state by the subscription id from the slice.
func (s Subscriptions) GetByID(id SubscriptionID) *Subscription {
	if idx := slices.IndexFunc(s, func(e *Subscription) bool {
		return e.GetID() == id
	}); idx != -1 {
		return s[idx]
	}
	return nil
}

// GetFeedSubscriptions returns a new slice containing just the FeedSubscription subscriptions.
func (s Subscriptions) GetFeedSubscriptions() FeedSubscriptions {
	feedSubscriptions := make(FeedSubscriptions, 0, len(s))
	for s := range slices.Values(s) {
		if s.GetType() == SubscriptionTypeFeed {
			subscription, err := s.Data.AsFeedSubscription()
			if err != nil {
				continue
			}
			subscription.Metadata = s.Metadata
			feedSubscriptions = append(feedSubscriptions, &subscription)
		}
	}
	return feedSubscriptions
}

// GetSearchSubscriptions returns a new slice containing just the SearchSubscription subscriptions.
func (s Subscriptions) GetSearchSubscriptions() SearchSubscriptions {
	searchSubscriptions := make(SearchSubscriptions, 0, len(s))
	for s := range slices.Values(s) {
		if s.GetType() == SubscriptionTypeFeed {
			subscription, err := s.Data.AsSearchSubscription()
			if err != nil {
				continue
			}
			subscription.Metadata = s.Metadata
			searchSubscriptions = append(searchSubscriptions, &subscription)
		}
	}
	return searchSubscriptions
}

// Valid returns a boolean indicating whether the SubscriptionRequest is valid,
// and any validation errors if applicable.
func (r *AddFeedSubscriptionRequest) Valid() (bool, error) {
	valid, err := validation.ValidateStruct(r)
	if err != nil {
		return false, fmt.Errorf("subscription validation error: %w", err)
	}
	if !valid {
		return false, nil
	}
	return true, nil
}

// Sanitise will sanitise the input values of the SubscriptionRequest.
func (r *AddFeedSubscriptionRequest) Sanitise() error {
	r.URL = validation.SanitizeString(r.URL)
	if r.Nickname != "" {
		sanitizedNickname := validation.SanitizeString(r.Nickname)
		r.Nickname = sanitizedNickname
	}
	categories := make([]Category, 0, len(r.Categories))
	for category := range slices.Values(r.Categories) {
		category = validation.SanitizeString(category)
		categories = append(categories, category)
	}
	r.Categories = categories
	return nil
}

// GetURL returns the (feed) URL for the request.
func (r *AddFeedSubscriptionRequest) GetURL() string {
	return strings.TrimSpace(r.URL)
}

// GetNickname returns the nickname chosen for the subscription.
func (r *AddFeedSubscriptionRequest) GetNickname() string {
	if r.Nickname != "" {
		return r.Nickname
	}
	return ""
}

// HasError wil return true if the subscription request has errors associated with any of its fields.
func (r *AddFeedSubscriptionRequest) HasError() bool {
	return r.NicknameErr != nil || r.CategoriesErr != nil || r.URLErr != nil
}

// GetID retrieves the subscription ID.
func (s *EditSubscriptionRequest) GetID() SubscriptionID {
	return s.SubscriptionID
}

// GetNickname retrieves the nickname assigned to the subscription.
func (s *EditSubscriptionRequest) GetNickname() string {
	return s.Nickname
}

// GetCategories retrieves the categories assigned to the subscription.
func (s *EditSubscriptionRequest) GetCategories() Categories {
	return s.Categories
}

// Valid returns a boolean indicating if the Subscription contains valid data (true). If it contains invalid data
// (false) a non-nil error is also returned which contains validation issues.
func (s *EditSubscriptionRequest) Valid() (bool, error) {
	valid, err := validation.ValidateStruct(s)
	if err != nil || !valid {
		return false, fmt.Errorf("subscription is invalid: %w", err)
	}
	return true, nil
}

// Sanitise will sanitise the user input for a SubscriptionCustomisation.
func (s *EditSubscriptionRequest) Sanitise() error {
	s.Nickname = validation.SanitizeString(s.Nickname)
	categories := make([]Category, 0, len(s.Nickname))
	for category := range slices.Values(s.Categories) {
		category = validation.SanitizeString(category)
		categories = append(categories, category)
	}
	s.Categories = categories
	s.ArticleFilters.Authors = validation.SanitizeString(s.ArticleFilters.Authors)
	s.ArticleFilters.Categories = validation.SanitizeString(s.ArticleFilters.Categories)
	s.ArticleFilters.Text = validation.SanitizeString(s.ArticleFilters.Text)
	return nil
}

// HasError wil return true if the subscription request has errors associated with any of its fields.
func (r *EditSubscriptionRequest) HasError() bool {
	return r.NicknameErr != nil || r.CategoriesErr != nil || r.ImageErr != nil
}

// AddSubscriptionResult represents the result of creating a new subscription.
type AddSubscriptionResult struct {
	Subscription *Subscription
	Message      *UserMessage
}

// NewSubscriptionResult creates an object that represents the result of creating a new subscription. The subscription
// is optional as the result might be a failure to create. The message should always be non-nil.
func NewSubscriptionResult(subscription *Subscription, msg *UserMessage) *AddSubscriptionResult {
	return &AddSubscriptionResult{
		Subscription: subscription,
		Message:      msg,
	}
}

// GetDailyUpdates returns a nicely formatted value of daily update interval for a subscription.
func (s *SubscriptionStats) DailyUpdates() int {
	return int(math.Round(s.AvgDailyUpdates))
}

func (s *SubscriptionStats) DailyUpdateFrequency() string {
	switch {
	case s.DailyUpdates() > 1:
		return fmt.Sprintf("%d articles/day", s.DailyUpdates())
	case s.AvgDailyUpdates < 1 && s.AvgDailyUpdates > 0.5:
		return "A few times a week"
	case s.AvgDailyUpdates < 0.5 && s.AvgDailyUpdates > 0.25:
		return "About weekly"
	default:
		return "Infrequent"
	}
}

// GetUnreadCount returns the unread count of items in the subscription.
func (s *SubscriptionStats) UnreadTotal() int {
	return s.UnreadCount
}

// IsUnread returns a boolean indicating whether the subscription is considered unread.
func (s *SubscriptionStats) IsUnread() bool {
	return s.UnreadCount > 0
}
