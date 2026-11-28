package songs

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
)

// sheetLine is a sheet line timed one character at a time from start, step apart.
func sheetLine(text string, start, step int64) []TimedSpan {
	var spans []TimedSpan
	for i, r := range []rune(text) {
		spans = append(spans, TimedSpan{Text: string(r), StartMS: start + int64(i)*step})
	}
	return spans
}

func TestLineUpSheetMovesItsTimesOntoTheLyrics(t *testing.T) {
	t.Parallel()

	// The sheet's recording starts a second later than the lyrics' one; its lines leave out the punctuation.
	lines := []Line{
		{Position: 0, Text: "夜に駆ける、", StartMS: ms(5000)},
		{Position: 1, Text: "君と", StartMS: ms(8000)},
	}
	sheet := TimedSheet{Lines: [][]TimedSpan{sheetLine("夜に駆ける", 6000, 300), sheetLine("君と", 9100, 400)}}
	got := lineUpSheet(lines, sheet)
	// The lines' differences are -1000 and -1100; the upper middle one, -1000, moves every time.
	want := []LineTimes{
		{Line: 0, Times: []WordTime{{0, 5000}, {1, 5300}, {2, 5600}, {3, 5900}, {4, 6200}}},
		{Line: 1, Times: []WordTime{{0, 8100}, {1, 8500}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("lineUpSheet = %+v, want %+v", got, want)
	}
}

func TestLineUpSheetTimesAWordSpanAtItsFirstCharacter(t *testing.T) {
	t.Parallel()

	lines := []Line{{Position: 3, Text: "Hello world", StartMS: ms(1000)}}
	sheet := TimedSheet{Lines: [][]TimedSpan{{{Text: "Hello ", StartMS: 1000}, {Text: "world", StartMS: 1600}}}}
	got := lineUpSheet(lines, sheet)
	want := []LineTimes{{Line: 3, Times: []WordTime{{0, 1000}, {6, 1600}}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("lineUpSheet = %+v, want %+v", got, want)
	}
}

func TestLineUpSheetSkipsLinesItDoesntSpellOut(t *testing.T) {
	t.Parallel()

	lines := []Line{
		{Position: 0, Text: "あいう", StartMS: ms(1000)},
		{Position: 1, Text: "(Instrumental)", StartMS: ms(2000)},
		{Position: 2, Text: "かきく", StartMS: ms(3000)},
	}
	sheet := TimedSheet{Lines: [][]TimedSpan{sheetLine("あいう", 1000, 100), sheetLine("かきく", 3000, 100)}}
	got := lineUpSheet(lines, sheet)
	if len(got) != 2 || got[0].Line != 0 || got[1].Line != 2 {
		t.Errorf("lineUpSheet = %+v, want lines 0 and 2 timed", got)
	}
}

func TestLineUpSheetNeedsLineTimes(t *testing.T) {
	t.Parallel()

	lines := []Line{{Position: 0, Text: "あいう"}}
	sheet := TimedSheet{Lines: [][]TimedSpan{sheetLine("あいう", 1000, 100)}}
	if got := lineUpSheet(lines, sheet); got != nil {
		t.Errorf("lineUpSheet = %+v, want none without line times to line up with", got)
	}
}

type fakeTimings struct {
	refs   []SheetRef
	sheets map[string]TimedSheet
	read   []string
	err    error
}

func (f *fakeTimings) Code() string { return "amll" }
func (f *fakeTimings) Name() string { return "AMLL TTML DB" }

func (f *fakeTimings) Sheets(context.Context) ([]SheetRef, error) { return f.refs, f.err }

func (f *fakeTimings) Sheet(_ context.Context, ref SheetRef) (TimedSheet, error) {
	f.read = append(f.read, ref.ID)
	return f.sheets[ref.ID], nil
}

// timedSong is a synced song of n lines, each three different kana, a second apart.
func timedSong(n int) (Song, TimedSheet) {
	song := Song{ID: 1, Title: "夜に駆ける", Artist: "YOASOBI"}
	var sheet TimedSheet
	for i := range n {
		text := string([]rune{rune('あ' + 3*i), rune('い' + 3*i), rune('う' + 3*i)})
		song.Lines = append(song.Lines, Line{Position: i, Text: text, StartMS: ms(int64(i) * 1000)})
		sheet.Lines = append(sheet.Lines, sheetLine(text, int64(i)*1000, 200))
	}
	return song, sheet
}

func TestFindWordTimesSavesTheSheetThatSpellsTheSongOut(t *testing.T) {
	t.Parallel()

	song, sheet := timedSong(8)
	repo := newFakeRepo()
	repo.songs[1] = song
	src := &fakeTimings{
		refs: []SheetRef{
			{ID: "other.ttml", Title: "群青", Artist: "YOASOBI"},
			{ID: "yoru.ttml", Title: "夜に駆ける", Artist: "YOASOBI"},
			{ID: "unrelated.ttml", Title: "うっせぇわ", Artist: "Ado"},
		},
		sheets: map[string]TimedSheet{"yoru.ttml": sheet, "other.ttml": {}},
	}
	got, err := FindWordTimes(t.Context(), repo, fakeAnalyzer{}, []TimingSource{src}, 1)
	if err != nil {
		t.Fatalf("FindWordTimes: %v", err)
	}
	if want := (FoundTimes{Lines: 8, Of: 8, Source: "AMLL TTML DB"}); got != want {
		t.Errorf("FindWordTimes = %+v, want %+v", got, want)
	}
	// The sheet with the title and the artist is read first; one by the artist alone is a candidate too.
	if want := []string{"yoru.ttml", "other.ttml"}; !reflect.DeepEqual(src.read, want) {
		t.Errorf("sheets read = %v, want %v", src.read, want)
	}
	if len(repo.timeSaves) != 1 || repo.timeSaves[0].source != "amll" || len(repo.timeSaves[0].lines) != 8 {
		t.Fatalf("saved = %+v, want 8 lines from amll", repo.timeSaves)
	}
	if first := repo.timeSaves[0].lines[0].Times; !reflect.DeepEqual(first, []WordTime{{0, 0}, {1, 200}, {2, 400}}) {
		t.Errorf("first line's times = %+v", first)
	}
}

func TestFindWordTimesSavesNothingForASheetThatSpellsTooLittle(t *testing.T) {
	t.Parallel()

	song, sheet := timedSong(10)
	sheet.Lines = sheet.Lines[:5]
	repo := newFakeRepo()
	repo.songs[1] = song
	src := &fakeTimings{refs: []SheetRef{{ID: "yoru.ttml", Title: "夜に駆ける", Artist: "YOASOBI"}},
		sheets: map[string]TimedSheet{"yoru.ttml": sheet}}
	got, err := FindWordTimes(t.Context(), repo, fakeAnalyzer{}, []TimingSource{src}, 1)
	if err != nil {
		t.Fatalf("FindWordTimes: %v", err)
	}
	if got.Lines != 0 || len(repo.timeSaves) != 0 {
		t.Errorf("FindWordTimes = %+v with %d saves, want nothing for 5 of 10 lines", got, len(repo.timeSaves))
	}
}

func TestFindWordTimesFindsNothingWhenNoSheetIsTheSong(t *testing.T) {
	t.Parallel()

	song, _ := timedSong(8)
	repo := newFakeRepo()
	repo.songs[1] = song
	src := &fakeTimings{refs: []SheetRef{{ID: "a.ttml", Title: "群青", Artist: "Ado"}}}
	got, err := FindWordTimes(t.Context(), repo, fakeAnalyzer{}, []TimingSource{src}, 1)
	if err != nil {
		t.Fatalf("FindWordTimes: %v", err)
	}
	if got != (FoundTimes{Of: 8}) || len(src.read) != 0 {
		t.Errorf("FindWordTimes = %+v, read %v; want nothing found and nothing read", got, src.read)
	}
}

func TestFindWordTimesRefusesLyricsWithoutLineTimes(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	repo.songs[1] = Song{ID: 1, Title: "歌", Lines: []Line{{Position: 0, Text: "あいう"}}}
	_, err := FindWordTimes(t.Context(), repo, fakeAnalyzer{}, []TimingSource{&fakeTimings{}}, 1)
	var invalid *fault.InvalidError
	if !errors.As(err, &invalid) {
		t.Errorf("err = %v, want invalid", err)
	}
}

func TestFindWordTimesPassesOnASourcesFailure(t *testing.T) {
	t.Parallel()

	song, _ := timedSong(8)
	repo := newFakeRepo()
	repo.songs[1] = song
	failure := errors.New("unreachable")
	_, err := FindWordTimes(t.Context(), repo, fakeAnalyzer{}, []TimingSource{&fakeTimings{err: failure}}, 1)
	if !errors.Is(err, failure) {
		t.Errorf("err = %v, want the source's failure", err)
	}
}

func TestSetLineTimesSavesYourTimes(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	repo.songs[1] = Song{ID: 1, Lines: []Line{{Position: 0, Text: "君の声を", StartMS: ms(1000)}}}
	times := []WordTime{{0, 1000}, {2, 1500}, {3, 1500}}
	got, err := SetLineTimes(t.Context(), repo, LineTimesChange{SongID: 1, Line: 0, Times: times})
	if err != nil {
		t.Fatalf("SetLineTimes: %v", err)
	}
	if !reflect.DeepEqual(got, times) {
		t.Errorf("SetLineTimes = %+v, want %+v", got, times)
	}
	if len(repo.timeSaves) != 1 || repo.timeSaves[0].source != TimesByUser {
		t.Errorf("saved = %+v, want one save as yours", repo.timeSaves)
	}

	if got, err := SetLineTimes(t.Context(), repo, LineTimesChange{SongID: 1, Line: 0}); err != nil || got == nil {
		t.Errorf("removing = %v, %v; want an empty list", got, err)
	}
}

func TestSetLineTimesRefusesBadTimes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		change LineTimesChange
	}{
		{name: "no song", change: LineTimesChange{SongID: 0}},
		{name: "a negative line", change: LineTimesChange{SongID: 1, Line: -1}},
		{name: "past the line's end", change: LineTimesChange{SongID: 1, Times: []WordTime{{4, 1000}}}},
		{name: "a negative character", change: LineTimesChange{SongID: 1, Times: []WordTime{{-1, 1000}}}},
		{name: "a negative time", change: LineTimesChange{SongID: 1, Times: []WordTime{{0, -1}}}},
		{name: "past the longest song", change: LineTimesChange{SongID: 1, Times: []WordTime{{0, maxTimeMS + 1}}}},
		{name: "out of order", change: LineTimesChange{SongID: 1, Times: []WordTime{{2, 1000}, {1, 1200}}}},
		{name: "the same character twice", change: LineTimesChange{SongID: 1, Times: []WordTime{{1, 1000}, {1, 1200}}}},
		{name: "back in time", change: LineTimesChange{SongID: 1, Times: []WordTime{{0, 1200}, {1, 1000}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := newFakeRepo()
			repo.songs[1] = Song{ID: 1, Lines: []Line{{Position: 0, Text: "君の声を"}}}
			_, err := SetLineTimes(t.Context(), repo, tt.change)
			var invalid *fault.InvalidError
			if !errors.As(err, &invalid) {
				t.Errorf("err = %v, want invalid", err)
			}
			if len(repo.timeSaves) != 0 {
				t.Errorf("saved %+v, want nothing", repo.timeSaves)
			}
		})
	}
}

func TestSetLineTimesOnALineTheSongHasntIsNotFound(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	repo.songs[1] = Song{ID: 1, Lines: []Line{{Position: 0, Text: "君の声を"}}}
	_, err := SetLineTimes(t.Context(), repo, LineTimesChange{SongID: 1, Line: 5})
	if !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("err = %v, want not found", err)
	}
}

func TestLikelySheetsMatchesTitlesAcrossScripts(t *testing.T) {
	t.Parallel()

	// The fake analyzer reads every text as one noun with no reading, so romaji is the text itself: enough to check
	// that a romaji title matches a sheet whose title reads the same.
	song := Song{Title: "Fujii Kaze - Matsuri (Official Video)", Artist: "Fujii Kaze"}
	var refs []SheetRef
	for i := range 10 {
		refs = append(refs, SheetRef{ID: strconv.Itoa(i) + ".ttml", Title: "Song " + strconv.Itoa(i), Artist: "Someone"})
	}
	refs = append(refs, SheetRef{ID: "matsuri.ttml", Title: "Matsuri", Artist: "藤井 風"},
		SheetRef{ID: "kaze.ttml", Title: "Kirari", Artist: "Fujii Kaze"})
	got, err := likelySheets(t.Context(), fakeAnalyzer{}, song, refs)
	if err != nil {
		t.Fatalf("likelySheets: %v", err)
	}
	ids := make([]string, len(got))
	for i, r := range got {
		ids[i] = r.ID
	}
	if want := []string{"matsuri.ttml", "kaze.ttml"}; !reflect.DeepEqual(ids, want) {
		t.Errorf("likelySheets = %v, want %v", ids, want)
	}
}

func TestNamesMatchShortOnesWhole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		outer, inner string
		want         bool
	}{
		{outer: "beabadoobee", inner: "Ado", want: false},
		{outer: "Wasureranneyo", inner: "Era", want: false},
		{outer: "Ado", inner: "Ado", want: true},
		{outer: "Fujii Kaze - Matsuri (Official Video)", inner: "Matsuri", want: true},
		{outer: "東京事変", inner: "事変", want: true},
		{outer: "夜に駆ける", inner: "夜", want: false},
	}
	for _, tt := range tests {
		outer, err := nameOf(t.Context(), fakeAnalyzer{}, tt.outer)
		if err != nil {
			t.Fatal(err)
		}
		inner, err := nameOf(t.Context(), fakeAnalyzer{}, tt.inner)
		if err != nil {
			t.Fatal(err)
		}
		if got := outer.has(inner); got != tt.want {
			t.Errorf("%q has %q = %v, want %v", tt.outer, tt.inner, got, tt.want)
		}
	}
}
