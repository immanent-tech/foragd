// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spaolacci/murmur3"

	"github.com/immanent-tech/foragd/validation"
)

var ErrInvalidSubscriptionData = errors.New("invalid subscription data")

// NewFeedSubscription creates a new subscription for a feed from the request and feed details.
func NewFeedSubscription(ctx context.Context, feed *Feed, request *AddFeedSubscriptionRequest) (*Subscription, error) {
	// Create state based on feed and user data.
	feedSubscription := &FeedSubscription{
		URL:           feed.GetLink(),
		FeedID:        feed.GetID(),
		ArticleStates: make(map[ItemID]ArticleState),
	}

	settings := newSubscriptionSettings()

	customisation := newSubscriptionCustomisation(feed.GetTitle(), feed.GetImage().GetURL(), feed.GetCategories())
	// Override with any user customisations.
	if request != nil {
		if request.Nickname != "" {
			customisation.Nickname = request.Nickname
		}
		if len(request.Categories) > 0 {
			customisation.Categories = request.Categories
		}
	}

	subscription, err := newSubscription(ctx, *customisation, *settings, feedSubscription)
	if err != nil {
		return nil, fmt.Errorf("new feed subscription: %w", err)
	}

	return subscription, nil
}

// Valid returns a boolean indicating if the Subscription contains valid data (true). If it contains invalid data
// (false) a non-nil error is also returned which contains validation issues.
func (s *FeedSubscription) Valid() error {
	if err := validation.Validate.Struct(s); err != nil {
		return fmt.Errorf("feed subscription is invalid: %w", err)
	}
	return nil
}

// GetFeedID returns the feed ID.
func (s *FeedSubscription) GetFeedID() FeedID {
	return s.FeedID
}

// GetUnreadItems retrieves a list of ItemIDs for the feed subscription that
// user has explicitly marked as unread.
func (s *FeedSubscription) GetUnreadItems() []ItemID {
	ids := make([]ItemID, 0)
	for id, state := range s.ArticleStates {
		if !state.Read {
			ids = append(ids, id)
		}
	}
	return ids
}

// GetReadItems retrieves a list of ItemIDs for the feed subscription that
// user has explicitly marked as read.
func (s *FeedSubscription) GetReadItems() []ItemID {
	ids := make([]ItemID, 0)
	for id, state := range s.ArticleStates {
		if state.Read {
			ids = append(ids, id)
		}
	}
	return ids
}

// GetItemState retrieves the item state (read/unread/saved) from the
// subscription. By default it will return unread unless the user has explicitly
// marked or saved the item.
func (s *FeedSubscription) GetItemState(itemID ItemID) *ArticleState {
	// If the subscription has no article states, return unread state.
	if s.ArticleStates == nil {
		return &ArticleState{
			Read: false,
		}
	}
	// If a state is found return that.
	if state, found := s.ArticleStates[itemID]; found {
		return &state
	}
	// Return unread state if no state found.
	return &ArticleState{
		Read: false,
	}
}

// SetItemState will set the state of the item to the given state.
func (s *FeedSubscription) SetItemState(itemID ItemID, state *ArticleState) {
	// Create a new article state map if none exists.
	if s.ArticleStates == nil {
		s.ArticleStates = make(map[ItemID]ArticleState)
	}
	s.ArticleStates[itemID] = *state
}

// NewSearchSubscription creates a new SearchSubscription. A SearchSubscription collates articles that match a search
// into a single custom subscription.
func NewSearchSubscription(ctx context.Context, request *SearchSubscriptionRequest) (*Subscription, error) {
	searchSubscription := &SearchSubscription{
		Search: request.Search,
	}
	subscription, err := newSubscription(ctx, request.Customisation, request.Settings, searchSubscription)
	if err != nil {
		return nil, fmt.Errorf("new search subscription: %w", err)
	}
	subscription.Favorite = true
	return subscription, nil
}

// Valid returns a boolean indicating if the Subscription contains valid data (true). If it contains invalid data
// (false) a non-nil error is also returned which contains validation issues.
func (s *SearchSubscription) Valid() error {
	if err := validation.Validate.Struct(s); err != nil {
		return fmt.Errorf("search subscription is invalid: %w", err)
	}
	return nil
}

