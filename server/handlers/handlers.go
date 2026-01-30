// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package handlers contains chainable handlers/middleware for routing.
package handlers

import (
	"bufio"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"
	"github.com/go-resty/resty/v2"
	"github.com/justinas/alice"
	"github.com/russross/blackfriday/v2"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-syndication/sitemap"

	htmxext "github.com/immanent-tech/foragd/web/htmx"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/server/forms"
	"github.com/immanent-tech/foragd/server/session"
	"github.com/immanent-tech/foragd/web"
	"github.com/immanent-tech/foragd/web/templates"
)

var (
	// ErrInvalidContent indicates that the content for rendering is invalid.
	ErrInvalidContent = errors.New("invalid content")
	// ErrInvalidRequestParams indicates that the request parameters received were invalid.
	ErrInvalidRequestParams = errors.New("invalid request parameters")
)

var defaultHandlerChain = alice.New(storePath, pushCriticalAssets)
var listHandlerChain = defaultHandlerChain.Append(parseFilters)

var loadHTTPClient = sync.OnceValue(func() *resty.Client {
	return resty.New().SetHeader("User-Agent", config.AppName+"/"+config.Version)
})

var bufPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// StaticFileHandler handles serving content from the embedded filesystem containing static assets (i.e., images,
// etc.).
func StaticFileHandler(fs http.FileSystem) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// Check, if the requested file is existing.
		if _, err := fs.Open(req.URL.Path); err != nil {
			// If file is not found, return HTTP 404 error.
			http.NotFound(res, req)
			return
		}
		switch {
		case strings.HasSuffix(req.URL.Path, "js"):
			// JS files are cached for 1 week.
			res.Header().Set("Cache-Control", "public, max-age=604800")
		case strings.HasSuffix(req.URL.Path, "css"):
			// CSS files are cached for 1 week.
			res.Header().Set("Cache-Control", "public, max-age=604800")
		case strings.HasSuffix(req.URL.Path, "woff2"):
			// Fonts are cached for 1 year.
			res.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		case strings.HasSuffix(req.URL.Path, "png"):
			fallthrough
		case strings.HasSuffix(req.URL.Path, "jpg"):
			fallthrough
		case strings.HasSuffix(req.URL.Path, "webp"):
			fallthrough
		case strings.HasSuffix(req.URL.Path, "svg"):
			res.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		default:
			// Default is to cache for 1 week.
			res.Header().Set("Cache-Control", "public, max-age=604800, s-maxage=43200")
		}
		// File is found, return to standard http.FileServer.
		http.FileServer(fs).ServeHTTP(res, req)
	})
}

var loadRobotsTxt = sync.OnceValues(func() ([]byte, error) {
	robotsTxt, err := web.StaticContentFS.ReadFile("content/robots.txt")
	if err != nil {
		return nil, fmt.Errorf("read robots.txt: %w", err)
	}
	return robotsTxt, nil
})

// HandleRobots handles requests for robots.txt. In the future, it may handle more requests from non natural human
// clients...
func HandleRobots() http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		robotsTxt, err := loadRobotsTxt()
		if err != nil {
			http.NotFound(res, req)
			return
		}
		res.Header().Set("Cache-Control", "public, max-age=604800, s-maxage=43200")
		res.WriteHeader(http.StatusOK)
		if _, err := res.Write(robotsTxt); err != nil {
			slogctx.FromCtx(req.Context()).Error("Unable to send robots.txt response.",
				slog.Any("error", err),
			)
		}
	})
}

