package accounts

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/app"
	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/common/redact"
	"github.com/JasonPKnoll/utanabi/internal/identity"
)

const (
	sessionCookie  = "utanabi_session"
	ceremonyCookie = "utanabi_ceremony"
	// maxBody bounds a passkey response, which is a few kilobytes.
	maxBody = 64 << 10
	// maxCeremonies bounds the passkey ceremonies waiting for their second half, so nobody can fill memory.
	maxCeremonies = 10_000
	// signupLimit is how many accounts can be made in signupWindow, server-wide, so a leaked sign-up code can't be
	// used to fill the database.
	signupLimit  = 30
	signupWindow = time.Hour
	// wrongCodeLimit is how many wrong sign-up codes are tried in wrongCodeWindow, server-wide, before sign-up stops
	// answering for a while, so a code can't be guessed by brute force.
	wrongCodeLimit  = 20
	wrongCodeWindow = 10 * time.Minute
)

// contentSecurity keeps the page to its own scripts, styles, and connections, so injected markup can't load code from
// elsewhere or send data away, and the page can't be framed by another site. Svelte sets styles inline.
const contentSecurity = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; " +
	"form-action 'self'; object-src 'none'"

// HTTP is sign-in over HTTP, and the middleware that puts the signed-in user in every request's context. Sessions
// have to be cookies the page's scripts can't read, which only an HTTP response can set, so sign-in lives here rather
// than in a bound Wails service.
type HTTP struct {
	repo       Repository
	cer        Ceremonies
	sites      map[string]Site
	signupCode redact.Secret
	now        func() time.Time
	pending    *pendingCeremonies
	signups    *window
	wrongCodes *window
}

// NewHTTP returns sign-in for the given sites. With no signupCode, nobody can make an account.
func NewHTTP(repo Repository, cer Ceremonies, sites []Site, signupCode redact.Secret) *HTTP {
	bySite := make(map[string]Site, len(sites))
	for _, s := range sites {
		bySite[s.ID] = s
	}
	return &HTTP{
		repo: repo, cer: cer, sites: bySite, signupCode: signupCode, now: time.Now,
		pending:    &pendingCeremonies{byID: map[string]ceremony{}},
		signups:    &window{limit: signupLimit, span: signupWindow},
		wrongCodes: &window{limit: wrongCodeLimit, span: wrongCodeWindow},
	}
}

// SitesFor returns the sites passkeys may be used on: each exact host in hosts over HTTPS (patterns with * are left
// out), and localhost on port over HTTP, which browsers treat as secure.
func SitesFor(hosts []string, port int) []Site {
	sites := []Site{{ID: "localhost", Origin: "http://localhost:" + strconv.Itoa(port)}}
	for _, h := range hosts {
		h = strings.ToLower(strings.TrimSpace(h))
		if h == "" || h == "localhost" || strings.ContainsAny(h, "*/:") {
			continue
		}
		sites = append(sites, Site{ID: h, Origin: "https://" + h})
	}
	return sites
}

