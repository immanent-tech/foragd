// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"net/mail"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/operator"
	"github.com/zeebo/xxh3"

	"github.com/immanent-tech/go-syndication/sanitization"

	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/validation"
)

// BuildItemQueries generates a slices of queries for the given subscriptions, based on the given filters.
func BuildItemQueries(
	user *User,
	view View,
	subscriptions Subscriptions,
) []query.Option {
	queries := make([]query.Option, 0, len(subscriptions))
	// Work out what query to use based on the state filter.
	if len(subscriptions) == 0 {
		return nil
	}
	for subscription := range slices.Values(subscriptions) {
		// Ignore subscriptions that aren't based on a feed object.
		if subscription.GetFeedID() == "" {
			continue
		}

		switch view {
		case ViewRead:
			queries = append(queries, queryReadItems(user, subscription))
		case ViewAll:
			queries = append(queries, queryAllItems(user, subscription))
		case ViewUnread:
			fallthrough
		default:
			queries = append(queries, queryUnreadItems(user, subscription))
		}
	}
	return queries
}

// queryReadItems generates a query for finding read items for the given subscription.
func queryReadItems(user *User, source ItemSource) query.Option {
	// if subscription.GetSubscriptionType() != SubscriptionTypeFeed {
	// 	return nil
	// }
	return query.Bool(
		query.WithBoolQueryName(source.GetFeedID()+"_read_items"),
		query.Filter(
			// Must match this feed.
			query.Term("feed_id", source.GetFeedID()),
			// And should be between the user max history and last read time.
			query.Bool(
				query.Should(
					query.Between("published", user.GetMaxHistory(), source.GetMarkedReadAt()),
					query.Between("updated", user.GetMaxHistory(), source.GetMarkedReadAt()),
					query.Terms("item_id", source.GetReadItems(), query.WithQueryName[*query.TermsQuery]("read-items")),
				),
				// Must not match any unread items for the feed
				query.MustNot(
					query.Terms(
						"item_id",
						source.GetUnreadItems(),
						query.WithQueryName[*query.TermsQuery]("unread-items"),
					),
				),
			),
		),
		// User-specified field-level filtering.
		ArticleFiltersQueryClause(source),
	)
}

// QueryUnreadItems generates a query for finding unread items for the given subscription.
func queryUnreadItems(_ *User, source ItemSource) query.Option {
	return query.Bool(
		query.WithBoolQueryName(source.GetFeedID()+"_unread_items"),
		query.Filter(
			// Must match this feed.
			query.Term("feed_id", source.GetFeedID()),
			query.Bool(
				query.Should(
					query.Since("published", source.GetMarkedReadAt()),
					query.Since("updated", source.GetMarkedReadAt()),
					query.Terms(
						"item_id",
						source.GetUnreadItems(),
						query.WithQueryName[*query.TermsQuery]("unread-items"),
					),
				),
			),
		),
		// Must not match any read items for the feed
		query.MustNot(
			query.Terms("item_id", source.GetReadItems(), query.WithQueryName[*query.TermsQuery]("read-items")),
		),
		// User-specified field-level filtering.
		ArticleFiltersQueryClause(source),
	)
}

// subscriptionQueryReadItems generates a query for finding all items for the given subscription.
func queryAllItems(user *User, source ItemSource) query.Option {
	maxHistory := user.GetMaxHistory()
	return query.Bool(
		query.WithBoolQueryName(source.GetFeedID()+"_all_items"),
		query.Filter(
			// Must match this feed.
			query.Term("feed_id", source.GetFeedID()),
			// And be published/updated since the user max history.
			query.Bool(
				query.Should(
					query.Since("published", maxHistory),
					query.Since("updated", maxHistory),
				),
			),
		),
		// User-specified field-level filtering.
		ArticleFiltersQueryClause(source),
	)
}

func ArticleFiltersQueryClause(source ItemSource) query.BoolOption {
	return query.Must(
		query.SimpleQueryString(
			query.WithSimpleQueryStringText(source.GetArticleFilters().Text),
			query.WithSimpleQueryStringFields("title", "description", "content"),
			query.WithSimpleQueryStringOperator(&operator.And),
		),
		query.SimpleQueryString(
			query.WithSimpleQueryStringText(source.GetArticleFilters().Authors),
			query.WithSimpleQueryStringFields("authors", "contributors"),
			query.WithSimpleQueryStringOperator(&operator.And),
		),
		query.SimpleQueryString(
			query.WithSimpleQueryStringText(source.GetArticleFilters().Categories),
			query.WithSimpleQueryStringFields("categories"),
			query.WithSimpleQueryStringOperator(&operator.And),
		),
	)
}