var loadSitemapXML = sync.OnceValues(func() ([]byte, error) {
	// Set up default URLs.
	site := sitemap.NewURLSet(
		sitemap.URL{
			Loc:      "https://foragd.app",
			Priority: 1.0,
		},
		sitemap.URL{
			Loc:      "https://foragd.app/viewer",
			Priority: 0.9,
		},
		sitemap.URL{
			Loc:      "https://foragd.app/about",
			Priority: 0.75,
		},
	)
	// Add all posts.
	posts, err := getPosts()
	if err != nil {
		return nil, fmt.Errorf("generate sitemap.xml: %w", err)
	}
	for post := range slices.Values(posts.Docs) {
		site.URLs = append(site.URLs,
			sitemap.URL{
				Loc:      sitemap.LOC("https://foragd.app/posts/" + post.Path),
				Priority: 0.5,
			})
	}

	data, err := xml.Marshal(site)
	if err != nil {
		return nil, fmt.Errorf("generate sitemap.xml: %w", err)
	}
	return []byte(xml.Header + string(data)), nil
})

// HandleSitemap handles requests for sitemap.xml. In the future, it may handle more requests from non natural human
// clients...
func HandleSitemap() http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {

		sitemap, err := loadSitemapXML()
		if err != nil {
			http.NotFound(res, req)
			return
		}
		res.Header().Set("Cache-Control", "public, max-age=604800, s-maxage=43200, must-revalidate")
		res.WriteHeader(http.StatusOK)
		if _, err := res.Write(sitemap); err != nil {
			slogctx.FromCtx(req.Context()).Error("Unable to send sitemap.xml response.",
				slog.Any("error", err),
			)
		}
	})
}

var getPolicyDocs = sync.OnceValues(func() (*models.DocsDirectory, error) {
	var policies models.DocsDirectory
	if _, err := toml.DecodeFS(web.DocsFS, "assets/docs/policies/directory.toml", &policies); err != nil {
		return nil, fmt.Errorf("read policy docs directory.toml: %w", err)
	}
	return &policies, nil
})

// PolicyDocsHandler handles serving policy Markdown documents from directory in the embedded fs.
func PolicyDocsHandler() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		doc := chi.URLParam(req, "*")
		// Check, if the requested file is existing.
		polices, err := getPolicyDocs()
		if err != nil {
			// If file is not found, return HTTP 404 error.
			slogctx.FromCtx(req.Context()).Error("Could not read policy document.",
				slog.String("doc", doc),
				slog.Any("error", err),
			)
			http.NotFound(res, req)
		}

		idx := slices.IndexFunc(polices.Docs, func(e models.DocMetadata) bool {
			return strings.HasPrefix(e.File, doc)
		})

		if idx == -1 {
			res.WriteHeader(http.StatusNotFound)
			return
		}

		metadata := polices.Docs[idx]
		contents, err := web.DocsFS.ReadFile(filepath.Join("assets/docs/policies", metadata.File))
		if err != nil {
			// If file is not found, return HTTP 404 error.
			slogctx.FromCtx(req.Context()).Error("Could not read policy document.",
				slog.String("doc", doc),
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusInternalServerError)
			return
		}

		res.Header().Set("Cache-Control", "public, max-age=604800, s-maxage=43200")
		output := blackfriday.Run(contents, blackfriday.WithExtensions(blackfriday.AutoHeadingIDs))
		template := templates.CreatePage(
			templates.Document(output),
			templates.WithPageTitle(metadata.Title),
			templates.WithPageDescription(metadata.Description),
		)
		templ.Handler(template).ServeHTTP(res, req)
	}
}

// DocumentationHandler handles serving Markdown documents for help/documentation from directory in the embedded fs.
func DocumentationHandler() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// doc := chi.URLParam(req, "*")
		// Check, if the requested file is existing.
		contents, err := web.DocsFS.ReadFile(filepath.Join("assets", "docs", "help", "index.md"))
		if err != nil {
			// If file is not found, return HTTP 404 error.
			slogctx.FromCtx(req.Context()).Error("Could not read document.",
				slog.Any("error", err),
			)
			http.NotFound(res, req)
		}
		res.Header().Set("Cache-Control", "public, max-age=604800, s-maxage=43200")
		output := blackfriday.Run(contents, blackfriday.WithExtensions(blackfriday.AutoHeadingIDs))
		template := templates.CreatePage(
			templates.Document(output),
			templates.WithPageTitle("Documentation"),
		)
		templ.Handler(template).ServeHTTP(res, req)
	}
}

