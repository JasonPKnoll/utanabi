package spotify

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/identity"
)

// recordingAPI is fakeAPI, remembering the verifier and redirect URI a sign-in finishes with.
type recordingAPI struct {
	*fakeAPI
	verifier, redirectURI string
}

func (r *recordingAPI) Exchange(ctx context.Context, code, verifier, redirectURI string) (Tokens, error) {
	r.verifier, r.redirectURI = verifier, redirectURI
	return r.fakeAPI.Exchange(ctx, code, verifier, redirectURI)
}

func newTestHTTP(repo *fakeRepo, api API) *HTTP {
	h := NewHTTP(repo, api, "client-id", []string{"https://utanabi.example.com", "http://127.0.0.1:8080"})
	h.now = func() time.Time { return clock }
	return h
}

func get(t *testing.T, h *HTTP, target string, user *identity.User) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	if user != nil {
		req = req.WithContext(identity.Into(req.Context(), *user))
	}
	rec := httptest.NewRecorder()
	h.Middleware(http.NotFoundHandler()).ServeHTTP(rec, req)
	return rec
}

func TestConnectSendsTheUserToSpotifyWithPKCE(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	repo.link = nil
	api := &recordingAPI{fakeAPI: newFakeAPI()}
	h := newTestHTTP(repo, api)
	hana := identity.User{ID: 5, Name: "hana"}

	rec := get(t, h, "https://utanabi.example.com/auth/spotify/connect", &hana)
	to, err := url.Parse(rec.Header().Get("Location"))
	if err != nil || rec.Code != http.StatusSeeOther || to.Host != "accounts.spotify.com" || to.Path != "/authorize" {
		t.Fatalf("connect: %d to %q", rec.Code, rec.Header().Get("Location"))
	}
	q := to.Query()
	if q.Get("client_id") != "client-id" || q.Get("response_type") != "code" || q.Get("scope") != Scopes ||
		q.Get("redirect_uri") != "https://utanabi.example.com/auth/spotify/callback" ||
		q.Get("code_challenge_method") != "S256" || q.Get("state") == "" {
		t.Errorf("authorize query = %v", q)
	}

	// Spotify sends the user back with the state and a code.
	back := "https://utanabi.example.com/auth/spotify/callback?code=c1&state=" + url.QueryEscape(q.Get("state"))
	rec = get(t, h, back, nil)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "https://utanabi.example.com/#/settings" {
		t.Errorf("callback: %d to %q", rec.Code, rec.Header().Get("Location"))
	}
	sum := sha256.Sum256([]byte(api.verifier))
	if base64.RawURLEncoding.EncodeToString(sum[:]) != q.Get("code_challenge") {
		t.Error("the verifier the code was traded with doesn't match the challenge Spotify was sent")
	}
	if api.redirectURI != "https://utanabi.example.com/auth/spotify/callback" || repo.link == nil {
		t.Errorf("redirect URI %q, link %+v; want the link saved", api.redirectURI, repo.link)
	}

	// The state works once.
	repo.link = nil
	get(t, h, back, nil)
	if repo.link != nil {
		t.Error("a used state connected Spotify again")
	}
}

func TestConnectNeedsASignedInUserOnAKnownAddress(t *testing.T) {
	t.Parallel()

	h := newTestHTTP(newFakeRepo(), newFakeAPI())
	hana := identity.User{ID: 5}
	for name, rec := range map[string]*httptest.ResponseRecorder{
		"not signed in":   get(t, h, "https://utanabi.example.com/auth/spotify/connect", nil),
		"unknown address": get(t, h, "https://elsewhere.example.com/auth/spotify/connect", &hana),
	} {
		if loc := rec.Header().Get("Location"); rec.Code != http.StatusSeeOther || strings.Contains(loc, "spotify.com") {
			t.Errorf("%s: %d to %q, want back to the app", name, rec.Code, loc)
		}
	}
}

func TestCallbackTurnedDownOrForgedSavesNothing(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	repo.link = nil
	h := newTestHTTP(repo, newFakeAPI())
	hana := identity.User{ID: 5}
	to, err := url.Parse(get(t, h, "http://127.0.0.1:8080/auth/spotify/connect", &hana).Header().Get("Location"))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	state := url.QueryEscape(to.Query().Get("state"))

	rec := get(t, h, "http://127.0.0.1:8080/auth/spotify/callback?error=access_denied&state="+state, nil)
	if rec.Header().Get("Location") != "http://127.0.0.1:8080/#/settings" || repo.link != nil {
		t.Errorf("turned down: to %q, link %+v", rec.Header().Get("Location"), repo.link)
	}
	rec = get(t, h, "http://127.0.0.1:8080/auth/spotify/callback?code=c1&state=forged", nil)
	if rec.Header().Get("Location") != "/#/settings" || repo.link != nil {
		t.Errorf("forged: to %q, link %+v", rec.Header().Get("Location"), repo.link)
	}
}

func TestMiddlewarePassesOtherRequestsOn(t *testing.T) {
	t.Parallel()

	if rec := get(t, newTestHTTP(newFakeRepo(), newFakeAPI()), "https://utanabi.example.com/", nil); rec.Code != http.StatusNotFound {
		t.Errorf("status %d, want the next handler's 404", rec.Code)
	}
}