// NewFeedSubscription creates a new subscription for a feed with any user customisations given.
func NewFeedSubscription(
	ctx context.Context,
	feed *Feed,
	customisation *SubscriptionCustomisation,
) (*Subscription, error) {
	// Create state based on feed and user data.
	feedSubscription := &FeedSubscription{
		FeedID:        feed.GetID(),
		ArticleStates: make(map[ItemID]ArticleState),
	}

	// Set up subscription customisation.
	if customisation == nil {
		customisation = &SubscriptionCustomisation{}
	}
	// Make sure nickname is not empty.
	if customisation.GetNickname() == "" {
		customisation.Nickname = new(feed.GetTitle())
	}
	// Add the feed image if the user has not specified one.
	if customisation.ImageURL == nil && feed.GetImage() != nil {
		customisation.ImageURL = new(feed.GetImage().GetURL())
	}

	// Create the subscription with the feed data and customisations.
	subscription, err := newSubscription(ctx, *customisation, newSubscriptionSettings(), feedSubscription)
	if err != nil {
		return nil, fmt.Errorf("new feed subscription: %w", err)
	}

	// Validate.
	if err := subscription.Valid(); err != nil {
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

// NewSearchSubscription creates a new SearchSubscription. A SearchSubscription collates articles that match a search
// into a single custom subscription.
func NewSearchSubscription(ctx context.Context, request *SearchSubscriptionRequest) (*Subscription, error) {
	searchSubscription := &SearchSubscription{
		Search: request.Search,
	}
	subscription, err := newSubscription(ctx, *request.Customisation, request.Settings, searchSubscription)
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
		Subscriptions: slices.Collect(maps.Keys(request.Subscriptions)),
	}
	subscription, err := newSubscription(ctx, *request.Customisation, request.Settings, groupSubscription)
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

func NewEmailSubscription(
	ctx context.Context,
	userID UserID,
	from *mail.Address,
) (*Subscription, error) {
	// Validate sender address.
	if from.Address == "" {
		return nil, fmt.Errorf("%w: blank sender address", validation.ErrInvalid)
	}
	if err := validation.Validate.Var(from.Address, "required,email"); err != nil {
		return nil, fmt.Errorf("%w: sender address: %w", validation.ErrInvalid, err)
	}

	emailSubscription := &EmailSubscription{
		EmailSenderID: from.Address,
	}
	customisation := newSubscriptionCustomisation(from.String(), nil, nil)
	settings := newSubscriptionSettings()

	subscription, err := newSubscription(ctx, *customisation, settings, emailSubscription)
	if err != nil {
		return nil, fmt.Errorf("new group subscription: %w", err)
	}

	// Override the default SubscriptionID generation
	subscription.SubscriptionID = "sub_" + strconv.FormatUint(
		xxh3.Hash([]byte(userID+from.Address)),
		10,
	)

	// Generate a "virtual" FeedID.
	subscription.EmailData.FeedID = strings.ReplaceAll(subscription.GetID(), "sub_", "feed_")

	return subscription, nil
}

// Valid returns a non-nil error if the EmailSubscription contains invalid data.
func (s *EmailSubscription) Valid() error {
	if err := validation.Validate.Struct(s); err != nil {
		return fmt.Errorf("email subscription is invalid: %w", err)
	}
	return nil
}

// Valid returns a boolean indicating whether the SubscriptionRequest is valid,
// and any validation errors if applicable.
func (r *EditEmailSubscriptionRequest) Valid() error {
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("email subscription validation error: %w", err)
	}
	return nil
}

// Sanitise will sanitise the input values of the SubscriptionRequest.
func (r *EditEmailSubscriptionRequest) Sanitise() error {
	r.Customisation.Sanitise()
	return nil
}

// Valid returns a boolean indicating if the Subscription contains valid data (true). If it contains invalid data
// (false) a non-nil error is also returned which contains validation issues.
func (s *Subscription) Valid() error {
	if err := validation.Validate.Struct(s); err != nil {
		return fmt.Errorf("subscription is invalid: %w", err)
	}
	// Subscriptions require a nickname set.
	if s.Customisation.GetNickname() == "" {
		return fmt.Errorf("%w: nickname is required", validation.ErrInvalid)
	}
	return nil
}

// GetID returns the unqiue ID of the subscription.
func (s *Subscription) GetID() SubscriptionID {
	return s.SubscriptionID
}

// GetFeedID returns the unqiue FeedID of the subscription. Not all subscription types will have a FeedID.
func (s *Subscription) GetFeedID() FeedID {
	switch s.Type {
	case SubscriptionTypeFeed:
		return s.FeedData.FeedID
	case SubscriptionTypeEmail:
		return s.EmailData.FeedID
	default:
		return ""
	}
}

// GetSubscriptionType returns the type of subscription (i.e., feed, search, group, etc.).
func (s *Subscription) GetSubscriptionType() SubscriptionType {
	return s.Type
}

// GetUpdatedDate returns the timestamp that represents when the subscription was last updated. Usually, this means the
// timestamp of the newest article in the subscription.
func (s *Subscription) GetUpdatedDate() time.Time {
	if s.Stats == nil {
		return UnixEpoch
	}
	return s.Stats.LastUpdate
}

// GetTitle returns the title (or user nickname if assigned) of the subscription.
func (s *Subscription) GetTitle() string {
	return s.Customisation.GetNickname()
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
	if s.Customisation != nil {
		if s.Customisation.ImageURL != nil {
			return *s.Customisation.ImageURL
		}
	}
	return ""
}

// GetLink returns the source feed link. For a search subscription, there is no source so this returns an empty string.
func (s *Subscription) GetLink() string {
	switch s.Type {
	case SubscriptionTypeFeed:
		return ""
	default:
		return ""
	}
}

func (s *Subscription) GetArticleFilters() SubscriptionArticleFilters {
	switch {
	case s.Type == SubscriptionTypeFeed && s.FeedData.ArticleFilters != nil:
		return *s.FeedData.ArticleFilters
	case s.Type == SubscriptionTypeEmail && s.EmailData.ArticleFilters != nil:
		return *s.EmailData.ArticleFilters
	case s.Type == SubscriptionTypeGroup && s.GroupData.ArticleFilters != nil:
		return *s.GroupData.ArticleFilters
	default:
		return SubscriptionArticleFilters{}
	}
}

// GetStats returns the stats object containing the dynamically generated stats (i.e., unread count, article rate) of
// the subscription.
func (s *Subscription) GetStats() *SubscriptionStats {
	if s.Stats != nil {
		return s.Stats
	}
	s.Stats = &SubscriptionStats{}
	return s.Stats
}

// IsFavorite returns a boolean indicating whether the user has marked this subscription as a favorite. Note for some
// subscription types, this will always return true.
func (s *Subscription) IsFavorite() bool {
	return s.Favorite
}

func (s *Subscription) GetMarkedReadAt() time.Time {
	if s.MarkedReadAt != nil {
		return *s.MarkedReadAt
	}
	return UnixEpoch
}

// GetReadItems retrieves a list of ItemIDs for the feed subscription that
// user has explicitly marked as read.
func (s *Subscription) GetReadItems() []ItemID {
	ids := make([]ItemID, 0)
	states := make(map[ItemID]ArticleState)
	switch s.Type {
	case SubscriptionTypeFeed:
		maps.Copy(states, s.FeedData.ArticleStates)
	case SubscriptionTypeEmail:
		maps.Copy(states, s.EmailData.ArticleStates)
	default:
		return nil
	}

	for id, state := range states {
		if state.Read {
			ids = append(ids, id)
		}
	}
	return ids
}

// GetUnreadItems retrieves a list of ItemIDs for the feed subscription that
// user has explicitly marked as unread.
func (s *Subscription) GetUnreadItems() []ItemID {
	ids := make([]ItemID, 0)
	states := make(map[ItemID]ArticleState)
	switch s.Type {
	case SubscriptionTypeFeed:
		maps.Copy(states, s.FeedData.ArticleStates)
	case SubscriptionTypeEmail:
		maps.Copy(states, s.EmailData.ArticleStates)
	default:
		return nil
	}

	for id, state := range states {
		if !state.Read {
			ids = append(ids, id)
		}
	}
	return ids
}

// GetItemState retrieves the item state (read/unread/saved) from the
// subscription. By default it will return unread unless the user has explicitly
// marked or saved the item.
func (s *Subscription) GetItemState(itemID ItemID) *ArticleState {
	states := make(map[ItemID]ArticleState)
	switch s.Type {
	case SubscriptionTypeFeed:
		maps.Copy(states, s.FeedData.ArticleStates)
	case SubscriptionTypeEmail:
		maps.Copy(states, s.EmailData.ArticleStates)
	default:
		return &ArticleState{
			Read: false,
		}
	}

	// If the subscription has no article states, return unread state.
	if len(states) == 0 {
		return &ArticleState{
			Read: false,
		}
	}

	// If a state is found return that.
	if state, found := states[itemID]; found {
		return &state
	}

	// Return unread state if no state found.
	return &ArticleState{
		Read: false,
	}
}

// SetItemState will set the state of the item to the given state.
func (s *Subscription) SetItemState(itemID ItemID, state *ArticleState) {
	switch s.Type {
	case SubscriptionTypeFeed:
		if s.FeedData.ArticleStates == nil {
			s.FeedData.ArticleStates = make(map[ItemID]ArticleState)
		}
		s.FeedData.ArticleStates[itemID] = *state
	case SubscriptionTypeEmail:
		if s.EmailData.ArticleStates == nil {
			s.EmailData.ArticleStates = make(map[ItemID]ArticleState)
		}
		s.EmailData.ArticleStates[itemID] = *state
	}
}

// Mark applies the given mark (read/unread) to a subscription.
func (s *Subscription) Mark(user *User, mark Mark) {
	var ts time.Time
	switch mark {
	case MarkRead:
		// Set marked at to now when marking read.
		ts = time.Now().UTC()
	case MarkUnread:
		// Set marked at to max history when marking unread.
		ts = user.GetMaxHistory()
	}
	s.MarkedReadAt = &ts
	// Reset article states for appropriate subscription types.
	switch s.GetSubscriptionType() {
	case SubscriptionTypeFeed:
		s.FeedData.ArticleStates = nil
	case SubscriptionTypeEmail:
		s.EmailData.ArticleStates = nil
	}
}

// MarkItemsRead will mark the given items as read for the subscription.
func (s *Subscription) MarkItemsRead(itemIDs ...ItemID) {
	for itemID := range slices.Values(itemIDs) {
		if !s.GetItemState(itemID).Read {
			s.SetItemState(itemID, &ArticleState{Read: true, UpdatedAt: time.Now().UTC()})
		}
	}
}

// MarkItemsUnread will mark the given items as unread for the subscription.
func (s *Subscription) MarkItemsUnread(itemIDs ...ItemID) {
	for itemID := range slices.Values(itemIDs) {
		if s.GetItemState(itemID).Read {
			s.SetItemState(itemID, &ArticleState{Read: false, UpdatedAt: time.Now().UTC()})
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
		case SubscriptionTypeFeed, SubscriptionTypeEmail:
			ids = append(ids, subscription.GetFeedID())
		case SubscriptionTypeGroup:
			ids = append(ids, subscription.GroupData.Subscriptions...)
		}
	}
	return slices.Compact(ids)
}

// GetCategories returns all categories across all the subscriptions. Duplicates are removed.
func (s Subscriptions) GetCategories() Categories {
	categories := make(Categories, 0)
	for subscription := range slices.Values(s) {
		categories = append(categories, subscription.GetCategories(0)...)
	}
	return slices.Compact(categories)
}

// GetByID will return the subscription that matches the given ID, if any.
func (s Subscriptions) GetByID(id SubscriptionID) *Subscription {
	if idx := slices.IndexFunc(s, func(e *Subscription) bool {
		return e.GetID() == id
	}); idx != -1 {
		return s[idx]
	}
	return nil
}

// GetByFeedID will return the subscription that matches the given feed ID, if any.
func (s Subscriptions) GetByFeedID(id FeedID) *Subscription {
	if idx := slices.IndexFunc(s, func(e *Subscription) bool {
		return e.GetFeedID() == id
	}); idx != -1 {
		return s[idx]
	}
	return nil
}

// FilterByIDs returns a new slice containing the subscriptions which have a SubscriptionID matching the given ids.
func (s Subscriptions) FilterByIDs(ids ...SubscriptionID) Subscriptions {
	if len(ids) == 0 {
		return s
	}
	return slices.Collect(FilterSlice(s, func(subscription *Subscription) bool {
		return slices.Contains(ids, subscription.GetID())
	}))
}

// FilterByFeedIDs returns a new slice containing the subscriptions which have a FeedID matching the given ids.
func (s Subscriptions) FilterByFeedIDs(ids ...FeedID) Subscriptions {
	if len(ids) == 0 {
		return s
	}
	return slices.Collect(FilterSlice(s, func(subscription *Subscription) bool {
		return slices.Contains(ids, subscription.GetFeedID())
	}))
}

// ExcludeIDs returns a new slice containing the subscriptions which DO NOT have an id matching the given IDs.
func (s Subscriptions) ExcludeIDs(ids ...SubscriptionID) Subscriptions {
	if len(ids) == 0 {
		return s
	}
	return slices.Collect(FilterSlice(s, func(subscription *Subscription) bool {
		return !slices.Contains(ids, subscription.GetID())
	}))
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
func (s Subscriptions) FilterByType(t ...SubscriptionType) Subscriptions {
	return slices.Collect(FilterSlice(s, func(subscription *Subscription) bool {
		return slices.Contains(t, subscription.GetSubscriptionType())
	}))
}

func (s Subscriptions) FilterEmailIDs(ids ...string) Subscriptions {
	if len(ids) == 0 {
		return s
	}
	return slices.Collect(FilterSlice(s.FilterByType(SubscriptionTypeEmail), func(subscription *Subscription) bool {
		return slices.Contains(ids, subscription.EmailData.EmailSenderID)
	}))
}

// Sort will sort the slice of subscriptions by the given sort option. Favorite subscriptions are always sorted before
// other subscriptions, and the sort option is used as a tiebreaker.
func (s Subscriptions) Sort(sort Sort) Subscriptions {
	if len(s) == 0 {
		return s
	}
	sort = setValidSort(sort)
	switch sort {
	case SortNewestFirst, SortOldestFirst:
		slices.SortFunc(s, func(subscriptionA, subscriptionB *Subscription) int {
			// switch {
			// case subscriptionA.IsFavorite() && !subscriptionB.IsFavorite(): // Favorites before non-favorites.
			// 	return 1
			// case !subscriptionA.IsFavorite() && subscriptionB.IsFavorite(): // Favorites before non-favorites.
			// 	return -1
			// default:
			return subscriptionA.GetUpdatedDate().Compare(subscriptionB.GetUpdatedDate())
			// }
		})
		// Reverse sort for newest first.
		if sort == SortNewestFirst {
			slices.Reverse(s)
		}
	case SortMostUnread, SortLeastUnread:
		// Sort by unread count, with favorite or search subscriptions before non-favorites/non-search subscriptions.
		slices.SortFunc(s, func(subscriptionA, subscriptionB *Subscription) int {
			// switch {
			// case subscriptionA.IsFavorite() && !subscriptionB.IsFavorite(): // Favorites before non-favorites.
			// 	return 1
			// case !subscriptionA.IsFavorite() && subscriptionB.IsFavorite(): // Favorites before non-favorites.
			// 	return -1
			// case subscriptionA.GetSubscriptionType() != SubscriptionTypeFeed && subscriptionB.GetSubscriptionType() == SubscriptionTypeFeed: // Non-feed type before feed type.
			// 	return 1
			// case subscriptionA.GetSubscriptionType() == SubscriptionTypeFeed && subscriptionB.GetSubscriptionType() != SubscriptionTypeFeed: // Non-feed type before feed type.
			// 	return -1
			// case subscriptionA.GetSubscriptionType() != SubscriptionTypeFeed && subscriptionB.GetSubscriptionType() != SubscriptionTypeFeed: // Use title is tiebreaker where both non feed type.
			// 	return cmp.Compare(subscriptionA.GetTitle(), subscriptionB.GetTitle())
			// default:
			cmpValue := cmp.Compare(subscriptionA.GetStats().UnreadTotal(), subscriptionB.GetStats().UnreadTotal())
			if cmpValue == 0 { // Use date as tiebreaker for equal unread counts.
				return subscriptionA.GetUpdatedDate().Compare(subscriptionB.GetUpdatedDate())
			}
			return cmpValue
			// }
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
	newPagination := strconv.Itoa(to)
	return s[from:to], newPagination
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

// Valid checks that the MarkSubscriptionRequest contains valid data.
func (s *MarkSubscriptionRequest) Valid() error {
	if err := validation.Validate.Struct(s); err != nil {
		return fmt.Errorf("mark subscription request is invalid: %w", err)
	}
	return nil
}

// Sanitise will sanitise the MarkSubscriptionRequest, ensuring it contains valid field values.
func (s *MarkSubscriptionRequest) Sanitise() error {
	return nil
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

// Valid checks that the FavoriteSubscriptionsRequest contains valid data.
func (s *FavoriteSubscriptionRequest) Valid() error {
	if err := validation.Validate.Struct(s); err != nil {
		return fmt.Errorf("favorite subscriptions request is invalid: %w", err)
	}
	return nil
}

// Sanitise will sanitise the FavoriteSubscriptionsRequest, ensuring it contains valid field values.
func (s *FavoriteSubscriptionRequest) Sanitise() error {
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
		return fmt.Sprintf("%.0f items/day", s.AvgDailyUpdates)
	case s.AvgDailyUpdates < 1 && s.AvgDailyUpdates > 0.5:
		return fmt.Sprintf("%.0f items/week", s.AvgDailyUpdates*7)
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
	settings *SubscriptionSettings,
	data any,
) (*Subscription, error) {
	user := UserFromCtx(ctx)
	if user == nil {
		return nil, fmt.Errorf("get user data: %w", ErrCtxValueNotFound)
	}
	ts := time.Now().UTC()
	mr := user.GetMaxHistory()
	subscription := &Subscription{
		SubscriptionID: "sub_" + strconv.FormatUint(xxh3.Hash([]byte(user.GetID()+customisation.GetNickname())), 10),
		UserID:         user.GetID(),
		UpdatedAt:      &ts,
		CreatedAt:      ts,
		MarkedReadAt:   &mr,
		Customisation:  &customisation,
		Settings:       *newSubscriptionSettings(),
		Favorite:       false,
	}
	if settings != nil {
		subscription.Settings = *settings
	}

	switch typeData := data.(type) {
	case *FeedSubscription:
		subscription.Type = SubscriptionTypeFeed
		subscription.FeedData = typeData
	case *SearchSubscription:
		subscription.Type = SubscriptionTypeSearch
		subscription.SearchData = typeData
		subscription.Favorite = true
	case *GroupSubscription:
		subscription.Type = SubscriptionTypeGroup
		subscription.GroupData = typeData
		subscription.Favorite = true
	case *EmailSubscription:
		subscription.Type = SubscriptionTypeEmail
		subscription.EmailData = typeData
	default:
		return nil, fmt.Errorf("new subscription: %w", ErrInvalidAPIResult)
	}

	return subscription, nil
}

const (
	ParamCustomisationCategories = "customisation.categories"
	ParamCustomisationNickname   = "customisation.nickname"
)

func newSubscriptionCustomisation(
	nickname string,
	image *RemoteImage,
	categories Categories,
) *SubscriptionCustomisation {
	customisation := &SubscriptionCustomisation{
		Nickname:   new(nickname),
		Categories: categories,
	}
	if image != nil {
		customisation.ImageURL = new(image.GetURL())
	}
	return customisation
}

func (c *SubscriptionCustomisation) GetNickname() string {
	if c.Nickname != nil {
		return *c.Nickname
	}
	return ""
}

func (c *SubscriptionCustomisation) GetImageURL() string {
	if c.ImageURL != nil {
		return *c.ImageURL
	}
	return ""
}

func (c *SubscriptionCustomisation) GetCategories() Categories {
	return c.Categories
}

func (c *SubscriptionCustomisation) Valid() error {
	return validation.Validate.Struct(c)
}

func (c *SubscriptionCustomisation) Sanitise() error {
	if c != nil {
		for idx := range c.Categories {
			c.Categories[idx] = sanitization.SanitizeString(c.Categories[idx])
		}
		slices.Sort(c.Categories)
		c.Categories = slices.Compact(c.Categories)
		if c.Nickname != nil {
			c.Nickname = new(sanitization.SanitizeString(*c.Nickname))
		}
	}
	return nil
}

func (f *SubscriptionArticleFilters) Valid() error {
	return validation.Validate.Struct(f)
}

func (f *SubscriptionArticleFilters) Sanitise() error {
	if f != nil {
		if f.Authors != nil {
			cleanAuthorFilters := validation.SanitizeString(*f.Authors)
			f.Authors = &cleanAuthorFilters
		}
		if f.Categories != nil {
			cleanCategoryFilters := validation.SanitizeString(*f.Categories)
			f.Categories = &cleanCategoryFilters
		}
		if f.Text != nil {
			cleanTextFilters := validation.SanitizeString(*f.Text)
			f.Text = &cleanTextFilters
		}
	}
	return nil
}

func newSubscriptionSettings() *SubscriptionSettings {
	return &SubscriptionSettings{
		ShowFullArticleContent: false,
	}
}

func (f *AddFeedSubscriptionRequest) Valid() error {
	return validation.Validate.Struct(f)
}

func (f *AddFeedSubscriptionRequest) Sanitise() error {
	f.URL = validation.SanitizeString(f.URL)
	f.FeedID = validation.SanitizeString(f.FeedID)
	return nil
}