// notifyOnError handles showing a notification to the user when an error occurs while handling the request. It should
// only be used for HTMX requests.
func notifyOnError(f func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		if err := f(res, req); err != nil {
			var apiErr *models.APIError
			if errors.As(err, &apiErr) {
				apiErr.WriteLog(req.Context())
				switch {
				case htmx.IsHTMX(req): // show notification.
					res.WriteHeader(apiErr.HTTPStatus())
					res.Header().Add(htmx.HeaderReswap, "none")
					renderPartial(
						templates.NewPartial(
							templates.Notification(apiErr.GetUserMessage(), templates.DefaultNotificationTimeout),
						),
					).ServeHTTP(res, req)
				default: // called with non-HTMX request. Show plain error and log problem.
					slogctx.FromCtx(req.Context()).Debug("notifyOnError called in non-HTMX request.")
					http.Error(res, apiErr.GetUserMessage().String(), apiErr.HTTPStatus())
				}
			} else {
				slogctx.FromCtx(req.Context()).Error("Unknown error occurred.",
					slog.Any("error", err),
				)
				res.WriteHeader(http.StatusInternalServerError)
			}
		}
	}
}

// showOnError handles either rendering an error page or partial to the user when an error occurs while handling the
// request. It can be used to handle both HTMX and non-HTMX requests. For non-HTMX requests, a full page with the error
// message will be shown. For HTMX requests, the error message will be rendered in place of the main content.
func showOnError(f func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		if err := f(res, req); err != nil {
			var apiErr *models.APIError
			if errors.As(err, &apiErr) {
				user := models.UserFromCtx(req.Context())
				apiErr.WriteLog(req.Context())
				res.WriteHeader(apiErr.HTTPStatus())
				renderPage(
					templates.NewPage(
						wrapContent(req, templates.InternalError(user, apiErr.GetUserMessage())),
					),
				).ServeHTTP(res, req)
			} else {
				slogctx.FromCtx(req.Context()).Error("Unknown error occurred.",
					slog.Any("error", err),
				)
				res.WriteHeader(http.StatusInternalServerError)
			}
		}
	}
}

// setRedirect adds the HX-Location header with the given values to the response, which triggers a client side
// redirection without reloading the whole page.
//
// https://htmx.org/headers/hx-location/
func setRedirect(res http.ResponseWriter, request htmxext.HXLocationRequest) error {
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("set redirect: marshal request: %w", err)
	}
	res.Header().Set(htmx.HeaderLocation, string(requestJSON))
	return nil
}

// renderPage will render the given template either as a full page or as partial content. For partial content, it will
// also update the page title (if one is given).
func renderPage(page *templates.Page) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if page.PartialTemplate() == nil {
			// If there is no response, return 204: No Content.
			res.WriteHeader(http.StatusNoContent)
			return
		}
		switch {
		case !htmx.IsHTMX(req) || htmx.IsHistoryRestoreRequest(req): // Non-HTMX or HistoryRestoreRequests render a full-page.
			if htmx.IsHistoryRestoreRequest(req) {
				res.Header().Set("Cache-Control", "private, max-age=0, must-revalidate")
			}
			templ.Handler(page.FullTemplate()).ServeHTTP(res, req)
			return
		default: // HTMX request renders partial content.
			template := page.PartialTemplate()
			// Render template (or template fragment).
			if target := templates.FragmentKey(req.Header.Get(htmx.HeaderTarget)); target != "" &&
				target != templates.FragmentContent {
				templ.Handler(template, templ.WithFragments(target)).ServeHTTP(res, req)
			} else {
				templ.Handler(template).ServeHTTP(res, req)
			}
		}
	})
}

// renderPartial will render the given template only as a partial update.
func renderPartial(partial *templates.Partial) http.Handler {
	return templ.Handler(partial.PartialTemplate())
}

