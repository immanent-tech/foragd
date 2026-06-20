// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package security

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/a-h/templ"

	"github.com/immanent-tech/foragd/config"
)

type CSP struct {
	// DefaultSrc defines the default policy for fetching resources such as JavaScript, Images, CSS, Fonts, AJAX
	// requests, Frames, HTML5 Media. Not all directives fallback to default-src.
	DefaultSrc []string `koanf:"defaultsrc"`
	// ScriptSrc defines valid sources of JavaScript.
	ScriptSrc []string `koanf:"scriptsrc"`
	// ScriptSrc defines valid sources of JavaScript.
	ScriptSrcAttr []string `koanf:"scriptsrcattr"`
	// StyleSrc defines valid sources of CSS.
	StyleSrc []string `koanf:"stylesrc"`
	// StyleSrc defines valid sources of CSS.
	StyleSrcAttr []string `koanf:"stylesrcattr"`
	// StyleSrc defines valid sources of images.
	ImgSrc []string `koanf:"imgsrc"`
	// ConnectSrc applies to XMLHttpRequest (AJAX), WebSocket, fetch(), <a ping> or EventSource. If not allowed the
	// browser emulates a 400 HTTP status code.
	ConnectSrc []string `koanf:"connectsrc"`
	// FontSrc defines valid sources of font resources (loaded via @font-face).
	FontSrc []string `koanf:"fontsrc"`
	// MediaSrc defines valid sources of audio and video, eg HTML5 <audio>, <video> elements.
	MediaSrc []string `koanf:"mediasrc"`
	// FrameSrc defines valid sources for loading frames. In CSP Level 2 frame-src was deprecated in favor of the
	// child-src directive. CSP Level 3, has undeprecated frame-src and it will continue to defer to child-src if not
	// present.
	FrameSrc []string `koanf:"framesrc"`
	// Sandbox enables a sandbox for the requested resource similar to the iframe sandbox attribute. The sandbox applies
	// a same origin policy, prevents popups, plugins and script execution is blocked. You can keep the sandbox value
	// empty to keep all restrictions in place, or add flags: allow-forms allow-same-origin allow-scripts allow-popups,
	// allow-modals, allow-orientation-lock, allow-pointer-lock, allow-presentation, allow-popups-to-escape-sandbox, and
	// allow-top-navigation
	Sandbox []string `koanf:"sandbox"`
	// ReportURI instructs the browser to POST a reports of policy failures to this URI. You can also use
	// Content-Security-Policy-Report-Only as the HTTP header name to instruct the browser to only send reports (does
	// not block anything). This directive is deprecated in CSP Level 3 in favor of the report-to directive.
	ReportURI string `koanf:"reporturi"`
	// ChildSrc defines valid sources for web workers and nested browsing contexts loaded using elements such as <frame>
	// and <iframe>.
	ChildSrc []string `koanf:"childsrc"`
	// FormAction defines valid sources that can be used as an HTML <form> action.
	FormAction []string `koanf:"formaction"`
	// FrameAncestors defines valid sources for embedding the resource using <frame> <iframe> <object> <embed> <applet>.
	// Setting this directive to 'none' should be roughly equivalent to X-Frame-Options: DENY.
	FrameAncestors []string `koanf:"frameancestors"`
	// BaseURI defines a set of allowed URLs which can be used in the src attribute of a HTML base tag.
	BaseURI []string `koanf:"baseuri"`
	// ReportTo defines a reporting group name defined by a Report-To HTTP response header. See the Reporting API for
	// more info.
	ReportTo string `koanf:"reportto"`
	// WorkerSrc restricts the URLs which may be loaded as a Worker, SharedWorker or ServiceWorker.
	WorkerSrc []string `koanf:"workersrc"`
	// ManifestSrc restricts the URLs that application manifests can be loaded.
	ManifestSrc []string `koanf:"manifestsrc"`
	// PrefetchSrc defines valid sources for request prefetch and prerendering, for example via the link tag with rel="prefetch" or rel="prerender":
	PrefetchSrc []string `koanf:"prefetchsrc"`
}

// directive writes a single directive, e.g. "script-src 'self' foo.com bar.com; ".
// baseline is written first if non-empty (e.g. "'self'", "'none'"), followed
// by any configured extra sources. If both baseline and sources are empty,
// nothing is written — letting the directive correctly inherit default-src.
func directive(policy *strings.Builder, name, baseline string, sources []string) {
	if baseline == "" && len(sources) == 0 {
		return
	}
	policy.WriteString(name)
	policy.WriteString(" ")
	if baseline != "" {
		policy.WriteString(baseline)
		policy.WriteString(" ")
	}
	if len(sources) > 0 {
		policy.WriteString(strings.Join(sources, " "))
		policy.WriteString(" ")
	}
	policy.WriteString("; ")
}