// Middleware signs requests in from their session cookie. In server mode every call to a bound service needs a
// signed-in user; the page itself and its scripts load without one, so the sign-in screen can show. Any request that
// changes something must come from one of the app's own origins.
func (h *HTTP) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header.Set("Content-Security-Policy", contentSecurity)
		header.Set("X-Frame-Options", "DENY")
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("Referrer-Policy", "no-referrer")
		if strings.HasPrefix(r.URL.Path, "/auth/") {
			h.serveAuth(w, r)
			return
		}
		if user, ok := h.sessionUser(r); ok {
			r = r.WithContext(identity.Into(r.Context(), user))
		} else if needsSignIn(r.URL.Path) {
			h.failCall(w, r, fault.ErrUnauthorized)
			return
		}
		if changes(r.Method) && !h.fromOwnOrigin(r) {
			h.failCall(w, r, errForeignOrigin)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// needsSignIn: everything Wails serves under /wails/ (calls to bound services, event payloads, streams), except the
// scripts the page needs to load.
func needsSignIn(path string) bool {
	switch path {
	case "/wails/runtime.js", "/wails/transport.js", "/wails/custom.js":
		return false
	}
	return strings.HasPrefix(path, "/wails/")
}

func changes(method string) bool {
	return method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions
}

var (
	errForeignOrigin = fault.Invalid("origin", "isn't this site")
	errNotJSON       = fault.Invalid("request", "must be JSON")
	errNoCeremony    = fault.Invalid("passkey", "took too long or was already used; please start again")
	errBusy          = fault.Invalid("passkey", "can't be checked right now; please try again in a minute")
	errTooMany       = fault.Invalid("signup", "has had too many new accounts lately; please try again later")
	errUnknownSite   = fault.Invalid("site", "isn't one passkeys work on here")
)

func (h *HTTP) serveAuth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	site, ok := h.site(r)
	if !ok {
		h.fail(w, r, errUnknownSite)
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/auth/me" {
		h.me(w, r)
		return
	}
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	if r.Header.Get("Origin") != site.Origin {
		h.fail(w, r, errForeignOrigin)
		return
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		h.fail(w, r, errNotJSON)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	switch r.URL.Path {
	case "/auth/signup/begin":
		h.signUpBegin(w, r, site)
	case "/auth/signup/finish":
		h.signUpFinish(w, r, site)
	case "/auth/signin/begin":
		h.signInBegin(w, r, site)
	case "/auth/signin/finish":
		h.signInFinish(w, r, site)
	case "/auth/passkeys/begin":
		h.passkeyBegin(w, r, site)
	case "/auth/passkeys/finish":
		h.passkeyFinish(w, r, site)
	case "/auth/signout":
		h.signOut(w, r, site)
	default:
		http.NotFound(w, r)
	}
}

type userJSON struct {
	Name string `json:"name"`
}

// me says who's signed in, and whether this server takes new accounts.
func (h *HTTP) me(w http.ResponseWriter, r *http.Request) {
	body := struct {
		User   *userJSON `json:"user"`
		SignUp bool      `json:"signUp"`
	}{SignUp: !h.signupCode.IsZero()}
	if u, ok := h.sessionUser(r); ok {
		body.User = &userJSON{Name: u.Name}
	}
	writeJSON(w, http.StatusOK, body)
}

func (h *HTTP) signUpBegin(w http.ResponseWriter, r *http.Request, site Site) {
	var in struct {
		Name string `json:"name"`
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		h.fail(w, r, errNotJSON)
		return
	}
	name, err := Name(in.Name)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if h.wrongCodes.full(h.now()) {
		h.fail(w, r, errTooMany)
		return
	}
	if err := CheckSignupCode(in.Code, h.signupCode); err != nil {
		if !errors.Is(err, ErrSignupClosed) {
			h.wrongCodes.allow(h.now())
		}
		h.fail(w, r, err)
		return
	}
	if !h.signups.allow(h.now()) {
		h.fail(w, r, errTooMany)
		return
	}
	handle, err := NewHandle()
	if err != nil {
		h.fail(w, r, err)
		return
	}
	draft := Account{Handle: handle, Name: name}
	options, state, err := h.cer.BeginRegistration(site, draft)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.startCeremony(w, r, site, ceremony{kind: "signup", account: draft, state: state}, options)
}

func (h *HTTP) signUpFinish(w http.ResponseWriter, r *http.Request, site Site) {
	c, response, ok := h.finishCeremony(w, r, site, "signup")
	if !ok {
		return
	}
	a, err := SignUp(r.Context(), h.repo, h.cer, site, c.account, c.state, response, h.now())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.startSession(w, r, site, identity.User{ID: a.ID, Name: a.Name})
}

func (h *HTTP) signInBegin(w http.ResponseWriter, r *http.Request, site Site) {
	var in struct {
		Conditional bool `json:"conditional"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		h.fail(w, r, errNotJSON)
		return
	}
	options, state, err := h.cer.BeginSignIn(site, in.Conditional)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.startCeremony(w, r, site, ceremony{kind: "signin", state: state}, options)
}

func (h *HTTP) signInFinish(w http.ResponseWriter, r *http.Request, site Site) {
	c, response, ok := h.finishCeremony(w, r, site, "signin")
	if !ok {
		return
	}
	a, err := SignIn(r.Context(), h.repo, h.cer, site, c.state, response, h.now())
	if errors.Is(err, fault.ErrNotFound) {
		err = ErrPasskeyRejected
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.startSession(w, r, site, identity.User{ID: a.ID, Name: a.Name})
}

func (h *HTTP) passkeyBegin(w http.ResponseWriter, r *http.Request, site Site) {
	u, ok := h.sessionUser(r)
	if !ok {
		h.fail(w, r, fault.ErrUnauthorized)
		return
	}
	a, err := h.repo.AccountByID(r.Context(), u.ID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	options, state, err := h.cer.BeginRegistration(site, a)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.startCeremony(w, r, site, ceremony{kind: "passkey", account: a, state: state}, options)
}

func (h *HTTP) passkeyFinish(w http.ResponseWriter, r *http.Request, site Site) {
	u, ok := h.sessionUser(r)
	if !ok {
		h.fail(w, r, fault.ErrUnauthorized)
		return
	}
	c, response, ok := h.finishCeremony(w, r, site, "passkey")
	if !ok {
		return
	}
	// The ceremony was begun for the signed-in user; a session swapped in between can't add a key to it.
	if c.account.ID != u.ID {
		h.fail(w, r, errNoCeremony)
		return
	}
	if err := AddPasskey(r.Context(), h.repo, h.cer, site, c.account, c.state, response, h.now()); err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, struct{}{})
}

func (h *HTTP) signOut(w http.ResponseWriter, r *http.Request, site Site) {
	if c, err := r.Cookie(sessionCookie); err == nil && validToken(c.Value) {
		if err := h.repo.DeleteSession(r.Context(), TokenHash(c.Value)); err != nil {
			h.fail(w, r, err)
			return
		}
	}
	h.setCookie(w, site, sessionCookie, "", "/", -1, http.SameSiteLaxMode)
	w.WriteHeader(http.StatusNoContent)
}

func (h *HTTP) startCeremony(w http.ResponseWriter, r *http.Request, site Site, c ceremony, options json.RawMessage) {
	c.site = site
	c.expires = h.now().Add(ceremonyTimeout)
	id, err := h.pending.put(c, h.now())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.setCookie(w, site, ceremonyCookieFor(c.kind), id, "/auth/", int(ceremonyTimeout/time.Second),
		http.SameSiteStrictMode)
	writeJSON(w, http.StatusOK, struct {
		Options json.RawMessage `json:"options"`
	}{options})
}

// finishCeremony takes the ceremony the browser's cookie points at, once: a challenge can't be answered twice.
func (h *HTTP) finishCeremony(w http.ResponseWriter, r *http.Request, site Site, kind string) (
	ceremony, []byte, bool,
) {
	h.setCookie(w, site, ceremonyCookieFor(kind), "", "/auth/", -1, http.SameSiteStrictMode)
	cookie, err := r.Cookie(ceremonyCookieFor(kind))
	if err != nil {
		h.fail(w, r, errNoCeremony)
		return ceremony{}, nil, false
	}
	c, ok := h.pending.take(cookie.Value, h.now())
	if !ok || c.kind != kind || c.site != site {
		h.fail(w, r, errNoCeremony)
		return ceremony{}, nil, false
	}
	response, err := io.ReadAll(r.Body)
	if err != nil {
		h.fail(w, r, errNotJSON)
		return ceremony{}, nil, false
	}
	return c, response, true
}

func (h *HTTP) startSession(w http.ResponseWriter, r *http.Request, site Site, u identity.User) {
	token, hash, err := NewSessionToken()
	if err == nil {
		now := h.now()
		err = h.repo.CreateSession(r.Context(), hash, u.ID, now, now.Add(SessionLife))
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.setCookie(w, site, sessionCookie, token, "/", int(SessionLife/time.Second), http.SameSiteLaxMode)
	writeJSON(w, http.StatusOK, struct {
		User userJSON `json:"user"`
	}{userJSON{Name: u.Name}})
}

// sessionUser is the signed-in user of the request's session cookie, if it's a live session on one of the app's
// sites.
func (h *HTTP) sessionUser(r *http.Request) (identity.User, bool) {
	if _, ok := h.site(r); !ok {
		return identity.User{}, false
	}
	c, err := r.Cookie(sessionCookie)
	if err != nil || !validToken(c.Value) {
		return identity.User{}, false
	}
	u, err := h.repo.SessionUser(r.Context(), TokenHash(c.Value), h.now())
	if err != nil {
		return identity.User{}, false
	}
	return u, true
}

// validToken checks a cookie looks like a session token before it costs a database lookup.
func validToken(v string) bool {
	if len(v) != 43 {
		return false
	}
	for _, r := range v {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' && r != '_' {
			return false
		}
	}
	return true
}

func (h *HTTP) site(r *http.Request) (Site, bool) {
	host := r.Host
	if name, _, err := net.SplitHostPort(host); err == nil {
		host = name
	}
	s, ok := h.sites[strings.ToLower(host)]
	return s, ok
}

func (h *HTTP) fromOwnOrigin(r *http.Request) bool {
	s, ok := h.site(r)
	return ok && r.Header.Get("Origin") == s.Origin
}

// setCookie sets a cookie scripts can't read, sent only over HTTPS except on localhost.
func (h *HTTP) setCookie(w http.ResponseWriter, site Site, name, value, path string, maxAge int, same http.SameSite) {
	// G124: Secure is off only for localhost, which is plain HTTP; a browser drops a Secure cookie there.
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // G124: see above.
		Name:     name,
		Value:    value,
		Path:     path,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   strings.HasPrefix(site.Origin, "https://"),
		SameSite: same,
	})
}

// fail answers with the same error shape bound services use, through app.Fail, which logs it once and keeps
// internal detail out of the message.
func (h *HTTP) fail(w http.ResponseWriter, r *http.Request, err error) {
	var e *app.Error
	if !errors.As(app.Fail(r.Context(), err), &e) {
		e = &app.Error{Code: app.CodeInternal, Message: "Something went wrong. Please try again."}
	}
	status := http.StatusInternalServerError
	switch e.Code {
	case app.CodeInvalidInput:
		status = http.StatusBadRequest
	case app.CodeUnauthorized:
		status = http.StatusUnauthorized
	case app.CodeNotFound:
		status = http.StatusNotFound
	case app.CodeConflict:
		status = http.StatusConflict
	case app.CodeTimeout:
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, e)
}

// failCall answers a refused call to a bound service the way Wails answers a failed one: the frontend's runtime takes
// the error from "cause", which is where toAppError looks.
func (h *HTTP) failCall(w http.ResponseWriter, r *http.Request, err error) {
	var e *app.Error
	if !errors.As(app.Fail(r.Context(), err), &e) {
		e = &app.Error{Code: app.CodeInternal, Message: "Something went wrong. Please try again."}
	}
	status := http.StatusBadRequest
	if e.Code == app.CodeUnauthorized {
		status = http.StatusUnauthorized
	}
	writeJSON(w, status, struct {
		Message string     `json:"message"`
		Cause   *app.Error `json:"cause"`
	}{e.Message, e})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	b, err := json.Marshal(body)
	if err != nil {
		http.Error(w, "Something went wrong. Please try again.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	// The status is sent; a client that hung up has nothing left to read.
	_, _ = w.Write(append(b, '\n'))
}

// ceremonyCookieFor names the cookie of each kind of ceremony, so a sign-in waiting quietly in the background never
// stands in the way of signing up.
func ceremonyCookieFor(kind string) string {
	return ceremonyCookie + "_" + kind
}

// ceremony is the first half of a passkey ceremony, waiting for the browser's answer.
type ceremony struct {
	kind    string
	site    Site
	account Account
	state   []byte
	expires time.Time
}

type pendingCeremonies struct {
	mu   sync.Mutex
	byID map[string]ceremony
}

func (p *pendingCeremonies) put(c ceremony, now time.Time) (string, error) {
	token, _, err := NewSessionToken()
	if err != nil {
		return "", err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.byID) >= maxCeremonies {
		for id, old := range p.byID {
			if now.After(old.expires) {
				delete(p.byID, id)
			}
		}
		if len(p.byID) >= maxCeremonies {
			return "", errBusy
		}
	}
	p.byID[token] = c
	return token, nil
}

func (p *pendingCeremonies) take(id string, now time.Time) (ceremony, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	c, ok := p.byID[id]
	delete(p.byID, id)
	return c, ok && now.Before(c.expires)
}

// window allows limit events in any span of time.
type window struct {
	mu    sync.Mutex
	limit int
	span  time.Duration
	times []time.Time
}

func (w *window) allow(now time.Time) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.drop(now)
	if len(w.times) >= w.limit {
		return false
	}
	w.times = append(w.times, now)
	return true
}

// full reports whether the window has had its limit, without counting this call.
func (w *window) full(now time.Time) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.drop(now)
	return len(w.times) >= w.limit
}

func (w *window) drop(now time.Time) {
	kept := w.times[:0]
	for _, t := range w.times {
		if now.Sub(t) < w.span {
			kept = append(kept, t)
		}
	}
	w.times = kept
}
