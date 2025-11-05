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

// NewSubscription creates a new from the request and feed details.
func NewFeedSubscription(feed *Feed, request *SubscriptionRequest, favorite bool) *FeedSubscription {
	ts := time.Now().UTC()
	// Create state based on feed and user data.
	subscription := &FeedSubscription{
		SubscriptionID: NewID(SubscriptionPFX),
		UpdatedAt:      ts,
		CreatedAt:      ts,
		FeedID:         feed.GetID(),
		Customisation: SubscriptionCustomisation{
			Nickname:   feed.GetTitle(),
			Categories: feed.GetCategories(),
		},
		ItemStates: make(map[ItemID]ArticleState),
		Favorite:   favorite,
	}
	// Add any user customisations.
	if request != nil {
		if request.Nickname != "" {
			subscription.Customisation.Nickname = request.Nickname
		}
		if len(request.Categories) > 0 {
			subscription.Customisation.Categories = request.Categories
		}
	}
	return subscription
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
	return s.SubscriptionID
}

// GetFeedID returns the feed ID.
func (s *FeedSubscription) GetFeedID() FeedID {
	return s.FeedID
}

// GetTitle returns the title of the subscription. Either the user's nickname or original feed title.
func (s *FeedSubscription) GetTitle() string {
	if s.Customisation.Nickname != "" {
		return s.Customisation.Nickname
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
	if s.Customisation.Categories != nil {
		all = slices.Compact(slices.Concat(s.Customisation.Categories, s.Feed.GetCategories()))
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

// SetUnreadCount sets the unread count of the subscription to the given value.
func (s *FeedSubscription) SetUnreadCount(count int) {
	s.Stats.UnreadCount = count
}

// IsFavorite returns a boolean indicating whether the subscription has been favorited.
func (s *FeedSubscription) IsFavorite() bool {
	return s.Favorite
}

// Mark will mark the subscription as read. Any individual item states are cleared as a result of calling this method.
func (s *FeedSubscription) Mark(mark Mark, markedAt time.Time) {
	switch mark {
	case MarkRead:
		s.MarkedReadAt = markedAt
	case MarkUnread:
		s.MarkedReadAt = markedAt
	}
	s.ItemStates = nil
}

// GetUnreadItems retrieves a list of ItemIDs for the subscription feed that
// user has explicitly marked as unread.
func (s *FeedSubscription) GetUnreadItems() []ItemID {
	ids := make([]ItemID, 0, len(s.ItemStates))
	for id, state := range s.ItemStates {
		if !state.Read {
			ids = append(ids, id)
		}
	}
	return ids
}

// GetReadItems retrieves a list of ItemIDs for the subscription feed that
// user has explicitly marked as read.
func (s *FeedSubscription) GetReadItems() []ItemID {
	ids := make([]ItemID, 0, len(s.ItemStates))
	for id, state := range s.ItemStates {
		if state.Read {
			ids = append(ids, id)
		}
	}
	return ids
}

// GetItemState retrieves the item state (read/unread/saved) from the
// subscription. By default it will return unread unless the user has explicitly
// marked or saved the item.
func (s *FeedSubscription) GetItemState(id ItemID) *ArticleState {
	// Retrieve any explicitly set state of the item.
	if state, found := s.ItemStates[id]; found {
		return &state
	}
	// If an item doesn't have an explicit state, its state should reflect the subscription state.
	return &ArticleState{
		Read:      false,
		UpdatedAt: s.UpdatedAt,
	}
}

// SetItemState will set the state of the item to the given state.
func (s *FeedSubscription) SetItemState(id ItemID, state *ArticleState) {
	if s.ItemStates == nil {
		s.ItemStates = make(map[ItemID]ArticleState)
	}
	s.ItemStates[id] = *state
}

// MarkItemsRead will mark the given items as read for the subscription.
func (s *FeedSubscription) MarkItemsRead(ids ...ItemID) {
	for id := range slices.Values(ids) {
		state := s.GetItemState(id)
		if state == nil {
			state = &ArticleState{}
		}
		state.MarkRead(time.Now().UTC())
		s.SetItemState(id, state)
	}
}

// MarkItemsUnread will mark the given items as unread for the subscription.
func (s *FeedSubscription) MarkItemsUnread(ids ...ItemID) {
	for id := range slices.Values(ids) {
		state := s.GetItemState(id)
		if state == nil {
			state = &ArticleState{}
		}
		state.MarkUnread(time.Now().UTC())
		s.SetItemState(id, state)
	}
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

// IssueURL returns an app URL for reporting issues with the subscription.
func (s *FeedSubscription) IssueURL() string {
	return "/issue/subscription/" + s.GetID()
}

// Subscriptions is a slice of Subscription objects.
type Subscriptions []*FeedSubscription

// FilterByIDs returns a new slice containing the subscriptions with the given ids only.
func (s Subscriptions) FilterByIDs(ids ...SubscriptionID) Subscriptions {
	return slices.Collect(
		FilterSlice(s, func(e *FeedSubscription) bool {
			return slices.Contains(ids, e.GetID())
		}),
	)
}

// FilterByFeedIDs returns a new slice containing the subscriptions with the given feed ids only. If no ids are
// provided, it returns the unfiltered slice.
func (s Subscriptions) FilterByFeedIDs(ids ...FeedID) Subscriptions {
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
func (s Subscriptions) FilterByCategories(categories ...Category) Subscriptions {
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
func (s Subscriptions) FilterByView(view View) Subscriptions {
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
func (s Subscriptions) Search(text string) Subscriptions {
	return slices.Collect(
		FilterSlice(s, func(e *FeedSubscription) bool {
			return strings.Contains(strings.ToLower(e.Customisation.Nickname), strings.ToLower(text)) ||
				slices.ContainsFunc(e.Customisation.Categories, func(e Category) bool {
					return strings.Contains(strings.ToLower(e), strings.ToLower(text))
				})
		}),
	)
}

// GetFeedIDs returns the feed ids for all subscription states in the slice.
func (s Subscriptions) GetFeedIDs() []FeedID {
	ids := make([]FeedID, 0, len(s))
	for state := range slices.Values(s) {
		ids = append(ids, state.GetFeedID())
	}
	return ids
}

// GetIDs returns the subscription ids for all subscription states in the slice.
func (s Subscriptions) GetIDs() []SubscriptionID {
	ids := make([]SubscriptionID, 0, len(s))
	for state := range slices.Values(s) {
		ids = append(ids, state.GetID())
	}
	return ids
}

// GetByID retrieves a state by the subscription id from the slice.
func (s Subscriptions) GetByID(id SubscriptionID) *FeedSubscription {
	if idx := slices.IndexFunc(s, func(e *FeedSubscription) bool {
		return e.GetID() == id
	}); idx != -1 {
		return s[idx]
	}
	return nil
}

// GetByFeedID retrieves a state by the FeedID from the slice.
func (s Subscriptions) GetByFeedID(id FeedID) *FeedSubscription {
	if idx := slices.IndexFunc(s, func(e *FeedSubscription) bool {
		return e.GetFeedID() == id
	}); idx != -1 {
		return s[idx]
	}
	return nil
}

// SortByTitle sorts the slice of subscriptions by their title.
func (s Subscriptions) SortByTitle() Subscriptions {
	sort.Slice(s, func(i, j int) bool { return s[i].Customisation.Nickname < s[j].Customisation.Nickname })
	return s
}

// Sort will sort the slice of subscriptions by the given sort.
func (s Subscriptions) Sort(sort *Sort) Subscriptions {
	if sort == nil {
		sort = &Sort{
			SortBy:    SortByUnreadCount,
			SortOrder: SortOrderDesc,
		}
	}
	switch sort.SortBy {
	case SortByLastUpdated:
		slices.SortFunc(s, func(a, b *FeedSubscription) int {
			return a.GetUpdatedDate().Compare(b.GetUpdatedDate())
		})
	case SortByUnreadCount:
		slices.SortFunc(s, func(a, b *FeedSubscription) int {
			cmpValue := cmp.Compare(a.GetStats().UnreadTotal(), b.GetStats().UnreadTotal())
			if cmpValue == 0 {
				return a.GetUpdatedDate().Compare(b.GetUpdatedDate())
			}
			return cmpValue
		})
	}
	if sort.SortOrder == SortOrderDesc {
		slices.Reverse(s)
	}
	return s
}

// Paginate will paginate through a slice of subscriptions, returning a new slice of subscriptions and the next
// pagination value (if any).
func (s Subscriptions) Paginate(pagination Pagination, count int) (Subscriptions, Pagination) {
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
func (s Subscriptions) GetTotalUnreadCount() int {
	var unread int
	for subscription := range slices.Values(s) {
		unread += subscription.GetStats().UnreadTotal()
	}
	return unread
}

// GetCategoryCounts returns a count of the occurrence of a Category across all
// the Subscriptions.
func (s Subscriptions) GetCategoryCounts() CategoryCounts {
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

// Valid returns a boolean indicating whether the SubscriptionRequest is valid,
// and any validation errors if applicable.
func (r *SubscriptionRequest) Valid() (bool, error) {
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
func (r *SubscriptionRequest) Sanitise() error {
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
func (r *SubscriptionRequest) GetURL() string {
	return strings.TrimSpace(r.URL)
}

// GetNickname returns the nickname chosen for the subscription.
func (r *SubscriptionRequest) GetNickname() string {
	if r.Nickname != "" {
		return r.Nickname
	}
	return ""
}

// HasError wil return true if the subscription request has errors associated with any of its fields.
func (r *SubscriptionRequest) HasError() bool {
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
