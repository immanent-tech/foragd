/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package handlers

import (
	"bytes"
	"context"
	"net/url"
	"time"

	"github.com/immanent-tech/go-base/config"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/auth0"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/providers/resend"
	"github.com/immanent-tech/foragd/service"
)

type AppConfig interface {
	GetAppID() string
	GetAppVersion() string
	GetAppName() string
	GetAppEnvironment() config.Environment
	IsProduction() bool
	GetBaseURL() *url.URL
}

type ImageCache interface {
	GetImage(ctx context.Context, key string, buf *bytes.Buffer) error
	SaveImage(ctx context.Context, id string, data []byte) error
	GetAvatar(ctx context.Context, key string, buf *bytes.Buffer) error
	SaveAvatar(ctx context.Context, id string, data []byte) error
	GetThumbnail(ctx context.Context, key string, buf *bytes.Buffer) error
	SaveThumbnail(ctx context.Context, id string, data []byte) error
	GetScreenshot(ctx context.Context, key string, buf *bytes.Buffer) error
	SaveScreenshot(ctx context.Context, id string, data []byte) error
}

type ElasticService interface {
	GetIndexRO(name service.Index) string
	GetIndexRW(name service.Index) string
}

type FeedService interface {
	GetFeed(ctx context.Context, id models.FeedID) (*models.Feed, error)
	GetFeeds(ctx context.Context, ids ...models.FeedID) (models.Feeds, error)
	AddFeed(ctx context.Context, feed *models.Feed) error
	SuggestYoutubeFeeds(ctx context.Context, text string) (*models.SuggestFeedsResults, error)
	SuggestGoogleNewsFeeds(ctx context.Context, text string) (*models.SuggestFeedsResults, error)
	SuggestFeeds(ctx context.Context, request *models.SuggestFeedsRequest) (*models.SuggestFeedsResults, error)
	FindOrCreateFeed(ctx context.Context, feedURL string) (*models.Feed, bool, error)
	FetchFeed(ctx context.Context, feedURL string, options ...service.FetchOption) (*models.Feed, error)
	GenerateOPML(ctx context.Context, feedIDs ...models.FeedID) ([]byte, error)
}

type SubscriptionsService interface {
	GetAllSubscriptions(ctx context.Context) (models.Subscriptions, error)
	GetSubscription(ctx context.Context, id models.SubscriptionID) (*models.Subscription, error)
	BulkGetSubscriptions(ctx context.Context, ids ...models.SubscriptionID) (models.Subscriptions, error)
	UpdateSubscriptions(ctx context.Context, subscriptions ...*models.Subscription) error
	MarkSubscriptions(
		ctx context.Context,
		mark models.Mark,
		subscriptionIDs ...models.SubscriptionID,
	) error
	MarkArticles(
		ctx context.Context,
		mark models.Mark,
		subscriptionID models.SubscriptionID,
		itemIDs ...models.ItemID,
	) error
	AddSubscriptions(ctx context.Context, subscriptions ...*models.Subscription) error
	RemoveSubscriptions(ctx context.Context, ids ...models.SubscriptionID) error
	UpdateSubscriptionDynamicInfo(
		ctx context.Context,
		subscriptions models.Subscriptions,
	) error
	GetSubscriptionSuggestions(
		ctx context.Context,
		text string,
		count int,
		ignoredSubscriptions []models.SubscriptionID,
	) (models.Subscriptions, error)
	GetSubscriptionCategorySuggestions(
		ctx context.Context,
		feedIDs []models.FeedID,
		excludedCategories []models.Category,
	) []models.Category
	GetLatestArticles(ctx context.Context, view models.View, subscriptions models.Subscriptions)
	Invalidate(userID models.UserID)
}

