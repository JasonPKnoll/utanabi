package accounts

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/common/redact"
	"github.com/JasonPKnoll/utanabi/internal/identity"
)

const testCode = "a long sign-up code"

var (
	ngrok = Site{ID: "kaze.ngrok-free.dev", Origin: "https://kaze.ngrok-free.dev"}
	local = Site{ID: "localhost", Origin: "http://localhost:8080"}
)

// server is the middleware around a handler that reports who the request is for, with a browser's cookie jar.
type server struct {
	t       *testing.T
	h       *HTTP
	repo    *fakeRepo
	handler http.Handler
	cookies map[string]*http.Cookie
}

func newServer(t *testing.T, code string) *server {
	t.Helper()
	repo := newFakeRepo()
	h := NewHTTP(repo, newFakeCeremonies(), []Site{local, ngrok}, redact.New(code))
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := identity.From(r.Context())
		if ok {
			_, _ = w.Write([]byte("user " + u.Name))
			return
		}
		_, _ = w.Write([]byte("no one"))
	})
	return &server{t: t, h: h, repo: repo, handler: h.Middleware(inner), cookies: map[string]*http.Cookie{}}
}

// do sends a request as the app's own page on site would, keeping cookies like a browser.
func (s *server) do(site Site, method, path, body string, edit ...func(*http.Request)) *httptest.ResponseRecorder {
	s.t.Helper()
	r := httptest.NewRequestWithContext(s.t.Context(), method, site.Origin+path, strings.NewReader(body))
	if method != http.MethodGet {
		r.Header.Set("Origin", site.Origin)
		r.Header.Set("Content-Type", "application/json")
	}
	for _, c := range s.cookies {
		r.AddCookie(c)
	}
	for _, e := range edit {
		e(r)
	}
	w := httptest.NewRecorder()
	s.handler.ServeHTTP(w, r)
	for _, c := range w.Result().Cookies() {
		if c.MaxAge < 0 {
			delete(s.cookies, c.Name)
		} else {
			s.cookies[c.Name] = c
		}
	}
	return w
}

func (s *server) signUp(site Site, name string) {
	s.t.Helper()
	if w := s.do(site, http.MethodPost, "/auth/signup/begin", `{"name":"`+name+`","code":"`+testCode+`"}`); w.Code != http.StatusOK {
		s.t.Fatalf("sign-up begin = %d %s", w.Code, w.Body)
	}
	if w := s.do(site, http.MethodPost, "/auth/signup/finish", "good"); w.Code != http.StatusOK {
		s.t.Fatalf("sign-up finish = %d %s", w.Code, w.Body)
	}
}