// String renders the CSP header value for a single request. scriptNonce and
// styleNonce, if non-empty, are appended to script-src/style-src as
// 'nonce-...' sources so callers must generate a fresh nonce per request
// rather than mutate the shared CSP config.
func (csp CSP) String(scriptNonce, styleNonce string) string {
	var policy strings.Builder

	// --- Directives that do NOT fall back to default-src: always emit a
	// baseline so they can never silently end up unrestricted. ---

	directive(&policy, "default-src", "'self'", csp.DefaultSrc)
	directive(&policy, "base-uri", "'self'", csp.BaseURI)
	directive(&policy, "form-action", "'self'", csp.FormAction)

	// object-src 'none' is non-negotiable and not configurable: combining
	// 'none' with extra sources is contradictory per spec, and there's no
	// case here where allowing <object>/<embed>/<applet> is desirable.
	policy.WriteString("object-src 'none'; ")

	// frame-ancestors: 'none' must appear alone. If you've configured
	// origins allowed to embed this site, use 'self' as the baseline
	// instead of 'none' so the combination is valid.
	if len(csp.FrameAncestors) == 0 {
		policy.WriteString("frame-ancestors 'none'; ")
	} else {
		directive(&policy, "frame-ancestors", "'self'", csp.FrameAncestors)
	}

	// --- Directives that DO fall back to default-src 'self' when omitted,
	// so it's safe to only emit them when configured. ---

	directive(&policy, "child-src", "", csp.ChildSrc)
	directive(&policy, "connect-src", "", csp.ConnectSrc)
	directive(&policy, "font-src", "'self'", csp.FontSrc)
	directive(&policy, "frame-src", "", csp.FrameSrc)
	directive(&policy, "manifest-src", "", csp.ManifestSrc)
	directive(&policy, "media-src", "", csp.MediaSrc)
	directive(&policy, "prefetch-src", "", csp.PrefetchSrc)
	directive(&policy, "worker-src", "", csp.WorkerSrc)

	if len(csp.ImgSrc) > 0 {
		directive(&policy, "img-src", "", csp.ImgSrc)
	} else {
		policy.WriteString("img-src 'self' data:; ")
	}

	// script-src / style-src: nonces are appended per-request. 'unsafe-inline'
	// and 'unsafe-eval', if present, come purely from your env config — they
	// are not hardcoded here, since they're security-sensitive choices you
	// should make explicitly.
	scriptSrc := csp.ScriptSrc
	if scriptNonce != "" {
		scriptSrc = append(append([]string{}, scriptSrc...), "'nonce-"+scriptNonce+"'")
	}
	directive(&policy, "script-src", "", scriptSrc)
	directive(&policy, "script-src-attr", "", csp.ScriptSrcAttr)

	styleSrc := csp.StyleSrc
	if styleNonce != "" {
		styleSrc = append(append([]string{}, styleSrc...), "'nonce-"+styleNonce+"'")
	}
	// 'self' is always the baseline for style-src; 'unsafe-inline' is kept
	// as a fallback for browsers that don't support nonces (browsers that do
	// support nonces ignore 'unsafe-inline' automatically when a nonce is
	// present per the CSP spec).
	directive(&policy, "style-src", "'self' 'unsafe-inline'", styleSrc)
	directive(&policy, "style-src-attr", "", csp.StyleSrcAttr)

	// --- Reporting / sandbox: no sensible default, configured-only. ---

	directive(&policy, "sandbox", "", csp.Sandbox)
	if csp.ReportURI != "" {
		policy.WriteString("report-uri ")
		policy.WriteString(csp.ReportURI)
		policy.WriteString("; ")
	}
	if csp.ReportTo != "" {
		policy.WriteString("report-to ")
		policy.WriteString(csp.ReportTo)
		policy.WriteString("; ")
	}

	return strings.TrimSpace(policy.String())
}

var (
	cspCfg  CSP
	loadCSP = sync.OnceValues(func() (CSP, error) {
		if err := config.Load(config.ConfigEnvPrefix+"CSP_", &cspCfg); err != nil {
			return cspCfg, fmt.Errorf("load csp config: %w", err)
		}
		return cspCfg, nil
	})
)

// nonceContextKey is used to retrieve the per-request style nonce later in
// the handler chain, if you need it outside of templ's own nonce context
// (e.g. for non-templ rendering paths).
type nonceContextKey struct{}

// ContentSecurityPolicy middleware injects a Content-Security-Policy header
// into responses, with a fresh nonce generated per request.
func ContentSecurityPolicy(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		csp, err := loadCSP()
		if err != nil {
			http.Error(res, fmt.Sprintf("failed to load CSP: %v", err), http.StatusInternalServerError)
			return
		}

		nonce, err := generateNonce()
		if err != nil {
			http.Error(res, fmt.Sprintf("failed to generate CSP nonce: %v", err), http.StatusInternalServerError)
			return
		}

		// Same nonce used for both script and style in this case; split
		// into two generateNonce() calls if you want them independent.
		res.Header().Set("Content-Security-Policy", csp.String("", ""))

		ctx := templ.WithNonce(req.Context(), nonce)
		ctx = context.WithValue(ctx, nonceContextKey{}, nonce)
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

func generateNonce() (string, error) {
	const nonceSize = 16
	byt := make([]byte, nonceSize)
	if _, err := rand.Read(byt); err != nil {
		return "", fmt.Errorf("read random: %w", err)
	}
	return base64.URLEncoding.EncodeToString(byt), nil
}
