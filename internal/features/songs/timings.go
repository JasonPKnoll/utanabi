package songs

import (
	"context"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/common/jtext"
	"github.com/JasonPKnoll/utanabi/internal/common/morph"
)

// TimesByUser marks word times you set by tapping along with the song.
const TimesByUser = "user"

// The longest a song's word times may run, as the database allows.
const maxTimeMS = 3600000

// WordTime is when a word of a line starts: At is the character of the line's text it starts at, counted in Unicode
// code points, and MS the time, in the lyrics' own time, before the offset onto the track.
type WordTime struct {
	At int   `json:"at"`
	MS int64 `json:"ms"`
}

// LineTimes is the word times of one line, by its position.
type LineTimes struct {
	Line  int
	Times []WordTime
}

// LineTimesChange is the word times you tapped for one line of a song. No times removes the line's.
type LineTimesChange struct {
	SongID int64      `json:"songId"`
	Line   int        `json:"line"`
	Times  []WordTime `json:"times"`
}

// FoundTimes says how many of a song's lines got word times from a timing source, and from which; none when no
// source has the song.
type FoundTimes struct {
	Lines  int    `json:"lines"`
	Of     int    `json:"of"`
	Source string `json:"source"`
}

// SheetRef is one song in a timing source's index.
type SheetRef struct {
	ID     string
	Title  string
	Artist string
}

// TimedSheet is a song timed word by word: its sung lines, each a list of spans with the time each starts, in the
// source's own recording's time.
type TimedSheet struct {
	Lines [][]TimedSpan
}

// TimedSpan is a run of a line's text, usually one character or word, and when it starts.
type TimedSpan struct {
	Text    string
	StartMS int64
}

// TimingSource is a database of songs timed word by word: Sheets lists what it has, and Sheet reads one.
type TimingSource interface {
	Code() string
	Name() string
	Sheets(ctx context.Context) ([]SheetRef, error)
	Sheet(ctx context.Context, ref SheetRef) (TimedSheet, error)
}

// How a sheet is taken to be the song's: at most this many of the source's songs are read to check, and a sheet must
// spell out this many of the song's lines, and this share of them.
const (
	maxSheetsTried = 6
	minLinesTimed  = 6
	minShareTimed  = 0.6
)

// SetLineTimes saves the word times you tapped for one line, replacing the line's, or removes them when there are
// none. Each starts at a character of the line, in order, no earlier than the one before.
func SetLineTimes(ctx context.Context, repo Repository, change LineTimesChange) ([]WordTime, error) {
	if change.SongID <= 0 {
		return nil, fault.Invalid("songId", "must be positive")
	}
	if change.Line < 0 {
		return nil, fault.Invalid("line", "must not be negative")
	}
	song, err := repo.SongByID(ctx, change.SongID)
	if err != nil {
		return nil, err
	}
	i := slices.IndexFunc(song.Lines, func(l Line) bool { return l.Position == change.Line })
	if i < 0 {
		return nil, fault.ErrNotFound
	}
	length := utf8.RuneCountInString(song.Lines[i].Text)
	for j, t := range change.Times {
		switch {
		case t.At < 0 || t.At >= length:
			return nil, fault.Invalid("times", "must start at characters of the line")
		case t.MS < 0 || t.MS > maxTimeMS:
			return nil, fault.Invalid("times", "must be within the song")
		case j > 0 && t.At <= change.Times[j-1].At:
			return nil, fault.Invalid("times", "must be in the line's order, one for each word")
		case j > 0 && t.MS < change.Times[j-1].MS:
			return nil, fault.Invalid("times", "must not go back in time")
		}
	}
	times := slices.Clone(change.Times)
	if times == nil {
		times = []WordTime{}
	}
	if err := repo.SetWordTimes(ctx, change.SongID, TimesByUser, []LineTimes{{Line: change.Line, Times: times}}); err != nil {
		return nil, err
	}
	return times, nil
}

// ClearWordTimes removes every word time of a song, yours and found ones, so its words are guessed from its lines
// again.
func ClearWordTimes(ctx context.Context, repo Repository, songID int64) error {
	if songID <= 0 {
		return fault.Invalid("songId", "must be positive")
	}
	return repo.ClearWordTimes(ctx, songID)
}

