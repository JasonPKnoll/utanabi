// Package songs keeps the songs the user is learning, with their lyrics as lines.
package songs

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/common/jtext"
	"github.com/JasonPKnoll/utanabi/internal/common/morph"
)

// SourceUser marks lyrics the user typed in themselves.
const SourceUser = "user"

const (
	maxTitleLen  = 200
	maxArtistLen = 200
	maxLyricsLen = 20000
	maxLineLen   = 500
)

// Line is one sung line. Stanza goes up by one at each verse break. StartMS is when the line starts, in milliseconds
// from the start of the song, and is set only for synced lyrics.
type Line struct {
	Position int    `json:"position"`
	Stanza   int    `json:"stanza"`
	Text     string `json:"text"`
	StartMS  *int64 `json:"startMs"`
	// Ruby is the line in pieces, with furigana over the kanji, and Romaji is the line in Latin letters. Both are
	// worked out from Tokens and never stored. A line without tokens is one piece with no reading, and no romaji.
	Ruby   []RubyPart `json:"ruby"`
	Romaji string     `json:"romaji"`
	// Words are the line's learnable words, in order, and Pieces the whole line cut at their edges, so each word can
	// be shown, with its furigana, as one thing to tap. Gloss is the direct, word-by-word translation, such as
	// "night · 's · sky · in · star · (subject) · shine". All three are worked out when the song is read, never stored.
	Words  []Word      `json:"words"`
	Pieces []LinePiece `json:"pieces"`
	Gloss  string      `json:"gloss"`
	// Tokens are the line's stored analysis. Nil means the line hasn't been analysed, or its tokens didn't fit it.
	Tokens []Token `json:"-"`
	// Analysis is the line's stored words. Nil means its words haven't been found yet.
	Analysis []WordAnalysis `json:"-"`
}

// LinePiece is a run of a line: one word, or text that belongs to no word, such as punctuation. Word is the index of
// the word in Line.Words, or -1.
type LinePiece struct {
	Ruby []RubyPart `json:"ruby"`
	Word int        `json:"word"`
}

// Word is one learnable word of a line, as the song view shows it. It covers the line's tokens FirstToken to
// LastToken. Lemma is its dictionary form, Gloss the short meaning it has in this line ("to search for"), Short its
// part of the line's word-by-word gloss, shown under the word ("search for", "'s", "(subject)"), and Form a plain
// sentence about how it is conjugated ("探す, in the 〜ている form"). Grammar is what a particle or set phrase does
// here. EntryID is 0, and SenseIndex null, when the dictionary doesn't have the word or isn't installed.
type Word struct {
	FirstToken   int            `json:"firstToken"`
	LastToken    int            `json:"lastToken"`
	Text         string         `json:"text"`
	Lemma        string         `json:"lemma"`
	LemmaReading string         `json:"lemmaReading"`
	LemmaRomaji  string         `json:"lemmaRomaji"`
	Kind         WordKind       `json:"kind"`
	ConjClass    string         `json:"conjClass"`
	EntryID      int64          `json:"entryId"`
	SenseIndex   *int           `json:"senseIndex"`
	Gloss        string         `json:"gloss"`
	Short        string         `json:"short"`
	Grammar      *GrammarPoint  `json:"grammar"`
	Steps        []GrammarPoint `json:"steps"`
	Form         string         `json:"form"`
}

// GrammarPoint is one use of a particle, one conjugation, or one pattern, from the app's grammar list.
type GrammarPoint struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Pattern     string `json:"pattern"`
	Title       string `json:"title"`
	Meaning     string `json:"meaning"`
	Explanation string `json:"explanation"`
}

// RubyPart is a piece of a line, with the furigana that goes over it. Reading is empty for kana and anything else
// that needs none.
type RubyPart struct {
	Text    string `json:"text"`
	Reading string `json:"reading"`
}

