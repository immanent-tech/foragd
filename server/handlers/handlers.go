/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

// Package handlers contains chainable handlers/middleware for routing.
package handlers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-resty/resty/v2"
	"github.com/immanent-tech/go-base/config"
	"github.com/immanent-tech/go-base/server/forms"
	"github.com/indaco/teseo/opengraph"
	"github.com/indaco/teseo/schemaorg"
	slogctx "github.com/veqryn/slog-context"
	"go.opentelemetry.io/otel"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/auth0"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/service"
	"github.com/immanent-tech/foragd/web/templates"
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
	SuggestGoogleNewsFeeds(
		ctx context.Context,
		httpClient *resty.Client,
		text string,
	) (*models.SuggestFeedsResults, error)
	SuggestFeeds(
		ctx context.Context,
		httpClient *resty.Client,
		request *models.SuggestFeedsRequest,
	) (*models.SuggestFeedsResults, error)
	FindOrCreateFeed(ctx context.Context, httpClient *resty.Client, feedURL string) (*models.Feed, bool, error)
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
}

type ArticleService interface {
	FilterArticles(ctx context.Context, request *models.ListRequest) (models.Articles, models.Pagination, error)
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
	CountItems(ctx context.Context, query query.Option) (int64, error)
	SuggestItems(ctx context.Context, request *models.SearchRequest) (models.Items, error)
	GetTopCategoriesForItems(ctx context.Context, itemsQuery query.Option) (models.CategoryCounts, error)
}

type UserService interface {
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	GetUserByExternalID(ctx context.Context, externalID string) (*models.User, error)
	GetUserBySubscriptionEmail(ctx context.Context, emails ...string) (*models.User, error)
	GetUserByPurchaseToken(ctx context.Context, token string) (*models.User, error)
	GetUserByCustomerID(ctx context.Context, id string) (*models.User, error)
	UpdateUser(ctx context.Context, user *models.User, updates map[string]any) error
	SyncUser(res http.ResponseWriter, req *http.Request, user *models.User)
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
	IsAuthenticated(ctx context.Context, session SessionManager) bool
	GenerateAuthURL(req *http.Request) (AuthURLResult, error)
	PutState(ctx context.Context, session SessionManager, state string)
	GetState(ctx context.Context, session SessionManager) (string, error)
	PutCodeVerifier(ctx context.Context, session SessionManager, verifier string)
	GetCodeVerifier(ctx context.Context, session SessionManager) (string, error)
	PerformExchange(ctx context.Context, code, verifier string) (*auth0.TokenResponse, *auth0.UserProfile, error)
	SaveTokens(ctx context.Context, session SessionManager, token *auth0.TokenResponse)
	ClearState(ctx context.Context, session SessionManager)
	GetReturnTo(ctx context.Context, session SessionManager) (string, error)
}

type AuthURLResult interface {
	GetURL() string
	GetState() string
	GetCodeVerifier() string
}

type Breadcrumbs interface {
	Previous(ctx context.Context) (*url.URL, bool)
}

// Manager contains the common interfaces that nearly all handlers require access to.
type Manager struct {
	AppConfig   AppConfig
	SessionMgr  SessionManager
	Breadcrumbs Breadcrumbs
}

func (m *Manager) NewPageServices() pageServices {
	return pageServices{
		appCfg:      m.AppConfig,
		sessionMgr:  m.SessionMgr,
		breadcrumbs: m.Breadcrumbs,
	}
}

var (
	// ErrInvalidContent indicates that the content for rendering is invalid.
	ErrInvalidContent = errors.New("invalid content")
	// ErrInvalidRequestParams indicates that the request parameters received were invalid.
	ErrInvalidRequestParams = errors.New("invalid request parameters")
)

var tracer = otel.Tracer("github.com/immanent-tech/foragd/server/handlers")

// RedirectTo performs route redirection for routes that have moved.
func RedirectTo(target string, code int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dest := target
		if r.URL.RawQuery != "" {
			dest += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, dest, code)
	}
}

// RedirectParam performs route redirection for routes with parameters that have moved.
func RedirectParam(paramName, targetTmpl string, code int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		val := chi.URLParam(r, paramName)
		dest := fmt.Sprintf(targetTmpl, val)
		if r.URL.RawQuery != "" {
			dest += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, dest, code)
	}
}

