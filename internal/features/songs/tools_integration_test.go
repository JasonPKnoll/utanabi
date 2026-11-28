//go:build integration

package songs

import (
	"errors"
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
)

func TestSQLiteReplacesASongsLinesAndKeepsTheSong(t *testing.T) {
	t.Parallel()

	repo, hana, kaze := twoUsers(t)
	song, err := repo.AddSong(hana, Song{Title: "歌", Source: "lrclib", SourceRef: "7", Lines: []Line{searchingLine()}})
	if err != nil {
		t.Fatalf("AddSong: %v", err)
	}
	if err := repo.SetWordTimes(hana, song.ID, TimesByUser, []LineTimes{{Line: 0, Times: []WordTime{{0, 1000}}}}); err != nil {
		t.Fatalf("SetWordTimes: %v", err)
	}
	// Something of another slice's that belongs to the song, which a refresh must keep.
	if _, err := repo.db.Querier().ExecContext(t.Context(),
		`INSERT INTO lyric_offsets (owner, song_id, offset_ms) VALUES (1, ?, -1000)`, song.ID); err != nil {
		t.Fatalf("insert offset: %v", err)
	}
	if err := repo.ReplaceLines(kaze, song.ID, 1000, sampleLines()); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("kaze replacing a song not in their library: %v, want not found", err)
	}

	start := int64(5000)
	fresh := sampleLines()
	fresh[0].StartMS = &start
	if err := repo.ReplaceLines(hana, song.ID, 215000, fresh); err != nil {
		t.Fatalf("ReplaceLines: %v", err)
	}
	got, err := repo.SongByID(hana, song.ID)
	if err != nil {
		t.Fatalf("SongByID: %v", err)
	}
	if len(got.Lines) != len(fresh) || got.Lines[0].Text != fresh[0].Text || *got.Lines[0].StartMS != 5000 {
		t.Errorf("lines = %+v, want the new ones", got.Lines)
	}
	if got.DurationMS != 215000 {
		t.Errorf("duration = %d, want 215000", got.DurationMS)
	}
	if len(got.Lines[0].Times) != 0 {
		t.Errorf("word times = %+v, want them gone with the old lines", got.Lines[0].Times)
	}
	var offset int64
	if err := repo.db.Querier().QueryRowContext(t.Context(),
		`SELECT offset_ms FROM lyric_offsets WHERE owner = 1 AND song_id = ?`, song.ID).Scan(&offset); err != nil || offset != -1000 {
		t.Errorf("offset after replacing = %d, %v; want it kept", offset, err)
	}
}
