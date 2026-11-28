package youtube

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/common/redact"
)

const testKey = "AIzaTestKeyTestKeyTestKeyTestKey123"

func fakeYouTube(t *testing.T, handle func(w http.ResponseWriter, r *http.Request)) *DataAPI {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Goog-Api-Key") != testKey {
			t.Error("the request didn't carry the API key in its header")
		}
		if strings.Contains(r.URL.RawQuery, testKey) {
			t.Error("the API key was in the URL, where a log could keep it")
		}
		handle(w, r)
	}))
	t.Cleanup(srv.Close)
	return NewDataAPI(srv.URL)
}

func writeBody(t *testing.T, w io.Writer, body string) {
	t.Helper()
	if _, err := io.WriteString(w, body); err != nil {
		t.Errorf("write: %v", err)
	}
}

func TestVideosSearchesThenReadsLengths(t *testing.T) {
	t.Parallel()

	api := fakeYouTube(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			q := r.URL.Query()
			if q.Get("q") != "Fujii Kaze Matsuri" || q.Get("type") != "video" || q.Get("videoEmbeddable") != "true" {
				t.Errorf("search query %v", q)
			}
			writeBody(t, w, `{"items":[{"id":{"videoId":"AAAAAAAAAAA"}},{"id":{"videoId":"BBBBBBBBBBB"}},`+
				`{"id":{"channelId":"UC123"}}]}`)
		case "/videos":
			if got := r.URL.Query().Get("id"); got != "AAAAAAAAAAA,BBBBBBBBBBB" {
				t.Errorf("videos asked for %q", got)
			}
			writeBody(t, w, `{"items":[`+
				`{"id":"BBBBBBBBBBB","snippet":{"title":"Fujii Kaze - &quot;Matsuri&quot; (Official Video)",`+
				`"channelTitle":"Fujii Kaze"},"contentDetails":{"duration":"PT4M9S"}},`+
				`{"id":"AAAAAAAAAAA","snippet":{"title":"Matsuri","channelTitle":"Fujii Kaze - Topic"},`+
				`"contentDetails":{"duration":"PT3M46S"}}]}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})
	got, err := api.Videos(t.Context(), redact.New(testKey), "Fujii Kaze Matsuri")
	if err != nil {
		t.Fatalf("Videos: %v", err)
	}
	want := []Video{
		{ID: "AAAAAAAAAAA", Title: "Matsuri", Channel: "Fujii Kaze - Topic", DurationMS: 226000},
		{ID: "BBBBBBBBBBB", Title: `Fujii Kaze - "Matsuri" (Official Video)`, Channel: "Fujii Kaze", DurationMS: 249000},
	}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("Videos = %+v, want %+v, in the search's order", got, want)
	}
}

func TestVideoLooksOneUp(t *testing.T) {
	t.Parallel()

	api := fakeYouTube(t, func(w http.ResponseWriter, _ *http.Request) { writeBody(t, w, `{"items":[]}`) })
	if _, err := api.Video(t.Context(), redact.New(testKey), "AAAAAAAAAAA"); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("a video YouTube doesn't have: %v, want not found", err)
	}
}

func TestQuotaSpent(t *testing.T) {
	t.Parallel()

	api := fakeYouTube(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		writeBody(t, w, `{"error":{"code":403,"errors":[{"reason":"quotaExceeded"}]}}`)
	})
	if _, err := api.Videos(t.Context(), redact.New(testKey), "x"); !errors.Is(err, ErrQuota) {
		t.Errorf("err = %v, want the quota spent", err)
	}
}

func TestARefusalNamesNoKey(t *testing.T) {
	t.Parallel()

	api := fakeYouTube(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		writeBody(t, w, `{"error":{"code":400,"errors":[{"reason":"keyInvalid"}]}}`)
	})
	_, err := api.Videos(t.Context(), redact.New(testKey), "x")
	if err == nil || strings.Contains(err.Error(), testKey) {
		t.Errorf("err = %v, want a refusal that doesn't name the key", err)
	}
}

func TestISOMillis(t *testing.T) {
	t.Parallel()

	tests := map[string]int64{"PT3M46S": 226000, "PT1H2M3S": 3723000, "PT45S": 45000, "PT4M": 240000, "P0D": 0, "": 0}
	for in, want := range tests {
		if got := isoMillis(in); got != want {
			t.Errorf("isoMillis(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestKeyRefusals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{name: "a key that isn't valid", status: 400,
			body: `{"error":{"code":400,"errors":[{"reason":"badRequest"}],` +
				`"details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"API_KEY_INVALID"}]}}`,
			want: ErrKeyRejected},
		{name: "a key restricted to other APIs", status: 403,
			body: `{"error":{"code":403,"details":[{"reason":"API_KEY_SERVICE_BLOCKED"}]}}`, want: ErrKeyRejected},
		{name: "a project without the YouTube Data API", status: 403,
			body: `{"error":{"code":403,"errors":[{"reason":"accessNotConfigured"}],` +
				`"details":[{"reason":"SERVICE_DISABLED"}]}}`, want: ErrAPIOff},
		{name: "a day's quota spent", status: 403, body: `{"error":{"errors":[{"reason":"quotaExceeded"}]}}`,
			want: ErrQuota},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			api := fakeYouTube(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				writeBody(t, w, tt.body)
			})
			if _, err := api.Video(t.Context(), redact.New(testKey), "AAAAAAAAAAA"); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}
