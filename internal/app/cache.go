package app

import (
	"net/http"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// cacheHeaders tells browsers how long they may keep what the app serves. The built scripts and styles under
// /assets/ have a hash of their content in their names, so a new build gives them new names and they can be kept for
// good. Everything else, the page above all, is checked with the server every time: with no header at all, Safari on
// iOS keeps the page and goes on running an old build, with only its calls reaching the new one.
func cacheHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		next.ServeHTTP(w, r)
	})
}

// withCacheHeaders puts cacheHeaders in front of the app's own middleware, when it has one.
func withCacheHeaders(middleware application.Middleware) application.Middleware {
	if middleware == nil {
		return cacheHeaders
	}
	return func(next http.Handler) http.Handler {
		return cacheHeaders(middleware(next))
	}
}
