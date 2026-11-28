package songs

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
)

const amllIndex = `{"metadata":[["album",["夜に駆ける"]],["artists",["YOASOBI"]],["musicName",["夜に駆ける"]]],"rawLyricFile":"1689406537000-83578994-376644ed.ttml"}
{"metadata":[["artists",["稲葉曇","歌愛ユキ"]],["musicName",["ラグトレイン"]]],"rawLyricFile":"1689227417000-83578994-dbf4bcef.ttml"}
not json
{"metadata":[["musicName",["Escape"]]],"rawLyricFile":"../secret.ttml"}
{"metadata":[["artists",["No Title"]]],"rawLyricFile":"notitle.ttml"}
`

const amllSheet = `<tt xmlns="http://www.w3.org/ns/ttml" xmlns:ttm="http://www.w3.org/ns/ttml#metadata"><body>
<div><p begin="00:01.737" end="00:06.722"><span begin="00:01.737" end="00:02.175">沈</span><span begin="00:02.175" end="00:02.592">む</span><span ttm:role="x-translation">像是沉溺</span><span ttm:role="x-bg"><span begin="00:03.000" end="00:03.500">(oh)</span></span></p>
<p begin="1:02.5" end="1:04"><span begin="62.5s" end="63">Hello </span><span begin="1:03.250" end="1:04">world</span></p>
<p begin="00:10.000" end="00:11.000"><span ttm:role="x-roman">only romaji</span></p></div></body></tt>`

func amllServer(t *testing.T, hits *atomic.Int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got != "Utanabi test (https://github.com/JasonPKnoll/utanabi)" {
			t.Errorf("User-Agent = %q", got)
		}
		switch r.URL.Path {
		case "/metadata/raw-lyrics-index.jsonl":
			hits.Add(1)
			_, _ = w.Write([]byte(amllIndex))
		case "/raw-lyrics/1689406537000-83578994-376644ed.ttml":
			_, _ = w.Write([]byte(amllSheet))
		case "/raw-lyrics/busy.ttml":
			w.WriteHeader(http.StatusTooManyRequests)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestAMLLReadsItsIndexAndKeepsIt(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	src := NewAMLL(amllServer(t, &hits).URL, "test")
	now := time.Date(2026, 11, 28, 12, 0, 0, 0, time.UTC)
	src.now = func() time.Time { return now }

	got, err := src.Sheets(t.Context())
	if err != nil {
		t.Fatalf("Sheets: %v", err)
	}
	want := []SheetRef{
		{ID: "1689406537000-83578994-376644ed.ttml", Title: "夜に駆ける", Artist: "YOASOBI"},
		{ID: "1689227417000-83578994-dbf4bcef.ttml", Title: "ラグトレイン", Artist: "稲葉曇 歌愛ユキ"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Sheets = %+v, want %+v", got, want)
	}
	if _, err := src.Sheets(t.Context()); err != nil || hits.Load() != 1 {
		t.Errorf("second Sheets: %v after %d reads, want the kept copy", err, hits.Load())
	}
	now = now.Add(13 * time.Hour)
	if _, err := src.Sheets(t.Context()); err != nil || hits.Load() != 2 {
		t.Errorf("Sheets after 13 hours: %v after %d reads, want it read again", err, hits.Load())
	}
}

func TestAMLLReadsASheetsLeadVocal(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	src := NewAMLL(amllServer(t, &hits).URL, "test")
	got, err := src.Sheet(t.Context(), SheetRef{ID: "1689406537000-83578994-376644ed.ttml"})
	if err != nil {
		t.Fatalf("Sheet: %v", err)
	}
	want := TimedSheet{Lines: [][]TimedSpan{
		{{Text: "沈", StartMS: 1737}, {Text: "む", StartMS: 2175}},
		{{Text: "Hello ", StartMS: 62500}, {Text: "world", StartMS: 63250}},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Sheet = %+v, want %+v", got, want)
	}
}

func TestAMLLTurnsFailuresIntoTheProjectsErrors(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	src := NewAMLL(amllServer(t, &hits).URL, "test")
	if _, err := src.Sheet(t.Context(), SheetRef{ID: "missing.ttml"}); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("missing sheet: %v, want not found", err)
	}
	if _, err := src.Sheet(t.Context(), SheetRef{ID: "busy.ttml"}); !errors.Is(err, fault.ErrTimeout) {
		t.Errorf("busy: %v, want a timeout", err)
	}
	var invalid *fault.InvalidError
	if _, err := src.Sheet(t.Context(), SheetRef{ID: "../secret.ttml"}); !errors.As(err, &invalid) {
		t.Errorf("a path outside the database: %v, want invalid", err)
	}
}

func TestTTMLTime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want int64
		ok   bool
	}{
		{in: "00:01.737", want: 1737, ok: true},
		{in: "1:02:03.5", want: 3723500, ok: true},
		{in: "62.345", want: 62345, ok: true},
		{in: "62.345s", want: 62345, ok: true},
		{in: "-1", ok: false},
		{in: "soon", ok: false},
	}
	for _, tt := range tests {
		got, ok := ttmlTime(tt.in)
		if got != tt.want || ok != tt.ok {
			t.Errorf("ttmlTime(%q) = %d, %v; want %d, %v", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}
