package songs

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
)

type fakeRepo struct {
	added   []Song
	songs   map[int64]Song
	deleted []int64
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{songs: map[int64]Song{}}
}

func (f *fakeRepo) AddSong(_ context.Context, song Song) (Song, error) {
	song.ID = int64(len(f.added) + 1)
	f.added = append(f.added, song)
	f.songs[song.ID] = song
	return song, nil
}

func (f *fakeRepo) Songs(context.Context) ([]Summary, error) {
	list := []Summary{}
	for _, s := range f.songs {
		list = append(list, Summary{ID: s.ID, Title: s.Title, Artist: s.Artist, Source: s.Source})
	}
	return list, nil
}

func (f *fakeRepo) SongByID(_ context.Context, id int64) (Song, error) {
	song, ok := f.songs[id]
	if !ok {
		return Song{}, fault.ErrNotFound
	}
	return song, nil
}

func (f *fakeRepo) DeleteSong(_ context.Context, id int64) error {
	f.deleted = append(f.deleted, id)
	return nil
}

func (f *fakeRepo) SongIDsBySource(_ context.Context, source string, refs []string) (map[string]int64, error) {
	found := map[string]int64{}
	for _, ref := range refs {
		for id, s := range f.songs {
			if s.Source == source && s.SourceRef == ref {
				found[ref] = id
			}
		}
	}
	return found, nil
}

func TestAddSongRejectsBadInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		in        NewSong
		wantField string
	}{
		{name: "missing title", in: NewSong{Lyrics: "星が光る"}, wantField: "title"},
		{name: "title of only spaces", in: NewSong{Title: " 　 ", Lyrics: "星が光る"}, wantField: "title"},
		{name: "title on two lines", in: NewSong{Title: "星の\n歌", Lyrics: "星が光る"}, wantField: "title"},
		{name: "title too long", in: NewSong{Title: strings.Repeat("歌", 201), Lyrics: "星"}, wantField: "title"},
		{name: "artist too long", in: NewSong{Title: "星の歌", Artist: strings.Repeat("a", 201), Lyrics: "星"},
			wantField: "artist"},
		{name: "missing lyrics", in: NewSong{Title: "星の歌"}, wantField: "lyrics"},
		{name: "lyrics of only blank lines", in: NewSong{Title: "星の歌", Lyrics: "\n  \r\n　\n"}, wantField: "lyrics"},
		{name: "lyrics too long", in: NewSong{Title: "星の歌", Lyrics: strings.Repeat("あ\n", 10001)}, wantField: "lyrics"},
		{name: "one line too long", in: NewSong{Title: "星の歌", Lyrics: "星\n" + strings.Repeat("あ", 501)},
			wantField: "lyrics"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := newFakeRepo()
			_, err := AddSong(t.Context(), repo, tt.in)

			var invalid *fault.InvalidError
			if !errors.As(err, &invalid) {
				t.Fatalf("err = %v, want *fault.InvalidError", err)
			}
			if got, want := invalid.Field, tt.wantField; got != want {
				t.Errorf("field = %q, want %q", got, want)
			}
			if len(repo.added) != 0 {
				t.Errorf("saved %d songs, want none", len(repo.added))
			}
		})
	}
}

func TestAddSongSplitsLyricsIntoLinesAndStanzas(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		lyrics string
		want   []Line
	}{
		{name: "one verse", lyrics: "夜の空に\n星が光る", want: []Line{
			{Position: 0, Stanza: 0, Text: "夜の空に"},
			{Position: 1, Stanza: 0, Text: "星が光る"},
		}},
		{name: "a blank line starts a new stanza", lyrics: "夜の空に\n\n星が光る", want: []Line{
			{Position: 0, Stanza: 0, Text: "夜の空に"},
			{Position: 1, Stanza: 1, Text: "星が光る"},
		}},
		{name: "several blank lines start only one new stanza", lyrics: "一\n\n\n\n二\n三\n\n四", want: []Line{
			{Position: 0, Stanza: 0, Text: "一"},
			{Position: 1, Stanza: 1, Text: "二"},
			{Position: 2, Stanza: 1, Text: "三"},
			{Position: 3, Stanza: 2, Text: "四"},
		}},
		{name: "blank lines at the start and end are dropped", lyrics: "\n\r\n星\n\n", want: []Line{
			{Position: 0, Stanza: 0, Text: "星"},
		}},
		{name: "spaces at either end of a line are dropped", lyrics: "  夜の 空に　\t\r\n", want: []Line{
			{Position: 0, Stanza: 0, Text: "夜の 空に"},
		}},
		{name: "half-width katakana is made full-width", lyrics: "ﾗｲﾌﾞ", want: []Line{
			{Position: 0, Stanza: 0, Text: "ライブ"},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := AddSong(t.Context(), newFakeRepo(), NewSong{Title: "星の歌", Lyrics: tt.lyrics})
			if err != nil {
				t.Fatalf("AddSong: %v", err)
			}
			if !reflect.DeepEqual(got.Lines, tt.want) {
				t.Errorf("lines = %+v, want %+v", got.Lines, tt.want)
			}
		})
	}
}

func TestAddSongCleansTitleAndMarksUserSource(t *testing.T) {
	t.Parallel()

	got, err := AddSong(t.Context(), newFakeRepo(), NewSong{Title: "  ﾗｲﾌﾞの歌 ", Artist: " Demo Artist ", Lyrics: "星"})
	if err != nil {
		t.Fatalf("AddSong: %v", err)
	}
	if got.Title != "ライブの歌" || got.Artist != "Demo Artist" || got.Source != SourceUser || got.ID != 1 {
		t.Errorf("song = %+v, want title ライブの歌, artist Demo Artist, source user, ID 1", got)
	}
}

func TestAddSongAllowsNoArtist(t *testing.T) {
	t.Parallel()

	got, err := AddSong(t.Context(), newFakeRepo(), NewSong{Title: "春の手紙", Lyrics: "春が来た"})
	if err != nil {
		t.Fatalf("AddSong: %v", err)
	}
	if got.Artist != "" {
		t.Errorf("artist = %q, want empty", got.Artist)
	}
}

func TestGetAndDeleteRejectBadID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		call func(repo *fakeRepo) error
	}{
		{name: "get zero", call: func(repo *fakeRepo) error {
			_, err := GetSong(t.Context(), repo, 0)
			return err
		}},
		{name: "get negative", call: func(repo *fakeRepo) error {
			_, err := GetSong(t.Context(), repo, -3)
			return err
		}},
		{name: "delete zero", call: func(repo *fakeRepo) error { return DeleteSong(t.Context(), repo, 0) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := newFakeRepo()
			err := tt.call(repo)

			var invalid *fault.InvalidError
			if !errors.As(err, &invalid) || invalid.Field != "id" {
				t.Fatalf("err = %v, want invalid id", err)
			}
			if len(repo.deleted) != 0 {
				t.Errorf("deleted %v, want nothing", repo.deleted)
			}
		})
	}
}

func TestGetSongPassesNotFoundThrough(t *testing.T) {
	t.Parallel()

	_, err := GetSong(t.Context(), newFakeRepo(), 42)
	if !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("err = %v, want fault.ErrNotFound", err)
	}
}
