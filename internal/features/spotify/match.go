package spotify

import (
	"context"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// sameLengthMS is how close in length a track with the song's title must be to be the song when its artist's name
// doesn't match: Spotify can list an artist by a translated name (東京事変 as Tokyo Incidents), which no romaji
// reaches, while a cover is rarely within a second or two of the song.
const sameLengthMS = 2000

// name is a title or an artist's name in the two forms names are compared in: as written, and in romaji.
type name struct{ written, latin string }

// wanted is the song a track must be.
type wanted struct {
	title name
	// who is the artist and the title together, since a title from a lyrics source often has the artist in it
	// ("Fujii Kaze - Shinunoga E-Wa").
	who        name
	durationMS int64
}

func (p *Player) nameOf(ctx context.Context, s string) (name, error) {
	r, err := p.romaji.Romaji(ctx, s)
	if err != nil {
		return name{}, err
	}
	return name{written: plain(s), latin: folded(r)}, nil
}

func (p *Player) wantedFor(ctx context.Context, song Song) (wanted, error) {
	title, err := p.nameOf(ctx, song.Title)
	if err != nil {
		return wanted{}, err
	}
	who, err := p.nameOf(ctx, song.Artist+" "+song.Title)
	return wanted{title: title, who: who, durationMS: song.DurationMS}, err
}

// verdict is how well a track fits the song: whether it's the song at all, and whether its artist matched too.
type verdict struct{ same, byArtist bool }

func (p *Player) judge(ctx context.Context, t Track, w wanted) (verdict, error) {
	title, err := p.nameOf(ctx, t.Name)
	if err != nil || !titled(title, w.title) {
		return verdict{}, err
	}
	for _, a := range t.Artists {
		artist, err := p.nameOf(ctx, a)
		if err != nil {
			return verdict{}, err
		}
		if contains(w.who, artist) {
			return verdict{same: true, byArtist: true}, nil
		}
	}
	sameLength := w.durationMS > 0 && t.DurationMS > 0 && abs(t.DurationMS-w.durationMS) <= sameLengthMS
	return verdict{same: sameLength}, nil
}

// bestMatch picks the search result that is the song: one with its title, by its artist, closest in length. A track
// whose artist doesn't match is the song only when it's as long as the song, and loses to one whose artist does.
// When none fits, nothing is picked, rather than playing the wrong song: another song by the artist can be as long
// as the lyrics are (Hana for Matsuri's video). Length only chooses among versions, since lyrics timed to a video can
// be longer than the track.
func (p *Player) bestMatch(ctx context.Context, tracks []Track, song Song) (Track, bool, error) {
	w, err := p.wantedFor(ctx, song)
	if err != nil {
		return Track{}, false, err
	}
	var best Track
	var bestVerdict verdict
	for _, t := range tracks {
		v, err := p.judge(ctx, t, w)
		if err != nil {
			return Track{}, false, err
		}
		if !v.same {
			continue
		}
		better := !bestVerdict.same || (v.byArtist && !bestVerdict.byArtist) ||
			(v.byArtist == bestVerdict.byArtist && closer(t, best, song.DurationMS))
		if better {
			best, bestVerdict = t, v
		}
	}
	return best, bestVerdict.same, nil
}

// sameSong reports whether a track played from Spotify is the song.
func (p *Player) sameSong(ctx context.Context, t Track, song Song) (bool, error) {
	w, err := p.wantedFor(ctx, song)
	if err != nil {
		return false, err
	}
	v, err := p.judge(ctx, t, w)
	return v.same, err
}

// closer reports whether a is closer to the song's length than b. Without a length to go by, neither is.
func closer(a, b Track, songMS int64) bool {
	if songMS <= 0 || a.DurationMS <= 0 {
		return false
	}
	return b.DurationMS <= 0 || abs(a.DurationMS-songMS) < abs(b.DurationMS-songMS)
}

// otherLength is how long the song's lyrics are timed for, when that is more than a few seconds off the track
// playing, and 0 otherwise: lyrics from a video or a live recording don't line up with the studio track.
func otherLength(song Song, track Track) int64 {
	if song.DurationMS <= 0 || track.DurationMS <= 0 || abs(song.DurationMS-track.DurationMS) <= 5000 {
		return 0
	}
	return song.DurationMS
}

// titled reports whether a track's name is the song's title, or the title has it, in either form.
func titled(track, title name) bool {
	return contains(title, track) || contains(track, title)
}

// contains reports whether outer has inner in it, as written or in romaji. A short romaji name must match whole:
// 愛 is ai, which too many titles have in them.
func contains(outer, inner name) bool {
	if inner.written != "" && strings.Contains(outer.written, inner.written) {
		return true
	}
	if utf8.RuneCountInString(inner.latin) < 4 {
		return inner.latin != "" && outer.latin == inner.latin
	}
	return strings.Contains(outer.latin, inner.latin)
}

// plain is text for comparing: NFKC, lower case, letters and digits only, so "Shinunoga E-Wa" is "shinunogaewa".
func plain(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, norm.NFKC.String(s))
}

// folded is romaji for comparing: plain, with no accents, and long vowels written one way, since romaji spells them
// many ways: Tōkyō, Toukyou, and Tokyo are all tokyo.
func folded(romaji string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(romaji) {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(r)
		}
	}
	s := plain(b.String())
	var out []rune
	for _, r := range s {
		if n := len(out); n > 0 && isVowel(r) && (out[n-1] == r || (r == 'u' && out[n-1] == 'o')) {
			continue
		}
		out = append(out, r)
	}
	return string(out)
}

func isVowel(r rune) bool {
	return slices.Contains([]rune("aeiou"), r)
}
