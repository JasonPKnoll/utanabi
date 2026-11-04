package config_test

import (
	"log/slog"
	"os"
	"path/filepath"
	"slices"
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

func TestLoadRequiresDataFolder(t *testing.T) {
	t.Parallel()

	_, err := config.Load(config.Platform{Name: "desktop"})
	if err == nil {
		t.Fatal("Load() error = nil, want an error")
	}
}
