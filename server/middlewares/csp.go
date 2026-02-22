// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
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
	// ObjectSrc defines valid sources of plugins, eg <object>, <embed> or <applet>.
	ObjectSrc []string `koanf:"objectsrc"`
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
	// PluginTypes defines valid MIME types for plugins invoked via <object> and <embed>. To load an <applet> you must
	// specify application/x-java-applet.
	PluginTypes []string `koanf:"plugintypes"`
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

func (csp *CSP) String() string {
	var policy strings.Builder

	if len(csp.BaseURI) > 0 {
		policy.WriteString("base-uri " + strings.Join(csp.BaseURI, " ") + "; ")
	}
	if len(csp.ChildSrc) > 0 {
		policy.WriteString("child-src " + strings.Join(csp.ChildSrc, " ") + "; ")
	}
	if len(csp.ConnectSrc) > 0 {
		policy.WriteString("connect-src " + strings.Join(csp.ConnectSrc, " ") + "; ")
	}
	if len(csp.DefaultSrc) > 0 {
		policy.WriteString("default-src " + strings.Join(csp.DefaultSrc, " ") + "; ")
	}
	if len(csp.FontSrc) > 0 {
		policy.WriteString("font-src " + strings.Join(csp.FontSrc, " ") + "; ")
	}
	if len(csp.FormAction) > 0 {
		policy.WriteString("form-action " + strings.Join(csp.FormAction, " ") + "; ")
	}
	if len(csp.FrameAncestors) > 0 {
		policy.WriteString("frame-ancestors " + strings.Join(csp.FrameAncestors, " ") + "; ")
	}
	if len(csp.FrameSrc) > 0 {
		policy.WriteString("frame-src " + strings.Join(csp.FrameSrc, " ") + "; ")
	}
	if len(csp.ImgSrc) > 0 {
		policy.WriteString("img-src " + strings.Join(csp.ImgSrc, " ") + "; ")
	}
	if len(csp.ManifestSrc) > 0 {
		policy.WriteString("manifest-src " + strings.Join(csp.ManifestSrc, " ") + "; ")
	}
	if len(csp.MediaSrc) > 0 {
		policy.WriteString("media-src " + strings.Join(csp.MediaSrc, " ") + "; ")
	}
	if len(csp.ObjectSrc) > 0 {
		policy.WriteString("object-src " + strings.Join(csp.ObjectSrc, " ") + "; ")
	}
	if len(csp.PluginTypes) > 0 {
		policy.WriteString("plugin-types " + strings.Join(csp.PluginTypes, " ") + "; ")
	}
	if len(csp.PrefetchSrc) > 0 {
		policy.WriteString("prefetch-src " + strings.Join(csp.PrefetchSrc, " ") + "; ")
	}
	if csp.ReportTo != "" {
		policy.WriteString("report-to " + csp.ReportTo + "; ")
	}
	if csp.ReportURI != "" {
		policy.WriteString("report-uri " + csp.ReportURI + "; ")
	}
	if len(csp.Sandbox) > 0 {
		policy.WriteString("sandbox " + strings.Join(csp.Sandbox, " ") + "; ")
	}
	if len(csp.ScriptSrc) > 0 {
		policy.WriteString("script-src " + strings.Join(csp.ScriptSrc, " ") + "; ")
	}
	if len(csp.ScriptSrcAttr) > 0 {
		policy.WriteString("script-src-attr " + strings.Join(csp.ScriptSrcAttr, " ") + "; ")
	}
	if len(csp.StyleSrc) > 0 {
		policy.WriteString("style-src " + strings.Join(csp.StyleSrc, " ") + "; ")
	}
	if len(csp.StyleSrcAttr) > 0 {
		policy.WriteString("style-src-attr " + strings.Join(csp.StyleSrcAttr, " ") + "; ")
	}
	if len(csp.WorkerSrc) > 0 {
		policy.WriteString("worker-src " + strings.Join(csp.WorkerSrc, " ") + "; ")
	}

	return strings.TrimSpace(policy.String())
}

// LoadConfigOnce loads the auth0 configuration and ensures this is only done
// one time, no matter how many times it is called.
var loadCSP = sync.OnceValues(func() (CSP, error) {
	csp, err := config.Load[CSP](config.ConfigEnvPrefix + "CSP_")
	if err != nil {
		return csp, fmt.Errorf("load csp config: %w", err)
	}
	return csp, nil
})

var currentNonce string

// ContentSecurityPolicy middleware injects a Content-Security-Policy header into requests.
func ContentSecurityPolicy(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		var err error
		csp, err := loadCSP()
		if err != nil {
			http.Error(res, fmt.Sprintf("failed to load CSP: %v", err), http.StatusInternalServerError)
			return
		}
		ctx := req.Context()
		// if !htmx.IsHTMX(req) {
		// 	// Add nonces.
		// 	currentNonce, err = generateNonce()
		// 	if err != nil {
		// 		http.Error(
		// 			res,
		// 			fmt.Sprintf("failed to generate nonce for style-src: %v", err),
		// 			http.StatusInternalServerError,
		// 		)
		// 		return
		// 	}
		// }
		// csp.StyleSrc = append(csp.StyleSrc, "'nonce-"+currentNonce+"'")
		// csp.ScriptSrc = append(csp.ScriptSrc, "'nonce-"+currentNonce+"'")
		// Write header.
		res.Header().Add("Content-Security-Policy", csp.String())
		ctx = templ.WithNonce(ctx, currentNonce)
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

func generateNonce() (string, error) {
	const nonceSize = 16 // Size of nonce.
	byt := make([]byte, nonceSize)
	if _, err := rand.Read(byt); err != nil {
		return "", fmt.Errorf("read random: %w", err)
	}
	return base64.URLEncoding.EncodeToString(byt), nil
}