// NewGroupSubscription creates a GroupSubscription. A GroupSubscription is a kind of meta-subscription that aggregates
// all articles from multiple individual subscriptions into a single custom subscription.
func NewGroupSubscription(ctx context.Context, request *GroupSubscriptionRequest) (*Subscription, error) {
	groupSubscription := &GroupSubscription{
		Subscriptions: request.Subscriptions,
	}
	subscription, err := newSubscription(ctx, request.Customisation, request.Settings, groupSubscription)
	if err != nil {
		return nil, fmt.Errorf("new group subscription: %w", err)
	}
	subscription.Favorite = true
	return subscription, nil
}

// Valid returns a boolean indicating if the Subscription contains valid data (true). If it contains invalid data
// (false) a non-nil error is also returned which contains validation issues.
func (s *GroupSubscription) Valid() error {
	if err := validation.Validate.Struct(s); err != nil {
		return fmt.Errorf("feed subscription is invalid: %w", err)
	}
	return nil
}

// Valid returns a boolean indicating if the Subscription contains valid data (true). If it contains invalid data
// (false) a non-nil error is also returned which contains validation issues.
func (s *Subscription) Valid() error {
	if err := validation.Validate.Struct(s); err != nil {
		return fmt.Errorf("subscription is invalid: %w", err)
	}
	return nil
}

// GetID returns the unqiue ID of the subscription.
func (s *Subscription) GetID() SubscriptionID {
	return s.SubscriptionID
}

// GetSubscriptionType returns the type of subscription (i.e., feed, search, group, etc.).
func (s *Subscription) GetSubscriptionType() SubscriptionType {
	return s.Type
}

// GetUpdatedDate returns the timestamp that represents when the subscription was last updated. Usually, this means the
// timestamp of the newest article in the subscription.
func (s *Subscription) GetUpdatedDate() time.Time {
	return s.Stats.LastUpdate
}

// GetTitle returns the title (or user nickname if assigned) of the subscription.
func (s *Subscription) GetTitle() string {
	return s.Customisation.Nickname
}

// GetCategories returns the categories of the subscription. It is the combined list of any user-assigned categories and
// the categories in the feed content.
func (s *Subscription) GetCategories(maxCount int) Categories {
	var all []Category
	if s.Customisation.Categories != nil {
		all = s.Customisation.Categories
		if maxCount != 0 {
			if len(all) > maxCount {
				return all[:maxCount]
			}
			return all
		}
	}
	return all
}

// GetImage retrieves the image that represents the subscription, or nil if no image is available.
func (s *Subscription) GetImage() URL {
	return s.Customisation.ImageURL
}

// GetLink returns the source feed link. For a search subscription, there is no source so this returns an empty string.
func (s *Subscription) GetLink() string {
	switch s.Type {
	case SubscriptionTypeFeed:
		return s.FeedData.URL
	default:
		return ""
	}
}

// GetStats returns the stats object containing the dynamically generated stats (i.e., unread count, article rate) of
// the subscription.
func (s *Subscription) GetStats() *SubscriptionStats {
	return &s.Stats
}

// IsFavorite returns a boolean indicating whether the user has marked this subscription as a favorite. Note for some
// subscription types, this will always return true.
func (s *Subscription) IsFavorite() bool {
	return s.Favorite
}

// Mark applies the given mark (read/unread) to a subscription.
func (s *Subscription) Mark(user *User, mark Mark) {
	switch mark {
	case MarkRead:
		// Set marked at to now when marking read.
		s.MarkedReadAt = time.Now().UTC()
	case MarkUnread:
		// Set marked at to max history when marking unread.
		s.MarkedReadAt = user.GetMaxHistory()
	}
	// Reset article states for feed subscriptions as well
	if s.GetSubscriptionType() == SubscriptionTypeFeed {
		s.FeedData.ArticleStates = nil
	}
}

// MarkItemsRead will mark the given items as read for the subscription.
func (s *Subscription) MarkItemsRead(itemIDs ...ItemID) {
	for itemID := range slices.Values(itemIDs) {
		if !s.FeedData.GetItemState(itemID).Read {
			s.FeedData.SetItemState(itemID, &ArticleState{Read: true, UpdatedAt: time.Now().UTC()})
		}
	}
}

