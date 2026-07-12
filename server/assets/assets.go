// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package assets

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	slogctx "github.com/veqryn/slog-context"
	"github.com/zeebo/xxh3"
)

var manifest *Manifest

// Manifest maps a logical asset path ("scripts.js") to its hashed,
// servable path ("scripts.a1b2c3d4.js"). Build it once at startup with New.
type Manifest struct {
	// logical -> hashed, e.g. "scripts.js" -> "scripts.a1b2c3d4.js"
	pathFor map[string]string
	// hashed -> logical, used by the Handler to resolve incoming requests
	fileFor map[string]string
	fsys    fs.FS
	modTime time.Time // single build-time stamp for Last-Modified
}

// New walks fsys, computes a short content hash for every regular file,
// and returns a Manifest you can use to both build asset URLs and serve them.
//
// root, if non-empty, is stripped as a prefix when building logical paths
// (useful if your embed.FS embeds "web/content" but you want asset paths
// like "scripts.js" rather than "web/content/scripts.js").
func New(fsys fs.FS, root string) error {
	err := sync.OnceValue(func() error {
		manifest = &Manifest{
			pathFor: make(map[string]string),
			fileFor: make(map[string]string),
			fsys:    fsys,
			modTime: time.Now(),
		}

		root = strings.Trim(root, "/")

		err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}

			data, err := fs.ReadFile(fsys, p)
			if err != nil {
				return fmt.Errorf("reading %s: %w", p, err)
			}

			hash := strconv.FormatUint(xxh3.Hash(data), 36)

			logical := p
			if root != "" {
				logical = strings.TrimPrefix(logical, root+"/")
			}

			hashed := hashFilename(logical, hash)

			manifest.pathFor[logical] = hashed
			manifest.fileFor[hashed] = p // keep the real fs path for Open

			return nil
		})
		if err != nil {
			return fmt.Errorf("build: %w", err)
		}

		slog.Debug("Assets loaded and hashed.")

		return nil
	})()
	if err != nil {
		return fmt.Errorf("new: %w", err)
	}
	return nil
}

// hashFilename turns "scripts.js" + "a1b2c3d4" into "scripts.a1b2c3d4.js".
// Files without an extension get the hash appended after a dot.
func hashFilename(logical, hash string) string {
	ext := path.Ext(logical)
	base := strings.TrimSuffix(logical, ext)
	if ext == "" {
		return fmt.Sprintf("%s.%s", base, hash)
	}
	return fmt.Sprintf("%s.%s%s", base, hash, ext)
}

// GetPath returns the hashed, servable path for a logical asset name.
// Use this from templ templates: assets.GetPath(m, "scripts.js") -> "/assets/scripts.a1b2c3d4.js"
//
// If the asset isn't found, it logs and returns the logical name unchanged
// so a missed rename fails loudly (404) rather than silently breaking the page.
func GetPath(ctx context.Context, logical string) string {
	logical = strings.TrimPrefix(logical, "/")
	hashed, ok := manifest.pathFor[logical]
	if !ok {
		slogctx.Warn(ctx, "No manifest entry for file (check the filename / build output).",
			slog.String("file", logical))
		return logical
	}
	return hashed
}

// HandleAssets returns an http.Handler that serves assets at urlPrefix.
// Hashed paths (the normal case) get long-lived immutable caching.
// Unhashed/unknown paths fall through with no special caching, which lets
// you 404 cleanly or serve other static files from the same prefix if needed.
func HandleAssets(urlPrefix string, unHashed bool) http.Handler {
	if urlPrefix != "" {
		urlPrefix = "/" + strings.Trim(urlPrefix, "/") + "/"
	}
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet && req.Method != http.MethodHead {
			http.Error(res, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		reqPath := strings.TrimPrefix(req.URL.Path, urlPrefix)
		reqPath = strings.TrimPrefix(reqPath, "/")

		var realPath string
		if !unHashed {
			var found bool
			realPath, found = manifest.fileFor[reqPath]
			if !found {
				http.NotFound(res, req)
				return
			}
		} else {
			realPath = filepath.Join("content", reqPath)
		}

		f, err := manifest.fsys.Open(realPath)
		if err != nil {
			http.NotFound(res, req)
			return
		}
		defer f.Close()

		if !unHashed {
			// Hashed filenames are content-addressed: the same name will always
			// mean the same bytes, so it's safe to cache for a very long time.
			res.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			// Default is to cache for 1 week.
			res.Header().Set("Cache-Control", "public, max-age=604800, s-maxage=43200")
		}
		res.Header().Set("Content-Type", contentTypeFor(realPath))

		rs, ok := f.(interface {
			fs.File
			Seek(offset int64, whence int) (int64, error)
		})
		if ok {
			http.ServeContent(res, req, realPath, manifest.modTime, rs)
			return
		}

		// Fallback for files that don't support seeking (rare with embed.FS,
		// but kept for safety).
		data, err := fs.ReadFile(manifest.fsys, realPath)
		if err != nil {
			http.Error(res, "internal error", http.StatusInternalServerError)
			return
		}
		res.Write(data)
	})
}

func contentTypeFor(p string) string {
	switch path.Ext(p) {
	case ".js", ".mjs":
		return "text/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".woff2":
		return "font/woff2"
	case ".json":
		return "application/json; charset=utf-8"
	case ".txt":
		return "text/plain; charset=utf-8"
	case ".ico":
		return "image/vnd.microsoft.icon"
	default:
		return "application/octet-stream"
	}
}