// wrapContent will wrap the given template with additional structure suitable for replacing the content target on a
// page. The additional structure will place a header, footer/sidebar around the content.
func wrapContent(req *http.Request, template templ.Component) templ.Component {
	switch {
	case !htmx.IsHTMX(req) || htmx.IsHistoryRestoreRequest(req): // Non-HTMX or HistoryRestoreRequests render a full-page.
		user := models.UserFromCtx(req.Context())
		if user == nil {
			return templates.InternalError(
				user,
				models.NewErrorMessage("Invalid request", "This might be a temporary error, please try again."),
			)
		}
		return templates.Content(&models.ViewComponent{
			URL:       req.URL,
			User:      *user,
			Component: template,
		})
	default: // HTMX request renders partial content.
		return templ.Join(template,
			templates.SideBar(templ.Attributes{"hx-swap-oob": "true"}),
			templates.Dock(templ.Attributes{"hx-swap-oob": "true"}),
		)
	}
}

func parseFilters(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		filters, valid, err := forms.DecodeForm[*models.ListFilters](req)
		ctx := req.Context()
		switch {
		case err != nil:
			slogctx.FromCtx(ctx).Warn("Unable to parse new filters. Using filters from session.",
				slog.Any("error", err),
				slog.Any("filters", filters),
			)
			// Try to restore filters from session.
			restored, err := session.Restore[models.ListFilters](ctx, "filters_"+req.URL.Path)
			if err != nil {
				// Use new filters if unable to restore from session or form data.
				restored = models.NewListDisplayFilters()
			}
			filters = &restored
			ctx = models.PageFiltersToCtx(ctx, req.URL.Path, filters)
		case !valid:
			newFilters := models.NewListDisplayFilters()
			session.Save(ctx, "filters_"+req.URL.Path, newFilters)
			ctx = models.PageFiltersToCtx(ctx, req.URL.Path, filters)
			slogctx.FromCtx(ctx).Warn("Invalid filters. Creating new filters.",
				slog.Any("error", err),
			)
		default:
			slogctx.FromCtx(ctx).Debug("Saving filters",
				slog.Any("filters", filters),
			)
			ctx = models.PageFiltersToCtx(ctx, req.URL.Path, filters)
		}
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

func storePath(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		ctx := models.PathToCtx(req.Context(), req.URL.Path)
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

// pushCriticalAssets will optimistically send our custom script/css bundles to a client before it asks for them, which
// hopefully will speed up first page load.
func pushCriticalAssets(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if pusher, ok := res.(http.Pusher); ok {
			if err := pusher.Push("/content/scripts.js?v="+config.Version, nil); err != nil {
				slogctx.FromCtx(req.Context()).Error("Push scripts failed.",
					slog.Any("error", err),
				)
			}
			if err := pusher.Push("/content/styles.css?v="+config.Version, nil); err != nil {
				slogctx.FromCtx(req.Context()).Error("Push styles failed.",
					slog.Any("error", err),
				)
			}
			if err := pusher.Push("/content/inter.css?v="+config.Version, nil); err != nil {
				slogctx.FromCtx(req.Context()).Error("Push styles failed.",
					slog.Any("error", err),
				)
			}
		}
		next.ServeHTTP(res, req)
	})
}

// WatchList handles watching a list of object for any updates and rendering a notification to the user to refresh the page.
func WatchList() http.HandlerFunc {
	return defaultHandlerChain.Append(parseFilters).ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		filters := models.PageFiltersFromCtx(req.Context(), req.URL.Path)
		query, err := models.BuildItemsQuery(req.Context(), filters)
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Cannot generate query for updates.",
				slog.Any("error", err))
			res.WriteHeader(http.StatusInternalServerError)
			return
		}
		// Watch list for updates.
		watchForUpdates(query).ServeHTTP(res, req)
	}).ServeHTTP
}

