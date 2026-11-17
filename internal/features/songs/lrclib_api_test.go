package songs

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
)

const lrclibSearchBody = `[
  {"id": 11, "trackName": "星の歌", "artistName": "Demo Artist", "albumName": "夜", "duration": 215.5,
   "instrumental": false, "plainLyrics": "夜の空に\n星が光る", "syncedLyrics": "[00:12.00]夜の空に\n[00:15.50]星が光る"},
  {"id": 12, "trackName": "星の歌 (Live)", "artistName": "Demo Artist", "albumName": "", "duration": 230,
   "instrumental": false, "plainLyrics": "夜の空に", "syncedLyrics": null},
  {"id": 13, "trackName": "Interlude", "artistName": "Demo Artist", "albumName": "", "duration": 60,
   "instrumental": true, "plainLyrics": null, "syncedLyrics": null}
]`

func lrclibServer(t *testing.T, handler http.HandlerFunc) *LRCLIB {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewLRCLIB(srv.URL, "1.2.3")
}

func TestLRCLIBSearch(t *testing.T) {
	t.Parallel()

	var gotPath, gotQuery, gotAgent string
	l := lrclibServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery, gotAgent = r.URL.Path, r.URL.Query().Get("q"), r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(lrclibSearchBody))
		if err != nil {
			t.Errorf("write response: %v", err)
		}
	})

	got, err := l.Search(t.Context(), "星の歌 & more")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if gotPath != "/api/search" || gotQuery != "星の歌 & more" {
		t.Errorf("request = %s?q=%q, want /api/search with the query", gotPath, gotQuery)
	}
	if want := "Utanabi 1.2.3 (https://github.com/JasonPKnoll/utanabi)"; gotAgent != want {
		t.Errorf("User-Agent = %q, want %q", gotAgent, want)
	}
	want := []Match{
		{Source: "lrclib", SourceName: "LRCLIB", SourceRef: "11", Title: "星の歌", Artist: "Demo Artist", Album: "夜",
			DurationMS: 215500, Synced: true},
		{Source: "lrclib", SourceName: "LRCLIB", SourceRef: "12", Title: "星の歌 (Live)", Artist: "Demo Artist",
			DurationMS: 230000},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("matches = %+v, want %+v (instrumental left out)", got, want)
	}
}

func TestLRCLIBLyrics(t *testing.T) {
	t.Parallel()

	var gotPath string
	l := lrclibServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, err := w.Write([]byte(`{"id": 11, "trackName": "星の歌", "artistName": "Demo Artist", "duration": 215,
			"instrumental": false, "plainLyrics": "夜の空に", "syncedLyrics": "[00:12.00]夜の空に"}`))
		if err != nil {
			t.Errorf("write response: %v", err)
		}
	})

	got, err := l.Lyrics(t.Context(), "11")
	if err != nil {
		t.Fatalf("Lyrics: %v", err)
	}
	if gotPath != "/api/get/11" {
		t.Errorf("path = %s, want /api/get/11", gotPath)
	}
	if got.Title != "星の歌" || got.Plain != "夜の空に" || got.Synced != "[00:12.00]夜の空に" || got.DurationMS != 215000 {
		t.Errorf("fetched = %+v, want the song with both kinds of lyrics", got)
	}
}

func TestLRCLIBLyricsRejectsABadRefWithoutARequest(t *testing.T) {
	t.Parallel()

	called := false
	l := lrclibServer(t, func(http.ResponseWriter, *http.Request) { called = true })

	_, err := l.Lyrics(t.Context(), "../admin")
	var invalid *fault.InvalidError
	if !errors.As(err, &invalid) || invalid.Field != "ref" {
		t.Errorf("err = %v, want invalid ref", err)
	}
	if called {
		t.Error("sent a request for a bad ref")
	}
}

func TestLRCLIBErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		status  int
		body    string
		wantErr error
		wantMsg string
	}{
		{name: "not found", status: http.StatusNotFound, body: `{"statusCode":404}`, wantErr: fault.ErrNotFound},
		{name: "rate limited", status: http.StatusTooManyRequests, wantErr: fault.ErrTimeout, wantMsg: "busy"},
		{name: "overloaded", status: http.StatusServiceUnavailable, wantErr: fault.ErrTimeout, wantMsg: "busy"},
		{name: "server error", status: http.StatusInternalServerError, wantMsg: "status 500"},
		{name: "not JSON", status: http.StatusOK, body: "<html>", wantMsg: "unexpected response"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			l := lrclibServer(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				if _, err := w.Write([]byte(tt.body)); err != nil {
					t.Errorf("write response: %v", err)
				}
			})
			_, err := l.Search(t.Context(), "星")

			if err == nil {
				t.Fatal("Search succeeded, want an error")
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantMsg != "" && !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("err = %v, want it to mention %q", err, tt.wantMsg)
			}
		})
	}
}

func TestLRCLIBTimesOut(t *testing.T) {
	t.Parallel()

	l := lrclibServer(t, func(_ http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	if _, err := l.Search(ctx, "星"); !errors.Is(err, fault.ErrTimeout) {
		t.Errorf("err = %v, want fault.ErrTimeout", err)
	}
}

func TestLRCLIBCanceled(t *testing.T) {
	t.Parallel()

	l := lrclibServer(t, func(_ http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := l.Search(ctx, "星"); !errors.Is(err, fault.ErrCanceled) {
		t.Errorf("err = %v, want fault.ErrCanceled", err)
	}
}