// MarkItemsUnread will mark the given items as unread for the subscription.
func (s *Subscription) MarkItemsUnread(itemIDs ...ItemID) {
	for itemID := range slices.Values(itemIDs) {
		if s.FeedData.GetItemState(itemID).Read {
			s.FeedData.SetItemState(itemID, &ArticleState{Read: false, UpdatedAt: time.Now().UTC()})
		}
	}
}

// MarkItems marks the given items in a user subscription the given mark.
func (s *Subscription) MarkItems(mark Mark, itemIDs ...ItemID) {
	switch mark {
	case MarkRead:
		s.MarkItemsRead(itemIDs...)
	case MarkUnread:
		s.MarkItemsUnread(itemIDs...)
	}
}

// Subscriptions is a slice of subscriptions of any type.
type Subscriptions []*Subscription

// GetIDs returns the subscription ids for all subscription states in the slice.
func (s Subscriptions) GetIDs() []SubscriptionID {
	ids := make([]SubscriptionID, 0, len(s))
	for subscription := range slices.Values(s) {
		ids = append(ids, subscription.GetID())
	}
	return ids
}

// GetFeedIDs returns the IDs of feeds the subscriptions are for. This may return an empty slice if the subscriptions
// are only of type search, for example as those subscriptions do not represent any particular feed.
func (s Subscriptions) GetFeedIDs() []FeedID {
	ids := make([]FeedID, 0)
	for subscription := range slices.Values(s) {
		switch subscription.Type {
		case SubscriptionTypeFeed:
			ids = append(ids, subscription.FeedData.FeedID)
		case SubscriptionTypeGroup:
			ids = append(ids, subscription.GroupData.Subscriptions...)
		}
	}
	return slices.Compact(ids)
}

// GetByFeedID will return the subscription that matches the given feed ID, if any.
func (s Subscriptions) GetByFeedID(id FeedID) *Subscription {
	if idx := slices.IndexFunc(s, func(e *Subscription) bool {
		return e.FeedData.GetFeedID() == id
	}); idx != -1 {
		return s[idx]
	}
	return nil
}

