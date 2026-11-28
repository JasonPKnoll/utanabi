package spotify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/common/redact"
)

// The Spotify hosts: its sign-in, which hands out tokens, and its Web API.
const (
	AccountsURL = "https://accounts.spotify.com"
	APIURL      = "https://api.spotify.com"
)

const (
	apiTimeout  = 10 * time.Second
	apiMaxBytes = 1 << 20
)

// WebAPI is the API for spotify.com. It signs in as a public client with PKCE, so it has a client ID and no secret:
// nothing in the app could keep one.
type WebAPI struct {
	accounts string
	api      string
	clientID string
	client   *http.Client
}

// NewWebAPI returns the API at accounts and api (AccountsURL and APIURL, or test servers), for the app's client ID.
func NewWebAPI(accounts, api, clientID string) *WebAPI {
	accounts, api = strings.TrimRight(accounts, "/"), strings.TrimRight(api, "/")
	hosts := map[string]bool{}
	for _, base := range []string{accounts, api} {
		if u, err := url.Parse(base); err == nil {
			hosts[u.Host] = true
		}
	}
	return &WebAPI{
		accounts: accounts, api: api, clientID: clientID,
		client: &http.Client{
			Timeout: apiTimeout,
			// The app talks only to Spotify, so a redirect elsewhere is refused rather than followed.
			CheckRedirect: func(req *http.Request, _ []*http.Request) error {
				if !hosts[req.URL.Host] {
					return fmt.Errorf("spotify: redirected to another host (%s)", req.URL.Host)
				}
				return nil
			},
		},
	}
}

type tokenJSON struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	Scope        string `json:"scope"`
	Error        string `json:"error"`
}

func (w *WebAPI) Exchange(ctx context.Context, code, verifier, redirectURI string) (Tokens, error) {
	return w.token(ctx, url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {redirectURI},
		"client_id": {w.clientID}, "code_verifier": {verifier},
	})
}

func (w *WebAPI) Refresh(ctx context.Context, refresh redact.Secret) (Tokens, error) {
	return w.token(ctx, url.Values{
		"grant_type": {"refresh_token"}, "refresh_token": {refresh.Reveal()}, "client_id": {w.clientID},
	})
}

func (w *WebAPI) token(ctx context.Context, form url.Values) (Tokens, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.accounts+"/api/token",
		strings.NewReader(form.Encode()))
	if err != nil {
		return Tokens{}, fmt.Errorf("spotify: build token request (%v)", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	status, raw, err := w.do(req)
	if err != nil {
		return Tokens{}, err
	}
	var body tokenJSON
	if err := json.Unmarshal(raw, &body); err != nil {
		return Tokens{}, fmt.Errorf("spotify: token response isn't JSON (status %d, %v)", status, err)
	}
	switch {
	case status == http.StatusBadRequest && body.Error == "invalid_grant":
		return Tokens{}, ErrRejected
	case status != http.StatusOK:
		return Tokens{}, fmt.Errorf("spotify: token request refused (status %d, %s)", status, body.Error)
	case body.AccessToken == "":
		return Tokens{}, errors.New("spotify: token response had no access token")
	}
	return Tokens{Access: redact.New(body.AccessToken), Refresh: redact.New(body.RefreshToken),
		ExpiresIn: time.Duration(body.ExpiresIn) * time.Second, Scope: body.Scope}, nil
}

func (w *WebAPI) DisplayName(ctx context.Context, access redact.Secret) (string, error) {
	var me struct {
		DisplayName string `json:"display_name"`
		ID          string `json:"id"`
	}
	if err := w.call(ctx, access, http.MethodGet, "/v1/me", nil, &me); err != nil {
		return "", err
	}
	if me.DisplayName == "" {
		return me.ID, nil
	}
	return me.DisplayName, nil
}

type trackJSON struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	DurationMS int64  `json:"duration_ms"`
	Artists    []struct {
		Name string `json:"name"`
	} `json:"artists"`
}

func (t trackJSON) track() Track {
	out := Track{ID: t.ID, Name: t.Name, DurationMS: t.DurationMS}
	for _, a := range t.Artists {
		out.Artists = append(out.Artists, a.Name)
	}
	return out
}

type deviceJSON struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	IsActive bool   `json:"is_active"`
}

func (d deviceJSON) device() Device {
	return Device{ID: d.ID, Name: d.Name, Type: d.Type, Active: d.IsActive}
}

func (w *WebAPI) PlaybackState(ctx context.Context, access redact.Secret) (State, bool, error) {
	var body struct {
		Device     deviceJSON `json:"device"`
		ProgressMS int64      `json:"progress_ms"`
		IsPlaying  bool       `json:"is_playing"`
		Type       string     `json:"currently_playing_type"`
		Item       *trackJSON `json:"item"`
	}
	found := false
	err := w.callFound(ctx, access, http.MethodGet, "/v1/me/player", nil, &body, &found)
	if err != nil || !found {
		return State{}, false, err
	}
	state := State{Playing: body.IsPlaying, ProgressMS: body.ProgressMS, Device: body.Device.device()}
	if body.Item != nil && body.Type == "track" {
		state.Track = body.Item.track()
	}
	return state, true, nil
}

