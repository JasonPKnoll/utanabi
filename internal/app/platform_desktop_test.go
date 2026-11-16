//go:build !server && !ios && !android

package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDesktopDirs(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	env := map[string]string{
		"APPDATA":         filepath.Join("C:", "Roaming"),
		"LOCALAPPDATA":    filepath.Join("C:", "Local"),
		"XDG_CONFIG_HOME": "/xdg/config",
		"XDG_DATA_HOME":   "relative/is/ignored",
	}

	tests := []struct {
		goos     string
		wantData string
		wantLog  string
	}{
		{
			goos:     "darwin",
			wantData: filepath.Join(home, "Library", "Application Support", "Utanabi"),
			wantLog:  filepath.Join(home, "Library", "Logs", "Utanabi"),
		},
		{
			goos:     "windows",
			wantData: filepath.Join("C:", "Roaming", "Utanabi"),
			wantLog:  filepath.Join("C:", "Local", "Utanabi", "logs"),
		},
		{
			goos:     "linux",
			wantData: filepath.Join(home, ".local", "share", "utanabi"),
			wantLog:  filepath.Join(home, ".local", "state", "utanabi", "logs"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			t.Parallel()
			got, err := desktopDirs(tt.goos, func(k string) string { return env[k] })
			if err != nil {
				t.Fatalf("desktopDirs: %v", err)
			}
			if got.Data != tt.wantData {
				t.Errorf("Data = %q, want %q", got.Data, tt.wantData)
			}
			if got.Log != tt.wantLog {
				t.Errorf("Log = %q, want %q", got.Log, tt.wantLog)
			}
		})
	}
}

func TestDesktopDirsLinuxUsesAbsoluteXDG(t *testing.T) {
	t.Parallel()

	got, err := desktopDirs("linux", func(k string) string {
		if k == "XDG_CONFIG_HOME" {
			return "/xdg/config"
		}
		return ""
	})
	if err != nil {
		t.Fatalf("desktopDirs: %v", err)
	}
	if want := filepath.Join("/xdg/config", "utanabi"); got.Config != want {
		t.Errorf("Config = %q, want %q", got.Config, want)
	}
}

func TestDesktopDirsWindowsNeedsEnv(t *testing.T) {
	t.Parallel()

	if _, err := desktopDirs("windows", func(string) string { return "" }); err == nil {
		t.Error("desktopDirs() error = nil, want an error")
	}
}