// FilterByCategories returns a new slice containing the subscriptions which have a category matching the given
// categories.
func (s Subscriptions) FilterByCategories(categories ...Category) Subscriptions {
	if len(categories) == 0 {
		return s
	}
	return slices.Collect(FilterSlice(s, func(subscription *Subscription) bool {
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
		return slices.Collect(FilterSlice(s, func(subscription *Subscription) bool {
			return !subscription.GetStats().IsUnread()
		}))
	case ViewUnread:
		return slices.Collect(FilterSlice(s, func(subscription *Subscription) bool {
			return subscription.GetStats().IsUnread()
		}))
	default:
		return s
	}
}

// FilterByFavorites returns a slice containing only favorite subscriptions.
func (s Subscriptions) FilterByFavorites(value bool) Subscriptions {
	if !value {
		return s
	}
	return slices.Collect(FilterSlice(s, func(subscription *Subscription) bool {
		return subscription.IsFavorite()
	}))
}

// FilterByType returns a slice containing subscriptions of the specified type.
func (s Subscriptions) FilterByType(t SubscriptionType) Subscriptions {
	return slices.Collect(FilterSlice(s, func(subscription *Subscription) bool {
		return subscription.GetSubscriptionType() == t
	}))
}

// Sort will sort the slice of subscriptions by the given sort option. Favorite subscriptions are always sorted before
// other subscriptions, and the sort option is used as a tiebreaker.
func (s Subscriptions) Sort(sort Sort) Subscriptions {
	sort = setValidSort(sort)
	switch sort {
	case SortNewestFirst, SortOldestFirst:
		slices.SortFunc(s, func(subscriptionA, subscriptionB *Subscription) int {
			switch {
			case subscriptionA.IsFavorite() && !subscriptionB.IsFavorite(): // Favorites before non-favorites.
				return 1
			case !subscriptionA.IsFavorite() && subscriptionB.IsFavorite(): // Favorites before non-favorites.
				return -1
			default:
				return subscriptionA.GetUpdatedDate().Compare(subscriptionB.GetUpdatedDate())
			}
		})
		// Reverse sort for newest first.
		if sort == SortNewestFirst {
			slices.Reverse(s)
		}
	case SortMostUnread, SortLeastUnread:
		// Sort by unread count, with favorite or search subscriptions before non-favorites/non-search subscriptions.
		slices.SortFunc(s, func(subscriptionA, subscriptionB *Subscription) int {
			switch {
			case subscriptionA.IsFavorite() && !subscriptionB.IsFavorite(): // Favorites before non-favorites.
				return 1
			case !subscriptionA.IsFavorite() && subscriptionB.IsFavorite(): // Favorites before non-favorites.
				return -1
			case subscriptionA.GetSubscriptionType() != SubscriptionTypeFeed && subscriptionB.GetSubscriptionType() == SubscriptionTypeFeed: // Non-feed type before feed type.
				return 1
			case subscriptionA.GetSubscriptionType() == SubscriptionTypeFeed && subscriptionB.GetSubscriptionType() != SubscriptionTypeFeed: // Non-feed type before feed type.
				return -1
			case subscriptionA.GetSubscriptionType() != SubscriptionTypeFeed && subscriptionB.GetSubscriptionType() != SubscriptionTypeFeed: // Use title is tiebreaker where both non feed type.
				return cmp.Compare(subscriptionA.GetTitle(), subscriptionB.GetTitle())
			default:
				cmpValue := cmp.Compare(subscriptionA.GetStats().UnreadTotal(), subscriptionB.GetStats().UnreadTotal())
				if cmpValue == 0 { // Use date as tiebreaker for equal unread counts.
					return subscriptionA.GetUpdatedDate().Compare(subscriptionB.GetUpdatedDate())
				}
				return cmpValue
			}
		})
		// Reverse sort for most unread.
		if sort == SortMostUnread {
			slices.Reverse(s)
		}
	}
	return s
}

// Paginate will paginate through a slice of subscriptions, returning a new slice of subscriptions and the next
// pagination value (if any).
func (s Subscriptions) Paginate(pagination Pagination, count int) (Subscriptions, Pagination) {
	var from, to int
	if pagination != "" {
		if value, err := strconv.Atoi(pagination); err == nil {
			from = value
		}
	}
	to = min(from+count, len(s))
	pagination = strconv.Itoa(to)
	return s[from:to], pagination
}

// GetCategoryCounts returns a count of the occurrence of a Category across all
// the Subscriptions.
func GetCategoryCounts(subscriptions ...*Subscription) CategoryCounts {
	countsMap := make(map[Category]int)
	for object := range slices.Values(subscriptions) {
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

// Valid will return an error if the request object does not pass validation.
func (r *ListRequest) Valid() error {
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("validate list subscription request: %w", err)
	}
	if err := r.Filters.Valid(); err != nil {
		return fmt.Errorf("validate filters: %w", err)
	}
	return nil
}

// Valid returns a boolean indicating whether the SubscriptionRequest is valid,
// and any validation errors if applicable.
func (r *AddFeedSubscriptionRequest) Valid() error {
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("subscription validation error: %w", err)
	}
	return nil
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

// Valid returns a boolean indicating whether the SubscriptionRequest is valid,
// and any validation errors if applicable.
func (r *SearchSubscriptionRequest) Valid() error {
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("subscription validation error: %w", err)
	}
	return nil
}

// Sanitise will sanitise the input values of the SubscriptionRequest.
func (r *SearchSubscriptionRequest) Sanitise() error {
	if err := r.Search.Sanitise(); err != nil {
		return err
	}
	if r.Customisation.Nickname != "" {
		r.Customisation.Nickname = validation.SanitizeString(r.Customisation.Nickname)
	}
	categories := make([]Category, 0, len(r.Customisation.Categories))
	for category := range slices.Values(r.Customisation.Categories) {
		category = validation.SanitizeString(category)
		categories = append(categories, category)
	}
	r.Customisation.Categories = categories
	return nil
}

// Valid returns a boolean indicating whether the GroupSubscriptionRequest is valid,
// and any validation errors if applicable.
func (r *GroupSubscriptionRequest) Valid() error {
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("group subscription error: %w", err)
	}
	return nil
}

// Sanitise will sanitise the input values of the GroupSubscriptionRequest.
func (r *GroupSubscriptionRequest) Sanitise() error {
	if r.Customisation.Nickname != "" {
		r.Customisation.Nickname = validation.SanitizeString(r.Customisation.Nickname)
	}
	categories := make([]Category, 0, len(r.Customisation.Categories))
	for category := range slices.Values(r.Customisation.Categories) {
		category = validation.SanitizeString(category)
		categories = append(categories, category)
	}
	r.Customisation.Categories = categories
	return nil
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
func (s *EditSubscriptionRequest) Valid() error {
	if err := validation.Validate.Struct(s); err != nil {
		return fmt.Errorf("subscription is invalid: %w", err)
	}
	return nil
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
func (s *EditSubscriptionRequest) HasError() bool {
	return s.NicknameErr != nil || s.CategoriesErr != nil || s.ImageErr != nil
}

// Valid checks that the MarkSubscriptionsRequest contains valid data.
func (s *MarkSubscriptionsRequest) Valid() error {
	if err := validation.Validate.Struct(s); err != nil {
		return fmt.Errorf("mark subscriptions request is invalid: %w", err)
	}
	return nil
}

// Sanitise will sanitise the MarkSubscriptionsRequest, ensuring it contains valid field values.
func (s *MarkSubscriptionsRequest) Sanitise() error {
	for idx, id := range s.Subscriptions {
		s.Subscriptions[idx] = validation.SanitizeString(id)
	}
	s.View = setValidView(s.View)
	return nil
}

// Valid checks that the RemoveSubscriptionRequest contains valid data.
func (r *RemoveSubscriptionRequest) Valid() error {
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("remove subscription request is invalid: %w", err)
	}
	return nil
}

// AddSubscriptionResult represents the result of creating a new subscription.
type AddSubscriptionResult struct {
	Subscription *FeedSubscription
	Message      *UserMessage
}

// UpdateFrequency returns a string that roughly indicates how often the subscription is updated.
func (s *SubscriptionStats) UpdateFrequency() string {
	switch {
	case s.AvgDailyUpdates > 1:
		return fmt.Sprintf("%.0f articles/day", s.AvgDailyUpdates)
	case s.AvgDailyUpdates < 1 && s.AvgDailyUpdates > 0.5:
		return "A few times a week"
	case s.AvgDailyUpdates < 0.5 && s.AvgDailyUpdates > 0.25:
		return "About weekly"
	default:
		return "Infrequent"
	}
}

// UnreadTotal returns the unread count of items in the subscription.
func (s *SubscriptionStats) UnreadTotal() int {
	return s.UnreadCount
}

// IsUnread returns a boolean indicating whether the subscription is considered unread.
func (s *SubscriptionStats) IsUnread() bool {
	return s.UnreadCount > 0
}

func newSubscription(
	ctx context.Context,
	customisation SubscriptionCustomisation,
	settings SubscriptionSettings,
	data any,
) (*Subscription, error) {
	user, err := UserFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("get user data: %w", err)
	}
	ts := time.Now().UTC()
	subscription := &Subscription{
		SubscriptionID: strings.Join(
			[]string{"sub_", strconv.FormatUint(murmur3.Sum64([]byte(user.GetID()+customisation.Nickname)), 10)},
			"_",
		),
		UserID:        user.GetID(),
		UpdatedAt:     ts,
		CreatedAt:     ts,
		MarkedReadAt:  user.GetMaxHistory(),
		Customisation: customisation,
		Settings:      settings,
		Favorite:      false,
	}

	switch typeData := data.(type) {
	case *FeedSubscription:
		subscription.Type = SubscriptionTypeFeed
		subscription.FeedData = *typeData
	case *SearchSubscription:
		subscription.Type = SubscriptionTypeSearch
		subscription.SearchData = *typeData
		subscription.Favorite = true
	case *GroupSubscription:
		subscription.Type = SubscriptionTypeGroup
		subscription.GroupData = *typeData
		subscription.Favorite = true
	default:
		return nil, fmt.Errorf("new subscription: %w", ErrInvalidSubscriptionData)
	}

	return subscription, nil
}

func newSubscriptionCustomisation(nickname, url string, categories Categories) *SubscriptionCustomisation {
	return &SubscriptionCustomisation{
		Nickname:   nickname,
		ImageURL:   url,
		Categories: categories,
	}
}

func newSubscriptionSettings() *SubscriptionSettings {
	return &SubscriptionSettings{
		ShowFullArticleContent: false,
	}
}
