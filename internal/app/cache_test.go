package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCacheHeaders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path string
		want string
	}{
		{"/", "no-cache"},
		{"/index.html", "no-cache"},
		{"/favicon.svg", "no-cache"},
		{"/assets/index-28441nmy.js", "public, max-age=31536000, immutable"},
		{"/assets/index-DwVNIRQ1.css", "public, max-age=31536000, immutable"},
	}
	ok := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			cacheHeaders(ok).ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, tt.path, nil))
			if got := rec.Header().Get("Cache-Control"); got != tt.want {
				t.Errorf("Cache-Control = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWithCacheHeadersKeepsTheAppsMiddleware(t *testing.T) {
	t.Parallel()

	ran := false
	signIn := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ran = true
			next.ServeHTTP(w, r)
		})
	}
	rec := httptest.NewRecorder()
	ok := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	withCacheHeaders(signIn)(ok).ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	if !ran {
		t.Error("the app's middleware didn't run")
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", got)
	}
}