// errCode is the error code in a refused call, where the frontend's Wails runtime reads it: in "cause".
func errCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var e struct {
		Message string `json:"message"`
		Cause   struct {
			Code string `json:"code"`
		} `json:"cause"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatalf("error body %q: %v", w.Body, err)
	}
	if e.Message == "" {
		t.Errorf("error body %q has no message for the runtime", w.Body)
	}
	return e.Cause.Code
}

func TestServiceCallsNeedASignedInUser(t *testing.T) {
	t.Parallel()

	s := newServer(t, testCode)
	if w := s.do(local, http.MethodPost, "/wails/runtime", "{}"); w.Code != http.StatusUnauthorized || errCode(t, w) != "unauthorized" {
		t.Errorf("a call with no session = %d %s, want 401 unauthorized", w.Code, w.Body)
	}
	for _, path := range []string{"/", "/assets/index.js", "/wails/runtime.js", "/wails/transport.js"} {
		if w := s.do(local, http.MethodGet, path, ""); w.Code != http.StatusOK || w.Body.String() != "no one" {
			t.Errorf("GET %s with no session = %d %s, want it served", path, w.Code, w.Body)
		}
	}
	for _, path := range []string{"/wails/eventpayload/1", "/wails/stream/1"} {
		if w := s.do(local, http.MethodGet, path, ""); w.Code != http.StatusUnauthorized {
			t.Errorf("GET %s with no session = %d, want 401", path, w.Code)
		}
	}

	s.signUp(local, "hana")
	if w := s.do(local, http.MethodPost, "/wails/runtime", "{}"); w.Code != http.StatusOK || w.Body.String() != "user hana" {
		t.Errorf("a call after signing up = %d %s, want it to run as hana", w.Code, w.Body)
	}
}

func TestSessionCookieIsLockedDown(t *testing.T) {
	t.Parallel()

	s := newServer(t, testCode)
	s.signUp(ngrok, "hana")
	c := s.cookies[sessionCookie]
	if c == nil || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.Path != "/" ||
		c.MaxAge != int(SessionLife/time.Second) || !validToken(c.Value) {
		t.Errorf("session cookie = %+v, want HttpOnly, Secure, SameSite=Lax, a 30-day token", c)
	}
	if _, ok := s.cookies[ceremonyCookie]; ok {
		t.Error("the ceremony cookie outlived its ceremony")
	}
	// The cookie is only a lookup key: the server keeps its hash, never the token.
	for hash := range s.repo.sessions {
		if hash == c.Value {
			t.Error("the session token is stored as is")
		}
	}

	s2 := newServer(t, testCode)
	s2.signUp(local, "kaze")
	if c := s2.cookies[sessionCookie]; c == nil || c.Secure {
		t.Errorf("localhost session cookie = %+v, want it without Secure, which plain HTTP can't send", c)
	}
}

func TestRequestsFromOtherOriginsAreRefused(t *testing.T) {
	t.Parallel()

	s := newServer(t, testCode)
	s.signUp(local, "hana")
	foreign := func(r *http.Request) { r.Header.Set("Origin", "https://evil.example") }
	noOrigin := func(r *http.Request) { r.Header.Del("Origin") }
	if w := s.do(local, http.MethodPost, "/wails/runtime", "{}", foreign); w.Code != http.StatusBadRequest {
		t.Errorf("a call from another origin = %d %s, want refused", w.Code, w.Body)
	}
	if w := s.do(local, http.MethodPost, "/wails/runtime", "{}", noOrigin); w.Code != http.StatusBadRequest {
		t.Errorf("a call with no origin = %d %s, want refused", w.Code, w.Body)
	}
	if w := s.do(local, http.MethodPost, "/auth/signin/begin", "{}", foreign); w.Code != http.StatusBadRequest {
		t.Errorf("sign-in from another origin = %d %s, want refused", w.Code, w.Body)
	}
	form := func(r *http.Request) { r.Header.Set("Content-Type", "application/x-www-form-urlencoded") }
	if w := s.do(local, http.MethodPost, "/auth/signout", "{}", form); w.Code != http.StatusBadRequest {
		t.Errorf("a form post to sign out = %d %s, want refused", w.Code, w.Body)
	}
	unknown := Site{ID: "evil.example", Origin: "https://evil.example"}
	if w := s.do(unknown, http.MethodGet, "/auth/me", ""); w.Code != http.StatusBadRequest {
		t.Errorf("/auth/me on a host that isn't ours = %d, want refused", w.Code)
	}
}

func TestSignUpRules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, code, body, wantField string
	}{
		{name: "closed", code: "", body: `{"name":"hana","code":""}`, wantField: "signup"},
		{name: "wrong code", code: testCode, body: `{"name":"hana","code":"guess"}`, wantField: "code"},
		{name: "bad name", code: testCode, body: `{"name":"花子","code":"` + testCode + `"}`, wantField: "name"},
		{name: "not JSON", code: testCode, body: `name=hana`, wantField: "request"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w := newServer(t, tt.code).do(local, http.MethodPost, "/auth/signup/begin", tt.body)
			var e struct {
				Field string `json:"field"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
				t.Fatalf("error body %q: %v", w.Body, err)
			}
			if w.Code != http.StatusBadRequest || e.Field != tt.wantField {
				t.Errorf("sign-up begin = %d %s, want 400 on %s", w.Code, w.Body, tt.wantField)
			}
		})
	}
}

func TestATakenNameIsAConflict(t *testing.T) {
	t.Parallel()

	s := newServer(t, testCode)
	s.signUp(local, "hana")
	other := newServerSharing(t, s)
	other.do(local, http.MethodPost, "/auth/signup/begin", `{"name":"HANA","code":"`+testCode+`"}`)
	if w := other.do(local, http.MethodPost, "/auth/signup/finish", "good"); w.Code != http.StatusConflict {
		t.Errorf("signing up as a taken name = %d %s, want 409", w.Code, w.Body)
	}
}

// newServerSharing is another browser on the same server.
func newServerSharing(t *testing.T, s *server) *server {
	t.Helper()
	return &server{t: t, h: s.h, repo: s.repo, handler: s.handler, cookies: map[string]*http.Cookie{}}
}

