package spotify

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/common/redact"
)

// fakeSpotify answers as Spotify's two hosts do, from one test server.
func fakeSpotify(t *testing.T, handle func(w http.ResponseWriter, r *http.Request)) *WebAPI {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(handle))
	t.Cleanup(srv.Close)
	return NewWebAPI(srv.URL, srv.URL, "client-id")
}

func writeBody(t *testing.T, w io.Writer, body string) {
	t.Helper()
	if _, err := io.WriteString(w, body); err != nil {
		t.Errorf("write: %v", err)
	}
}

func TestExchangeSendsTheVerifierAndNoSecret(t *testing.T) {
	t.Parallel()

	var form url.Values
	api := fakeSpotify(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/token" || r.Method != http.MethodPost {
			t.Errorf("request %s %s", r.Method, r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm: %v", err)
		}
		form = r.PostForm
		if r.Header.Get("Authorization") != "" {
			t.Error("the token request carried an Authorization header; a PKCE client has no secret to send")
		}
		w.Header().Set("Content-Type", "application/json")
		writeBody(t, w, `{"access_token":"a1","refresh_token":"r1","expires_in":3600,"token_type":"Bearer"}`)
	})
	tokens, err := api.Exchange(t.Context(), "the-code", "the-verifier", "https://x/auth/spotify/callback")
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	want := url.Values{"grant_type": {"authorization_code"}, "code": {"the-code"},
		"redirect_uri": {"https://x/auth/spotify/callback"}, "client_id": {"client-id"}, "code_verifier": {"the-verifier"}}
	if form.Encode() != want.Encode() {
		t.Errorf("form = %v, want %v", form, want)
	}
	if tokens.Access.Reveal() != "a1" || tokens.Refresh.Reveal() != "r1" || tokens.ExpiresIn.Seconds() != 3600 {
		t.Errorf("tokens = %s %s %v", tokens.Access.Reveal(), tokens.Refresh.Reveal(), tokens.ExpiresIn)
	}
}

func TestRefreshTurnedDown(t *testing.T) {
	t.Parallel()

	api := fakeSpotify(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		writeBody(t, w, `{"error":"invalid_grant","error_description":"Refresh token revoked"}`)
	})
	if _, err := api.Refresh(t.Context(), redact.New("r1")); !errors.Is(err, ErrRejected) {
		t.Errorf("err = %v, want ErrRejected", err)
	}
}

func TestPlaybackState(t *testing.T) {
	t.Parallel()

	api := fakeSpotify(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer a1" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		writeBody(t, w, `{"device":{"id":"d1","name":"iPhone","type":"Smartphone","is_active":true},
			"progress_ms":61000,"is_playing":true,"currently_playing_type":"track",
			"item":{"id":"`+kazeTrack+`","name":"Shinunoga E-Wa","duration_ms":185000,"artists":[{"name":"Fujii Kaze"}]}}`)
	})
	got, active, err := api.PlaybackState(t.Context(), redact.New("a1"))
	if err != nil || !active {
		t.Fatalf("PlaybackState: %v, active %v", err, active)
	}
	want := State{Playing: true, ProgressMS: 61000,
		Track:  Track{ID: kazeTrack, Name: "Shinunoga E-Wa", Artists: []string{"Fujii Kaze"}, DurationMS: 185000},
		Device: Device{ID: "d1", Name: "iPhone", Type: "Smartphone", Active: true}}
	if got.Playing != want.Playing || got.ProgressMS != want.ProgressMS || got.Track.ID != want.Track.ID ||
		got.Track.Artists[0] != "Fujii Kaze" || got.Device != want.Device {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestPlaybackStateWithNothingPlaying(t *testing.T) {
	t.Parallel()

	api := fakeSpotify(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	if _, active, err := api.PlaybackState(t.Context(), redact.New("a1")); err != nil || active {
		t.Errorf("got active %v, %v; want nothing playing", active, err)
	}
}

func TestSearchTracks(t *testing.T) {
	t.Parallel()

	api := fakeSpotify(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		// market=from_token needs a scope the app doesn't ask for, so Spotify refuses the search with it.
		if r.URL.Path != "/v1/search" || q.Get("q") != "夜に駆ける YOASOBI" || q.Get("type") != "track" ||
			q.Has("market") {
			t.Errorf("search request %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		writeBody(t, w, `{"tracks":{"items":[{"id":"b","name":"夜に駆ける","duration_ms":261000,
			"artists":[{"name":"YOASOBI"}]}]}}`)
	})
	got, err := api.SearchTracks(t.Context(), redact.New("a1"), "夜に駆ける YOASOBI")
	if err != nil || len(got) != 1 || got[0].ID != "b" || got[0].Artists[0] != "YOASOBI" {
		t.Errorf("got %+v, %v", got, err)
	}
}

func TestPlaySendsTheTrackPositionAndDevice(t *testing.T) {
	t.Parallel()

	var body map[string]any
	api := fakeSpotify(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/v1/me/player/play" || r.URL.Query().Get("device_id") != "d1" {
			t.Errorf("request %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	if err := api.Play(t.Context(), redact.New("a1"), "d1", kazeTrack, 42000); err != nil {
		t.Fatalf("Play: %v", err)
	}
	uris, ok := body["uris"].([]any)
	if !ok || len(uris) != 1 || uris[0] != "spotify:track:"+kazeTrack || body["position_ms"] != float64(42000) {
		t.Errorf("body = %v", body)
	}
}

func TestRefusals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{name: "a token that has run out", status: 401, body: `{"error":{"status":401,"message":"expired"}}`,
			want: ErrRejected},
		{name: "too many requests", status: 429, want: ErrBusy},
		{name: "no active device", status: 404,
			body: `{"error":{"status":404,"message":"No active device found","reason":"NO_ACTIVE_DEVICE"}}`,
			want: ErrNoDevice},
		{name: "no Premium", status: 403,
			body: `{"error":{"status":403,"message":"Premium required","reason":"PREMIUM_REQUIRED"}}`, want: ErrPremium},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			api := fakeSpotify(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				writeBody(t, w, tt.body)
			})
			if err := api.Seek(t.Context(), redact.New("a1"), 1000); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestAnUnknownRefusalIsAnErrorOfItsOwn(t *testing.T) {
	t.Parallel()

	api := fakeSpotify(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		writeBody(t, w, "<html>bad gateway</html>")
	})
	err := api.Pause(t.Context(), redact.New("a1"))
	if err == nil || errors.Is(err, ErrRejected) || errors.Is(err, ErrNoDevice) {
		t.Errorf("err = %v, want a plain refusal", err)
	}
}
