package spotify

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/app"
	"github.com/JasonPKnoll/utanabi/internal/identity"
)

// The two pages of Spotify's sign-in: connect sends the signed-in user to Spotify, and Spotify sends them back to the
// callback, which must be registered, exactly, as a redirect URI of the app on Spotify's developer dashboard.
const (
	connectPath  = "/auth/spotify/connect"
	callbackPath = "/auth/spotify/callback"
	// A sign-in left half done is forgotten after this, and a few hundred at most are kept.
	pendingFor = 10 * time.Minute
	maxPending = 500
)

// HTTP answers Spotify's sign-in. It uses the authorization code flow with PKCE: the server makes a secret verifier
// for each sign-in and sends Spotify only its hash, so the code Spotify hands back is worthless without it, and the
// app needs no client secret.
type HTTP struct {
	repo      Repository
	api       API
	clientID  string
	authorize string
	// bases are the addresses Spotify may send people back to, by host: https://<host>, and http://127.0.0.1:<port>.
	bases map[string]string
	now   func() time.Time

	mu      sync.Mutex
	pending map[string]pending
}

// pending is a sign-in under way: who started it, its PKCE verifier, and where it came from.
type pending struct {
	user        identity.User
	verifier    string
	base        string
	redirectURI string
	expires     time.Time
}

// NewHTTP returns the sign-in for the app's client ID. bases are the app's own addresses, such as
// https://utanabi.example.com; Spotify only accepts the ones registered on the dashboard.
func NewHTTP(repo Repository, api API, clientID string, bases []string) *HTTP {
	byHost := map[string]string{}
	for _, b := range bases {
		if u, err := url.Parse(b); err == nil && u.Host != "" {
			byHost[strings.ToLower(u.Host)] = strings.TrimRight(b, "/")
		}
	}
	return &HTTP{repo: repo, api: api, clientID: clientID, authorize: AccountsURL + "/authorize", bases: byHost,
		now: time.Now, pending: map[string]pending{}}
}

// Middleware answers the sign-in's two pages and passes everything else on. It runs inside the accounts middleware,
// so a signed-in user is already in the request's context.
func (h *HTTP) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == connectPath:
			w.Header().Set("Cache-Control", "no-store")
			h.connect(w, r)
		case r.Method == http.MethodGet && r.URL.Path == callbackPath:
			w.Header().Set("Cache-Control", "no-store")
			h.callback(w, r)
		default:
			next.ServeHTTP(w, r)
		}
	})
}

func (h *HTTP) connect(w http.ResponseWriter, r *http.Request) {
	user, ok := identity.From(r.Context())
	base, known := h.bases[strings.ToLower(r.Host)]
	if !ok || !known {
		// Not signed in, or on an address Spotify won't send people back to: back to the app, which says what to do.
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	// rand.Text is 26 characters of base32, 128 random bits: a state, and, twice over, a PKCE verifier of 52
	// characters, within the 43 to 128 allowed.
	state, verifier := rand.Text(), rand.Text()+rand.Text()
	p := pending{user: user, verifier: verifier, base: base, redirectURI: base + callbackPath,
		expires: h.now().Add(pendingFor)}
	if !h.remember(state, p) {
		http.Error(w, "Too many sign-ins are under way. Please try again in a few minutes.",
			http.StatusServiceUnavailable)
		return
	}
	challenge := sha256.Sum256([]byte(verifier))
	q := url.Values{
		"response_type": {"code"}, "client_id": {h.clientID}, "scope": {Scopes}, "redirect_uri": {p.redirectURI},
		"state": {state}, "code_challenge_method": {"S256"},
		"code_challenge": {base64.RawURLEncoding.EncodeToString(challenge[:])},
	}
	http.Redirect(w, r, h.authorize+"?"+q.Encode(), http.StatusSeeOther)
}

func (h *HTTP) callback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	p, ok := h.take(q.Get("state"))
	if !ok {
		// An old, used, or forged sign-in: start again from Settings.
		http.Redirect(w, r, "/#/settings", http.StatusSeeOther)
		return
	}
	settings := p.base + "/#/settings"
	if q.Get("error") != "" {
		// Turned down on Spotify's page: nothing to save.
		http.Redirect(w, r, settings, http.StatusSeeOther)
		return
	}
	ctx := identity.Into(r.Context(), p.user)
	// A failure is logged, and Settings then shows Spotify as not connected, to try again.
	app.Note(ctx, Connect(ctx, h.repo, h.api, q.Get("code"), p.verifier, p.redirectURI, h.now()))
	http.Redirect(w, r, settings, http.StatusSeeOther)
}

// remember keeps a sign-in under way, dropping ones that ran out. It refuses when too many are under way.
func (h *HTTP) remember(state string, p pending) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := h.now()
	for k, v := range h.pending {
		if now.After(v.expires) {
			delete(h.pending, k)
		}
	}
	if len(h.pending) >= maxPending {
		return false
	}
	h.pending[state] = p
	return true
}

// take returns a sign-in under way and forgets it, so its state works once.
func (h *HTTP) take(state string) (pending, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	p, ok := h.pending[state]
	delete(h.pending, state)
	if !ok || h.now().After(p.expires) {
		return pending{}, false
	}
	return p, true
}