//nolint:gocognit
func watchForUpdates(watch query.Option) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		user := models.UserFromCtx(req.Context())
		if user == nil {
			res.WriteHeader(http.StatusNoContent)
			slogctx.FromCtx(req.Context()).Error("Unable to watch for updates.",
				slog.Any("error", models.ErrCtxValueNotFound),
			)
			return
		}

		// Set headers for SSE.
		res.Header().Set("Content-Type", "text/event-stream")
		res.Header().Set("Cache-Control", "no-cache")
		res.Header().Set("Connection", "keep-alive")
		res.Header().Set("X-Accel-Buffering", "no")
		if f, ok := res.(http.Flusher); ok {
			f.Flush()
		} else {
			slogctx.FromCtx(req.Context()).Error("Cannot write SSE headers.")
			res.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Set up counters.
		var (
			currentCount int64
			prevCount    int64
			err          error
		)

		// Get an initial count.
		prevCount, err = models.CountItems(req.Context(), watch)
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Cannot get updates count.",
				slog.Any("error", err))
			res.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Set up updatesTicker.
		updatesTicker := time.NewTicker(user.GetUpdatesFrequency())
		defer updatesTicker.Stop()
		keepAliveTicker := time.NewTicker(20 * time.Second)
		defer keepAliveTicker.Stop()
		slogctx.FromCtx(req.Context()).Debug("Checking for updates...",
			slog.Duration("interval", user.GetUpdatesFrequency()),
			slog.Group("request",
				slog.String("path", req.URL.Path),
			),
		)

		// Watch for updates.
		for {
			select {
			case <-req.Context().Done():
				slogctx.FromCtx(req.Context()).Debug("Closing SSE connection.",
					slog.Group("request",
						slog.String("path", req.URL.Path),
					),
				)
				res.WriteHeader(http.StatusOK)
				res.Header().Set("Connection", "close")
				if f, ok := res.(http.Flusher); ok {
					f.Flush()
				}
				keepAliveTicker.Stop()
				updatesTicker.Stop()
				return
			case <-updatesTicker.C:
				currentCount, err = models.CountItems(req.Context(), watch)
				if err != nil {
					slogctx.FromCtx(req.Context()).Warn("Cannot get updates count.",
						slog.Any("error", err))
					continue
				}
				// Show updates toast if new items found.
				if currentCount > prevCount {
					slogctx.FromCtx(req.Context()).Debug("Subscription updates found.")
					respBuf, ok := bufPool.Get().(*bytes.Buffer)
					if !ok {
						res.WriteHeader(http.StatusNoContent)
						slogctx.FromCtx(req.Context()).Error("Get response buffer failed.")
						continue
					}
					respBuf.Reset()
					defer bufPool.Put(respBuf)

					template := bufio.NewWriter(respBuf)
					if err := templates.UpdatesToast().Render(req.Context(), template); err != nil {
						slogctx.FromCtx(req.Context()).Warn("Unable to render template.",
							slog.Any("error", err))
						continue
					}
					if err = template.Flush(); err != nil {
						slogctx.FromCtx(req.Context()).Error("Failed to flush SSE message buffer.",
							slog.Any("error", err))
					}
					if _, err = fmt.Fprintf(res, "event: updates\ndata: %s\n\n", respBuf.String()); err != nil {
						slogctx.FromCtx(req.Context()).Error("Failed to send update SSE message.",
							slog.Any("error", err))
					}
					if f, ok := res.(http.Flusher); ok {
						f.Flush()
					}
				}

				slogctx.FromCtx(req.Context()).Debug("No updates")
				prevCount = currentCount
			case <-keepAliveTicker.C:
				slogctx.FromCtx(req.Context()).Debug("Sending keep-alive message on SSE stream.",
					slog.Group("request",
						slog.String("path", req.URL.Path),
					),
				)
				if _, err = fmt.Fprint(res, ": keep-alive\n\n"); err != nil {
					slogctx.FromCtx(req.Context()).Error("Failed to send keep-alive SSE message.",
						slog.Any("error", err),
						slog.Group("request",
							slog.String("path", req.URL.Path),
						),
					)
				}
				if f, ok := res.(http.Flusher); ok {
					f.Flush()
				}
			}
		}
	})
}
