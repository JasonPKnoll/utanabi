//go:build integration

package dictbuild_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/dictionary/dictbuild"
	_ "github.com/JasonPKnoll/utanabi/internal/store" // registers the sqlite driver
)

func openFixture(t *testing.T) *os.File {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "testdata", "jmdict-eng-common-fixture.json"))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	t.Cleanup(func() {
		if err := f.Close(); err != nil {
			t.Errorf("close fixture: %v", err)
		}
	})
	return f
}

func TestBuildWritesEveryTable(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "out", "dictionary.db")
	grammar := readGrammarList(t)
	builtAt := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)

	stats, err := dictbuild.Build(t.Context(), path, openFixture(t), grammar, builtAt)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if stats.Entries != 15 || stats.GrammarPoints != len(grammar) || stats.JMdictDate != "2026-11-16" {
		t.Errorf("stats = %+v, want 15 entries, %d grammar points, and JMdict 2026-11-16", stats, len(grammar))
	}

	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		t.Fatalf("open built file: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close built file: %v", err)
		}
	})
	counts := map[string]int{
		"entries":        stats.Entries,
		"entry_forms":    stats.Forms,
		"senses":         stats.Senses,
		"tags":           stats.Tags,
		"grammar_points": stats.GrammarPoints,
		"meta":           4,
	}
	for table, want := range counts {
		var got int
		if err := db.QueryRowContext(t.Context(), "SELECT count(*) FROM "+table).Scan(&got); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if got != want || got == 0 {
			t.Errorf("%s has %d rows, want %d", table, got, want)
		}
	}

	var builtAtValue, journal string
	err = db.QueryRowContext(t.Context(), `SELECT value FROM meta WHERE key = 'built_at'`).Scan(&builtAtValue)
	if err != nil {
		t.Fatalf("read built_at: %v", err)
	}
	if want := "2026-11-17T12:00:00Z"; builtAtValue != want {
		t.Errorf("built_at = %q, want %q", builtAtValue, want)
	}
	if err := db.QueryRowContext(t.Context(), `PRAGMA journal_mode`).Scan(&journal); err != nil {
		t.Fatalf("read journal mode: %v", err)
	}
	if journal != "delete" {
		t.Errorf("journal_mode = %q, want delete, so the file opens read-only with no -wal or -shm file", journal)
	}

	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatalf("read output folder: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("output folder holds %d files, want only dictionary.db", len(entries))
	}
}

func TestBuildFailureKeepsTheOldDictionary(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "dictionary.db")
	if err := os.WriteFile(path, []byte("the old dictionary"), 0o600); err != nil {
		t.Fatalf("write old file: %v", err)
	}

	const star = `{"id":"1","kanji":[],"kana":[{"text":"ほし","appliesToKanji":["*"]}],` +
		`"sense":[{"gloss":[{"lang":"eng","text":"star"}]}]}`
	broken := strings.NewReader(`{"version":"3.6.2","dictDate":"2026-11-16","languages":["eng"],"words":[` +
		star + `,` + star + `]}`)
	if _, err := dictbuild.Build(t.Context(), path, broken, readGrammarList(t), time.Now()); err == nil {
		t.Fatal("Build with a repeated entry ID returned no error")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read old file: %v", err)
	}
	if string(got) != "the old dictionary" {
		t.Errorf("a failed build changed dictionary.db to %q", got)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read folder: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("a failed build left %d files, want only the old dictionary.db", len(entries))
	}
}

func TestBuildRefusesABadGrammarList(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "dictionary.db")
	if _, err := dictbuild.Build(t.Context(), path, openFixture(t), nil, time.Now()); err == nil {
		t.Fatal("Build with an empty grammar list returned no error")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("Stat after a refused build = %v, want no file", err)
	}
}