func (w *WebAPI) Devices(ctx context.Context, access redact.Secret) ([]Device, error) {
	var body struct {
		Devices []deviceJSON `json:"devices"`
	}
	if err := w.call(ctx, access, http.MethodGet, "/v1/me/player/devices", nil, &body); err != nil {
		return nil, err
	}
	out := make([]Device, 0, len(body.Devices))
	for _, d := range body.Devices {
		out = append(out, d.device())
	}
	return out, nil
}

func (w *WebAPI) SearchTracks(ctx context.Context, access redact.Secret, query string) ([]Track, error) {
	var body struct {
		Tracks struct {
			Items []trackJSON `json:"items"`
		} `json:"tracks"`
	}
	// No market: with a user's token, Spotify searches their own country's catalogue anyway, and market=from_token
	// would need the user-read-private scope, which the app doesn't ask for.
	path := "/v1/search?" + url.Values{"q": {query}, "type": {"track"}, "limit": {"10"}}.Encode()
	if err := w.call(ctx, access, http.MethodGet, path, nil, &body); err != nil {
		return nil, err
	}
	out := make([]Track, 0, len(body.Tracks.Items))
	for _, t := range body.Tracks.Items {
		out = append(out, t.track())
	}
	return out, nil
}

func (w *WebAPI) Play(ctx context.Context, access redact.Secret, deviceID, trackID string, positionMS int64) error {
	body, err := json.Marshal(map[string]any{"uris": []string{uri(trackID)}, "position_ms": positionMS})
	if err != nil {
		return fmt.Errorf("spotify: encode play request (%v)", err)
	}
	path := "/v1/me/player/play?" + url.Values{"device_id": {deviceID}}.Encode()
	return w.call(ctx, access, http.MethodPut, path, body, nil)
}

func (w *WebAPI) Resume(ctx context.Context, access redact.Secret) error {
	return w.call(ctx, access, http.MethodPut, "/v1/me/player/play", nil, nil)
}

func (w *WebAPI) Pause(ctx context.Context, access redact.Secret) error {
	return w.call(ctx, access, http.MethodPut, "/v1/me/player/pause", nil, nil)
}

func (w *WebAPI) Seek(ctx context.Context, access redact.Secret, positionMS int64) error {
	path := "/v1/me/player/seek?position_ms=" + strconv.FormatInt(positionMS, 10)
	return w.call(ctx, access, http.MethodPut, path, nil, nil)
}

func (w *WebAPI) call(ctx context.Context, access redact.Secret, method, path string, body []byte, out any) error {
	return w.callFound(ctx, access, method, path, body, out, nil)
}

// callFound calls the Web API and reads its JSON answer into out, when there is one. A 204 with nothing to say sets
// *found false, when found isn't nil; Spotify answers that way when nothing is playing.
func (w *WebAPI) callFound(ctx context.Context, access redact.Secret, method, path string, body []byte, out any,
	found *bool,
) error {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, w.api+path, reader)
	if err != nil {
		return fmt.Errorf("spotify: build request (%v)", err)
	}
	req.Header.Set("Authorization", "Bearer "+access.Reveal())
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	status, raw, err := w.do(req)
	if err != nil {
		return err
	}
	if found != nil {
		*found = status != http.StatusNoContent
	}
	switch {
	case status >= 200 && status < 300:
		if out == nil || len(bytes.TrimSpace(raw)) == 0 {
			return nil
		}
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("spotify: response isn't JSON (%v)", err)
		}
		return nil
	case status == http.StatusUnauthorized:
		return ErrRejected
	case status == http.StatusTooManyRequests:
		return ErrBusy
	}
	var failed struct {
		Error struct {
			Message string `json:"message"`
			Reason  string `json:"reason"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &failed); err != nil {
		return fmt.Errorf("spotify: request refused (%s %s, status %d)", method, pathOnly(path), status)
	}
	switch {
	case status == http.StatusNotFound && failed.Error.Reason == "NO_ACTIVE_DEVICE":
		return ErrNoDevice
	case strings.Contains(path, "device_id=") && (status == http.StatusNotFound || status >= 500):
		// A device asked for by ID that Spotify no longer lists ("Device not found"), or that failed to start
		// (Bad gateway), as the page's own player does when it can't play.
		return ErrDeviceFailed
	case status == http.StatusForbidden && failed.Error.Reason == "PREMIUM_REQUIRED":
		return ErrPremium
	}
	return fmt.Errorf("spotify: request refused (%s %s, status %d, %s)", method, pathOnly(path), status,
		failed.Error.Message)
}

func pathOnly(path string) string {
	return strings.SplitN(path, "?", 2)[0]
}

// do sends req and returns its status and body. Only a failure to reach Spotify is an error; the status is the
// caller's to judge.
func (w *WebAPI) do(req *http.Request) (int, []byte, error) {
	resp, err := w.client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("spotify: can't reach Spotify (%v)", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, apiMaxBytes))
	if err != nil {
		return 0, nil, fmt.Errorf("spotify: read response (%v)", err)
	}
	return resp.StatusCode, raw, nil
}