func parseForm[T forms.FormInput](req *http.Request) (T, error) {
	request, err := forms.DecodeForm[T](req)
	if err != nil {
		return request, &models.APIError{
			InternalError: fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
			StatusCode:    http.StatusInternalServerError,
			UserMessage: models.NewErrorMessage(
				"Unable to parse input",
				"This might be a temporary issue, please try again.",
			),
		}
	}
	return request, nil
}

func parseMultipartForm[T forms.FormInput](req *http.Request) (T, error) {
	request, err := forms.DecodeMultiPartForm[T](req)
	if err != nil {
		return request, &models.APIError{
			InternalError: fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
			StatusCode:    http.StatusInternalServerError,
			UserMessage: models.NewErrorMessage(
				"Unable to parse input",
				"This might be a temporary issue, please try again.",
			),
		}
	}
	return request, nil
}

// FileUpload represents file data uploaded through a mutlipart form.
type FileUpload interface {
	Set(hdr *multipart.FileHeader, data multipart.File)
}

// DecodeMultipartFile will the file represented by the given field in a multipart form
// submission. It will perform validation of the file and will return the file
// object and a boolean true if it is valid. If decoding fails, a non-nill error
// is returned.
func decodeMultipartFile(req *http.Request, field string) (*models.FileUpload, error) {
	// defaultMaxSize for a multipart for submission is 32 MB.
	const defaultMaxSize = 32 << 20

	// Parse form values in request.
	if err := req.ParseMultipartForm(defaultMaxSize); err != nil {
		return nil, fmt.Errorf("decode multipart form: %w", err)
	}
	// Decode the form values.
	data, hdr, err := req.FormFile(field)
	if err != nil {
		return nil, fmt.Errorf("decode form file: %w", err)
	}
	// Create a models.FileUpload object.
	upload := &models.FileUpload{
		Data:   data,
		Header: hdr,
	}
	// Validate file upload.
	if err := upload.Validate(); err != nil {
		return nil, fmt.Errorf("validate file upload: %w", err)
	}
	return upload, nil
}

// PostHandlerHook is a function that can be run after a handler has done its main processing. Used mainly to perform
// route-specific or other conditional logic without complicating the handler code.
type PostHandlerHook func(res http.ResponseWriter, req *http.Request) error

type pageMetadata struct {
	Title       templates.PageTitle
	Description string
	Path        string
	ImagePath   string
	baseURL     *url.URL
}

func newPageMetadata(title templates.PageTitle, description, path, imagePath string, baseURL *url.URL) pageMetadata {
	return pageMetadata{
		Title:       title,
		Description: description,
		Path:        path,
		ImagePath:   imagePath,
		baseURL:     baseURL.Clone(),
	}
}

func (m pageMetadata) CanonicalLink() string {
	return m.baseURL.JoinPath(m.Path).String()
}

func (m pageMetadata) OpengraphData() *opengraph.WebSite {
	return opengraph.NewWebSite(
		m.Title.String(),
		m.baseURL.Clone().JoinPath(m.Path).String(),
		m.Description,
		m.baseURL.Clone().JoinPath(m.ImagePath).String(),
	)
}

func (m pageMetadata) JSONLD() *schemaorg.WebPage {
	return schemaorg.NewWebPage(
		m.baseURL.Clone().JoinPath(m.Path).String(),
		m.Title.Summary,
		m.Title.Description,
		m.Description,
		"",
		"",
		"en",
		m.baseURL.String(),
		"",
		m.baseURL.Clone().JoinPath(m.ImagePath).String(),
		"",
		"",
	)
}

func generateSiteJSONLD(baseURL *url.URL) *schemaorg.WebSite {
	return schemaorg.NewWebSite(
		baseURL.String(),
		"Foragd",
		"Foragd RSS and Atom Feed Reader",
		"Foragd is a web-based RSS and Atom Feed Reader with a responsive design, no ads and no algorithm directing you.",
		nil,
	)
}

func generateSiteOG(baseURL *url.URL) *opengraph.WebSite {
	return opengraph.NewWebSite(
		"Foragd",
		baseURL.String(),
		"Foragd is a web-based RSS and Atom Feed Reader with a responsive design, no ads and no algorithm directing you.",
		baseURL.Clone().JoinPath("/content/logo-vertical-light.webp").String(),
	)
}