// FindWordTimes looks the song up in the timing sources, in order, and saves the word times of the first sheet that
// spells out its lyrics, lined up with its line times. Lines you timed yourself keep yours. A song no source has
// gets none, and that's no error.
func FindWordTimes(
	ctx context.Context, repo Repository, analyzer Analyzer, sources []TimingSource, songID int64,
) (FoundTimes, error) {
	if songID <= 0 {
		return FoundTimes{}, fault.Invalid("songId", "must be positive")
	}
	song, err := repo.SongByID(ctx, songID)
	if err != nil {
		return FoundTimes{}, err
	}
	if !slices.ContainsFunc(song.Lines, func(l Line) bool { return l.StartMS != nil }) {
		return FoundTimes{}, fault.Invalid("song", "has no line times to line word times up with")
	}
	found := FoundTimes{Of: len(song.Lines)}
	for _, src := range sources {
		lines, err := findIn(ctx, analyzer, src, song)
		if err != nil {
			return FoundTimes{}, err
		}
		if len(lines) == 0 {
			continue
		}
		if err := repo.SetWordTimes(ctx, songID, src.Code(), lines); err != nil {
			return FoundTimes{}, err
		}
		found.Lines, found.Source = len(lines), src.Name()
		return found, nil
	}
	return found, nil
}

// findIn returns the song's word times from the source's sheet that spells out most of its lines, or none.
func findIn(ctx context.Context, analyzer Analyzer, src TimingSource, song Song) ([]LineTimes, error) {
	refs, err := src.Sheets(ctx)
	if err != nil {
		return nil, err
	}
	candidates, err := likelySheets(ctx, analyzer, song, refs)
	if err != nil {
		return nil, err
	}
	sung := 0
	for _, l := range song.Lines {
		if lineKey(l.Text) != "" {
			sung++
		}
	}
	var best []LineTimes
	for _, ref := range candidates {
		sheet, err := src.Sheet(ctx, ref)
		if err != nil {
			return nil, err
		}
		lines := lineUpSheet(song.Lines, sheet)
		if len(lines) > len(best) {
			best = lines
		}
	}
	if len(best) < minLinesTimed || float64(len(best)) < minShareTimed*float64(sung) {
		return nil, nil
	}
	return best, nil
}

// name is a title or an artist's name in the two forms names are compared in: as written, and in romaji.
type name struct{ written, latin string }

func isJapanese(r rune) bool {
	return unicode.In(r, unicode.Hiragana, unicode.Katakana, unicode.Han)
}

func nameOf(ctx context.Context, analyzer Analyzer, s string) (name, error) {
	tokens, err := analyzer.Analyze(ctx, jtext.Normalize(s))
	if err != nil {
		return name{}, err
	}
	in := make([]jtext.RomajiToken, len(tokens))
	for i, t := range tokens {
		in[i] = jtext.RomajiToken{Surface: t.Surface, Reading: t.Reading, Particle: t.POS == morph.Particle}
	}
	return name{written: jtext.Plain(s), latin: jtext.FoldRomaji(jtext.RomajiLine(in))}, nil
}

// has reports whether outer has inner in it, as written or in romaji. A short name must match whole: 愛 is ai, and Ado
// is in beabadoobee, which too many names have in them. Two characters of kana or kanji are name enough.
func (outer name) has(inner name) bool {
	short := utf8.RuneCountInString(inner.written) < 2 ||
		(utf8.RuneCountInString(inner.written) < 4 && !strings.ContainsFunc(inner.written, isJapanese))
	if inner.written != "" && (outer.written == inner.written || !short && strings.Contains(outer.written, inner.written)) {
		return true
	}
	if utf8.RuneCountInString(inner.latin) < 4 {
		return inner.latin != "" && outer.latin == inner.latin
	}
	return strings.Contains(outer.latin, inner.latin)
}

