package store

import (
	"strings"
	"testing"
	"testing/fstest"
)

func file(sql string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte(sql)}
}

func TestLoadMigrationsSortsByVersion(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"03_third.sql":  file("SELECT 3"),
		"2_second.sql":  file("SELECT 2"),
		"001_first.sql": file("SELECT 1"),
		"README.md":     file("not a migration"),
		"old/004_x.sql": file("SELECT 4"),
	}

	all, err := loadMigrations(fsys)
	if err != nil {
		t.Fatalf("loadMigrations: %v", err)
	}
	if got, want := len(all), 3; got != want {
		t.Fatalf("len = %d, want %d", got, want)
	}
	for i, m := range all {
		if m.version != i+1 {
			t.Errorf("all[%d].version = %d, want %d", i, m.version, i+1)
		}
	}
	if got, want := all[1].name, "2_second.sql"; got != want {
		t.Errorf("all[1].name = %q, want %q", got, want)
	}
	if got, want := all[0].sql, "SELECT 1"; got != want {
		t.Errorf("all[0].sql = %q, want %q", got, want)
	}
}

func TestLoadMigrationsRejectsBadSets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		fsys     fstest.MapFS
		wantFile string
	}{
		{name: "no version", fsys: fstest.MapFS{"init.sql": file("")}, wantFile: "init.sql"},
		{name: "no underscore", fsys: fstest.MapFS{"001.sql": file("")}, wantFile: "001.sql"},
		{name: "version zero", fsys: fstest.MapFS{"000_init.sql": file("")}, wantFile: "000_init.sql"},
		{name: "gap", fsys: fstest.MapFS{"001_a.sql": file(""), "003_c.sql": file("")}, wantFile: "003_c.sql"},
		{
			name:     "repeat",
			fsys:     fstest.MapFS{"001_a.sql": file(""), "002_b.sql": file(""), "002_c.sql": file("")},
			wantFile: "002_c.sql",
		},
		{name: "starts above one", fsys: fstest.MapFS{"002_b.sql": file("")}, wantFile: "002_b.sql"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := loadMigrations(tt.fsys)
			if err == nil {
				t.Fatalf("loadMigrations() = nil, want an error naming %q", tt.wantFile)
			}
			if !strings.Contains(err.Error(), tt.wantFile) {
				t.Errorf("loadMigrations() = %q, want it to name %q", err, tt.wantFile)
			}
		})
	}
}

func TestEmbeddedMigrationsLoad(t *testing.T) {
	t.Parallel()

	all, err := loadMigrations(Migrations())
	if err != nil {
		t.Fatalf("loadMigrations(Migrations()): %v", err)
	}
	if len(all) == 0 {
		t.Errorf("Migrations() has no files, want at least 001_init.sql")
	}
}