type ArticleService interface {
	FilterArticles(
		ctx context.Context,
		filters *models.ListFilters,
		extraQueries ...query.Option,
	) (models.Articles, models.Subscriptions, models.Pagination, error)
	FilterGeoArticles(
		ctx context.Context,
		filters *models.ListFilters,
	) (models.Articles, models.Subscriptions, models.Pagination, error)
	CountArticles(
		ctx context.Context,
		filters *models.ListFilters,
		extraQueries ...query.Option,
	) (int64, error)
	CountGeoArticles(
		ctx context.Context,
		filters *models.ListFilters,
	) (int64, error)
	FindSimilarArticles(ctx context.Context, count int, itemIDs ...models.ItemID) (models.Articles, error)
	GetArticles(ctx context.Context, itemIDs ...models.ItemID) (models.Articles, error)
	GetNextArticle(
		ctx context.Context,
		currentID models.ItemID,
		subscriptionID models.SubscriptionID,
		view models.View,
		direction string,
		ts time.Time,
	) (*models.Article, error)
	ArchiveArticle(ctx context.Context, article *models.ArticleArchive) error
	UnarchiveArticle(ctx context.Context, userID models.UserID, itemID models.ItemID) error
}

type ItemService interface {
	ArticleService
	AddItems(ctx context.Context, items models.Items) (map[string]models.Items, error)
	RetrieveItems(ctx context.Context, request *models.SearchRequest) (models.Items, models.Pagination, error)
	GetTopItemCategoriesForSearchResults(ctx context.Context, request *models.SearchRequest) (models.Categories, error)
	CountSearchResults(ctx context.Context, request *models.SearchRequest) (int64, error)
	SuggestItems(ctx context.Context, request *models.SearchRequest) (models.Items, error)
	GetTopCategoriesForItems(ctx context.Context, itemsQuery query.Option) (models.CategoryCounts, error)
	GetTopLanguageCountsForItems(ctx context.Context, itemsQuery query.Option) (map[string]int64, error)
	GetArticleRemoteContent(ctx context.Context, article *models.Article) error
}

type UserService interface {
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	GetUserByExternalID(ctx context.Context, externalID string) (*models.User, error)
	GetUserByPurchaseToken(ctx context.Context, token string) (*models.User, error)
	GetUserByCustomerID(ctx context.Context, id string) (*models.User, error)
	UpdateUser(ctx context.Context, user *models.User, updates map[string]any) error
	AddUser(ctx context.Context, user *models.User) error
	DeleteUser(ctx context.Context, user *models.User) error
}

type SessionManager interface {
	Get(ctx context.Context, key string) any
	Put(ctx context.Context, key string, value any)
	Remove(ctx context.Context, key string)
	RenewToken(ctx context.Context) error
	Clear(ctx context.Context) error
}

type Authenticator interface {
	IsAuthenticated(ctx context.Context) bool
	GenerateAuthURL(signup bool) (*auth0.AuthURLResult, error)
	StoreAuthRequest(ctx context.Context, r *auth0.AuthURLResult)
	ValidateState(ctx context.Context, got string) error
	PerformExchange(ctx context.Context, code string) (*models.UserProfileResponse, error)
	EnsureFresh(ctx context.Context) error
	PutReturnTo(ctx context.Context, path string)
	ConsumeReturnTo(ctx context.Context) (string, error)
	GenerateLogoutURL() (*url.URL, error)
	ClearAuth(ctx context.Context)
}

type AuthManager interface {
	GetUser(ctx context.Context, userID string) (*auth0.UserData, error)
	DeleteUser(ctx context.Context, userID string) error
	UpdateUserCustomisation(ctx context.Context, request *models.EditUserRequest) error
	ChangeUserPassword(ctx context.Context, request *models.ChangePasswordRequest) error
}

type Breadcrumbs interface {
	Previous(ctx context.Context) (*url.URL, bool)
}

type Importer interface {
	StartImport(ctx context.Context, file *models.OPMLFile) (string, error)
	GetImportStatus(ctx context.Context, jobID string) (*models.ImportStatus, []*models.ImportResult, error)
	GetAllImports(ctx context.Context, userID models.UserID) ([]*models.ImportStatus, error)
}

type EmailSender interface {
	Send(ctx context.Context, options ...resend.EmailOption) error
}