// likelySheets picks the source's songs worth reading to check against the song: those with its title and artist
// first, then its title, then its artist, at most maxSheetsTried. A title from a lyrics source often has the artist
// and more in it ("Fujii Kaze - Shinunoga E-Wa (Official Video)"), so a sheet's title only has to be in it.
func likelySheets(ctx context.Context, analyzer Analyzer, song Song, refs []SheetRef) ([]SheetRef, error) {
	title, err := nameOf(ctx, analyzer, song.Title)
	if err != nil {
		return nil, err
	}
	artist, err := nameOf(ctx, analyzer, song.Artist)
	if err != nil {
		return nil, err
	}
	var both, titled, byArtist []SheetRef
	for _, ref := range refs {
		// Romaji costs an analysis, so a sheet is read in romaji only when its written forms don't already settle it.
		t, a := name{written: jtext.Plain(ref.Title)}, name{written: jtext.Plain(ref.Artist)}
		titleOK, artistOK := title.has(t), artist.written != "" && (artist.has(a) || a.has(artist))
		if !titleOK || !artistOK {
			if t, err = nameOf(ctx, analyzer, ref.Title); err != nil {
				return nil, err
			}
			if a, err = nameOf(ctx, analyzer, ref.Artist); err != nil {
				return nil, err
			}
			titleOK = titleOK || title.has(t)
			artistOK = artistOK || (artist.latin != "" && (artist.has(a) || a.has(artist)))
		}
		switch {
		case titleOK && artistOK:
			both = append(both, ref)
		case titleOK:
			titled = append(titled, ref)
		case artistOK:
			byArtist = append(byArtist, ref)
		}
	}
	picked := slices.Concat(both, titled, byArtist)
	return picked[:min(len(picked), maxSheetsTried)], nil
}

// keyed is a line's text for lining up: its letters and digits only, each with the character of the text it came
// from, and for a sheet the time its span starts at, on the span's first one (-1 on the rest).
type keyed struct {
	runes []rune
	from  []int
	at    []int64
}

func keyOf(text string) keyed {
	var k keyed
	for i, r := range []rune(text) {
		for _, p := range jtext.Plain(string(r)) {
			k.runes, k.from, k.at = append(k.runes, p), append(k.from, i), append(k.at, -1)
		}
	}
	return k
}

func lineKey(text string) string { return string(keyOf(text).runes) }

func sheetKey(spans []TimedSpan) keyed {
	var k keyed
	for _, s := range spans {
		first := true
		for _, r := range s.Text {
			for _, p := range jtext.Plain(string(r)) {
				at := int64(-1)
				if first {
					at, first = s.StartMS, false
				}
				k.runes, k.at = append(k.runes, p), append(k.at, at)
			}
		}
	}
	return k
}

// sheetLookahead is how many sheet lines past the last match a song line may match: a sheet and the lyrics can each
// have a line the other leaves out, such as a spoken intro.
const sheetLookahead = 6

// lineUpSheet gives the song's lines the word times of the sheet's lines that spell them out exactly, letters and
// digits compared, in order. The sheet is timed for its own recording, so its times are moved onto the lyrics' by
// the middle difference between the two's line starts.
func lineUpSheet(lines []Line, sheet TimedSheet) []LineTimes {
	keys := make([]keyed, len(sheet.Lines))
	for j, spans := range sheet.Lines {
		keys[j] = sheetKey(spans)
	}
	type pair struct {
		line int
		key  keyed
		mine keyed
	}
	var pairs []pair
	next := 0
	for i, l := range lines {
		mine := keyOf(l.Text)
		if len(mine.runes) == 0 {
			continue
		}
		for j := next; j < min(len(keys), next+sheetLookahead); j++ {
			if slices.Equal(keys[j].runes, mine.runes) {
				pairs = append(pairs, pair{line: i, key: keys[j], mine: mine})
				next = j + 1
				break
			}
		}
	}

	var diffs []int64
	for _, p := range pairs {
		if start := lines[p.line].StartMS; start != nil && p.key.at[0] >= 0 {
			diffs = append(diffs, *start-p.key.at[0])
		}
	}
	if len(diffs) == 0 {
		return nil
	}
	slices.Sort(diffs)
	shift := diffs[len(diffs)/2]

	out := make([]LineTimes, 0, len(pairs))
	for _, p := range pairs {
		var times []WordTime
		for k, at := range p.key.at {
			if at < 0 {
				continue
			}
			t := WordTime{At: p.mine.from[k], MS: min(max(at+shift, 0), maxTimeMS)}
			if n := len(times); n > 0 && (t.At <= times[n-1].At || t.MS < times[n-1].MS) {
				continue
			}
			times = append(times, t)
		}
		if len(times) > 0 {
			out = append(out, LineTimes{Line: lines[p.line].Position, Times: times})
		}
	}
	return out
}