// Token is one piece of a line from the analyzer: 探している is 探し, て, and いる. Surface is exactly as written, so a
// line's surfaces join back into its text. Reading is hiragana, empty for punctuation and Latin text.
//
// The other fields are the IPA dictionary's, which finding words needs: Detail and Detail2 are the part of speech's
// sub-categories (格助詞, 引用), BaseForm the dictionary form, and ConjType and ConjForm how the token conjugates
// (五段・サ行, 連用形). Only Surface, Reading, and POS are stored, so tokens read back from the database don't have them.
type Token struct {
	Surface  string
	Reading  string
	POS      morph.POS
	Detail   string
	Detail2  string
	BaseForm string
	ConjType string
	ConjForm string
}

// Analyzer splits a line into tokens with their readings, parts of speech, and IPA fields.
type Analyzer interface {
	Analyze(ctx context.Context, text string) ([]Token, error)
}

// ErrNoMeanings means the dictionary of word meanings isn't installed. Words are still found, and particles and
// conjugations still explained, without it.
var ErrNoMeanings = errors.New("songs: word meanings are not installed")

// Dictionary looks up word meanings and grammar points. Lookup, Entries, and Tags return ErrNoMeanings when the
// dictionary of meanings isn't installed; the grammar list is always there.
type Dictionary interface {
	// Lookup returns the entries with a spelling that matches form, and reading when it isn't empty, most common
	// first.
	Lookup(ctx context.Context, form, reading string) ([]Entry, error)
	// Entries returns the entries with the given IDs. IDs the dictionary doesn't have are left out.
	Entries(ctx context.Context, ids []int64) ([]Entry, error)
	// Tags describes dictionary codes in words: "Godan verb with 'su' ending" for v5s.
	Tags(ctx context.Context, codes []string) (map[string]string, error)
	// GrammarPoint returns the grammar point with the given ID, and false when there is none.
	GrammarPoint(id string) (GrammarPoint, bool)
	// Attribution is the credit the dictionary's licence requires wherever its meanings are shown.
	Attribution() string
}

// Entry is one dictionary word, with its spellings and its senses in dictionary order.
type Entry struct {
	ID     int64
	Forms  []Form
	Senses []Sense
}

// Form is one spelling of an entry with one of its readings, in hiragana. Common is true when JMdict marks the
// spelling as in common use.
type Form struct {
	Text    string
	Reading string
	Common  bool
}

// Sense is one meaning of an entry. PartsOfSpeech and Misc are JMdict codes, such as v5s and uk.
type Sense struct {
	Index         int
	Glosses       []string
	PartsOfSpeech []string
	Misc          []string
}

// Song is one song with its lines. SourceRef is the lyrics source's own ID for the song, empty for your own lyrics.
// DurationMS is 0 when the length isn't known.
type Song struct {
	ID         int64  `json:"id"`
	Title      string `json:"title"`
	Artist     string `json:"artist"`
	Source     string `json:"source"`
	SourceRef  string `json:"sourceRef"`
	DurationMS int64  `json:"durationMs"`
	Lines      []Line `json:"lines"`
}

// Summary is a song without its lines, for lists.
type Summary struct {
	ID     int64  `json:"id"`
	Title  string `json:"title"`
	Artist string `json:"artist"`
	Source string `json:"source"`
}

// NewSong is a song the user adds with their own lyrics, pasted or typed as text: one sung line per line, and a blank
// line between verses.
type NewSong struct {
	Title  string `json:"title"`
	Artist string `json:"artist"`
	Lyrics string `json:"lyrics"`
}

