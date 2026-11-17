package config_test

import (
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/config"
)

func platform(t *testing.T, name string, env map[string]string) config.Platform {
	t.Helper()
	dir := t.TempDir()
	return config.Platform{
		Name:   name,
		Dirs:   config.Dirs{Config: dir, Data: dir, Log: dir},
		Getenv: func(k string) string { return env[k] },
	}
}

func writeSettings(t *testing.T, p config.Platform, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(p.Dirs.Config, config.SettingsFile), []byte(body), 0o600); err != nil {
		t.Fatalf("write settings: %v", err)
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Parallel()

	p := platform(t, "desktop", nil)
	cfg, err := config.Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, want := cfg.LogLevel, slog.LevelInfo; got != want {
		t.Errorf("LogLevel = %v, want %v", got, want)
	}
	if got, want := cfg.DBPath, filepath.Join(p.Dirs.Data, "utanabi.db"); got != want {
		t.Errorf("DBPath = %q, want %q", got, want)
	}
	if got, want := cfg.DBBusyTimeout, 5*time.Second; got != want {
		t.Errorf("DBBusyTimeout = %v, want %v", got, want)
	}
	if got, want := cfg.DictionaryPath, filepath.Join(p.Dirs.Data, "dictionary.db"); got != want {
		t.Errorf("DictionaryPath = %q, want %q", got, want)
	}
	if got, want := cfg.Platform, "desktop"; got != want {
		t.Errorf("Platform = %q, want %q", got, want)
	}
}

func TestLoadReadsSettingsFile(t *testing.T) {
	t.Parallel()

	p := platform(t, "desktop", nil)
	writeSettings(t, p, `{"log_level":"debug","db_busy_timeout":"2s"}`)

	cfg, err := config.Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, want := cfg.LogLevel, slog.LevelDebug; got != want {
		t.Errorf("LogLevel = %v, want %v", got, want)
	}
	if got, want := cfg.DBBusyTimeout, 2*time.Second; got != want {
		t.Errorf("DBBusyTimeout = %v, want %v", got, want)
	}
}

func TestLoadServerReadsEnv(t *testing.T) {
	t.Parallel()

	p := platform(t, "server", map[string]string{
		"UTANABI_LOG_LEVEL":       "warn",
		"UTANABI_ALLOWED_ORIGINS": "https://utanabi.example.com, https://*.example.com",
		"WAILS_SERVER_PORT":       "9000",
	})
	cfg, err := config.Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, want := cfg.LogLevel, slog.LevelWarn; got != want {
		t.Errorf("LogLevel = %v, want %v", got, want)
	}
	want := []string{"https://utanabi.example.com", "https://*.example.com"}
	if got := cfg.Server.AllowedOrigins; !slices.Equal(got, want) {
		t.Errorf("AllowedOrigins = %q, want %q", got, want)
	}
}

func TestLoadDesktopIgnoresEnv(t *testing.T) {
	t.Parallel()

	p := platform(t, "desktop", nil)
	p.Getenv = nil

	cfg, err := config.Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, want := cfg.LogLevel, slog.LevelInfo; got != want {
		t.Errorf("LogLevel = %v, want %v", got, want)
	}
}

func TestLoadRejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		platform string
		settings string
		env      map[string]string
		wantName string
	}{
		{name: "unknown log level", platform: "desktop", settings: `{"log_level":"loud"}`, wantName: "log_level"},
		{name: "bad busy timeout", platform: "desktop", settings: `{"db_busy_timeout":"soon"}`, wantName: "db_busy_timeout"},
		{name: "negative busy timeout", platform: "desktop", settings: `{"db_busy_timeout":"-1s"}`, wantName: "db_busy_timeout"},
		{name: "unknown setting", platform: "desktop", settings: `{"colour":"blue"}`, wantName: "colour"},
		{
			name: "dictionary path names a folder", platform: "desktop",
			settings: `{"dictionary_path":"reference/"}`, wantName: "dictionary_path",
		},
		{
			name: "dictionary path with spaces around it", platform: "desktop",
			settings: `{"dictionary_path":" dictionary.db"}`, wantName: "dictionary_path",
		},
		{
			name: "bad env dictionary path", platform: "server",
			env: map[string]string{"UTANABI_DICTIONARY_PATH": "dict\x00.db"}, wantName: "dictionary_path",
		},
		{name: "bad env log level", platform: "server", env: map[string]string{"UTANABI_LOG_LEVEL": "loud"}, wantName: "log_level"},
		{name: "bad port", platform: "server", env: map[string]string{"WAILS_SERVER_PORT": "http"}, wantName: "WAILS_SERVER_PORT"},
		{name: "port out of range", platform: "server", env: map[string]string{"WAILS_SERVER_PORT": "70000"}, wantName: "WAILS_SERVER_PORT"},
		{
			name: "wildcard origin", platform: "server",
			env:      map[string]string{"UTANABI_ALLOWED_ORIGINS": "*"},
			wantName: "UTANABI_ALLOWED_ORIGINS",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := platform(t, tt.platform, tt.env)
			if tt.settings != "" {
				writeSettings(t, p, tt.settings)
			}
			_, err := config.Load(p)
			if err == nil {
				t.Fatal("Load() error = nil, want an error")
			}
			if !strings.Contains(err.Error(), tt.wantName) {
				t.Errorf("Load() error = %q, want it to name %q", err, tt.wantName)
			}
		})
	}
}

