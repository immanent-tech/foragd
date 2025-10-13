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
	"strconv"
	"strings"
	"time"

	"github.com/immanent-tech/go-syndication/types"

	"github.com/immanent-tech/foragd/validation"
)

var ErrInvalidSubscriptionState = errors.New("invalid subscription state")

// GenerateSubscription creates a subscription from the given data sources: a feed, any user customisation of feed
// values, subscription state and an unread count. All data besides the feed is optional.
func GenerateSubscription(metadata *SubscriptionMetadata, feed *Feed, count int, favorite bool) (*Subscription, error) {
	subscription := &Subscription{
		Metadata:    *metadata,
		Feed:        *feed,
		Favorite:    favorite,
		UnreadCount: count,
	}
	// Validate the subscription.
	valid, err := subscription.Valid()
	if err != nil || !valid {
		return nil, fmt.Errorf("subscription data is invalid: %w", err)
	}
	return subscription, nil
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

// func (s *Subscription) String() string {
// 	return s.GetTitle()
// }

func (s *Subscription) GetID() string {
	return s.Metadata.GetID()
}

func (s *Subscription) GetFeedID() FeedID {
	return s.Feed.GetID()
}

func (s *Subscription) GetTitle() string {
	if s.Metadata.Customisation.Nickname != "" {
		return s.Metadata.Customisation.Nickname
	}
	return s.Feed.GetTitle()
}

func (s *Subscription) GetLink() string {
	return s.Feed.URL
}

func (s *Subscription) GetDescription() string {
	return s.Feed.GetDescription()
}

func (s *Subscription) GetCategories(max int) []Category {
	var all []Category
	if s.Metadata.Customisation.Categories != nil {
		all = slices.Compact(slices.Concat(s.Metadata.Customisation.Categories, s.Feed.GetCategories()))
	} else {
		all = s.Feed.GetCategories()
	}
	if max != 0 {
		if len(all) > max {
			return all[:max]
		} else {
			return all
		}
	}
	return all
}

func (s *Subscription) GetAuthors() []string {
	return s.Feed.GetAuthors()
}

func (s *Subscription) GetUpdatedDate() time.Time {
	return s.Feed.LastFetched
}

// GetImage retrieves the image that represents the subscription, or nil if no image is available.
func (s *Subscription) GetImage() *types.ImageInfo {
	if s.Feed.GetImage() != nil && s.Feed.GetImage().GetURL() != "" {
		return s.Feed.GetImage()
	}
	return nil
}

func (s *Subscription) GetUnreadCount() int {
	return s.UnreadCount
}

func (s *Subscription) SetUnreadCount(count int) {
	s.UnreadCount = count
}

// IsUnread returns a boolean indicating whether the subscription is considered unread.
func (s *Subscription) IsUnread() bool {
	return s.UnreadCount > 0
}

// IsFavorite returns a boolean indicating whether the subscription has been favorited.
func (s *Subscription) IsFavorite() bool {
	return s.Favorite
}

// Type returns the type of the object, in this case, "subscription".
func (s *Subscription) Type() string {
	return "subscription"
}

func (s *Subscription) ViewURL() string {
	return "/articles"
}

func (s *Subscription) MarkURL(mark Mark) string {
	return "/subscription/" + s.GetID() + "/mark/" + string(mark)
}

type SubscriptionsSlice []*Subscription

func (s SubscriptionsSlice) FilterByCategories(categories ...Category) SubscriptionsSlice {
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

func (s SubscriptionsSlice) FilterByView(view View) SubscriptionsSlice {
	switch view {
	case ViewRead:
		return slices.Collect(FilterSlice(s, func(subscription *Subscription) bool {
			return !subscription.IsUnread()
		}))
	case ViewUnread:
		return slices.Collect(FilterSlice(s, func(subscription *Subscription) bool {
			return subscription.IsUnread()
		}))
	default:
		return s
	}
}

func (s SubscriptionsSlice) Sort(sort *Sort) SubscriptionsSlice {
	if sort == nil {
		sort = &Sort{
			SortBy:    SortByUnreadCount,
			SortOrder: SortOrderDesc,
		}
	}
	switch sort.SortBy {
	case SortByLastUpdated:
		slices.SortFunc(s, func(a, b *Subscription) int {
			return a.GetUpdatedDate().Compare(b.GetUpdatedDate())
		})
	case SortByUnreadCount:
		slices.SortFunc(s, func(a, b *Subscription) int {
			cmpValue := cmp.Compare(a.GetUnreadCount(), b.GetUnreadCount())
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

func (s SubscriptionsSlice) Paginate(pagination Pagination, count int) (SubscriptionsSlice, Pagination) {
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
func (s SubscriptionsSlice) GetTotalUnreadCount() int {
	var unread int
	for subscription := range slices.Values(s) {
		unread += subscription.GetUnreadCount()
	}
	return unread
}

// GetCategoryCounts returns a count of the occurrence of a Category across all
// the Subscriptions.
func (s SubscriptionsSlice) GetCategoryCounts() CategoryCounts {
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

// GetFeedIDs returns the feed ids of all subscriptions in the slice.
func (s SubscriptionsSlice) GetFeedIDs() []FeedID {
	ids := make([]FeedID, 0, len(s))
	for subscription := range slices.Values(s) {
		ids = append(ids, subscription.GetFeedID())
	}
	return ids
}

// GetSubscriptionMetadata returns the metadata for each subscription in the slice.
func (s SubscriptionsSlice) GetSubscriptionMetadata() SubscriptionMetadataSlice {
	metadata := make(SubscriptionMetadataSlice, 0, len(s))
	for subscription := range slices.Values(s) {
		metadata = append(metadata, &subscription.Metadata)
	}
	return metadata
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

func (r *SubscriptionRequest) GetURL() string {
	return strings.TrimSpace(r.URL)
}

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

type SubscriptionRequests []*SubscriptionRequest

// NewSubscriptionMetadata creates a new subscription state with the given subscription and feed ids.
func NewSubscriptionMetadata(user *User, feed *Feed, request *SubscriptionRequest) *SubscriptionMetadata {
	ts := time.Now().UTC()
	// Create state based on feed and user data.
	state := &SubscriptionMetadata{
		SubscriptionID: NewID(SubscriptionPFX),
		UpdatedAt:      ts,
		CreatedAt:      ts,
		MarkedReadAt:   user.GetMaxHistory(),
		FeedID:         feed.GetID(),
		Customisation: SubscriptionCustomisation{
			Nickname:   feed.GetTitle(),
			Categories: feed.GetCategories(),
		},
		ItemStates: make(map[ItemID]ArticleState),
	}
	// Add any user customisations.
	if request != nil {
		if request.Nickname != "" {
			state.Customisation.Nickname = request.Nickname
		}
		if len(request.Categories) > 0 {
			state.Customisation.Categories = request.Categories
		}
	}
	return state
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
	return nil
}

// HasError wil return true if the subscription request has errors associated with any of its fields.
func (r *EditSubscriptionRequest) HasError() bool {
	return r.NicknameErr != nil || r.CategoriesErr != nil || r.ImageErr != nil
}

// Valid returns a boolean indicating whether the object is valid.
func (r *MarkSubscriptionsRequest) Valid() (bool, error) {
	if r == nil {
		return false, fmt.Errorf("request is invalid: %w", validation.ErrNilObject)
	}
	valid, err := validation.ValidateStruct(r)
	if !valid || err != nil {
		return false, fmt.Errorf("request is invalid: %w", err)
	}
	return true, nil
}

// Sanitise will sanitise the input values.
func (r *MarkSubscriptionsRequest) Sanitise() error {
	return nil
}

// Valid returns a boolean indicating whether the object is valid.
func (r *RemoveSubscriptionsRequest) Valid() (bool, error) {
	if r == nil {
		return false, fmt.Errorf("request is invalid: %w", validation.ErrNilObject)
	}
	valid, err := validation.ValidateStruct(r)
	if !valid || err != nil {
		return false, fmt.Errorf("request is invalid: %w", err)
	}
	return true, nil
}

// Sanitise will sanitise the input values.
func (r *RemoveSubscriptionsRequest) Sanitise() error {
	if r.Confirmation == "" {
		r.Confirmation = UserConfirmationNo
	}
	return nil
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

// Valid returns a boolean indicating whether the data for the subscription issue is valid.
func (r *SubscriptionIssue) Valid() (bool, error) {
	valid, err := validation.ValidateStruct(r)
	if !valid || err != nil {
		return false, fmt.Errorf("request is invalid: %w", err)
	}
	return true, nil
}

// Sanitise will clean the data in the subscription issue.
func (r *SubscriptionIssue) Sanitise() error {
	r.Details = validation.SanitizeString(r.Details)
	return nil
}

func (s *SubscriptionStats) GetDailyUpdates() int {
	return int(math.Round(s.AvgDailyUpdates))
}