// Repository stores songs.
type Repository interface {
	AddSong(ctx context.Context, song Song) (Song, error)
	Songs(ctx context.Context) ([]Summary, error)
	SongByID(ctx context.Context, id int64) (Song, error)
	DeleteSong(ctx context.Context, id int64) error
	// SongIDsBySource returns the library's song ID for each of refs that is already saved from source.
	SongIDsBySource(ctx context.Context, source string, refs []string) (map[string]int64, error)
	// AddSavedSong adds a song already saved from source, by someone else, to this library and returns its ID; ok is
	// false when no one has saved it.
	AddSavedSong(ctx context.Context, source, ref string) (id int64, ok bool, err error)
	// SaveAnalysis stores the tokens and words of a saved song's lines that have none yet, and leaves lines that
	// already have them as they are.
	SaveAnalysis(ctx context.Context, songID int64, lines []Line) error
	// SaveMeanings sets the dictionary entry, meaning, and conjugation class of words found without one. Words that
	// gained one meanwhile, and words you corrected, are left as they are.
	SaveMeanings(ctx context.Context, words []WordAnalysis) error
}

// AddSong validates a song the user typed in, splits its lyrics into lines, analyses them, and saves it.
func AddSong(ctx context.Context, repo Repository, analyzer Analyzer, dict Dictionary, in NewSong) (Song, error) {
	title, err := oneLine("title", in.Title, maxTitleLen)
	if err != nil {
		return Song{}, err
	}
	if title == "" {
		return Song{}, fault.Invalid("title", "is required")
	}
	artist, err := oneLine("artist", in.Artist, maxArtistLen)
	if err != nil {
		return Song{}, err
	}
	lines, err := splitLyrics(in.Lyrics)
	if err != nil {
		return Song{}, err
	}
	if err := analyzeLines(ctx, analyzer, dict, lines); err != nil {
		return Song{}, err
	}
	song, err := repo.AddSong(ctx, Song{Title: title, Artist: artist, Source: SourceUser, Lines: lines})
	if err != nil {
		return Song{}, err
	}
	return present(ctx, dict, song)
}

// ListSongs returns every song, newest first.
func ListSongs(ctx context.Context, repo Repository) ([]Summary, error) {
	return repo.Songs(ctx)
}

// GetSong returns one song with its lines, their furigana, and their romaji.
func GetSong(ctx context.Context, repo Repository, analyzer Analyzer, dict Dictionary, id int64) (Song, error) {
	if id <= 0 {
		return Song{}, fault.Invalid("id", "must be positive")
	}
	return loadSong(ctx, repo, analyzer, dict, id)
}

// DeleteSong removes one song and everything that belongs to it.
func DeleteSong(ctx context.Context, repo Repository, id int64) error {
	if id <= 0 {
		return fault.Invalid("id", "must be positive")
	}
	return repo.DeleteSong(ctx, id)
}

func oneLine(field, s string, maxLen int) (string, error) {
	s = strings.TrimSpace(jtext.Normalize(s))
	if strings.ContainsRune(s, '\n') {
		return "", fault.Invalid(field, "must be one line")
	}
	if utf8.RuneCountInString(s) > maxLen {
		return "", tooLong(field, maxLen)
	}
	return s, nil
}

// splitLyrics turns pasted lyrics into lines. Spaces at either end of a line are dropped, and any run of blank lines
// between two sung lines starts a new stanza.
func splitLyrics(lyrics string) ([]Line, error) {
	lyrics = jtext.Normalize(lyrics)
	if utf8.RuneCountInString(lyrics) > maxLyricsLen {
		return nil, tooLong("lyrics", maxLyricsLen)
	}

	var lines []Line
	stanza, gap := 0, false
	for raw := range strings.SplitSeq(lyrics, "\n") {
		text := strings.TrimSpace(raw)
		if text == "" {
			gap = true
			continue
		}
		if utf8.RuneCountInString(text) > maxLineLen {
			return nil, fault.Invalid("lyrics", "has a line over "+strconv.Itoa(maxLineLen)+" characters")
		}
		if gap && len(lines) > 0 {
			stanza++
		}
		gap = false
		lines = append(lines, Line{Position: len(lines), Stanza: stanza, Text: text})
	}
	if len(lines) == 0 {
		return nil, fault.Invalid("lyrics", "is required")
	}
	return lines, nil
}

func tooLong(field string, maxLen int) error {
	return fault.Invalid(field, "must be at most "+strconv.Itoa(maxLen)+" characters")
}