type pageServices struct {
	appCfg      AppConfig
	sessionMgr  SessionManager
	breadcrumbs Breadcrumbs
}

const listFiltersCtxKey contextKey = "listFilters"
const listCountCtxKey contextKey = "listCount"

type contextKey string

// ListFiltersToSession stores the given list filters in the session. The path is used as a suffix so that filters are
// stored per-route.
func ListFiltersToSession(ctx context.Context, session SessionManager, path string, filters *models.ListFilters) {
	session.Put(ctx, string(listFiltersCtxKey)+path, *filters)
}

// ListFiltersFromSession retrieves the given list filters in the session. The path is used as a suffix so that filters
// are retrieved per-route.
func ListFiltersFromSession(ctx context.Context, session SessionManager, path string) *models.ListFilters {
	filters, ok := session.Get(ctx, string(listFiltersCtxKey)+path).(models.ListFilters)
	if !ok {
		slogctx.Warn(ctx, "Unable to restore filters from session. Using defaults.")
		return models.NewListFilters()
	}
	return &filters
}

// ListFiltersToCtx stores the given list filters in the context.
func ListFiltersToCtx(ctx context.Context, filters *models.ListFilters) context.Context {
	return context.WithValue(ctx, listFiltersCtxKey, *filters)
}

// ListFiltersFromCtx retrieves the given list filters in the context.
func ListFiltersFromCtx(ctx context.Context) *models.ListFilters {
	if filters, ok := ctx.Value(listFiltersCtxKey).(models.ListFilters); ok {
		return &filters
	}
	slogctx.Warn(ctx, "No filters in context. Returning new filters.")
	return models.NewListFilters()
}

// ListCountToSession stores the current count of objects displayed in the list in the session. The path is used as a
// suffix so that the count is stored per-route.
func ListCountToSession(ctx context.Context, session SessionManager, path string, count int) {
	session.Put(ctx, string(listCountCtxKey)+path, count)
}

// ListCountFromSession stores the current count of objects displayed in the list in the session. The path is used as a
// suffix so that the count is retrieved per-route.
func ListCountFromSession(ctx context.Context, session SessionManager, path string) int {
	count, ok := session.Get(ctx, string(listCountCtxKey)+path).(int)
	if !ok {
		slogctx.Warn(ctx, "Unable to restore list count from session. Using default.")
		return 9
	}
	return count
}

const searchParamsCtxKey contextKey = "searchParams"
const searchCountCtxKey contextKey = "searchCount"

// SearchParamsToSession stores the given search params in the session.
func SearchParamsToSession(ctx context.Context, session SessionManager, search *models.SearchRequest) {
	session.Put(ctx, string(searchParamsCtxKey), *search)
}

// SearchParamsFromSession retrieves the given search params in the session.
func SearchParamsFromSession(ctx context.Context, session SessionManager) *models.SearchRequest {
	search, ok := session.Get(ctx, string(searchParamsCtxKey)).(models.SearchRequest)
	if !ok {
		slogctx.Warn(ctx, "Unable to restore search params from session. Using defaults.")
		return models.NewSearchRequest()
	}
	return &search
}

// SearchCountToSession stores the current count of objects displayed in the search in the session.
func SearchCountToSession(ctx context.Context, session SessionManager, count int) {
	session.Put(ctx, string(searchCountCtxKey), count)
}

// SearchCountFromSession stores the current count of objects displayed in the search in the session.
func SearchCountFromSession(ctx context.Context, session SessionManager) int {
	count, ok := session.Get(ctx, string(searchCountCtxKey)).(int)
	if !ok {
		slogctx.Warn(ctx, "Unable to restore list search count from session. Using default.")
		return 9
	}
	return count
}

// SearchParamsToCtx stores the given search params in the context.
func SearchParamsToCtx(ctx context.Context, search *models.SearchRequest) context.Context {
	return context.WithValue(ctx, searchParamsCtxKey, *search)
}

// SearchParamsFromCtx retrieves the given search params in the context.
func SearchParamsFromCtx(ctx context.Context) *models.SearchRequest {
	if search, ok := ctx.Value(searchParamsCtxKey).(models.SearchRequest); ok {
		return &search
	}
	slogctx.Warn(ctx, "No search params in context. Returning new search params.")
	return models.NewSearchRequest()
}