func TestLoadDictionaryPath(t *testing.T) {
	t.Parallel()

	abs := filepath.Join(t.TempDir(), "reference", "dictionary.db")
	tests := []struct {
		name     string
		platform string
		settings string
		env      map[string]string
		want     func(data string) string
	}{
		{
			name: "absolute path from the settings file", platform: "desktop",
			settings: `{"dictionary_path":` + strconv.Quote(abs) + `}`,
			want:     func(string) string { return abs },
		},
		{
			name: "relative path is taken from the data folder", platform: "desktop",
			settings: `{"dictionary_path":"reference/dictionary.db"}`,
			want:     func(data string) string { return filepath.Join(data, "reference", "dictionary.db") },
		},
		{
			name: "env var overrides the settings file in server mode", platform: "server",
			settings: `{"dictionary_path":"ignored.db"}`,
			env:      map[string]string{"UTANABI_DICTIONARY_PATH": "server.db"},
			want:     func(data string) string { return filepath.Join(data, "server.db") },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := platform(t, tt.platform, tt.env)
			writeSettings(t, p, tt.settings)
			cfg, err := config.Load(p)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got, want := cfg.DictionaryPath, tt.want(p.Dirs.Data); got != want {
				t.Errorf("DictionaryPath = %q, want %q", got, want)
			}
		})
	}
}

func TestLoadRequiresDataFolder(t *testing.T) {
	t.Parallel()

	_, err := config.Load(config.Platform{Name: "desktop"})
	if err == nil {
		t.Fatal("Load() error = nil, want an error")
	}
}

func TestLoadLyricsSources(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		platform string
		settings string
		env      map[string]string
		want     []string
	}{
		{name: "on by default", platform: "desktop", want: []string{"lrclib"}},
		{name: "an empty list turns them off", platform: "desktop", settings: `{"lyrics_sources":[]}`, want: []string{}},
		{name: "none turns them off", platform: "desktop", settings: `{"lyrics_sources":["none"]}`, want: []string{}},
		{name: "named sources are on", platform: "desktop", settings: `{"lyrics_sources":["lrclib"]}`,
			want: []string{"lrclib"}},
		{name: "none written twice still turns them off", platform: "desktop",
			settings: `{"lyrics_sources":["none","none"]}`, want: []string{}},
		{name: "none with a blank entry still turns them off", platform: "desktop",
			settings: `{"lyrics_sources":["none",""]}`, want: []string{}},
		{name: "server env none, padded, turns them off", platform: "server",
			env: map[string]string{"UTANABI_LYRICS_SOURCES": " none , none "}, want: []string{}},
		{name: "names are trimmed, lowercased, and deduplicated", platform: "desktop",
			settings: `{"lyrics_sources":[" LRCLIB ","lrclib"]}`, want: []string{"lrclib"}},
		{name: "server env none turns them off", platform: "server",
			env: map[string]string{"UTANABI_LYRICS_SOURCES": "none"}, want: []string{}},
		{name: "server env overrides the settings file", platform: "server", settings: `{"lyrics_sources":[]}`,
			env: map[string]string{"UTANABI_LYRICS_SOURCES": "lrclib"}, want: []string{"lrclib"}},
		{name: "an empty server env var leaves the settings file in charge", platform: "server",
			settings: `{"lyrics_sources":[]}`, env: map[string]string{"UTANABI_LYRICS_SOURCES": ""}, want: []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := platform(t, tt.platform, tt.env)
			if tt.settings != "" {
				writeSettings(t, p, tt.settings)
			}
			cfg, err := config.Load(p)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if !slices.Equal(cfg.LyricsSources, tt.want) || cfg.LyricsSources == nil {
				t.Errorf("LyricsSources = %#v, want %#v", cfg.LyricsSources, tt.want)
			}
		})
	}
}

func TestLoadRejectsBadLyricsSources(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		platform string
		settings string
		env      map[string]string
	}{
		{name: "a source this build doesn't have", platform: "desktop", settings: `{"lyrics_sources":["spotify"]}`},
		{name: "none with a source", platform: "desktop", settings: `{"lyrics_sources":["none","lrclib"]}`},
		{name: "not a list", platform: "desktop", settings: `{"lyrics_sources":"lrclib"}`},
		{name: "bad server env", platform: "server", env: map[string]string{"UTANABI_LYRICS_SOURCES": "lrclib,genius"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := platform(t, tt.platform, tt.env)
			if tt.settings != "" {
				writeSettings(t, p, tt.settings)
			}
			_, err := config.Load(p)
			if err == nil || !strings.Contains(err.Error(), "lyrics_sources") {
				t.Errorf("err = %v, want an error naming lyrics_sources", err)
			}
		})
	}
}