func TestACeremonyIsAnsweredOnceOnItsOwnSite(t *testing.T) {
	t.Parallel()

	s := newServer(t, testCode)
	s.signUp(local, "hana")
	s.do(local, http.MethodPost, "/auth/signout", "{}")

	s.do(local, http.MethodPost, "/auth/signin/begin", "{}")
	ceremonyID := s.cookies[ceremonyCookie].Value
	if w := s.do(local, http.MethodPost, "/auth/signin/finish", "good:hana"); w.Code != http.StatusOK {
		t.Fatalf("sign-in = %d %s", w.Code, w.Body)
	}
	replay := func(r *http.Request) { r.AddCookie(&http.Cookie{Name: ceremonyCookie, Value: ceremonyID}) } //nolint:gosec // G124: a request cookie in a test.
	if w := s.do(local, http.MethodPost, "/auth/signin/finish", "good:hana", replay); w.Code != http.StatusBadRequest {
		t.Errorf("answering the same challenge again = %d %s, want refused", w.Code, w.Body)
	}

	// A ceremony begun on localhost can't be finished on the ngrok site.
	s.do(local, http.MethodPost, "/auth/signin/begin", "{}")
	moved := s.cookies[ceremonyCookie].Value
	carry := func(r *http.Request) { r.AddCookie(&http.Cookie{Name: ceremonyCookie, Value: moved}) } //nolint:gosec // G124: a request cookie in a test.
	if w := s.do(ngrok, http.MethodPost, "/auth/signin/finish", "good:hana", carry); w.Code != http.StatusBadRequest {
		t.Errorf("finishing on another site = %d %s, want refused", w.Code, w.Body)
	}

	// One that has run out can't be finished either.
	s.do(local, http.MethodPost, "/auth/signin/begin", "{}")
	s.h.now = func() time.Time { return time.Now().Add(ceremonyTimeout + time.Second) }
	if w := s.do(local, http.MethodPost, "/auth/signin/finish", "good:hana"); w.Code != http.StatusBadRequest {
		t.Errorf("finishing after the ceremony ran out = %d %s, want refused", w.Code, w.Body)
	}
}

func TestARejectedPasskeyDoesNotSignIn(t *testing.T) {
	t.Parallel()

	s := newServer(t, testCode)
	s.signUp(local, "hana")
	s.do(local, http.MethodPost, "/auth/signout", "{}")
	s.do(local, http.MethodPost, "/auth/signin/begin", "{}")
	if w := s.do(local, http.MethodPost, "/auth/signin/finish", "forged"); w.Code != http.StatusBadRequest {
		t.Errorf("a passkey that doesn't check out = %d %s, want refused", w.Code, w.Body)
	}
	s.do(local, http.MethodPost, "/auth/signin/begin", "{}")
	if w := s.do(local, http.MethodPost, "/auth/signin/finish", "good:nobody"); w.Code != http.StatusBadRequest {
		t.Errorf("a passkey for no account = %d %s, want refused", w.Code, w.Body)
	}
	if _, ok := s.cookies[sessionCookie]; ok {
		t.Error("a failed sign-in left a session cookie")
	}
}

func TestSignOutEndsTheSession(t *testing.T) {
	t.Parallel()

	s := newServer(t, testCode)
	s.signUp(local, "hana")
	old := s.cookies[sessionCookie].Value
	if w := s.do(local, http.MethodPost, "/auth/signout", "{}"); w.Code != http.StatusNoContent {
		t.Fatalf("sign out = %d %s", w.Code, w.Body)
	}
	stale := func(r *http.Request) { r.AddCookie(&http.Cookie{Name: sessionCookie, Value: old}) } //nolint:gosec // G124: a request cookie in a test.
	if w := s.do(local, http.MethodPost, "/wails/runtime", "{}", stale); w.Code != http.StatusUnauthorized {
		t.Errorf("a call with the signed-out token = %d, want 401", w.Code)
	}
}

func TestAnExpiredSessionIsSignedOut(t *testing.T) {
	t.Parallel()

	s := newServer(t, testCode)
	s.signUp(local, "hana")
	s.h.now = func() time.Time { return time.Now().Add(SessionLife + time.Minute) }
	if w := s.do(local, http.MethodPost, "/wails/runtime", "{}"); w.Code != http.StatusUnauthorized {
		t.Errorf("a call after 30 days = %d, want 401", w.Code)
	}
}

func TestMe(t *testing.T) {
	t.Parallel()

	s := newServer(t, testCode)
	if w := s.do(local, http.MethodGet, "/auth/me", ""); w.Body.String() != `{"user":null,"signUp":true}`+"\n" {
		t.Errorf("/auth/me signed out = %s", w.Body)
	}
	s.signUp(local, "hana")
	w := s.do(local, http.MethodGet, "/auth/me", "")
	if w.Body.String() != `{"user":{"name":"hana"},"signUp":true}`+"\n" || w.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("/auth/me signed in = %s %v", w.Body, w.Header())
	}
	if w := newServer(t, "").do(local, http.MethodGet, "/auth/me", ""); !strings.Contains(w.Body.String(), `"signUp":false`) {
		t.Errorf("/auth/me with sign-up closed = %s", w.Body)
	}
}

