// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

const (
	ParamObjectType = "object"
	ParamObjectID   = "id"
	// ParamCategories is a list of category names, of any single object type (subscription or article).
	ParamCategories = "categories"
	// ParamCount is the number of results to show, where multiple results can be displayed.
	ParamCount = "count"
	// ParamPagination is the encoded pagination value, used to fetch the next set of results when paginating through a
	// list of objects.
	ParamPagination = "pagination"
	// ParamSort is the sort option to apply to a set of results.
	ParamSort = "sort"
	// ParamView is the view filter to apply to a set of results.
	ParamView = "view"
	// ParamMark is a mark to apply to either an object or set of objects in the request.
	ParamMark = "mark"
	// ParamFeedID identifies a single feed by its id.
	ParamFeedID = "feed_id"
	// ParamItemID identifies a single item by its id.
	ParamItemID = "item_id"
	// ParamSubscriptionID identifies a single subscription by its id.
	ParamSubscriptionID   = "subscription_id"
	ParamSubscriptionName = "subscription_name"
	// ParamSubscriptions is a list of subscription ids.
	ParamSubscriptions      = "subscriptions"
	ParamFullArticleContent = "show_full_content"
	ParamOnlyFavorites      = "only_favorites"
	// ParamPlanID is the ID of the subscription plan the user has chosen.
	ParamPlanID = "plan_id"
	// ParamThumbnail is a thumbnail image uploaded by the user.
	ParamThumbnail = "thumbnail"
)