func TestAddAPasskeyNeedsTheSignedInUser(t *testing.T) {
	t.Parallel()

	s := newServer(t, testCode)
	if w := s.do(local, http.MethodPost, "/auth/passkeys/begin", "{}"); w.Code != http.StatusUnauthorized {
		t.Errorf("adding a passkey signed out = %d, want 401", w.Code)
	}
	s.signUp(local, "hana")
	s.do(local, http.MethodPost, "/auth/passkeys/begin", "{}")
	if w := s.do(local, http.MethodPost, "/auth/passkeys/finish", "good"); w.Code != http.StatusOK {
		t.Fatalf("adding a passkey = %d %s", w.Code, w.Body)
	}
	if a, _ := s.repo.AccountByID(t.Context(), 1); len(a.Passkeys) != 2 {
		t.Errorf("hana's passkeys = %d, want 2", len(a.Passkeys))
	}

	// A ceremony begun as hana can't add a key once the browser is signed in as someone else.
	s.do(local, http.MethodPost, "/auth/passkeys/begin", "{}")
	pending := s.cookies[ceremonyCookie]
	other := newServerSharing(t, s)
	other.signUp(local, "kaze")
	other.cookies[ceremonyCookie] = pending
	if w := other.do(local, http.MethodPost, "/auth/passkeys/finish", "good"); w.Code != http.StatusBadRequest {
		t.Errorf("finishing hana's ceremony as kaze = %d %s, want refused", w.Code, w.Body)
	}
}

func TestSignUpsAreRateLimited(t *testing.T) {
	t.Parallel()

	s := newServer(t, testCode)
	s.h.signups.limit = 2
	body := `{"name":"hana","code":"` + testCode + `"}`
	for i := range 2 {
		if w := s.do(local, http.MethodPost, "/auth/signup/begin", body); w.Code != http.StatusOK {
			t.Fatalf("sign-up %d = %d", i, w.Code)
		}
	}
	if w := s.do(local, http.MethodPost, "/auth/signup/begin", body); w.Code != http.StatusBadRequest {
		t.Errorf("a third sign-up in the hour = %d, want refused", w.Code)
	}
}

func TestSitesFor(t *testing.T) {
	t.Parallel()

	got := SitesFor([]string{"Kaze.ngrok-free.dev", "*.example.com", "localhost", "host:1"}, 18080)
	want := []Site{{ID: "localhost", Origin: "http://localhost:18080"}, ngrok}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("SitesFor = %+v, want %+v", got, want)
	}
}

func TestWrongSignupCodesAreLimited(t *testing.T) {
	t.Parallel()

	s := newServer(t, testCode)
	s.h.wrongCodes.limit = 3
	guess := `{"name":"hana","code":"guess"}`
	for range 3 {
		s.do(local, http.MethodPost, "/auth/signup/begin", guess)
	}
	w := s.do(local, http.MethodPost, "/auth/signup/begin", `{"name":"hana","code":"`+testCode+`"}`)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "too many") {
		t.Errorf("the right code after 3 wrong guesses = %d %s, want sign-up paused", w.Code, w.Body)
	}
	s.h.now = func() time.Time { return time.Now().Add(wrongCodeWindow + time.Second) }
	if w := s.do(local, http.MethodPost, "/auth/signup/begin", `{"name":"hana","code":"`+testCode+`"}`); w.Code != http.StatusOK {
		t.Errorf("the right code once the window has passed = %d %s", w.Code, w.Body)
	}
}

func TestEveryResponseCarriesTheSecurityHeaders(t *testing.T) {
	t.Parallel()

	s := newServer(t, testCode)
	for _, path := range []string{"/", "/auth/me", "/wails/runtime"} {
		w := s.do(local, http.MethodGet, path, "")
		h := w.Header()
		if !strings.Contains(h.Get("Content-Security-Policy"), "frame-ancestors 'none'") ||
			h.Get("X-Frame-Options") != "DENY" || h.Get("X-Content-Type-Options") != "nosniff" ||
			h.Get("Referrer-Policy") != "no-referrer" {
			t.Errorf("GET %s headers = %v, want the security headers", path, h)
		}
	}
}
